// SPDX-License-Identifier: Apache-2.0
package billing

import (
	"context"
	"errors"
	"sort"
	"time"
)

type Setup struct {
	ID, URL           string
	Complete, Expired bool
}
type Invoice struct {
	ID, State, URL string
	AmountCents    int64
	Failed         bool
}

// Payments is implemented by the official Stripe SDK adapter. Test doubles must
// be named as such; they are not evidence of a Stripe account checkout.
type Payments interface {
	Customer(context.Context, string, string) (string, error)
	Setup(context.Context, string, string, string) (Setup, error)
	CheckSetup(context.Context, string, string) (Setup, error)
	Draft(context.Context, Job) (Invoice, error)
	Item(context.Context, Job) (string, error)
	Finalize(context.Context, Job) (Invoice, error)
	CheckInvoice(context.Context, Job) (Invoice, error)
	Portal(context.Context, string, string) (string, error)
}

func (d *Database) Queue(w, key, kind string, finalize bool, now time.Time) (Job, error) {
	if !label.MatchString(key) || !(kind == "setup" || kind == "portal" || kind == "invoice") {
		return Job{}, errors.New("invalid billing request ID/action")
	}
	id := w + "-" + key
	if old, ok := d.Jobs[id]; ok {
		if old.Kind != kind || old.Finalize != finalize {
			return old, errors.New("billing idempotency conflict")
		}
		return old, nil
	}
	a, ok := d.Accounts[w]
	if !ok {
		return Job{}, errors.New("unknown workspace")
	}
	if len(d.Jobs) >= 256 {
		return Job{}, errors.New("billing job retention limit reached")
	}
	if a.Canceled || a.Suspended {
		return Job{}, errors.New("account canceled/suspended")
	}
	for _, j := range d.Jobs {
		if j.Workspace == w && (j.Status == "queued" || j.Status == "retry" || j.Status == "review") {
			return Job{}, errors.New("finish or reconcile the existing workspace payment intent first")
		}
	}
	j := Job{ID: id, Workspace: w, Kind: kind, Created: now, Status: "queued", Customer: a.Customer, Finalize: finalize}
	if kind != "setup" && a.Customer == "" {
		return j, errors.New("associate a sandbox customer through setup first")
	}
	if kind == "invoice" {
		selected := map[string]bool{}
		var alreadyCents int64
		for _, old := range d.Jobs {
			if old.Workspace == w && old.Kind == "invoice" {
				for _, id := range old.Entries {
					selected[id] = true
				}
				alreadyCents += old.AmountCents
			}
		}
		var allMicros, unbilled int64
		for _, e := range d.Ledger {
			if e.Workspace == w && e.Status == "finalized" {
				allMicros += e.NetMicros
				if !selected[e.ID] {
					j.Entries = append(j.Entries, e.ID)
					unbilled += e.NetMicros
				}
			}
		}
		// Round the lifetime net total down once to cents. Fractional cents and
		// negative credits carry forward; retry/each session cannot inflate rounding.
		j.AmountCents = max(0, allMicros/MicrosPerCent) - alreadyCents
		if len(j.Entries) == 0 || j.AmountCents <= 0 || unbilled <= 0 {
			return j, errors.New("no positive whole-cent finalized usage available; fractional cents/credits carry forward")
		}
	}
	d.Jobs[id] = j
	return j, nil
}

