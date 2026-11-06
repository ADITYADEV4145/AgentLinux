// SPDX-License-Identifier: Apache-2.0
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
)

const StripeAPIVersion = stripe.APIVersion

var ErrPayment = errors.New("sandbox payment provider unavailable or incompatible; reconcile the existing intent, never recreate blindly")

type Stripe struct {
	client    *stripe.Client
	returnURL string
}
type quietTransport struct{ base http.RoundTripper }

func (t quietTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	v, e := t.base.RoundTrip(r)
	if v != nil {
		v.Header.Del("Stripe-Notice")
		v.Body = &limitedBody{ReadCloser: v.Body, remaining: 2 << 20}
	}
	return v, e
}

type limitedBody struct {
	io.ReadCloser
	remaining int64
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.remaining <= 0 {
		return 0, errors.New("oversized sandbox payment response")
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, e := b.ReadCloser.Read(p)
	b.remaining -= int64(n)
	return n, e
}
func NewStripe(key, returnURL string) (*Stripe, error) {
	if !strings.HasPrefix(key, "sk_test_") || len(key) < 16 || strings.ContainsAny(key, "\r\n ") {
		return nil, errors.New("only a server-side sk_test_ key is accepted; live/restricted keys are disabled")
	}
	u, e := url.Parse(returnURL)
	if e != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Hostname() == "" || (u.Scheme != "https" && !(u.Scheme == "http" && u.Hostname() == "127.0.0.1")) {
		return nil, errors.New("configure an operator-owned HTTPS return URL or literal-loopback development URL")
	}
	transport := &http.Transport{DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext, ResponseHeaderTimeout: 5 * time.Second, MaxIdleConnsPerHost: 2, IdleConnTimeout: 30 * time.Second}
	httpClient := &http.Client{Timeout: 8 * time.Second, Transport: quietTransport{transport}, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrPayment }}
	backends := stripe.NewBackendsWithConfig(&stripe.BackendConfig{HTTPClient: httpClient, MaxNetworkRetries: stripe.Int64(0), LeveledLogger: &stripe.LeveledLogger{Level: stripe.LevelNull}, EnableTelemetry: stripe.Bool(false)})
	return &Stripe{client: stripe.NewClient(key, stripe.WithBackends(backends)), returnURL: returnURL}, nil
}
func safeURL(raw, host string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.Hostname() == host && u.User == nil
}
func (s *Stripe) Customer(ctx context.Context, key, w string) (string, error) {
	p := &stripe.CustomerCreateParams{Metadata: map[string]string{"agent_linux_workspace": w, "agent_linux_mode": "sandbox"}}
	p.SetIdempotencyKey(key)
	c, e := s.client.V1Customers.Create(ctx, p)
	if e != nil || c == nil || c.ID == "" || c.Livemode || c.Deleted {
		return "", ErrPayment
	}
	return c.ID, nil
}
func (s *Stripe) Setup(ctx context.Context, customer, key, w string) (Setup, error) {
	p := &stripe.CheckoutSessionCreateParams{Mode: stripe.String("setup"), Customer: stripe.String(customer), Currency: stripe.String("usd"), PaymentMethodTypes: stripe.StringSlice([]string{"card"}), SuccessURL: stripe.String(s.returnURL), CancelURL: stripe.String(s.returnURL), ClientReferenceID: stripe.String(w)}
	p.SetIdempotencyKey(key)
	c, e := s.client.V1CheckoutSessions.Create(ctx, p)
	if e != nil || c == nil || c.ID == "" || c.Livemode || c.Mode != "setup" || c.Customer == nil || c.Customer.ID != customer || !safeURL(c.URL, "checkout.stripe.com") {
		return Setup{}, ErrPayment
	}
	return Setup{ID: c.ID, URL: c.URL}, nil
}
func (s *Stripe) CheckSetup(ctx context.Context, id, customer string) (Setup, error) {
	current, err := s.client.V1Customers.Retrieve(ctx, customer, nil)
	if err != nil || current == nil || current.ID != customer || current.Deleted || current.Livemode {
		return Setup{}, ErrPayment
	}
	p := &stripe.CheckoutSessionRetrieveParams{}
	p.AddExpand("setup_intent")
	c, e := s.client.V1CheckoutSessions.Retrieve(ctx, id, p)
	if e != nil || c == nil || c.ID != id || c.Livemode || c.Mode != "setup" || c.Customer == nil || c.Customer.ID != customer {
		return Setup{}, ErrPayment
	}
	complete := false
	if c.Status == "complete" {
		if c.SetupIntent == nil || c.SetupIntent.Customer == nil || c.SetupIntent.Customer.ID != customer || c.SetupIntent.Livemode || c.SetupIntent.Status != "succeeded" {
			return Setup{}, ErrPayment
		}
		complete = true
	}
	return Setup{ID: c.ID, Complete: complete, Expired: c.Status == "expired"}, nil
}
func invoiceView(c *stripe.Invoice, j Job, checkAmount bool) (Invoice, error) {
	if c == nil || c.ID == "" || c.Livemode || c.Customer == nil || c.Customer.ID != j.Customer || c.CollectionMethod != "send_invoice" || c.AutoAdvance || c.Currency != "usd" || c.Metadata["agent_linux_job"] != j.ID {
		return Invoice{}, ErrPayment
	}
	if j.Invoice != "" && c.ID != j.Invoice {
		return Invoice{}, ErrPayment
	}
	if checkAmount && c.Total != j.AmountCents {
		return Invoice{}, ErrPayment
	}
	if c.HostedInvoiceURL != "" && !safeURL(c.HostedInvoiceURL, "invoice.stripe.com") {
		return Invoice{}, ErrPayment
	}
	failed := c.Status == "uncollectible" || (c.Status == "open" && c.AmountRemaining > 0 && c.DueDate > 0 && c.DueDate < time.Now().Unix()) || (c.AttemptCount > 0 && c.AmountRemaining > 0 && c.Status != "paid" && c.Status != "void")
	return Invoice{ID: c.ID, State: string(c.Status), URL: c.HostedInvoiceURL, AmountCents: c.Total, Failed: failed}, nil
}
func (s *Stripe) Draft(ctx context.Context, j Job) (Invoice, error) {
	p := &stripe.InvoiceCreateParams{Customer: stripe.String(j.Customer), Currency: stripe.String("usd"), AutoAdvance: stripe.Bool(false), CollectionMethod: stripe.String("send_invoice"), DaysUntilDue: stripe.Int64(7), PendingInvoiceItemsBehavior: stripe.String("exclude"), Metadata: map[string]string{"agent_linux_job": j.ID, "agent_linux_workspace": j.Workspace, "agent_linux_mode": "sandbox"}}
	p.SetIdempotencyKey(j.ID + "-invoice")
	c, e := s.client.V1Invoices.Create(ctx, p)
	if e != nil {
		return Invoice{}, ErrPayment
	}
	return invoiceView(c, j, false)
}
func (s *Stripe) Item(ctx context.Context, j Job) (string, error) {
	p := &stripe.InvoiceItemCreateParams{Customer: stripe.String(j.Customer), Invoice: stripe.String(j.Invoice), Amount: stripe.Int64(j.AmountCents), Currency: stripe.String("usd"), Description: stripe.String("Agent Linux finalized desktop usage (SANDBOX; not finalized retail pricing)"), Discountable: stripe.Bool(false)}
	p.SetIdempotencyKey(j.ID + "-item")
	c, e := s.client.V1InvoiceItems.Create(ctx, p)
	if e != nil || c == nil || c.ID == "" || c.Livemode || c.Amount != j.AmountCents || c.Customer == nil || c.Customer.ID != j.Customer || c.Invoice == nil || c.Invoice.ID != j.Invoice {
		return "", ErrPayment
	}
	return c.ID, nil
}
func (s *Stripe) Finalize(ctx context.Context, j Job) (Invoice, error) {
	// Retrieve before finalizing to refuse any out-of-band line/tax changes.
	before, e := s.CheckInvoice(ctx, j)
	if e != nil {
		return Invoice{}, e
	}
	if before.State != "draft" {
		return before, nil
	}
	p := &stripe.InvoiceFinalizeInvoiceParams{AutoAdvance: stripe.Bool(false)}
	p.SetIdempotencyKey(j.ID + "-finalize")
	c, e := s.client.V1Invoices.FinalizeInvoice(ctx, j.Invoice, p)
	if e != nil {
		return Invoice{}, ErrPayment
	}
	return invoiceView(c, j, true)
}
func (s *Stripe) CheckInvoice(ctx context.Context, j Job) (Invoice, error) {
	c, e := s.client.V1Invoices.Retrieve(ctx, j.Invoice, nil)
	if e != nil {
		return Invoice{}, ErrPayment
	}
	return invoiceView(c, j, true)
}
func (s *Stripe) Portal(ctx context.Context, customer, key string) (string, error) {
	p := &stripe.BillingPortalSessionCreateParams{Customer: stripe.String(customer), ReturnURL: stripe.String(s.returnURL)}
	p.SetIdempotencyKey(key)
	c, e := s.client.V1BillingPortalSessions.Create(ctx, p)
	if e != nil || c == nil || !safeURL(c.URL, "billing.stripe.com") {
		return "", ErrPayment
	}
	return c.URL, nil
}
func VerifyWebhook(raw []byte, signature, secret string, now time.Time) (Receipt, error) {
	if !strings.HasPrefix(secret, "whsec_") || len(secret) < 16 || len(raw) > 256<<10 {
		return Receipt{}, errors.New("invalid webhook configuration/body")
	}
	event, e := webhook.ConstructEvent(raw, signature, secret)
	if e != nil || event.APIVersion != stripe.APIVersion || event.Livemode || (event.ID == "" || len(event.ID) > 128) || event.Account != "" {
		return Receipt{}, errors.New("webhook signature, mode or API version rejected")
	}
	// SDK tolerance rejects old signatures; also reject future signature clocks.
	for _, part := range strings.Split(signature, ",") {
		if strings.HasPrefix(part, "t=") {
			t, e := strconv.ParseInt(strings.TrimPrefix(part, "t="), 10, 64)
			if e != nil || time.Unix(t, 0).After(now.Add(time.Minute)) {
				return Receipt{}, errors.New("future webhook signature timestamp")
			}
		}
	}
	allowed := map[string]bool{"checkout.session.completed": true, "checkout.session.expired": true, "invoice.paid": true, "invoice.payment_failed": true, "invoice.voided": true, "invoice.marked_uncollectible": true, "customer.deleted": true}
	if !allowed[string(event.Type)] {
		return Receipt{}, errors.New("webhook event type not subscribed")
	}
	var object struct {
		ID       string `json:"id"`
		Livemode bool   `json:"livemode"`
	}
	if event.Data == nil || json.Unmarshal(event.Data.Raw, &object) != nil || (object.ID == "" || len(object.ID) > 128) || object.Livemode {
		return Receipt{}, errors.New("invalid sandbox webhook object")
	}
	return Receipt{ID: event.ID, Type: string(event.Type), Object: object.ID, Received: now, Status: "pending"}, nil
}

var _ Payments = (*Stripe)(nil)
