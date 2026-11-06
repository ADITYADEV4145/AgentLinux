// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/ADITYADEV4145/AgentLinux/internal/billing"
	"github.com/ADITYADEV4145/AgentLinux/internal/cloud"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

func runBilling(ctx context.Context, args []string, out, errout io.Writer) int {
	fail := func(e error) int { fmt.Fprintln(errout, e); return 1 }
	if len(args) == 0 || args[0] == "--help" {
		fmt.Fprintln(out, `Usage: agent-linux billing ACTION [flags]
User (authenticated --config FILE):
  status       Plan, trial, rates, estimates, reservations, cap, history, payment jobs
  setup        Queue sandbox customer/Checkout setup; --request-id 32_HEX
  portal       Queue sandbox portal link; --request-id 32_HEX
  cancel       Block new sessions; stop/delete active cloud sessions; requires --yes
Operator (server STOPPED; explicit --state-dir and --yes):
  configure    --pricing FILE: enable sandbox metering; no retroactive charges
  invoice      --workspace ID --request-id ID [--finalize]; no automatic collection
  suspend / resume  --workspace ID (resume requires fresh payment reconciliation)
  policy       --workspace ID --policy FILE: explicit trial/cap/concurrency limits
  adjust       --workspace ID --request-id ID --reference ENTRY --micros SIGNED --reason service_credit|provider_correction
  resolve      --workspace ID --request-id ID --reference REVIEW_ENTRY --resolved-seconds N
No live mode, products, subscriptions, raw cards, invoice send or charge operation.`)
		fmt.Fprintln(out, "Required snapshot webhook API version:", billing.StripeAPIVersion)
		return 0
	}
	action := args[0]
	f := flag.NewFlagSet("billing "+action, flag.ContinueOnError)
	f.SetOutput(errout)
	jsonOutput := f.Bool("json", false, "structured billing output for automation")
	config := f.String("config", "", "user: private cloud credential config")
	root := f.String("state-dir", "", "operator: existing private control-plane directory; stop server first")
	pricing := f.String("pricing", "", "operator: reviewed pricing JSON; currency usd, mode sandbox only")
	policy := f.String("policy", "", "operator: reviewed policy JSON (trial/cap in integer micro-USD)")
	workspace := f.String("workspace", "", "operator: exact workspace ID")
	key := f.String("request-id", "", "stable operation ID; user requests require 32 hex")
	reference := f.String("reference", "", "operator: existing ledger entry ID")
	micros := f.Int64("micros", 0, "signed micro-USD adjustment; never raw card data")
	reason := f.String("reason", "service_credit", "adjustment reason enum")
	resolved := f.Int64("resolved-seconds", -1, "operator-verified duration for a clock review; never inferred automatically")
	finalize := f.Bool("finalize", false, "explicitly finalize the SANDBOX invoice with automatic collection disabled")
	yes := f.Bool("yes", false, "explicitly authorize displayed account/configuration changes")
	if e := f.Parse(args[1:]); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if f.NArg() != 0 {
		return fail(errors.New("unexpected billing arguments"))
	}
	if action == "status" || action == "setup" || action == "portal" || action == "cancel" {
		if *root != "" || *workspace != "" {
			return fail(errors.New("user billing routes infer workspace from authentication; no operator flags"))
		}
		if action == "cancel" && !*yes {
			return fail(errors.New("cancellation blocks new cloud sessions and queues all active workspace machines for deletion; finalized usage/invoices remain; repeat with --yes"))
		}
		c, e := cloud.LoadClient(*config)
		if e != nil {
			return fail(e)
		}
		var raw json.RawMessage
		if action == "status" {
			raw, e = c.Billing(ctx)
		} else {
			raw, e = c.BillingAction(ctx, *key, action, *yes)
		}
		if e != nil {
			return fail(e)
		}
		if action == "status" && !*jsonOutput {
			return printBilling(out, raw)
		}
		fmt.Fprintln(out, string(raw))
		return 0
	}
	if *config != "" {
		return fail(errors.New("operator actions do not use user credentials"))
	}
	if !filepath.IsAbs(*root) {
		return fail(errors.New("operator action requires an explicit absolute state-dir"))
	}
	if !*yes {
		return fail(fmt.Errorf("%s changes sandbox billing state in the selected control plane; inspect docs/billing.md and repeat with --yes", action))
	}
	private, e := storage.ExistingRoot(*root)
	if e != nil {
		return fail(e)
	}
	unlock, e := storage.Lock(private)
	if e != nil {
		return fail(errors.New("stop the development server before operator edits; provider TTL continues during maintenance"))
	}
	defer unlock()
	if action == "configure" {
		c, e := cloud.ReadBillingConfig(*pricing)
		if e != nil {
			return fail(e)
		}
		if e = cloud.ConfigureBilling(private, c); e != nil {
			return fail(e)
		}
		fmt.Fprintln(out, `{"configured":"sandbox","liveBilling":false}`)
		return 0
	}
	q := cloud.BillingAdmin{Workspace: *workspace, Action: action, RequestID: *key, Reference: *reference, Reason: *reason, Micros: *micros, Finalize: *finalize}
	if action == "resolve" {
		if *resolved < 0 {
			return fail(errors.New("resolve requires explicit --resolved-seconds"))
		}
		q.ResolveSeconds = resolved
		q.Reason = "clock_resolution"
	}
	if action == "policy" {
		// Policy contains only bounded public numeric settings, not credentials.
		c, e := cloud.ReadBillingPolicy(*policy)
		if e != nil {
			return fail(e)
		}
		q.Policy = &c
	}
	result, e := cloud.AdminBilling(private, q)
	if e != nil {
		return fail(e)
	}
	_ = json.NewEncoder(out).Encode(result)
	return 0
}

