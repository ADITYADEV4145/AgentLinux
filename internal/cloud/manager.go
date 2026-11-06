// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

type frameRef struct {
	generation string
	captured   time.Time
}
type Manager struct {
	mu        sync.Mutex
	root      string
	db        Database
	provider  Provider
	now       func() time.Time
	busy      map[string]bool
	frames    map[string]map[string]frameRef
	poisoned  bool
	lastAudit map[string]string
	sequence  map[string]uint64
	leases    map[string]controlLease
}

func HashToken(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func New(root string, p Provider) (*Manager, error) {
	var db Database
	if e := storage.Read(root, "cloud.json", &db); e != nil {
		return nil, e
	}
	if db.Version != Version || !protocol.ValidID(db.Deployment) || !db.Limits.valid() || db.Provider != p.Name() || db.Identities == nil || db.Sessions == nil {
		return nil, errors.New("invalid cloud state/version/provider; do not replace state or change provider in place")
	}
	if len(db.Sessions) > 1000 || len(db.Identities) > 32 {
		return nil, errors.New("cloud state exceeds prototype bounds")
	}
	for id, i := range db.Identities {
		digest, err := hex.DecodeString(i.TokenHash)
		if i.ID != id || id == "" || !protocol.ValidID(i.Workspace) || err != nil || len(digest) != 32 || i.Expires.IsZero() {
			return nil, errors.New("invalid cloud identity state")
		}
	}
	for id, s := range db.Sessions {
		owner, exists := db.Identities[s.Owner]
		validState := slices.Contains([]string{"creating", "starting", "ready", "stopping", "stopped"}, s.State)
		if !exists || owner.Workspace != s.Workspace || !validState || !protocol.ValidID(s.RequestKey) || s.Created.IsZero() || s.LastAction.Before(s.Created) || s.ObservedSeconds < 0 || !protocol.ValidID(id) || s.ID != id || s.Provider != p.Name() || s.Expires.Sub(s.Created) < 30*time.Second || s.Expires.Sub(s.Created) > time.Duration(db.Limits.MaxSeconds)*time.Second {
			return nil, errors.New("invalid cloud session state")
		}
	}
	return &Manager{root: root, db: db, provider: p, now: time.Now, busy: map[string]bool{}, frames: map[string]map[string]frameRef{}, leases: map[string]controlLease{}, lastAudit: map[string]string{}, sequence: map[string]uint64{}}, nil
}
func (m *Manager) save() error {
	if m.poisoned {
		return errors.New("cloud durable state unavailable; restart requires operator inspection")
	}
	if e := storage.JSON(m.root, "cloud.json", m.db); e != nil {
		m.poisoned = true
		return e
	}
	for id, s := range m.db.Sessions {
		signature := s.State + "/" + s.ProviderID + "/" + s.Error
		if m.lastAudit[id] != signature {
			if e := m.audit("session_state", s); e != nil {
				m.poisoned = true
				return e
			}
			m.lastAudit[id] = signature
		}
	}
	return nil
}
func (m *Manager) Authenticate(token string) (Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.poisoned {
		return Identity{}, protocol.Fail("SERVICE_UNAVAILABLE", "durable state unavailable; no new control actions")
	}
	digest := HashToken(token)
	for _, i := range m.db.Identities {
		if subtle.ConstantTimeCompare([]byte(i.TokenHash), []byte(digest)) == 1 && !i.Revoked && m.now().Before(i.Expires) {
			return i, nil
		}
	}
	return Identity{}, protocol.Fail("UNAUTHORIZED", "missing, expired or revoked development credential")
}
func (m *Manager) owner(i Identity, id string) (Session, error) {
	current, ok := m.db.Identities[i.ID]
	if !ok || current.Revoked || !m.now().Before(current.Expires) || current.TokenHash != i.TokenHash {
		return Session{}, protocol.Fail("UNAUTHORIZED", "credential expired or revoked")
	}
	s, ok := m.db.Sessions[id]
	if !ok || s.Owner != i.ID || s.Workspace != current.Workspace {
		return Session{}, protocol.Fail("NOT_FOUND", "session not found")
	}
	return s, nil
}
func active(s Session) bool { return s.State != "stopped" }
func (m *Manager) Create(i Identity, key string, q Create) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.db.Identities[i.ID]
	if !ok || cur.TokenHash != i.TokenHash || cur.Revoked || !m.now().Before(cur.Expires) {
		return Session{}, protocol.Fail("UNAUTHORIZED", "credential expired or revoked")
	}
	if q.Workspace != cur.Workspace {
		return Session{}, protocol.Fail("FORBIDDEN", "workspace is not assigned to this identity")
	}
	if !protocol.ValidID(key) || q.Profile != "desktop" || q.Seconds < 30 || q.Seconds > m.db.Limits.MaxSeconds {
		return Session{}, protocol.Fail("INVALID_ARGUMENT", "use a 32-hex idempotency key, desktop profile and bounded duration")
	}
	count, own := 0, 0
	var budget int64
	for _, s := range m.db.Sessions {
		if s.Owner == i.ID && s.RequestKey == key {
			if int(s.Expires.Sub(s.Created).Seconds()) != q.Seconds || s.Workspace != q.Workspace {
				return Session{}, protocol.Fail("IDEMPOTENCY_CONFLICT", "key already used with different parameters")
			}
			return s, nil
		}
		if active(s) {
			count++
			if s.Owner == i.ID {
				own++
			}
			budget += int64(s.Expires.Sub(s.Created).Seconds())
		} else {
			budget += s.ObservedSeconds
		}
	}
	if count >= m.db.Limits.MaxSessions || own >= m.db.Limits.MaxPerOwner || budget+int64(q.Seconds) > m.db.Limits.BudgetSeconds || len(m.db.Sessions) >= 1000 {
		return Session{}, protocol.Fail("LIMIT_EXCEEDED", "concurrency, retained metadata or cumulative reserved-time budget exhausted")
	}
	now := m.now().UTC()
	s := Session{ID: protocol.ID(), Owner: i.ID, Workspace: q.Workspace, RequestKey: key, State: "creating", Provider: m.provider.Name(), Simulated: m.provider.Simulated(), Created: now, Expires: now.Add(time.Duration(q.Seconds) * time.Second), LastAction: now}
	m.db.Sessions[s.ID] = s
	if e := m.save(); e != nil {
		delete(m.db.Sessions, s.ID)
		return Session{}, e
	}
	return s, nil
}
func (m *Manager) Get(i Identity, id string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, e := m.owner(i, id)
	limits := m.db.Limits
	s.Policy = &limits
	if active(s) {
		s.EstimatedSeconds = max(0, int64(m.now().Sub(s.Created).Seconds()))
	} else {
		s.EstimatedSeconds = s.ObservedSeconds
	}
	s.IdleStopAt = s.LastAction.Add(time.Duration(limits.IdleSeconds) * time.Second)
	if s.IdleStopAt.After(s.Expires) {
		s.IdleStopAt = s.Expires
	}
	return s, e
}
func (m *Manager) Stop(i Identity, id string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, e := m.owner(i, id)
	if e != nil {
		return s, e
	}
	if s.State == "stopped" {
		return s, nil
	}
	s.State = "stopping"
	m.db.Sessions[id] = s
	delete(m.frames, id)
	delete(m.leases, id)
	return s, m.save()
}
func (m *Manager) Revoke(i Identity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.db.Identities[i.ID]
	if !ok || cur.TokenHash != i.TokenHash {
		return protocol.Fail("UNAUTHORIZED", "invalid identity")
	}
	cur.Revoked = true
	m.db.Identities[i.ID] = cur
	for id, s := range m.db.Sessions {
		if s.Owner == i.ID && active(s) {
			s.State = "stopping"
			m.db.Sessions[id] = s
			delete(m.frames, id)
			delete(m.leases, id)
		}
	}
	return m.save()
}
func (m *Manager) acquire(id string) error {
	if m.busy[id] {
		return protocol.Fail("MACHINE_BUSY", "session operation in progress; observe before retrying mutations")
	}
	m.busy[id] = true
	return nil
}
func (m *Manager) release(id string) { m.mu.Lock(); delete(m.busy, id); m.mu.Unlock() }
func (m *Manager) Call(ctx context.Context, i Identity, id string, q Operation) (Output, error) {
	m.mu.Lock()
	s, e := m.owner(i, id)
	if e == nil && (s.State != "ready" || !m.now().Before(s.Expires) || m.now().Sub(s.LastAction) >= time.Duration(m.db.Limits.IdleSeconds)*time.Second) {
		e = protocol.Fail("NOT_READY", "session unavailable, expired or idle; stop discards ephemeral data")
	}
	if e == nil {
		e = m.acquire(id)
	}
	if e != nil {
		m.mu.Unlock()
		return Output{}, e
	}
	defer m.release(id)
	fail := func(e error) (Output, error) { m.mu.Unlock(); return Output{}, e }
	if q.Method != "screenshot" {
		l, held := m.leases[id]
		if held && m.now().Before(l.Expires) && l.Owner != q.ControlNonce {
			return fail(protocol.Fail("INPUT_OWNED", "human viewer owns input; wait for handoff"))
		}
		if q.ControlNonce != "" && (!held || !m.now().Before(l.Expires) || l.Owner != q.ControlNonce || l.Generation != s.Generation) {
			return fail(protocol.Fail("INPUT_NOT_OWNED", "lease lost; observe and retake control"))
		}
	}
	caps := s.Capabilities
	if caps == nil {
		return fail(protocol.Fail("NOT_READY", "capabilities not established"))
	}
	switch q.Method {
	case "screenshot":
		if caps.Displays != 1 || caps.Format != "png" {
			return fail(protocol.Fail("UNSUPPORTED_OPERATION", "screenshots unavailable"))
		}
	case "input":
		if q.Input == nil {
			return fail(protocol.Fail("INVALID_ARGUMENT", "input required"))
		}
		if e = q.Input.Validate(); e != nil {
			return fail(e)
		}
		if q.Input.Kind == "text" && q.Input.PasteMode != "" && !slices.Contains(caps.PasteModes, q.Input.PasteMode) {
			return fail(protocol.Fail("UNSUPPORTED_OPERATION", "paste mode is not supported by this provider"))
		}
		if !slices.Contains(caps.Input, q.Input.Kind) {
			return fail(protocol.Fail("UNSUPPORTED_OPERATION", "input kind not supported"))
		}
		f, ok := m.frames[id][q.Input.FrameID]
		if !ok || f.generation != s.Generation || m.now().Sub(f.captured) > 10*time.Second {
			return fail(protocol.Fail("STALE_FRAME", "capture a fresh screenshot before acting"))
		}
	case "exec":
		if !caps.GuestExec {
			return fail(protocol.Fail("UNSUPPORTED_OPERATION", "provider cannot guarantee this command contract; no host fallback"))
		}
		if q.Exec == nil {
			return fail(protocol.Fail("INVALID_ARGUMENT", "exec request required"))
		}
		if e = q.Exec.Validate(); e != nil {
			return fail(e)
		}
	case "open":
		if !slices.Contains(caps.Apps, q.App) {
			return fail(protocol.Fail("UNSUPPORTED_OPERATION", "application launch target not supported"))
		}
	default:
		return fail(protocol.Fail("UNSUPPORTED_OPERATION", "operation not supported; cloud file transfer is not implemented"))
	}
	// Viewer/status traffic intentionally does not reset activity. Mutation intent
	// counts even when its outcome is unknown; it is never replayed automatically.
	if q.Method != "screenshot" {
		s.LastAction = m.now().UTC()
		m.db.Sessions[id] = s
		if e = m.save(); e != nil {
			return fail(e)
		}
	}
	m.mu.Unlock()
	opctx, cancel := context.WithTimeout(ctx, 65*time.Second)
	defer cancel()
	out, e := m.provider.Call(opctx, s, q)
	m.mu.Lock()
	defer m.mu.Unlock()
	current, auth := m.owner(i, id)
	if auth != nil {
		return Output{}, auth
	}
	if current.State != "ready" || !m.now().Before(current.Expires) {
		return Output{}, protocol.Fail("NOT_READY", "session stopped or expired during operation; outcome may be unknown")
	}
	if e != nil {
		return Output{}, e
	}
	if out.Generation != "" && out.Generation != s.Generation {
		current.State = "starting"
		var f *protocol.Failure
		if errors.As(e, &f) && f.Code == "RESOURCE_LIMIT" {
			current.State = "stopping"
		}
		current.Capabilities = nil
		m.db.Sessions[id] = current
		delete(m.frames, id)
		delete(m.leases, id)
		_ = m.save()
		return Output{}, protocol.Fail("STALE_FRAME", "provider desktop restarted; wait for a new readiness probe")
	}
	if q.Method == "screenshot" {
		if out.Generation == "" {
			return Output{}, protocol.Fail("INVALID_FRAME", "missing provider desktop generation")
		}
		m.sequence[id]++
		f := protocol.Frame{ID: protocol.ID(), SessionID: id, Generation: s.Generation, Sequence: m.sequence[id], CapturedAt: m.now().UTC(), Width: protocol.Width, Height: protocol.Height, Scale: 1, Format: "png", PNG: out.PNG}
		if e = f.Validate(); e != nil {
			return Output{}, e
		}
		if m.frames[id] == nil {
			m.frames[id] = map[string]frameRef{}
		}
		refs := m.frames[id]
		for k, v := range refs {
			if m.now().Sub(v.captured) > 10*time.Second {
				delete(refs, k)
			}
		}
		if len(refs) >= 16 {
			var oldest string
			var when time.Time
			for k, v := range refs {
				if oldest == "" || v.captured.Before(when) {
					oldest = k
					when = v.captured
				}
			}
			delete(refs, oldest)
		}
		refs[f.ID] = frameRef{s.Generation, m.now()}
		// Frame metadata is assembled at the authenticated control plane, not trusted
		// from the provider. Use Frame() below to retrieve it alongside these bytes.
		out.Frame = &f
		out.PNG = nil
	}
	if q.Method == "input" {
		if out.Input == nil || !out.Input.Dispatched {
			return Output{}, protocol.Fail("INVALID_MESSAGE", "missing input dispatch acknowledgement")
		}
		out.Input.FrameID = q.Input.FrameID
		out.Input.Generation = s.Generation
	}
	if q.Method == "exec" && (out.Execution == nil || len(out.Execution.Stdout) > q.Exec.OutputLimit || len(out.Execution.Stderr) > q.Exec.OutputLimit) {
		return Output{}, protocol.Fail("INVALID_MESSAGE", "malformed or oversized command result")
	}
	return out, nil
}

