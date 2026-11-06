// SPDX-License-Identifier: Apache-2.0
package billing

import (
	"context"
	"encoding/json"
	"fmt"
	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLiveKeysAndUnsafeReturnURLsRejected(t *testing.T) {
	for _, key := range []string{"", "sk_live_" + strings.Repeat("x", 24), "rk_test_" + strings.Repeat("x", 24)} {
		if _, e := NewStripe(key, "http://127.0.0.1:8787/return"); e == nil {
			t.Fatal("unsafe key accepted")
		}
	}
	for _, u := range []string{"http://example.com/return", "https://user:pass@example.com", "https://example.com/?secret=bad"} {
		if _, e := NewStripe("sk_test_"+strings.Repeat("x", 24), u); e == nil {
			t.Fatal(u)
		}
	}
}
func signed(t *testing.T, eventType, id, object string, live bool, at time.Time) ([]byte, string) {
	t.Helper()
	b, e := json.Marshal(map[string]any{"id": id, "object": "event", "type": eventType, "livemode": live, "api_version": stripe.APIVersion, "data": map[string]any{"object": map[string]any{"id": object, "livemode": live, "untrusted": "must not persist"}}})
	if e != nil {
		t.Fatal(e)
	}
	payload := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: b, Secret: "whsec_synthetic_fixture", Timestamp: at})
	return b, payload.Header
}
func TestOfficialWebhookSignaturesReplayVersionAndMode(t *testing.T) {
	now := time.Now()
	b, h := signed(t, "invoice.paid", "evt_fixture", "in_fixture", false, now)
	r, e := VerifyWebhook(b, h, "whsec_synthetic_fixture", now)
	if e != nil {
		t.Fatal(e)
	}
	d := fixture(t, 0)
	if e = d.Receive(r); e != nil {
		t.Fatal(e)
	}
	if e = d.Receive(r); e != nil || len(d.Receipts) != 1 {
		t.Fatal("replay not deduplicated", e)
	}
	r.Object = "in_other"
	if d.Receive(r) == nil {
		t.Fatal("conflicting replay accepted")
	}
	if _, e = VerifyWebhook(append(b, ' '), h, "whsec_synthetic_fixture", now); e == nil {
		t.Fatal("modified raw body accepted")
	}
	for _, tc := range []struct {
		live bool
		at   time.Time
	}{{true, now}, {false, now.Add(-10 * time.Minute)}, {false, now.Add(2 * time.Minute)}} {
		b, h = signed(t, "invoice.paid", "evt_fixture", "in_fixture", tc.live, tc.at)
		if _, e = VerifyWebhook(b, h, "whsec_synthetic_fixture", now); e == nil {
			t.Fatal("invalid mode/clock accepted")
		}
	}
	b, h = signed(t, "payment_intent.succeeded", "evt_fixture", "pi_fixture", false, now)
	if _, e = VerifyWebhook(b, h, "whsec_synthetic_fixture", now); e == nil {
		t.Fatal("unexpected event accepted")
	}
}