func money(m int64) string {
	sign := ""
	if m < 0 {
		sign = "-"
		m = -m
	}
	return fmt.Sprintf("%s$%d.%06d", sign, m/1000000, m%1000000)
}
func printBilling(out io.Writer, raw json.RawMessage) int {
	var s billing.Summary
	if json.Unmarshal(raw, &s) != nil || s.Mode != "sandbox" {
		fmt.Fprintln(out, string(raw))
		return 0
	}
	fmt.Fprintf(out, "SANDBOX ONLY — live billing disabled\nLocal: free\nPlan: %s\nTrial available after reservations: %s\nFinalized net usage: %s\nActive estimate: %s (not finalized)\nActive spending reservation: %s\nWorkspace lifetime spending cap: %s\nConcurrency: %d; maximum session: %d seconds\nPayment setup verified: %t; canceled: %t; suspended: %t\n", s.Plan, money(s.TrialRemainingMicros), money(s.FinalizedMicros), money(s.EstimatedMicros), money(s.ReservedMicros), money(s.Account.Policy.SpendingCapMicros), s.Account.Policy.Concurrency, s.Account.Policy.MaxSeconds, s.Account.PaymentReady, s.Account.Canceled, s.Account.Suspended)
	fmt.Fprintln(out, "Configured test rates (not finalized retail prices; no separate storage/network pass-through):")
	for _, r := range s.Rates {
		fmt.Fprintf(out, "  %s: %s / machine-hour; %d CPU, %d GiB RAM, up to %d GiB ephemeral disk; effective %s\n", r.ID, money(r.MicrosPerHour), r.CPUs, r.MemoryGiB, r.DiskGiB, r.EffectiveAt.Format("2006-01-02T15:04:05Z07:00"))
	}
	if s.Warning != "" {
		fmt.Fprintln(out, "Warning:", s.Warning)
	}
	for _, r := range s.Active {
		fmt.Fprintf(out, "Active %s: %d estimated seconds; expires %s; rate %s\n", r.ID, r.HighWaterSeconds, r.Expires.Format("2006-01-02T15:04:05Z07:00"), r.RateID)
	}
	for _, e := range s.History {
		fmt.Fprintf(out, "History %s / %s: %s, %d seconds, net %s, trial %s, rate %s\n", e.Session, e.ID, e.Status, e.QuantitySeconds, money(e.NetMicros), money(e.TrialMicros), e.RateID)
	}
	for _, j := range s.Jobs {
		fmt.Fprintf(out, "Payment %s %s: %s %s\n", j.Kind, j.ID, j.Status, j.Error)
		if j.URL != "" {
			fmt.Fprintln(out, "  Private sandbox link:", j.URL)
		}
	}
	fmt.Fprintln(out, "Idle time remains billable until confirmed shutdown. Failed provisioning is user-free. Taxes are not calculated; processing fees are operator costs. Limits allow provider cleanup/billing lag.")
	return 0
}
