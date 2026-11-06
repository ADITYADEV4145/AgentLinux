// SPDX-License-Identifier: Apache-2.0
// Package billing contains sandbox pricing, metering and payment records. It has
// no live-mode switch. Cloud lifecycle and this ledger share one atomic commit.
package billing

import "time"

const Version = "billing-sandbox-1"
const MicrosPerCent int64 = 10000

type Rate struct {
	ID            string    `json:"id"`
	EffectiveAt   time.Time `json:"effectiveAt"`
	MicrosPerHour int64     `json:"microsPerHour"`
	CPUs          int       `json:"cpus"`
	MemoryGiB     int       `json:"memoryGiB"`
	DiskGiB       int       `json:"diskGiB"`
}
type Policy struct {
	TrialMicros       int64 `json:"trialMicros"`
	SpendingCapMicros int64 `json:"spendingCapMicros"`
	Concurrency       int   `json:"concurrency"`
	MaxSeconds        int   `json:"maxSeconds"`
}
type Config struct {
	Version  string `json:"version"`
	Mode     string `json:"mode"`
	Currency string `json:"currency"`
	Rates    []Rate `json:"rates"`
	Policy   Policy `json:"policy"`
}
type Account struct {
	Workspace        string    `json:"workspace"`
	Policy           Policy    `json:"policy"`
	Customer         string    `json:"customer,omitempty"`
	Checkout         string    `json:"checkout,omitempty"`
	PaymentReady     bool      `json:"paymentReady"`
	PaymentCheckedAt time.Time `json:"paymentCheckedAt"`
	Suspended        bool      `json:"suspended"`
	Canceled         bool      `json:"canceled"`
}
type Reservation struct {
	ID               string    `json:"id"`
	Owner            string    `json:"owner"`
	Workspace        string    `json:"workspace"`
	RateID           string    `json:"rateId"`
	Created          time.Time `json:"created"`
	Expires          time.Time `json:"expires"`
	Start            time.Time `json:"start,omitempty"`
	LastObserved     time.Time `json:"lastObserved"`
	HighWaterSeconds int64     `json:"highWaterSeconds"`
	ReservedMicros   int64     `json:"reservedMicros"`
	TrialMicros      int64     `json:"trialMicros"`
	ClockReview      bool      `json:"clockReview"`
	Finalized        bool      `json:"finalized"`
}
type Entry struct {
	ID              string    `json:"id"`
	Owner           string    `json:"owner"`
	Workspace       string    `json:"workspace"`
	Session         string    `json:"session"`
	Resource        string    `json:"resource"`
	Kind            string    `json:"kind"`
	Status          string    `json:"status"`
	Reference       string    `json:"reference,omitempty"`
	Reason          string    `json:"reason,omitempty"`
	RateID          string    `json:"rateId"`
	QuantitySeconds int64     `json:"quantitySeconds"`
	Start           time.Time `json:"start"`
	End             time.Time `json:"end"`
	RecordedAt      time.Time `json:"recordedAt"`
	CostMicros      int64     `json:"costMicros"`
	TrialMicros     int64     `json:"trialMicros"`
	NetMicros       int64     `json:"netMicros"`
}
type Job struct {
	ID            string    `json:"id"`
	Workspace     string    `json:"workspace"`
	Kind          string    `json:"kind"`
	Created       time.Time `json:"created"`
	Status        string    `json:"status"`
	Error         string    `json:"error,omitempty"`
	Customer      string    `json:"customer,omitempty"`
	Checkout      string    `json:"checkout,omitempty"`
	Invoice       string    `json:"invoice,omitempty"`
	Item          string    `json:"item,omitempty"`
	AmountCents   int64     `json:"amountCents,omitempty"`
	Entries       []string  `json:"entries,omitempty"`
	URL           string    `json:"url,omitempty"`
	Finalize      bool      `json:"finalize"`
	InvoiceState  string    `json:"invoiceState,omitempty"`
	PaymentFailed bool      `json:"paymentFailed"`
}
type Receipt struct {
	ID       string    `json:"id"`
	Type     string    `json:"type"`
	Object   string    `json:"object"`
	Received time.Time `json:"received"`
	Status   string    `json:"status"`
	Error    string    `json:"error,omitempty"`
}
type Database struct {
	Changes      []AccountChange        `json:"accountChanges,omitempty"`
	Config       Config                 `json:"config"`
	Accounts     map[string]Account     `json:"accounts"`
	Reservations map[string]Reservation `json:"reservations"`
	Ledger       []Entry                `json:"ledger"`
	Jobs         map[string]Job         `json:"jobs"`
	Receipts     map[string]Receipt     `json:"receipts"`
	LastClock    time.Time              `json:"lastClock"`
	ClockBlocked bool                   `json:"clockBlocked"`
}
type Summary struct {
	Mode                 string        `json:"mode"`
	Plan                 string        `json:"plan"`
	Currency             string        `json:"currency"`
	Account              Account       `json:"account"`
	Rates                []Rate        `json:"rates"`
	FinalizedMicros      int64         `json:"finalizedNetMicros"`
	EstimatedMicros      int64         `json:"estimatedActiveNetMicros"`
	ReservedMicros       int64         `json:"reservedActiveNetMicros"`
	TrialRemainingMicros int64         `json:"trialRemainingMicros"`
	Warning              string        `json:"warning,omitempty"`
	History              []Entry       `json:"history"`
	Active               []Reservation `json:"active"`
	Jobs                 []Job         `json:"paymentJobs"`
	BillingHealthy       bool          `json:"billingHealthy"`
}
type SessionCost struct {
	Mode        string    `json:"mode"`
	Currency    string    `json:"currency"`
	Rate        Rate      `json:"rate"`
	Start       time.Time `json:"billableStart,omitempty"`
	Seconds     int64     `json:"seconds"`
	NetMicros   int64     `json:"netMicros"`
	TrialMicros int64     `json:"trialMicros"`
	Status      string    `json:"status"`
}

func (d *Database) SessionCost(id string) *SessionCost {
	r, ok := d.Reservations[id]
	if !ok {
		return nil
	}
	rate, _ := d.Rate(r.RateID)
	cost := Cost(r.HighWaterSeconds, rate.MicrosPerHour)
	trial := min(cost, r.TrialMicros)
	s := &SessionCost{Mode: "sandbox", Currency: d.Config.Currency, Rate: rate, Start: r.Start, Seconds: r.HighWaterSeconds, NetMicros: cost - trial, TrialMicros: trial, Status: "estimate"}
	if r.Finalized {
		s.Status = "finalized"
	}
	if r.ClockReview {
		s.Status = "review; not invoiceable"
	}
	return s
}

type AccountChange struct {
	At        time.Time `json:"at"`
	Workspace string    `json:"workspace"`
	Action    string    `json:"action"`
	Before    Account   `json:"before"`
	After     Account   `json:"after"`
}