// Uses actual official SDK request serialization and response decoding, talking
// only to a local fixture. This is not a hosted Stripe sandbox acceptance.
func TestOfficialSDKCustomerSetupAndManualUsageInvoice(t *testing.T) {
	calls := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if e := r.ParseForm(); e != nil {
			t.Error(e)
		}
		calls[r.Method+" "+r.URL.Path]++
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && r.Header.Get("Idempotency-Key") == "" {
			t.Error("missing stable idempotency key")
		}
		switch r.URL.Path {
		case "/v1/customers", "/v1/customers/cus_fixture":
			fmt.Fprint(w, `{"id":"cus_fixture","livemode":false}`)
		case "/v1/checkout/sessions":
			if r.Form.Get("mode") != "setup" || r.Form.Get("customer") != "cus_fixture" || r.Form.Get("line_items[0][price]") != "" {
				t.Error("checkout could charge")
			}
			fmt.Fprint(w, `{"id":"cs_test_fixture","livemode":false,"mode":"setup","customer":"cus_fixture","url":"https://checkout.stripe.com/c/pay/fixture"}`)
		case "/v1/checkout/sessions/cs_test_fixture":
			fmt.Fprint(w, `{"id":"cs_test_fixture","livemode":false,"mode":"setup","status":"complete","customer":"cus_fixture","setup_intent":{"id":"seti_fixture","customer":"cus_fixture","livemode":false,"status":"succeeded"}}`)
		case "/v1/invoiceitems":
			if r.Form.Get("invoice") != "in_fixture" || r.Form.Get("amount") != "12" {
				t.Error("invoice item is unbound or wrong")
			}
			fmt.Fprint(w, `{"id":"ii_fixture","livemode":false,"amount":12,"customer":"cus_fixture","invoice":"in_fixture"}`)
		case "/v1/invoices", "/v1/invoices/in_fixture", "/v1/invoices/in_fixture/finalize":
			total := 12
			status := "draft"
			if r.URL.Path == "/v1/invoices" {
				total = 0
				if r.Form.Get("auto_advance") != "false" || r.Form.Get("collection_method") != "send_invoice" || r.Form.Get("pending_invoice_items_behavior") != "exclude" {
					t.Error("automatic collection or unrelated items enabled")
				}
			}
			if strings.HasSuffix(r.URL.Path, "/finalize") {
				status = "open"
				if r.Form.Get("auto_advance") != "false" {
					t.Error("auto advance on finalization")
				}
			}
			fmt.Fprintf(w, `{"id":"in_fixture","livemode":false,"customer":"cus_fixture","collection_method":"send_invoice","auto_advance":false,"currency":"usd","status":%q,"total":%d,"metadata":{"agent_linux_job":"job_fixture"}}`, status, total)
		default:
			t.Error("unexpected Stripe endpoint", r.Method, r.URL.Path)
			w.WriteHeader(400)
		}
	}))
	defer server.Close()
	s, e := NewStripe("sk_test_"+strings.Repeat("x", 24), "http://127.0.0.1:8787/return")
	if e != nil {
		t.Fatal(e)
	}
	s.client = stripe.NewClient("sk_test_"+strings.Repeat("x", 24), stripe.WithBackends(stripe.NewBackendsWithConfig(&stripe.BackendConfig{URL: stripe.String(server.URL), HTTPClient: server.Client(), MaxNetworkRetries: stripe.Int64(0), LeveledLogger: &stripe.LeveledLogger{Level: stripe.LevelNull}})))
	ctx := context.Background()
	customer, e := s.Customer(ctx, "request-customer", "w1")
	if e != nil || customer != "cus_fixture" {
		t.Fatal(customer, e)
	}
	setup, e := s.Setup(ctx, customer, "request-setup", "w1")
	if e != nil {
		t.Fatal(e)
	}
	verified, e := s.CheckSetup(ctx, setup.ID, customer)
	if e != nil || !verified.Complete {
		t.Fatal(verified, e)
	}
	j := Job{ID: "job_fixture", Workspace: "w1", Customer: customer, AmountCents: 12}
	draft, e := s.Draft(ctx, j)
	if e != nil {
		t.Fatal(e)
	}
	j.Invoice = draft.ID
	j.Item, e = s.Item(ctx, j)
	if e != nil {
		t.Fatal(e)
	}
	invoice, e := s.Finalize(ctx, j)
	if e != nil || invoice.State != "open" {
		t.Fatal(invoice, e)
	}
	if calls["POST /v1/invoices/in_fixture/pay"] != 0 || calls["POST /v1/invoices/in_fixture/send"] != 0 {
		t.Fatal("payment/send called")
	}
}
func TestSignedOldAPIVersionRejectedAndPastDueInvoiceBlocked(t *testing.T) {
	now := time.Now()
	raw, _ := json.Marshal(map[string]any{"id": "evt_old", "object": "event", "type": "invoice.paid", "api_version": "2010-01-01", "livemode": false, "data": map[string]any{"object": map[string]any{"id": "in_fixture"}}})
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: raw, Secret: "whsec_synthetic_fixture", Timestamp: now})
	if _, e := VerifyWebhook(raw, signed.Header, "whsec_synthetic_fixture", now); e == nil {
		t.Fatal("wrong API version accepted")
	}
	j := Job{ID: "j", Customer: "cus_fixture", Invoice: "in_fixture", AmountCents: 1}
	invoice, e := invoiceView(&stripe.Invoice{ID: "in_fixture", Customer: &stripe.Customer{ID: "cus_fixture"}, CollectionMethod: "send_invoice", Currency: "usd", Metadata: map[string]string{"agent_linux_job": "j"}, Total: 1, AmountRemaining: 1, Status: "open", DueDate: now.Add(-time.Hour).Unix()}, j, true)
	if e != nil || !invoice.Failed {
		t.Fatal("past due invoice not blocked", invoice, e)
	}
}
