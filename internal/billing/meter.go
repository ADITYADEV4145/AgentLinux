// SPDX-License-Identifier: Apache-2.0
package billing

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"regexp"
	"sort"
	"time"
)

var label = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)

func (c Config) Validate() error {
	if c.Version != Version || c.Mode != "sandbox" || c.Currency != "usd" || len(c.Rates) == 0 || len(c.Rates) > 16 {
		return errors.New("billing requires billing-sandbox-1, sandbox, usd and 1..16 versioned rates; live mode is not implemented")
	}
	if e := c.Policy.Validate(); e != nil {
		return e
	}
	ids := map[string]bool{}
	times := map[int64]bool{}
	for _, r := range c.Rates {
		if !label.MatchString(r.ID) || ids[r.ID] || times[r.EffectiveAt.UnixNano()] || r.EffectiveAt.IsZero() || r.MicrosPerHour < 1 || r.MicrosPerHour > 100_000_000 || r.CPUs < 2 || r.CPUs > 4 || r.MemoryGiB < 4 || r.MemoryGiB > 8 || r.DiskGiB < 8 || r.DiskGiB > 40 {
			return errors.New("invalid rate/version/effective date or bounded desktop resource profile")
		}
		ids[r.ID] = true
		times[r.EffectiveAt.UnixNano()] = true
	}
	return nil
}
func (p Policy) Validate() error {
	if p.TrialMicros < 0 || p.TrialMicros > 100_000_000 || p.SpendingCapMicros < 0 || p.SpendingCapMicros > 100_000_000 || p.Concurrency < 1 || p.Concurrency > 4 || p.MaxSeconds < 30 || p.MaxSeconds > 900 {
		return errors.New("invalid sandbox trial/spending/concurrency/duration limits")
	}
	return nil
}
func New(c Config, workspaces []string) (*Database, error) {
	if e := c.Validate(); e != nil {
		return nil, e
	}
	c.Rates = append([]Rate(nil), c.Rates...)
	d := &Database{Config: c, Accounts: map[string]Account{}, Reservations: map[string]Reservation{}, Jobs: map[string]Job{}, Receipts: map[string]Receipt{}}
	for _, w := range workspaces {
		d.Accounts[w] = Account{Workspace: w, Policy: c.Policy}
	}
	return d, nil
}
func (d *Database) Validate() error {
	if e := d.Config.Validate(); e != nil {
		return e
	}
	if len(d.Changes) > 256 || len(d.Accounts) > 32 || len(d.Reservations) > 128 || len(d.Ledger) > 512 || len(d.Jobs) > 256 || len(d.Receipts) > 1024 || d.Accounts == nil || d.Reservations == nil || d.Jobs == nil || d.Receipts == nil {
		return errors.New("invalid or full bounded billing store")
	}
	for k, a := range d.Accounts {
		if a.Workspace != k {
			return errors.New("invalid account ownership")
		}
		if e := a.Policy.Validate(); e != nil {
			return e
		}
	}
	seen := map[string]bool{}
	for _, e := range d.Ledger {
		if seen[e.ID] || e.ID == "" || e.Resource != "desktop-second" || d.Accounts[e.Workspace].Workspace == "" || e.NetMicros != e.CostMicros-e.TrialMicros {
			return errors.New("corrupt usage ledger")
		}
		seen[e.ID] = true
	}
	selected := map[string]bool{}
	for id, j := range d.Jobs {
		if id != j.ID || d.Accounts[j.Workspace].Workspace == "" || (j.Kind != "setup" && j.Kind != "portal" && j.Kind != "invoice") || (j.Status != "queued" && j.Status != "retry" && j.Status != "done" && j.Status != "review") || j.AmountCents < 0 {
			return errors.New("corrupt payment intent")
		}
		if j.Kind == "invoice" {
			if j.AmountCents == 0 || len(j.Entries) == 0 {
				return errors.New("unbound invoice intent")
			}
			for _, ref := range j.Entries {
				if selected[ref] {
					return errors.New("overlapping invoice intents")
				}
				selected[ref] = true
				found := false
				for _, entry := range d.Ledger {
					if entry.ID == ref && entry.Workspace == j.Workspace && entry.Status == "finalized" {
						found = true
					}
				}
				if !found {
					return errors.New("invoice references missing/foreign/unfinalized usage")
				}
			}
		}
	}
	for id, r := range d.Receipts {
		if id != r.ID || len(id) > 128 || len(r.Object) > 128 || r.ID == "" || r.Object == "" || (r.Status != "pending" && r.Status != "done") {
			return errors.New("corrupt webhook receipt")
		}
	}
	for id, r := range d.Reservations {
		if id != r.ID || r.HighWaterSeconds < 0 || r.TrialMicros < 0 || r.ReservedMicros < 0 || r.Expires.Before(r.Created) || d.Accounts[r.Workspace].Workspace == "" {
			return errors.New("corrupt usage reservation")
		}
		if _, e := d.Rate(r.RateID); e != nil {
			return e
		}
	}
	return nil
}
func (d *Database) Rate(id string) (Rate, error) {
	for _, r := range d.Config.Rates {
		if r.ID == id {
			return r, nil
		}
	}
	return Rate{}, errors.New("unknown pinned rate version")
}
func (d *Database) RateAt(now time.Time) (Rate, error) {
	var found Rate
	for _, r := range d.Config.Rates {
		if !r.EffectiveAt.After(now) && (found.ID == "" || r.EffectiveAt.After(found.EffectiveAt)) {
			found = r
		}
	}
	if found.ID == "" {
		return found, errors.New("no effective rate configured")
	}
	return found, nil
}
func Cost(seconds, perHour int64) int64 {
	if seconds <= 0 {
		return 0
	}
	n := new(big.Int).Mul(big.NewInt(seconds), big.NewInt(perHour))
	n.Add(n, big.NewInt(3599))
	n.Div(n, big.NewInt(3600))
	if !n.IsInt64() {
		return 1 << 62
	}
	return n.Int64()
}
func secondsBetween(start, end time.Time) int64 {
	if start.IsZero() || !end.After(start) {
		return 0
	}
	elapsed := end.Sub(start)
	seconds := int64(elapsed / time.Second)
	if elapsed%time.Second != 0 {
		seconds++
	}
	return seconds
}
func (d *Database) clock(now time.Time) {
	if !d.LastClock.IsZero() && now.Before(d.LastClock) {
		d.ClockBlocked = true
	}
	if now.After(d.LastClock) {
		d.LastClock = now
	}
	if !now.Before(d.LastClock) {
		d.ClockBlocked = false
	}
}
func (d *Database) unresolved(id string) bool {
	for _, e := range d.Ledger {
		if e.Reference == id && e.Kind == "resolution" {
			return false
		}
	}
	return true
}
func (d *Database) Totals(w string) (final, estimate, reserved, trialLeft int64) {
	a := d.Accounts[w]
	trialLeft = a.Policy.TrialMicros
	for _, e := range d.Ledger {
		if e.Workspace == w && e.Status == "finalized" {
			final += e.NetMicros
			trialLeft -= e.TrialMicros
		}
	}
	for _, r := range d.Reservations {
		if r.Workspace != w {
			continue
		}
		if r.Finalized {
			if d.unresolved("usage-" + r.ID) {
				for _, e := range d.Ledger {
					if e.ID == "usage-"+r.ID && e.Status == "review" {
						reserved += r.ReservedMicros - r.TrialMicros
						trialLeft -= r.TrialMicros
					}
				}
			}
			continue
		}
		rate, _ := d.Rate(r.RateID)
		cost := Cost(r.HighWaterSeconds, rate.MicrosPerHour)
		estimate += max(0, cost-r.TrialMicros)
		reserved += max(0, r.ReservedMicros-r.TrialMicros)
		trialLeft -= r.TrialMicros
	}
	return final, estimate, reserved, max(0, trialLeft)
}
func (d *Database) blockedAccount(w string, now time.Time) bool {
	a := d.Accounts[w]
	if a.Canceled || a.Suspended || d.ClockBlocked {
		return true
	}
	for _, j := range d.Jobs {
		if j.Workspace == w && j.Kind == "invoice" && (j.PaymentFailed || j.InvoiceState == "uncollectible") {
			return true
		}
	}
	return false
}
func (d *Database) Reserve(owner, w, id string, created, expires time.Time) error {
	if old, ok := d.Reservations[id]; ok {
		if old.Owner != owner || old.Workspace != w || !old.Created.Equal(created) || !old.Expires.Equal(expires) {
			return errors.New("reservation idempotency conflict")
		}
		return nil
	}
	d.clock(created)
	a, ok := d.Accounts[w]
	if !ok || d.blockedAccount(w, created) {
		return errors.New("account canceled, suspended, delinquent or clock unsafe")
	}
	if len(d.Reservations) >= 128 || len(d.Ledger) >= 384 {
		return errors.New("billing retention bound reached; archive before admitting more usage")
	}
	rate, e := d.RateAt(created)
	if e != nil {
		return e
	}
	seconds := secondsBetween(created, expires)
	count := 0
	for _, r := range d.Reservations {
		if r.Workspace == w && !r.Finalized {
			count++
		}
	}
	final, _, reserved, trialLeft := d.Totals(w)
	amount := Cost(seconds, rate.MicrosPerHour)
	trial := min(trialLeft, amount)
	if count >= a.Policy.Concurrency || seconds < 30 || seconds > int64(a.Policy.MaxSeconds) || max(0, final)+reserved+amount-trial > a.Policy.SpendingCapMicros {
		return errors.New("workspace concurrency, duration, trial or spending reservation limit reached")
	}
	if amount > trial && (!a.PaymentReady || created.Sub(a.PaymentCheckedAt) > 2*time.Minute || created.Before(a.PaymentCheckedAt)) {
		return errors.New("verified sandbox payment setup is missing or stale; reconcile billing first")
	}
	d.Reservations[id] = Reservation{ID: id, Owner: owner, Workspace: w, RateID: rate.ID, Created: created, Expires: expires, LastObserved: created, ReservedMicros: amount, TrialMicros: trial}
	return nil
}

