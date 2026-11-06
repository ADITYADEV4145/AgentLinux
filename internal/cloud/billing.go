// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/billing"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

func (m *Manager) EnableSandboxPayments(p billing.Payments, secret string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.payments = p
	m.webhookSecret = secret
}
func (m *Manager) Billing(i Identity) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.db.Identities[i.ID]
	if !ok || current.TokenHash != i.TokenHash || current.Revoked || !m.now().Before(current.Expires) {
		return nil, protocol.Fail("UNAUTHORIZED", "credential invalid")
	}
	if m.db.Billing == nil {
		return map[string]any{"mode": "disabled", "localPlan": "free", "cloudPricing": "not configured; no finalized retail prices", "paymentIntegration": "sandbox only; live mode unavailable"}, nil
	}
	return m.db.Billing.Summary(current.Workspace, m.now().UTC()), nil
}
func (m *Manager) BillingAction(i Identity, key, action string, confirm bool) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.db.Identities[i.ID]
	if !ok || cur.TokenHash != i.TokenHash || cur.Revoked || !m.now().Before(cur.Expires) {
		return nil, protocol.Fail("UNAUTHORIZED", "credential invalid")
	}
	if m.db.Billing == nil {
		return nil, protocol.Fail("BILLING_UNAVAILABLE", "operator has not configured sandbox pricing/limits")
	}
	if action == "cancel" {
		if !confirm {
			return nil, protocol.Fail("CONFIRM_REQUIRED", "cancel blocks new sessions and queues active sessions for deletion; finalized usage/invoices remain; confirm explicitly")
		}
		a := m.db.Billing.Accounts[cur.Workspace]
		a.Canceled = true
		m.db.Billing.Accounts[cur.Workspace] = a
		m.stopBillingWorkspace(cur.Workspace)
		return map[string]bool{"canceled": true}, m.save()
	}
	if action != "setup" && action != "portal" {
		return nil, protocol.Fail("UNSUPPORTED_OPERATION", "billing action unavailable")
	}
	if !protocol.ValidID(key) {
		return nil, protocol.Fail("INVALID_ARGUMENT", "billing requires a stable 32-hex request-id")
	}
	if m.payments == nil {
		return nil, protocol.Fail("BILLING_UNAVAILABLE", "sandbox Stripe SDK is not enabled by the operator")
	}
	j, e := m.db.Billing.Queue(cur.Workspace, key, action, false, m.now().UTC())
	if e != nil {
		return nil, protocol.Fail("BILLING_LIMIT", e.Error())
	}
	return j, m.save()
}
func (m *Manager) stopBillingWorkspace(w string) {
	for id, s := range m.db.Sessions {
		if s.Workspace == w && active(s) {
			s.State = "stopping"
			m.db.Sessions[id] = s
			delete(m.frames, id)
			delete(m.leases, id)
		}
	}
}
func (m *Manager) tickBilling(parent context.Context) error {
	m.mu.Lock()
	if m.poisoned {
		// Metering failure closes admission. Cleanup does not require a successful
		// financial write: attempt owned provider termination, preserve state for
		// operator reconciliation rather than inventing a finalized invoice.
		owned := []Resource{}
		for _, s := range m.db.Sessions {
			if active(s) && s.ProviderID != "" {
				owned = append(owned, Resource{s.ProviderID, s.ID, m.db.Deployment, ""})
			}
		}
		m.mu.Unlock()
		ctx, cancel := context.WithTimeout(parent, 30*time.Second)
		defer cancel()
		for _, r := range owned {
			_ = m.provider.Delete(ctx, r)
		}
		return protocol.Fail("BILLING_UNAVAILABLE", "durable state unavailable; owned cleanup attempted, accounting requires operator reconciliation")
	}
	defer m.mu.Unlock()
	if m.db.Billing == nil {
		return nil
	}
	now := m.now().UTC()
	for id, s := range m.db.Sessions {
		when := now
		if s.State == "stopped" {
			when = s.StoppedAt
		}
		if e := m.db.Billing.Observe(id, s.ReadyAt, s.State == "stopped", when); e != nil {
			m.poisoned = true
			return e
		}
		if active(s) && m.db.Billing.ShouldStop(id, now) {
			s.State = "stopping"
			m.db.Sessions[id] = s
			delete(m.frames, id)
			delete(m.leases, id)
		}
	}
	if e := m.save(); e != nil {
		return e
	}
	// Bounded synchronous development worker. Each step is persisted separately;
	// production needs a transactional database/independent durable queue.
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	if m.payments != nil && now.Sub(m.lastPaymentStep) >= 5*time.Second {
		m.lastPaymentStep = now
		ids := m.db.Billing.PendingJobs()
		if len(ids) > 0 {
			_ = m.db.Billing.Step(ctx, ids[0], m.payments, now)
			if e := m.save(); e != nil {
				return e
			}
		}
	}
	if m.payments != nil && now.Sub(m.lastPaymentRefresh) >= 30*time.Second {
		m.lastPaymentRefresh = now
		_ = m.db.Billing.Refresh(ctx, m.payments, now)
		ids := []string{}
		for id, r := range m.db.Billing.Receipts {
			if r.Status != "done" {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		for _, id := range ids {
			if ctx.Err() != nil {
				break
			}
			_ = m.db.Billing.ProcessReceipt(ctx, id, m.payments, now)
		}
		if e := m.save(); e != nil {
			return e
		}
	}
	return nil
}
func (m *Manager) ReceiveWebhook(raw []byte, signature string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.poisoned || m.db.Billing == nil || m.payments == nil || m.webhookSecret == "" {
		return protocol.Fail("BILLING_UNAVAILABLE", "sandbox webhook receiver not configured")
	}
	r, e := billing.VerifyWebhook(raw, signature, m.webhookSecret, m.now().UTC())
	if e != nil {
		return protocol.Fail("INVALID_MESSAGE", "webhook signature, mode, version or type rejected")
	}
	if e = m.db.Billing.Receive(r); e != nil {
		return protocol.Fail("BILLING_UNAVAILABLE", "webhook inbox unavailable; retry delivery")
	}
	return m.save()
}
func (a *API) billingWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, 405, nil, protocol.Fail("INVALID_ARGUMENT", "POST required"))
		return
	}
	select {
	case a.gate <- struct{}{}:
		defer func() { <-a.gate }()
	default:
		writeJSON(w, 429, nil, protocol.Fail("RATE_LIMITED", "webhook capacity"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, protocol.MaxRequest)
	defer r.Body.Close()
	raw, e := io.ReadAll(r.Body)
	if e != nil {
		writeJSON(w, 400, nil, protocol.Fail("INVALID_MESSAGE", "oversized webhook"))
		return
	}
	e = a.Manager.ReceiveWebhook(raw, r.Header.Get("Stripe-Signature"))
	status := 200
	if e != nil {
		status = 503
		var f *protocol.Failure
		if errors.As(e, &f) && f.Code == "INVALID_MESSAGE" {
			status = 400
		}
	}
	writeJSON(w, status, map[string]bool{"received": e == nil}, e)
}
func (c *Client) Billing(ctx context.Context) (json.RawMessage, error) {
	var v json.RawMessage
	e := c.request(ctx, "GET", "/v1/billing", "", nil, &v)
	return v, e
}
func (c *Client) BillingAction(ctx context.Context, key, action string, confirm bool) (json.RawMessage, error) {
	var v json.RawMessage
	e := c.request(ctx, "POST", "/v1/billing", "", struct {
		Action    string `json:"action"`
		RequestID string `json:"requestId"`
		Confirm   bool   `json:"confirm"`
	}{action, key, confirm}, &v)
	return v, e
}

// ConfigureBilling/AdminBilling run under the existing exclusive directory lock
// with the server stopped. Neither method calls Stripe or any cloud API.
func ConfigureBilling(root string, c billing.Config) error {
	var db Database
	if e := storage.Read(root, "cloud.json", &db); e != nil {
		return e
	}
	if db.Version != Version {
		return errors.New("unsupported control-plane state version")
	}
	if db.Billing == nil {
		for _, s := range db.Sessions {
			if active(s) {
				return errors.New("stop existing sessions before enabling a new billing epoch; historical sessions are not retroactively charged")
			}
		}
		workspaces := []string{}
		for _, i := range db.Identities {
			workspaces = append(workspaces, i.Workspace)
		}
		var e error
		db.Billing, e = billing.New(c, workspaces)
		if e != nil {
			return e
		}
	} else {
		if e := db.Billing.UpdateRates(c); e != nil {
			return e
		}
	}
	return persistDatabase(root, db)
}

type BillingAdmin struct {
	Workspace      string
	Action         string
	RequestID      string
	Reference      string
	Reason         string
	Micros         int64
	ResolveSeconds *int64
	Policy         *billing.Policy
	Finalize       bool
}

func AdminBilling(root string, q BillingAdmin) (any, error) {
	var db Database
	if e := storage.Read(root, "cloud.json", &db); e != nil {
		return nil, e
	}
	if db.Billing == nil {
		return nil, errors.New("sandbox billing not configured")
	}
	if e := db.Billing.Validate(); e != nil {
		return nil, e
	}
	account, ok := db.Billing.Accounts[q.Workspace]
	if !ok {
		return nil, errors.New("select an existing workspace")
	}
	before := account
	if len(db.Billing.Changes) >= 256 {
		return nil, errors.New("account audit history full; archive before further admin mutations")
	}
	var result any = map[string]bool{"updated": true}
	var e error
	switch q.Action {
	case "invoice":
		result, e = db.Billing.Queue(q.Workspace, q.RequestID, "invoice", q.Finalize, time.Now().UTC())
	case "suspend":
		account.Suspended = true
		db.Billing.Accounts[q.Workspace] = account
	case "resume":
		account.Suspended = false
		account.Canceled = false
		account.PaymentReady = false
		db.Billing.Accounts[q.Workspace] = account
	case "policy":
		if q.Policy == nil {
			return nil, errors.New("policy required")
		}
		if e = q.Policy.Validate(); e == nil {
			account.Policy = *q.Policy
			db.Billing.Accounts[q.Workspace] = account
		}
	case "adjust", "resolve":
		found := false
		for _, entry := range db.Billing.Ledger {
			if entry.ID == q.Reference && entry.Workspace == q.Workspace {
				found = true
			}
		}
		if !found {
			return nil, errors.New("ledger entry is not owned by selected workspace")
		}
		e = db.Billing.Adjustment(q.RequestID, q.Reference, q.Reason, q.Micros, q.ResolveSeconds, time.Now().UTC())
	default:
		return nil, errors.New("unsupported billing admin action")
	}
	if e != nil {
		return nil, e
	}
	if q.Action == "suspend" {
		for id, s := range db.Sessions {
			if s.Workspace == q.Workspace && active(s) {
				s.State = "stopping"
				db.Sessions[id] = s
			}
		}
	}
	db.Billing.Changes = append(db.Billing.Changes, billing.AccountChange{At: time.Now().UTC(), Workspace: q.Workspace, Action: q.Action, Before: before, After: db.Billing.Accounts[q.Workspace]})
	if e = db.Billing.Validate(); e != nil {
		return nil, e
	}
	return result, persistDatabase(root, db)
}
func ReadBillingConfig(path string) (billing.Config, error) {
	var c billing.Config
	f, e := os.Open(path)
	if e != nil {
		return c, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Size() > 16384 {
		return c, errors.New("pricing config must be a bounded regular JSON file")
	}
	d := json.NewDecoder(io.LimitReader(f, 16385))
	d.DisallowUnknownFields()
	if e = d.Decode(&c); e != nil {
		return c, e
	}
	if d.Decode(new(any)) != io.EOF {
		return c, errors.New("trailing billing configuration")
	}
	return c, c.Validate()
}
func ReadBillingPolicy(path string) (billing.Policy, error) {
	var c billing.Policy
	f, e := os.Open(path)
	if e != nil {
		return c, e
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 8193))
	d.DisallowUnknownFields()
	if e = d.Decode(&c); e != nil {
		return c, e
	}
	if d.Decode(new(any)) != io.EOF {
		return c, errors.New("trailing policy data")
	}
	return c, c.Validate()
}

func persistDatabase(root string, db Database) error {
	encoded, e := json.MarshalIndent(db, "", "  ")
	if e != nil {
		return e
	}
	if len(encoded)+1 > 1<<20 {
		return errors.New("bounded durable cloud/billing store full; operator archival required; no prior state overwritten")
	}
	return storage.Write(root, "cloud.json", append(encoded, '\n'))
}