// Tick performs one bounded reconciliation pass. A single server owns the state
// directory lock. A failed/ambiguous create is discovered by labels, never retried.
func (m *Manager) Tick(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	m.mu.Lock()
	deployment := m.db.Deployment
	m.mu.Unlock()
	resources, e := m.provider.List(ctx, deployment)
	if e != nil {
		return protocol.Fail("RECONCILE_UNAVAILABLE", "provider inventory failed; no absence or cleanup success inferred")
	}
	if len(resources) > 1000 {
		return protocol.Fail("LIMIT_EXCEEDED", "provider inventory exceeds prototype bounds")
	}
	bySession := map[string]Resource{}
	for _, r := range resources {
		if r.Deployment != deployment {
			continue
		}
		m.mu.Lock()
		s, known := m.db.Sessions[r.Session]
		m.mu.Unlock()
		if !known || !active(s) || s.ProviderID != "" && s.ProviderID != r.ID {
			if e = m.provider.Delete(ctx, r); e != nil {
				return e
			}
			continue
		}
		if _, dup := bySession[r.Session]; dup {
			if e = m.provider.Delete(ctx, r); e != nil {
				return e
			}
			continue
		}
		bySession[r.Session] = r
	}
	m.mu.Lock()
	ids := make([]string, 0, len(m.db.Sessions))
	for id := range m.db.Sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		if e = m.reconcile(ctx, id, bySession); e != nil {
			return e
		}
	}
	return nil
}
func (m *Manager) reconcile(ctx context.Context, id string, inventory map[string]Resource) error {
	m.mu.Lock()
	s := m.db.Sessions[id]
	if !active(s) {
		m.mu.Unlock()
		return nil
	}
	cur := m.db.Identities[s.Owner]
	if !m.now().Before(s.Expires) || m.now().Sub(s.LastAction) >= time.Duration(m.db.Limits.IdleSeconds)*time.Second || cur.Revoked || !m.now().Before(cur.Expires) {
		s.State = "stopping"
	}
	if e := m.acquire(id); e != nil {
		m.db.Sessions[id] = s
		_ = m.save()
		m.mu.Unlock()
		return nil
	}
	defer m.release(id)
	r, found := inventory[id]
	if s.ProviderID != "" && !found {
		s.State = "stopping"
	}
	if found && s.ProviderID == "" {
		s.ProviderID = r.ID
		if s.State != "stopping" {
			s.State = "starting"
		}
	}
	m.db.Sessions[id] = s
	if e := m.save(); e != nil {
		m.mu.Unlock()
		return e
	}
	deployment := m.db.Deployment
	if s.State == "stopping" {
		m.mu.Unlock()
		if s.ProviderID != "" {
			if e := m.provider.Delete(ctx, Resource{ID: s.ProviderID, Session: id, Deployment: deployment}); e != nil {
				return e
			}
		} else if s.Attempted && m.now().Before(s.Expires.Add(time.Minute)) {
			return nil
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		s = m.db.Sessions[id]
		s.State = "stopped"
		s.StoppedAt = m.now().UTC()
		s.ObservedSeconds = int64(s.StoppedAt.Sub(s.Created).Seconds())
		if s.ObservedSeconds < 0 {
			s.ObservedSeconds = int64(s.Expires.Sub(s.Created).Seconds())
		}
		m.db.Sessions[id] = s
		delete(m.frames, id)
		delete(m.leases, id)
		return m.save()
	}
	if s.ProviderID == "" {
		if s.Attempted {
			m.mu.Unlock()
			return nil
		}
		s.Attempted = true
		m.db.Sessions[id] = s
		if e := m.save(); e != nil {
			m.mu.Unlock()
			return e
		}
		m.mu.Unlock()
		r, e := m.provider.Create(ctx, deployment, s)
		m.mu.Lock()
		defer m.mu.Unlock()
		current := m.db.Sessions[id]
		if e != nil {
			current.Error = "CREATE_OUTCOME_UNKNOWN"
			m.db.Sessions[id] = current
			return m.save()
		}
		if r.ID == "" || r.Session != id || r.Deployment != deployment {
			return fmt.Errorf("provider returned invalid allocation identity")
		}
		current.ProviderID = r.ID
		if current.State != "stopping" {
			current.State = "starting"
		}
		m.db.Sessions[id] = current
		return m.save()
	}
	m.mu.Unlock()
	probe, e := m.provider.Probe(ctx, s)
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.db.Sessions[id]
	if current.State == "stopping" {
		return nil
	}
	if e != nil {
		current.Error = "PROVIDER_UNAVAILABLE"
		current.State = "starting"
		var failure *protocol.Failure
		if errors.As(e, &failure) && failure.Code == "RESOURCE_LIMIT" {
			current.State = "stopping"
			current.Error = "RESOURCE_LIMIT"
		}
		current.Capabilities = nil
		delete(m.frames, id)
		delete(m.leases, id)
	} else if probe.Ready && probe.Generation != "" && probe.Capabilities != nil {
		current.State = "ready"
		current.Error = ""
		if current.Generation != probe.Generation {
			delete(m.frames, id)
			delete(m.leases, id)
		}
		current.Generation = probe.Generation
		current.Capabilities = probe.Capabilities
	} else {
		current.State = "starting"
		current.Capabilities = nil
		delete(m.frames, id)
		delete(m.leases, id)
	}
	m.db.Sessions[id] = current
	return m.save()
}
