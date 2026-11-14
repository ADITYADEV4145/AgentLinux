// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ADITYADEV4145/AgentLinux/internal/billing"
	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/software"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
	"os"
	"time"
)

type RemoteState struct {
	Billing                 *billing.SessionCost `json:"billing,omitempty"`
	Provider                string               `json:"provider"`
	ExpiresAt               time.Time            `json:"expiresAt"`
	IdleStopAt              time.Time            `json:"idleStopAt"`
	EstimatedRuntimeSeconds int64                `json:"estimatedRuntimeSeconds"`
	Persistent              bool                 `json:"persistent"`
	Simulated               bool                 `json:"simulated"`
}
type State struct {
	Backend        string         `json:"backend,omitempty"`
	Remote         *RemoteState   `json:"remote,omitempty"`
	Schema         int            `json:"schema"`
	MachineID      string         `json:"machineId"`
	State          string         `json:"state"`
	Observed       string         `json:"observed"`
	Stage          string         `json:"stage,omitempty"`
	Spec           config.Spec    `json:"spec"`
	BackendVersion string         `json:"backendVersion,omitempty"`
	Guest          *protocol.Info `json:"guest,omitempty"`
	LastError      string         `json:"lastError,omitempty"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}
type Observation struct {
	Exists bool
	Status string
	Port   int
}
type Backend interface {
	Observe(context.Context) (Observation, error)
	Create(context.Context, State) error
	Validate(context.Context) error
	Start(context.Context) error
	Stop(context.Context) error
	Handshake(context.Context, State) (*protocol.Info, error)
}
type Controller struct {
	ExpectedID   string // Pin long-lived clients and named selections to an exact identity.
	Reserve      func(context.Context, config.Spec) (func(), error)
	Root         string
	ControlOwner string // Transient random viewer nonce; never passed to the guest.
	Backend      Backend
	Preflight    func(config.Spec, bool) error
	Progress     func(string)
}

func (c *Controller) load() (State, error) {
	var s State
	e := storage.Read(c.Root, "state.json", &s)
	if os.IsNotExist(e) {
		return State{Schema: 1, State: "not_created", Observed: "absent"}, nil
	}
	if e != nil {
		return s, e
	}
	if c.ExpectedID != "" && s.MachineID != c.ExpectedID {
		return s, protocol.Fail("MACHINE_IDENTITY_CHANGED", "Selected machine was replaced; reconnect explicitly after checking status.")
	}
	if s.Schema != 1 || !protocol.ValidID(s.MachineID) {
		return s, errors.New("invalid stored state schema/identity; do not delete disks, inspect state backup")
	}
	switch s.State {
	case "creating", "starting", "ready", "stopping", "stopped", "error", "deleting":
	default:
		return s, errors.New("invalid stored lifecycle state")
	}
	return s, nil
}
func (c *Controller) save(s *State) error {
	if c.Progress != nil {
		c.Progress(s.State + ": " + s.Stage)
	}
	s.UpdatedAt = time.Now().UTC()
	if e := storage.JSON(c.Root, "state.json", s); e != nil {
		return e
	}
	return c.log(s.State, s.Stage, s.LastError)
}
func (c *Controller) log(state, stage, code string) error {
	if e := storage.SafeFile(c.Root, "events.jsonl"); e != nil {
		return e
	}
	path := c.Root + "/events.jsonl"
	if i, e := os.Stat(path); e == nil && i.Size() > 1<<20 {
		if e = storage.Write(c.Root, "events.jsonl", nil); e != nil {
			return e
		}
	}
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	return protocol.Write(f, map[string]any{"time": time.Now().UTC(), "state": state, "stage": stage, "code": code})
}
func (c *Controller) fail(s *State, err error) (State, error) {
	if !protocol.ValidID(s.MachineID) {
		return *s, err
	}
	if s.State != "deleting" {
		s.State = "error"
	}
	s.Guest = nil
	s.LastError = "BACKEND_OPERATION_FAILED"
	if errors.Is(err, context.Canceled) {
		s.LastError = "INTERRUPTED"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		s.LastError = "DEADLINE_EXCEEDED"
	}
	saveErr := c.save(s)
	if saveErr != nil {
		return *s, errors.Join(err, saveErr)
	}
	return *s, err
}
func (c *Controller) Up(ctx context.Context, spec config.Spec, creationFlags bool) (State, error) {
	unlock, e := storage.Lock(c.Root)
	if e != nil {
		return State{}, e
	}
	defer unlock()
	return c.upLocked(ctx, spec, creationFlags)
}
func (c *Controller) upLocked(ctx context.Context, spec config.Spec, creationFlags bool) (State, error) {
	s, e := c.load()
	if e != nil {
		return s, e
	}
	if e = c.allowBoot(ctx, s); e != nil {
		return s, e
	}
	if s.State == "deleting" {
		return s, errors.New("interrupted deletion; finish delete/reset with the original machine ID before starting")
	}
	o, e := c.Backend.Observe(ctx)
	if e != nil {
		return c.fail(&s, e)
	}
	if s.State == "not_created" {
		if o.Exists {
			return s, errors.New("unrecorded VM exists in owned namespace; refuse to adopt or overwrite it")
		}
		if e = spec.Validate(); e != nil {
			return s, e
		}
		if c.Preflight != nil {
			if e = c.Preflight(spec, true); e != nil {
				return s, e
			}
		}
		id := c.ExpectedID
		if id == "" {
			id = protocol.ID()
		}
		if !protocol.ValidID(id) {
			return s, errors.New("invalid expected machine identity")
		}
		s = State{Schema: 1, MachineID: id, State: "creating", Observed: "absent", Stage: "provision_configuration", Spec: spec, BackendVersion: config.LimaVersion}
		if e = c.save(&s); e != nil {
			return s, e
		}
	} else {
		if creationFlags {
			return s, errors.New("creation flags cannot change an existing machine; use up --local without them to resume")
		}
		if c.Preflight != nil {
			if e = c.Preflight(s.Spec, !o.Exists); e != nil {
				return s, e
			}
		}
	}
	if c.Reserve != nil {
		release, err := c.Reserve(ctx, s.Spec)
		if err != nil {
			return s, err
		}
		defer release()
	}
	if !o.Exists && s.Stage != "provision_configuration" {
		return c.fail(&s, errors.New("recorded VM is missing; refusing to silently recreate its disk"))
	}
	if !o.Exists {
		if e = c.Backend.Create(ctx, s); e != nil {
			return c.fail(&s, e)
		}
	}
	if e = c.Backend.Validate(ctx); e != nil {
		return c.fail(&s, e)
	}
	s.State = "starting"
	s.Stage = "boot_and_provision"
	s.Observed = o.Status
	s.Guest = nil
	s.LastError = ""
	if e = c.save(&s); e != nil {
		return s, e
	}
	if o.Status != "Running" {
		if e = c.Backend.Start(ctx); e != nil {
			return c.fail(&s, e)
		}
	}
	s.Stage = "guest_handshake"
	if e = c.save(&s); e != nil {
		return s, e
	}
	g, e := c.Backend.Handshake(ctx, s)
	if e != nil {
		return c.fail(&s, e)
	}
	s.State = "ready"
	s.Observed = "Running"
	s.Stage = "desktop_ready"
	s.Guest = g
	if e = c.confirmRecovery(ctx, s); e != nil {
		return c.fail(&s, e)
	}
	if e = c.save(&s); e != nil {
		return s, e
	}
	return s, nil
}
func (c *Controller) Status(ctx context.Context) (State, error) {
	unlock, e := storage.LockWait(ctx, c.Root, 2*time.Second)
	if e != nil {
		return State{}, e
	}
	defer unlock()
	s, e := c.load()
	if e != nil {
		return s, e
	}
	if err:=c.allowBoot(ctx,s);err!=nil { s.Stage="recovery_required"; return s,err }
	if s.State == "not_created" {
		return s, nil
	}
	o, e := c.Backend.Observe(ctx)
	if e != nil {
		s.Observed = "unknown"
		return c.fail(&s, e)
	}
	if s.State == "deleting" {
		s.Guest = nil
		s.Observed = o.Status
		if !o.Exists {
			s.Observed = "absent"
		}
		return s, c.save(&s)
	}
	s.Observed = o.Status
	s.Guest = nil
	if !o.Exists {
		s.Observed = "absent"
		return c.fail(&s, errors.New("recorded machine missing from runtime; preserving state, do not recreate automatically"))
	}
	if o.Status == "Stopped" {
		s.State = "stopped"
		s.Stage = ""
		s.LastError = ""
		e = c.save(&s)
		return s, e
	}
	if o.Status != "Running" {
		s.State = "error"
		s.LastError = "BACKEND_NOT_RUNNING"
		e = c.save(&s)
		return s, e
	}
	if e = c.Backend.Validate(ctx); e != nil {
		return c.fail(&s, e)
	}
	g, e := c.Backend.Handshake(ctx, s)
	if e != nil {
		return c.fail(&s, e)
	}
	s.State = "ready"
	s.Stage = "desktop_ready"
	s.Guest = g
	s.LastError = ""
	return s, c.save(&s)
}
func (c *Controller) Stop(ctx context.Context) (State, error) {
	unlock, e := storage.Lock(c.Root)
	if e != nil {
		return State{}, e
	}
	defer unlock()
	s, e := c.load()
	if e != nil {
		return s, e
	}
	if s.State == "not_created" {
		return s, nil
	}
	if s.State == "deleting" {
		return s, errors.New("interrupted deletion; finish explicit delete/reset instead of stopping or resuming")
	}
	o, e := c.Backend.Observe(ctx)
	if e != nil {
		s.Observed = "unknown"
		return c.fail(&s, e)
	}
	if !o.Exists {
		return c.fail(&s, errors.New("recorded VM missing; no unrelated runtime resources were changed"))
	}
	if o.Status == "Stopped" {
		s.State = "stopped"
		s.Observed = "Stopped"
		s.Guest = nil
		s.LastError = ""
		s.Stage = ""
		return s, c.save(&s)
	}
	if e = c.revokeLease(); e != nil {
		return s, e
	}
	s.State = "stopping"
	s.Stage = "graceful_shutdown"
	s.Guest = nil
	if e = c.save(&s); e != nil {
		return s, e
	}
	if e = c.Backend.Stop(ctx); e != nil {
		s.Observed = "unknown"
		return c.fail(&s, fmt.Errorf("shutdown unconfirmed; VM may still run, inspect status/logs: %w", e))
	}
	o, e = c.Backend.Observe(ctx)
	if e != nil || o.Status != "Stopped" {
		s.Observed = "unknown"
		return c.fail(&s, errors.New("shutdown not confirmed by runtime; VM may still run"))
	}
	s.State = "stopped"
	s.Observed = "Stopped"
	s.Stage = ""
	s.LastError = ""
	return s, c.save(&s)
}

// GuestCall holds the local machine lock throughout a single real guest request.
func (c *Controller) GuestCall(ctx context.Context, q protocol.Request) (*protocol.Info, error) {
	unlock, e := storage.LockWait(ctx, c.Root, 2*time.Second)
	if e != nil {
		return nil, e
	}
	defer unlock()
	s, e := c.load()
	if e != nil {
		return nil, e
	}
	if e=c.allowBoot(ctx,s);e!=nil{return nil,e}
	if s.State == "deleting" {
		return nil, protocol.Fail("NOT_READY", "deletion in progress; finish explicit delete/reset")
	}
	if s.State == "not_created" {
		return nil, protocol.Fail("NOT_READY", "create and start the local machine first")
	}
	if e = c.Backend.Validate(ctx); e != nil {
		return nil, e
	}
	backend, ok := c.Backend.(interface {
		Call(context.Context, State, protocol.Request) (*protocol.Info, error)
	})
	if !ok {
		return nil, protocol.Fail("UNSUPPORTED_OPERATION", "backend has no guest computer interface")
	}
	if q.Method == "input" || q.Method == "exec" || q.Method == "open" || q.Method == "files" {
		if e = c.checkInputLease(s); e != nil {
			return nil, e
		}
	}
	result, callError := backend.Call(ctx, s, q)
	if q.Method == "input" || q.Method == "exec" || q.Method == "open" || q.Method == "files" {
		// An ambiguous result may still have changed the guest. Wake other processes'
		// viewers without recording command text, pixels or input. Advisory failure
		// does not change the action result or invite a dangerous mutation retry.
		_ = storage.JSON(c.Root, "viewer-change.json", protocol.ID())
	}
	return result, callError
}

// Maintain is explicit host-operator guest administration, never an MCP tool.
func (c *Controller) Maintain(ctx context.Context, action string, packages []string) (*software.Result, error) {
	if e := software.Validate(action, packages); e != nil {
		return nil, protocol.Fail("INVALID_ARGUMENT", e.Error())
	}
	unlock, e := storage.Lock(c.Root)
	if e != nil {
		return nil, e
	}
	defer unlock()
	s, e := c.load()
	if e != nil {
		return nil, e
	}
	if e = c.checkInputLease(s); e != nil {
		return nil, e
	}
	if s.State == "not_created" {
		return nil, protocol.Fail("NOT_READY", "create and start the VM first")
	}
	if e = c.Backend.Validate(ctx); e != nil {
		return nil, e
	}
	b, ok := c.Backend.(interface {
		Maintain(context.Context, string, []string) (*software.Result, error)
	})
	if !ok {
		return nil, protocol.Fail("UNSUPPORTED_OPERATION", "backend has no explicit guest package maintenance")
	}
	return b.Maintain(ctx, action, packages)
}

// Cloud machines do not have a local VM config, image pathname or Lima spec.
func (s State) MarshalJSON() ([]byte, error) {
	type plain State
	if s.Backend != "cloud" {
		return json.Marshal(plain(s))
	}
	v := plain(s)
	return json.Marshal(struct {
		*plain
		Spec *config.Spec `json:"spec,omitempty"`
	}{plain: &v})
}

// ObservationRevision is an advisory signal, never evidence of guest readiness.
// Missing/malformed signals retain the viewer's ordinary bounded polling path.
func (c *Controller) ObservationRevision() string {
	var revision string
	if storage.Read(c.Root, "viewer-change.json", &revision) != nil || !protocol.ValidID(revision) {
		return ""
	}
	return revision
}
