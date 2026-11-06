// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"context"
	"encoding/json"
	"github.com/ADITYADEV4145/AgentLinux/internal/billing"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func metered(t *testing.T) (*Manager, *Fake, Identity, Identity, string) {
	t.Helper()
	m, p, a, b, root := fixture(t)
	now := time.Now().UTC()
	m.now = func() time.Time { return now }
	config := billing.Config{Version: billing.Version, Mode: "sandbox", Currency: "usd", Rates: []billing.Rate{{ID: "test-only", EffectiveAt: now.Add(-time.Hour), MicrosPerHour: 3600000, CPUs: 2, MemoryGiB: 4, DiskGiB: 24}}, Policy: billing.Policy{TrialMicros: 900000, SpendingCapMicros: 1000000, Concurrency: 1, MaxSeconds: 900}}
	var e error
	m.db.Billing, e = billing.New(config, []string{a.Workspace, b.Workspace})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.save(); e != nil {
		t.Fatal(e)
	}
	return m, p, a, b, root
}
func TestCloudLedgerAtomicStopRestartCancellationAndAccountIsolation(t *testing.T) {
	m, p, a, b, root := metered(t)
	s := ready(t, m, a)
	initial := m.now()
	m.now = func() time.Time { return initial.Add(10 * time.Second) }
	if e := m.Tick(context.Background()); e != nil {
		t.Fatal(e)
	}
	got, e := m.Get(a, s.ID)
	if e != nil || got.Billing == nil || got.Billing.Seconds != 10 {
		t.Fatal(got, e)
	}
	_, e = m.BillingAction(a, "", "cancel", false)
	code(t, e, "CONFIRM_REQUIRED")
	if _, e = m.BillingAction(a, "", "cancel", true); e != nil {
		t.Fatal(e)
	}
	if e = m.Tick(context.Background()); e != nil {
		t.Fatal(e)
	}
	restored, e := New(root, p)
	if e != nil {
		t.Fatal(e)
	}
	if len(restored.db.Billing.Ledger) != 1 || restored.db.Billing.Ledger[0].QuantitySeconds != 10 {
		t.Fatal(restored.db.Billing.Ledger)
	}
	restored.now = m.now
	_ = restored.Tick(context.Background())
	if len(restored.db.Billing.Ledger) != 1 {
		t.Fatal("duplicate finalized usage on restart")
	}
	_, e = restored.Create(a, protocol.ID(), Create{a.Workspace, 60, "desktop"})
	code(t, e, "BILLING_LIMIT")
	other, e := restored.Billing(b)
	if e != nil {
		t.Fatal(e)
	}
	summary := other.(billing.Summary)
	if len(summary.History) != 0 || summary.Account.Canceled {
		t.Fatal("other workspace data or cancellation leaked")
	}
}
func TestBillingAdmissionFailsClosedAndKeepsCleanupAvailable(t *testing.T) {
	m, p, a, _, root := metered(t)
	s := ready(t, m, a)
	// Simulate a filesystem failure. No new control request can authenticate;
	// the next reconciliation attempts owned provider deletion despite the ledger.
	if e := os.Rename(filepath.Join(root, "cloud.json"), filepath.Join(root, "saved-cloud.json")); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink("saved-cloud.json", filepath.Join(root, "cloud.json")); e != nil {
		t.Fatal(e)
	}
	if e := m.Tick(context.Background()); e == nil {
		t.Fatal("metering outage hidden")
	}
	if _, e := m.Authenticate("not-a-token"); e == nil {
		t.Fatal("admission remained open")
	}
	_ = m.Tick(context.Background())
	list, e := p.List(context.Background(), m.db.Deployment)
	if e != nil || len(list) != 0 {
		t.Fatal("owned cleanup not attempted", list, e)
	}
	if m.db.Sessions[s.ID].State == "stopped" {
		t.Fatal("invented durable finalized stop during storage outage")
	}
}
func TestSignedWebhookInboxIsDurablePrivateAndAuthenticatedBilling(t *testing.T) {
	m, _, a, b, root := metered(t)
	m.EnableSandboxPayments(&testPayments{}, "whsec_synthetic_fixture")
	api := NewAPI(m)
	server := httptest.NewServer(api)
	defer server.Close()
	c, e := LoadClient(filepath.Join(root, "alice.json"))
	if e != nil {
		t.Fatal(e)
	}
	c.Config.URL = server.URL
	raw, e := c.Billing(context.Background())
	if e != nil || strings.Contains(string(raw), b.Workspace) {
		t.Fatal("workspace billing exposure", e)
	}
	event, _ := json.Marshal(map[string]any{"id": "evt_fixture", "object": "event", "type": "checkout.session.completed", "api_version": stripe.APIVersion, "livemode": false, "data": map[string]any{"object": map[string]any{"id": "cs_test_fixture", "private_card_content": "never persist"}}})
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: event, Secret: "whsec_synthetic_fixture", Timestamp: time.Now()})
	post := func(signature string) int {
		r := httptest.NewRequest("POST", "http://example/v1/stripe/webhook", strings.NewReader(string(event)))
		r.Header.Set("Stripe-Signature", signature)
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w.Code
	}
	if code := post("bad"); code != http.StatusBadRequest {
		t.Fatal(code)
	}
	for i := 0; i < 2; i++ {
		if code := post(signed.Header); code != 200 {
			t.Fatal(code)
		}
	}
	if len(m.db.Billing.Receipts) != 1 {
		t.Fatal("webhook replay duplicated")
	}
	var stored Database
	if e = storage.Read(root, "cloud.json", &stored); e != nil {
		t.Fatal(e)
	}
	bytes, _ := os.ReadFile(filepath.Join(root, "cloud.json"))
	if strings.Contains(string(bytes), "never persist") || strings.Contains(string(bytes), "whsec_synthetic") {
		t.Fatal("webhook payload/secret persisted")
	}
	_, e = m.BillingAction(a, protocol.ID(), "setup", false)
	if e != nil {
		t.Fatal(e)
	}
}