// Step advances one durable intent step. The caller commits after every call.
// A retry older than Stripe's idempotency retention window is held for operator
// reconciliation rather than risking duplicate customers/items/invoices.
func (d *Database) Step(ctx context.Context, id string, p Payments, now time.Time) error {
	j, ok := d.Jobs[id]
	if !ok || !(j.Status == "queued" || j.Status == "retry") {
		return nil
	}
	if now.Before(j.Created) || now.Sub(j.Created) > 23*time.Hour {
		j.Status = "review"
		j.Error = "IDEMPOTENCY_WINDOW_REVIEW"
		d.Jobs[id] = j
		return nil
	}
	if p == nil {
		j.Status = "retry"
		j.Error = "PAYMENTS_UNAVAILABLE"
		d.Jobs[id] = j
		return errors.New("sandbox payments unavailable")
	}
	a := d.Accounts[j.Workspace]
	var e error
	if a.Canceled || a.Suspended {
		j.Status = "review"
		j.Error = "ACCOUNT_BLOCKED"
		d.Jobs[id] = j
		return nil
	}
	switch {
	case j.Customer == "":
		j.Customer, e = p.Customer(ctx, j.ID+"-customer", j.Workspace)
		if e == nil {
			a.Customer = j.Customer
			d.Accounts[j.Workspace] = a
		}
	case j.Kind == "setup":
		var s Setup
		s, e = p.Setup(ctx, j.Customer, j.ID+"-setup", j.Workspace)
		if e == nil {
			j.Checkout = s.ID
			j.URL = s.URL
			j.Status = "done"
			a.Checkout = s.ID
			d.Accounts[j.Workspace] = a
		}
	case j.Kind == "portal":
		j.URL, e = p.Portal(ctx, j.Customer, j.ID+"-portal")
		if e == nil {
			j.Status = "done"
		}
	case j.Invoice == "":
		var i Invoice
		i, e = p.Draft(ctx, j)
		if e == nil {
			j.Invoice = i.ID
			j.InvoiceState = i.State
		}
	case j.Item == "":
		j.Item, e = p.Item(ctx, j)
	case j.Finalize:
		var i Invoice
		i, e = p.Finalize(ctx, j)
		if e == nil {
			j.Status = "done"
			j.InvoiceState = i.State
			j.URL = i.URL
		}
	default:
		var i Invoice
		i, e = p.CheckInvoice(ctx, j)
		if e == nil {
			j.Status = "done"
			j.InvoiceState = i.State
			j.URL = i.URL
		}
	}
	if e != nil {
		j.Status = "retry"
		j.Error = "PAYMENTS_UNAVAILABLE"
	} else {
		j.Error = ""
		if j.Status == "retry" {
			j.Status = "queued"
		}
	}
	d.Jobs[id] = j
	return e
}
func (d *Database) PendingJobs() []string {
	ids := []string{}
	for id, j := range d.Jobs {
		if j.Status == "queued" || j.Status == "retry" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
func (d *Database) Refresh(ctx context.Context, p Payments, now time.Time) error {
	if p == nil {
		return errors.New("sandbox payment SDK unavailable")
	}
	var first error
	for w, a := range d.Accounts {
		if a.Checkout == "" || a.Canceled || a.Suspended {
			continue
		}
		setup, e := p.CheckSetup(ctx, a.Checkout, a.Customer)
		healthy := e == nil
		if e != nil && first == nil {
			first = e
		}
		for id, j := range d.Jobs {
			if j.Workspace != w || j.Kind != "invoice" || j.Invoice == "" || j.Status != "done" {
				continue
			}
			invoice, err := p.CheckInvoice(ctx, j)
			if err != nil {
				healthy = false
				if first == nil {
					first = err
				}
				continue
			}
			j.InvoiceState = invoice.State
			j.PaymentFailed = invoice.Failed
			j.URL = invoice.URL
			d.Jobs[id] = j
		}
		if healthy {
			a.PaymentReady = setup.Complete
			a.PaymentCheckedAt = now
			d.Accounts[w] = a
		}
	}
	return first
}
func (d *Database) Receive(r Receipt) error {
	if old, ok := d.Receipts[r.ID]; ok {
		if old.Type != r.Type || old.Object != r.Object {
			return errors.New("event id conflict")
		}
		return nil
	}
	if len(d.Receipts) >= 1024 {
		return errors.New("webhook inbox full; delivery must retry")
	}
	d.Receipts[r.ID] = r
	return nil
}

// Webhooks identify work; authoritative current objects are retrieved to avoid
// regressing state on duplicate, delayed, or out-of-order snapshot events.
func (d *Database) ProcessReceipt(ctx context.Context, id string, p Payments, now time.Time) error {
	r, ok := d.Receipts[id]
	if !ok || r.Status == "done" {
		return nil
	}
	if p == nil {
		return errors.New("sandbox payments unavailable")
	}
	matched := false
	switch r.Type {
	case "checkout.session.completed", "checkout.session.expired":
		for w, a := range d.Accounts {
			if a.Checkout != r.Object {
				continue
			}
			matched = true
			s, e := p.CheckSetup(ctx, r.Object, a.Customer)
			if e != nil {
				return e
			}
			a.PaymentReady = s.Complete
			a.PaymentCheckedAt = now
			d.Accounts[w] = a
		}
	case "invoice.paid", "invoice.payment_failed", "invoice.voided", "invoice.marked_uncollectible":
		for id, j := range d.Jobs {
			if j.Invoice != r.Object {
				continue
			}
			matched = true
			i, e := p.CheckInvoice(ctx, j)
			if e != nil {
				return e
			}
			j.InvoiceState = i.State
			j.PaymentFailed = i.Failed
			j.URL = i.URL
			d.Jobs[id] = j
		}
	case "customer.deleted":
		for w, a := range d.Accounts {
			if a.Customer == r.Object {
				matched = true
				a.PaymentReady = false
				a.Suspended = true
				d.Accounts[w] = a
			}
		}
	}
	// Unknown objects can precede a lost SDK reply; do not drop them. The worker
	// retries after recovering its durable intent, with a bounded inbox.
	if !matched {
		r.Error = "ASSOCIATION_PENDING"
		d.Receipts[id] = r
		return nil
	}
	r.Status = "done"
	r.Error = ""
	d.Receipts[id] = r
	return nil
}