// Observe uses control-plane observations, never guest clocks. Estimates retain
// a high water mark; uncertain clocks become review records, never invoices.
func (d *Database) Observe(id string, readyAt time.Time, stopped bool, now time.Time) error {
	r, ok := d.Reservations[id]
	if !ok || r.Finalized {
		return nil
	}
	d.clock(now)
	if now.Before(r.LastObserved) {
		r.ClockReview = true
	}
	if r.Start.IsZero() && !readyAt.IsZero() {
		r.Start = readyAt
	}
	if !r.Start.IsZero() {
		if now.Before(r.Start) || now.After(r.Expires.Add(time.Minute)) {
			r.ClockReview = true
		}
		r.HighWaterSeconds = max(r.HighWaterSeconds, secondsBetween(r.Start, now))
	}
	if now.After(r.LastObserved) {
		r.LastObserved = now
	}
	if stopped {
		rate, e := d.Rate(r.RateID)
		if e != nil {
			return e
		}
		cost := Cost(r.HighWaterSeconds, rate.MicrosPerHour)
		status := "finalized"
		trial := min(cost, r.TrialMicros)
		if r.ClockReview {
			status = "review"
			cost = 0
			trial = 0
		}
		entry := Entry{ID: "usage-" + id, Owner: r.Owner, Workspace: r.Workspace, Session: id, Resource: "desktop-second", Kind: "usage", Status: status, RateID: r.RateID, QuantitySeconds: r.HighWaterSeconds, Start: r.Start, End: now, RecordedAt: now, CostMicros: cost, TrialMicros: trial, NetMicros: cost - trial}
		if e := d.Append(entry); e != nil {
			return e
		}
		r.Finalized = true
	}
	d.Reservations[id] = r
	return nil
}
func (d *Database) Append(e Entry) error {
	for _, old := range d.Ledger {
		if old.ID == e.ID {
			if reflect.DeepEqual(old, e) {
				return nil
			}
			return errors.New("ledger idempotency conflict")
		}
	}
	if len(d.Ledger) >= 512 {
		return errors.New("ledger full; no financial history was pruned")
	}
	d.Ledger = append(d.Ledger, e)
	return nil
}
func (d *Database) ShouldStop(id string, now time.Time) bool {
	r, ok := d.Reservations[id]
	if !ok || r.Finalized {
		return false
	}
	if d.blockedAccount(r.Workspace, now) || r.ClockReview {
		return true
	}
	a := d.Accounts[r.Workspace]
	final, estimated, _, _ := d.Totals(r.Workspace)
	rate, _ := d.Rate(r.RateID)
	cost := Cost(r.HighWaterSeconds, rate.MicrosPerHour)
	if cost > r.TrialMicros && (!a.PaymentReady || now.Sub(a.PaymentCheckedAt) > 2*time.Minute) {
		return true
	}
	return final+estimated >= a.Policy.SpendingCapMicros && estimated > 0
}
func (d *Database) Summary(w string, now time.Time) Summary {
	a := d.Accounts[w]
	final, estimate, reserved, trial := d.Totals(w)
	s := Summary{Mode: "sandbox", Plan: "cloud-pay-as-you-go-test", Currency: d.Config.Currency, Account: a, Rates: d.Config.Rates, FinalizedMicros: final, EstimatedMicros: estimate, ReservedMicros: reserved, TrialRemainingMicros: trial, BillingHealthy: !d.blockedAccount(w, now) && (!a.PaymentReady || (!now.Before(a.PaymentCheckedAt) && now.Sub(a.PaymentCheckedAt) <= 2*time.Minute))}
	if a.Policy.TrialMicros > 0 {
		s.Plan = "cloud-pay-as-you-go-test (funded trial allocation enabled)"
	}
	if a.Policy.SpendingCapMicros == 0 {
		s.Warning = "paid cloud spending disabled; only unreserved funded trial quota can admit sessions"
	} else if final+reserved >= a.Policy.SpendingCapMicros*8/10 {
		s.Warning = "80% of workspace cap reserved or incurred; estimates and provider cleanup lag apply"
	}
	if !s.BillingHealthy {
		s.Warning = "account blocked; active sessions queued for stop"
	}
	for _, e := range d.Ledger {
		if e.Workspace == w {
			s.History = append(s.History, e)
		}
	}
	for _, r := range d.Reservations {
		if r.Workspace == w && !r.Finalized {
			s.Active = append(s.Active, r)
		}
	}
	for _, j := range d.Jobs {
		if j.Workspace == w {
			s.Jobs = append(s.Jobs, j)
		}
	}
	sort.Slice(s.Jobs, func(i, j int) bool { return s.Jobs[i].ID < s.Jobs[j].ID })
	return s
}
func (d *Database) Adjustment(id, reference, reason string, micros int64, resolveSeconds *int64, now time.Time) error {
	if !label.MatchString(id) || !(reason == "service_credit" || reason == "provider_correction" || reason == "clock_resolution") {
		return errors.New("use stable adjustment ID and documented reason enum")
	}
	var original Entry
	for _, e := range d.Ledger {
		if e.ID == reference {
			original = e
			break
		}
	}
	if original.ID == "" {
		return errors.New("unknown ledger reference")
	}
	row := Entry{ID: "adjust-" + id, Owner: original.Owner, Workspace: original.Workspace, Session: original.Session, Resource: "desktop-second", Kind: "adjustment", Status: "finalized", Reference: reference, Reason: reason, RateID: original.RateID, RecordedAt: now, Start: original.Start, End: original.End}
	if resolveSeconds != nil {
		if original.Status != "review" || *resolveSeconds < 0 || *resolveSeconds > 86400 {
			return errors.New("resolution requires a review entry and independently verified 0..86400 seconds")
		}
		for _, e := range d.Ledger {
			if e.Kind == "resolution" && e.Reference == reference {
				if e.ID == row.ID && e.QuantitySeconds == *resolveSeconds {
					return nil
				}
				return errors.New("review already resolved")
			}
		}
		rate, _ := d.Rate(original.RateID)
		row.Kind = "resolution"
		row.QuantitySeconds = *resolveSeconds
		row.CostMicros = Cost(*resolveSeconds, rate.MicrosPerHour)
		row.TrialMicros = min(row.CostMicros, d.Reservations[original.Session].TrialMicros)
	} else {
		if micros == 0 || micros < -100_000_000 || micros > 100_000_000 {
			return errors.New("adjustment amount outside sandbox bounds")
		}
		row.CostMicros = micros
	}
	row.NetMicros = row.CostMicros - row.TrialMicros
	for _, e := range d.Ledger {
		if e.ID == row.ID {
			row.RecordedAt = e.RecordedAt
		}
	}
	return d.Append(row)
}
func (d *Database) UpdateRates(c Config) error {
	if e := c.Validate(); e != nil {
		return e
	}
	for _, old := range d.Config.Rates {
		found := false
		for _, r := range c.Rates {
			if r.ID == old.ID {
				found = reflect.DeepEqual(r, old)
			}
		}
		if !found {
			return fmt.Errorf("existing rate %s is immutable", old.ID)
		}
	}
	if !reflect.DeepEqual(c.Policy, d.Config.Policy) {
		return errors.New("change account policy explicitly; rate update cannot reset trials or caps")
	}
	c.Rates = append([]Rate(nil), c.Rates...)
	d.Config = c
	return nil
}