type testPayments struct{}

func (*testPayments) Customer(context.Context, string, string) (string, error) {
	return "cus_fixture", nil
}
func (*testPayments) Setup(context.Context, string, string, string) (billing.Setup, error) {
	return billing.Setup{ID: "cs_test_fixture", URL: "https://checkout.stripe.com/fixture"}, nil
}
func (*testPayments) CheckSetup(context.Context, string, string) (billing.Setup, error) {
	return billing.Setup{}, nil
}
func (*testPayments) Draft(context.Context, billing.Job) (billing.Invoice, error) {
	return billing.Invoice{}, nil
}
func (*testPayments) Item(context.Context, billing.Job) (string, error) { return "", nil }
func (*testPayments) Finalize(context.Context, billing.Job) (billing.Invoice, error) {
	return billing.Invoice{}, nil
}
func (*testPayments) CheckInvoice(context.Context, billing.Job) (billing.Invoice, error) {
	return billing.Invoice{}, nil
}
func (*testPayments) Portal(context.Context, string, string) (string, error) { return "", nil }
func TestPersistedSizeGuardPreservesPreviousState(t *testing.T) {
	m, _, _, _, root := metered(t)
	previous, e := os.ReadFile(filepath.Join(root, "cloud.json"))
	if e != nil {
		t.Fatal(e)
	}
	// Use an intentionally oversized diagnostic value to exercise the exact
	// indented on-disk bound, not just compact JSON byte count.
	m.db.Sessions[protocol.ID()] = Session{Error: strings.Repeat("x", 1<<20)}
	if e = m.save(); e == nil {
		t.Fatal("oversized state persisted")
	}
	after, e := os.ReadFile(filepath.Join(root, "cloud.json"))
	if e != nil || string(after) != string(previous) {
		t.Fatal("known state overwritten")
	}
}
func TestProviderProfileMismatchNeverStartsCustomerBilling(t *testing.T) {
	m, _, a, _, _ := metered(t)
	m.db.Billing.Config.Rates[0].CPUs = 4 // Fake provider actually reports 2.
	s, e := m.Create(a, protocol.ID(), Create{a.Workspace, 60, "desktop"})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		if e = m.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
	got, _ := m.Get(a, s.ID)
	if got.State != "stopped" || !got.ReadyAt.IsZero() || len(m.db.Billing.Ledger) != 1 || m.db.Billing.Ledger[0].QuantitySeconds != 0 {
		t.Fatal("mismatched profile billed or remained allocated", got)
	}
}
func TestPaymentOutageFreshnessStopsPaidSession(t *testing.T) {
	m, _, a, _, _ := metered(t)
	account := m.db.Billing.Accounts[a.Workspace]
	account.Policy.TrialMicros = 0
	account.PaymentReady = true
	account.PaymentCheckedAt = m.now()
	m.db.Billing.Accounts[a.Workspace] = account
	s, e := m.Create(a, protocol.ID(), Create{a.Workspace, 300, "desktop"})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if e = m.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
	initial := m.now()
	m.now = func() time.Time { return initial.Add(121 * time.Second) }
	if e = m.Tick(context.Background()); e != nil {
		t.Fatal(e)
	}
	got, _ := m.Get(a, s.ID)
	if got.State != "stopped" {
		t.Fatal("stale payment service left paid compute running", got.State)
	}
	_, e = m.Create(a, protocol.ID(), Create{a.Workspace, 60, "desktop"})
	code(t, e, "BILLING_LIMIT")
}
