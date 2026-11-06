// SPDX-License-Identifier: Apache-2.0
package billing

import (
	"encoding/json"
	"testing"
	"time"
)

var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func fixture(t *testing.T, trial int64) *Database {
	t.Helper()
	d, e := New(Config{Version: Version, Mode: "sandbox", Currency: "usd", Rates: []Rate{{"rate-v1", epoch, 3600000, 2, 4, 24}}, Policy: Policy{trial, 1_000_000, 1, 900}}, []string{"w1", "w2"})
	if e != nil {
		t.Fatal(e)
	}
	for w, a := range d.Accounts {
		a.Customer = "cus_" + w
		a.Checkout = "cs_test_" + w
		a.PaymentReady = true
		a.PaymentCheckedAt = epoch
		d.Accounts[w] = a
	}
	return d
}
func TestMeterRoundsOnceRetainsHistoryAndRestart(t *testing.T) {
	d := fixture(t, 0)
	if e := d.Reserve("alice", "w1", "machine", epoch, epoch.Add(60*time.Second)); e != nil {
		t.Fatal(e)
	}
	if e := d.Observe("machine", epoch, false, epoch.Add(10100*time.Millisecond)); e != nil {
		t.Fatal(e)
	}
	if d.Reservations["machine"].HighWaterSeconds != 11 {
		t.Fatal(d.Reservations)
	}
	if e := d.Observe("machine", epoch, true, epoch.Add(10100*time.Millisecond)); e != nil {
		t.Fatal(e)
	}
	if e := d.Observe("machine", epoch, true, epoch.Add(20*time.Second)); e != nil {
		t.Fatal(e)
	}
	if len(d.Ledger) != 1 || d.Ledger[0].QuantitySeconds != 11 || d.Ledger[0].NetMicros != 11000 {
		t.Fatal(d.Ledger)
	}
	raw, _ := json.Marshal(d)
	var restored Database
	if json.Unmarshal(raw, &restored) != nil || restored.Validate() != nil {
		t.Fatal("restart invalid")
	}
	j, e := restored.Queue("w1", "invoice1", "invoice", false, epoch.Add(time.Minute))
	if e != nil || j.AmountCents != 1 {
		t.Fatal(j, e)
	}
	duplicate, e := restored.Queue("w1", "invoice1", "invoice", false, epoch.Add(2*time.Minute))
	if e != nil || duplicate.ID != j.ID {
		t.Fatal(duplicate, e)
	}
	_, e = restored.Queue("w1", "invoice1", "setup", false, epoch)
	if e == nil {
		t.Fatal("conflicting retry accepted")
	}
	if Cost(1, 1) != 1 || Cost(3600, 3600000) != 3600000 {
		t.Fatal("cost rounding")
	}
}
func TestWorkspaceReservationTrialConcurrentCapsAndFailedProvision(t *testing.T) {
	d := fixture(t, 60000)
	a := d.Accounts["w1"]
	a.PaymentReady = false
	a.Policy.SpendingCapMicros = 0
	d.Accounts["w1"] = a
	if e := d.Reserve("alice", "w1", "a", epoch, epoch.Add(time.Minute)); e != nil {
		t.Fatal(e)
	}
	if e := d.Reserve("other-user", "w1", "duplicate", epoch, epoch.Add(time.Minute)); e == nil {
		t.Fatal("workspace concurrency bypass")
	}
	if e := d.Reserve("bob", "w2", "b", epoch, epoch.Add(time.Minute)); e != nil {
		t.Fatal(e)
	}
	if e := d.Observe("a", time.Time{}, true, epoch.Add(10*time.Second)); e != nil {
		t.Fatal(e)
	}
	final, _, _, trial := d.Totals("w1")
	if final != 0 || trial != 60000 || d.Ledger[0].QuantitySeconds != 0 {
		t.Fatal(final, trial, d.Ledger)
	}
	if e := d.Reserve("alice", "w1", "c", epoch.Add(11*time.Second), epoch.Add(71*time.Second)); e != nil {
		t.Fatal("unused trial was not released", e)
	}
	if e := d.Observe("c", epoch.Add(11*time.Second), true, epoch.Add(71*time.Second)); e != nil {
		t.Fatal(e)
	}
	if e := d.Reserve("alice", "w1", "d", epoch.Add(72*time.Second), epoch.Add(132*time.Second)); e == nil {
		t.Fatal("exhausted trial admitted unpaid usage")
	}
}
func TestClockReviewAdjustmentAndOutOfOrderObservations(t *testing.T) {
	d := fixture(t, 0)
	_ = d.Reserve("alice", "w1", "a", epoch, epoch.Add(time.Minute))
	_ = d.Observe("a", epoch, false, epoch.Add(20*time.Second))
	_ = d.Observe("a", epoch, false, epoch.Add(10*time.Second))
	if d.Reservations["a"].HighWaterSeconds != 20 || !d.ShouldStop("a", epoch.Add(10*time.Second)) {
		t.Fatal("rollback reduced usage or did not fail closed")
	}
	_ = d.Observe("a", epoch, true, epoch.Add(15*time.Second))
	if d.Ledger[0].Status != "review" || d.Ledger[0].NetMicros != 0 {
		t.Fatal(d.Ledger)
	}
	_, e := d.Queue("w1", "inv", "invoice", false, epoch.Add(time.Minute))
	if e == nil {
		t.Fatal("review was invoiceable")
	}
	seconds := int64(18)
	if e = d.Adjustment("correction", "usage-a", "clock_resolution", 0, &seconds, epoch.Add(time.Minute)); e != nil {
		t.Fatal(e)
	}
	if e = d.Adjustment("correction", "usage-a", "clock_resolution", 0, &seconds, epoch.Add(2*time.Minute)); e != nil {
		t.Fatal(e)
	}
	if len(d.Ledger) != 2 || d.Ledger[1].NetMicros != 18000 {
		t.Fatal(d.Ledger)
	}
	if e = d.Adjustment("credit", "usage-a", "service_credit", -8000, nil, epoch.Add(time.Minute)); e != nil {
		t.Fatal(e)
	}
	j, e := d.Queue("w1", "inv2", "invoice", false, epoch.Add(time.Minute))
	if e != nil || j.AmountCents != 1 {
		t.Fatal(j, e)
	}
}
func TestImmutableRatesAndEffectiveVersionPinning(t *testing.T) {
	d := fixture(t, 0)
	_ = d.Reserve("alice", "w1", "a", epoch, epoch.Add(time.Minute))
	cfg := d.Config
	cfg.Rates = append(append([]Rate{}, cfg.Rates...), Rate{"rate-v2", epoch.Add(time.Hour), 7200000, 2, 4, 24})
	if e := d.UpdateRates(cfg); e != nil {
		t.Fatal(e)
	}
	if d.Reservations["a"].RateID != "rate-v1" {
		t.Fatal("retroactive rate mutation")
	}
	rate, e := d.RateAt(epoch.Add(time.Hour))
	if e != nil || rate.ID != "rate-v2" {
		t.Fatal(rate, e)
	}
	cfg.Rates[0].MicrosPerHour++
	if e = d.UpdateRates(cfg); e == nil {
		t.Fatal("mutated rate accepted")
	}
	cfg.Mode = "live"
	if cfg.Validate() == nil {
		t.Fatal("live billing enabled")
	}
}
func TestFractionalCentsCarryAcrossExplicitInvoices(t *testing.T) {
	d := fixture(t, 0)
	_ = d.Reserve("alice", "w1", "a", epoch, epoch.Add(time.Minute))
	_ = d.Observe("a", epoch, true, epoch.Add(11*time.Second))
	j, e := d.Queue("w1", "first", "invoice", false, epoch.Add(12*time.Second))
	if e != nil || j.AmountCents != 1 {
		t.Fatal(j, e)
	}
	j.Status = "done"
	d.Jobs[j.ID] = j
	_ = d.Reserve("alice", "w1", "b", epoch.Add(12*time.Second), epoch.Add(72*time.Second))
	_ = d.Observe("b", epoch.Add(12*time.Second), true, epoch.Add(21*time.Second))
	second, e := d.Queue("w1", "second", "invoice", false, epoch.Add(22*time.Second))
	if e != nil || second.AmountCents != 1 || len(second.Entries) != 1 || second.Entries[0] != "usage-b" {
		t.Fatal("fractional cent lost or previously bound entry invoiced again", second, e)
	}
}
