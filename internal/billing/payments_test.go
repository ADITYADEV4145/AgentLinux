// SPDX-License-Identifier: Apache-2.0
package billing

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakePayments struct {
	fail    bool
	paid    bool
	setup   bool
	creates int
}

func (p *fakePayments) Customer(context.Context, string, string) (string, error) {
	p.creates++
	return "cus_fixture", nil
}
func (p *fakePayments) Setup(context.Context, string, string, string) (Setup, error) {
	if p.fail {
		return Setup{}, ErrPayment
	}
	return Setup{ID: "cs_test_fixture", URL: "https://checkout.stripe.com/fixture"}, nil
}
func (p *fakePayments) CheckSetup(context.Context, string, string) (Setup, error) {
	if p.fail {
		return Setup{}, ErrPayment
	}
	return Setup{Complete: p.setup}, nil
}
func (p *fakePayments) Draft(context.Context, Job) (Invoice, error) {
	return Invoice{ID: "in_fixture", State: "draft"}, nil
}
func (p *fakePayments) Item(context.Context, Job) (string, error) { return "ii_fixture", nil }
func (p *fakePayments) Finalize(context.Context, Job) (Invoice, error) {
	return Invoice{ID: "in_fixture", State: "open"}, nil
}
func (p *fakePayments) CheckInvoice(context.Context, Job) (Invoice, error) {
	if p.fail {
		return Invoice{}, errors.New("provider down")
	}
	if p.paid {
		return Invoice{ID: "in_fixture", State: "paid"}, nil
	}
	return Invoice{ID: "in_fixture", State: "open", Failed: true}, nil
}
func (p *fakePayments) Portal(context.Context, string, string) (string, error) {
	return "https://billing.stripe.com/fixture", nil
}
func TestDurablePaymentIntentRetryAndOutOfOrderReceipts(t *testing.T) {
	d := fixture(t, 0)
	p := &fakePayments{setup: true}
	a := d.Accounts["w1"]
	a.Customer = ""
	d.Accounts["w1"] = a
	j, e := d.Queue("w1", "setup1", "setup", false, epoch)
	if e != nil {
		t.Fatal(e)
	}
	if e = d.Step(context.Background(), j.ID, p, epoch); e != nil {
		t.Fatal(e)
	}
	if p.creates != 1 || d.Accounts["w1"].Customer != "cus_fixture" {
		t.Fatal("customer association missing")
	}
	p.fail = true
	if d.Step(context.Background(), j.ID, p, epoch) == nil {
		t.Fatal("outage hidden")
	}
	p.fail = false
	if e = d.Step(context.Background(), j.ID, p, epoch); e != nil {
		t.Fatal(e)
	}
	if p.creates != 1 || d.Jobs[j.ID].Status != "done" {
		t.Fatal("customer duplicated", d.Jobs)
	}
	d.Jobs["invoice-job"] = Job{ID: "invoice-job", Workspace: "w1", Kind: "invoice", Invoice: "in_fixture", Status: "done"}
	for _, typ := range []string{"invoice.paid", "invoice.payment_failed"} {
		r := Receipt{ID: typ, Type: typ, Object: "in_fixture", Status: "pending"}
		_ = d.Receive(r)
		p.paid = true
		if e = d.ProcessReceipt(context.Background(), r.ID, p, epoch); e != nil {
			t.Fatal(e)
		}
	}
	if d.Jobs["invoice-job"].PaymentFailed || d.Jobs["invoice-job"].InvoiceState != "paid" {
		t.Fatal("old failed event regressed current paid state")
	}
	p.paid = false
	_ = d.Refresh(context.Background(), p, epoch.Add(time.Minute))
	if !d.blockedAccount("w1", epoch) {
		t.Fatal("payment failure did not block account")
	}
	last := d.Accounts["w1"].PaymentCheckedAt
	p.fail = true
	_ = d.Refresh(context.Background(), p, epoch.Add(3*time.Minute))
	if d.Accounts["w1"].PaymentCheckedAt != last {
		t.Fatal("outage refreshed payment health")
	}
}
func TestIdempotencyRetentionAndUnknownWebhookRecovery(t *testing.T) {
	d := fixture(t, 0)
	p := &fakePayments{}
	j, _ := d.Queue("w1", "old", "setup", false, epoch)
	_ = d.Step(context.Background(), j.ID, p, epoch.Add(24*time.Hour))
	if d.Jobs[j.ID].Status != "review" || p.creates != 0 {
		t.Fatal("old Stripe mutation replayed")
	}
	r := Receipt{ID: "evt_lost", Type: "checkout.session.completed", Object: "cs_test_lost", Status: "pending"}
	_ = d.Receive(r)
	_ = d.ProcessReceipt(context.Background(), r.ID, p, epoch)
	if d.Receipts[r.ID].Status != "pending" {
		t.Fatal("early webhook dropped")
	}
	a := d.Accounts["w2"]
	a.Checkout = "cs_test_lost"
	d.Accounts["w2"] = a
	p.setup = true
	_ = d.ProcessReceipt(context.Background(), r.ID, p, epoch)
	if d.Receipts[r.ID].Status != "done" || !d.Accounts["w2"].PaymentReady {
		t.Fatal("webhook association recovery failed")
	}
}
