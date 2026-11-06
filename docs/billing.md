# Phase 7: sandbox metering and billing foundation

Implemented 2026-11-05. **Live billing is disabled in code.** The official Stripe Go SDK is pinned at v86.4.2. No live key, production product, paid cloud resource, customer charge, public endpoint, paid service or release was created. No Stripe credentials were configured during implementation, so hosted sandbox Checkout/payment acceptance is **pending**, not passed. Local use remains free and does not initialize this ledger or call Stripe.

## Model and exact charging rules

Use cloud pay-as-you-go by a configured desktop bundle (CPU/RAM/maximum ephemeral root disk) per machine-hour, measured in one-second increments. No subscription/team plan: real team collaboration features do not exist. A funded capped trial is optional; it is **off by default** because pricing/billing is entirely unconfigured by default. Do not promise free provider credits or financially viable trials before measuring the selected provider and approving a trial budget. The example configuration is explicitly **illustrative sandbox arithmetic, not retail pricing**.

Rates live in reviewed JSON, not constants: version ID, effective UTC time, integer micro-USD/hour, CPU, GiB RAM and maximum GiB disk. A session pins the effective rate at admission. Existing rate versions cannot be edited or removed; add a new effective version. Current supported billing profile validation is 2–4 CPUs, 4–8 GiB RAM, 8–40 GiB maximum root disk. The actual provider CPU/RAM must match the quoted bundle and disk must fit its limit before readiness; mismatch queues deletion and no user billable start. This post-allocation validation cannot prevent the provider's initial cost. A simulated provider represents 2 CPUs/4 GiB/24 GiB and is visibly labeled.

Billable start is the control plane's **first confirmed desktop readiness**; billable end is **confirmed provider termination/absence**, including stopping/idle time and readiness interruptions after the first success. Calling stop is not proof that compute ceased. Failed provisioning that never reached readiness produces a zero-quantity finalized record; provider allocation/provisioning costs are the operator's responsibility. Guest clocks never determine charges. Idle viewing does not extend the existing idle deadline, but idle running time remains billable until cleanup completes.

One session's elapsed interval is rounded up once to whole seconds. Cost is `ceil(seconds × rateMicroUSDPerHour / 3600)`. Monetary estimates retain high water marks. Final records are not rewritten. Trial credit is reserved atomically at admission and applied to final cost; unused reservations are released. The lifetime workspace cap applies to net finalized usage plus full prospective active net reservations; it is not a monthly reset. Credits can reduce future net amounts, with an auditable adjustment reference.

Invoices aggregate finalized net amounts only. Lifetime net micro-USD is rounded **down** to whole cents, subtracting already reserved invoice amounts; fractional cents/negative credits carry to future invoices. This prevents per-session/per-retry rounding inflation. Below-cent usage is shown in history and remains uninvoiced. Only positive whole-cent totals can create an invoice. An invoice intent reserves its exact entry IDs before contacting Stripe, preventing overlapping invoices.

The bundle includes ephemeral storage and permitted browser/desktop streaming; there are no separate storage/network pass-through rates in this phase. No persistent cloud disk is offered. Unexpected provider disk, egress, storage/orphan costs remain operator costs, not silently added retail charges. Tax calculation/collection is **not implemented** and is not included in the usage estimate; no tax-compliance claim is made. Processing fees are operator costs in the financial model, not a hidden surcharge.

## Free deterministic development run

No account or paid resource is needed:

```sh
make build test check
python3 scripts/billing_acceptance.py --cli "$PWD/bin/agent-linux"
```

The acceptance script creates random development credentials in its own private temporary namespace, runs the real CLI/API against **SIMULATED** resources, verifies ready-to-confirmed-stop metering, trial allocation, workspace isolation and guarded cancellation, then removes only that fixture. It never calls Stripe or E2B.

For manual exploration, use a fresh private directory, outside the repository:

```sh
mkdir -m 700 /private/tmp/agent-linux-billing-demo
agent-linux cloud init --dev --state-dir /private/tmp/agent-linux-billing-demo
# Review/edit the illustrative sandbox file; this does not establish retail prices.
agent-linux billing configure --state-dir /private/tmp/agent-linux-billing-demo \
  --pricing "$PWD/examples/billing.sandbox.json" --yes
agent-linux cloud serve --dev --state-dir /private/tmp/agent-linux-billing-demo
# In another terminal:
agent-linux cloud up --config /private/tmp/agent-linux-billing-demo/alice.json \
  --request-id 0123456789abcdef0123456789abcdef --seconds 60 \
  --output /private/tmp/agent-linux-billing-demo/session.json
agent-linux billing status --config /private/tmp/agent-linux-billing-demo/alice.json
agent-linux billing status --json --config /private/tmp/agent-linux-billing-demo/alice.json
agent-linux cloud stop --config /private/tmp/agent-linux-billing-demo/session.json
```

Status shows plan, unreserved trial balance, finalized/estimated/reserved amounts, limits, configured rates, active expiry, session history, payment readiness and private links from completed setup/portal/invoice jobs. `cloud status`, MCP status and viewer initial metadata also expose the selected session's pinned rate and estimate through remote machine metadata. The terminal viewer does not render a live billing dashboard; use billing status for account details. Estimates are distinct from finalized usage and provider invoices.

## Ledger, concurrency and failure handling

Session state, pricing, account/workspace, reservations, ready time, finalized usage, adjustments, invoice intents and webhook inbox share the existing **atomic private cloud state file** and one exclusive server process. Persist intent before any external API. Guest identity, owner, workspace, session ID, resource `desktop-second`, quantity, interval, record time, rate reference and trial/net amounts are stored. No command text, clipboard text, screen content, raw webhook payload, raw card data, keys or signing secret is persisted. A signed webhook stores only its ID/type/object ID, receipt time and processing status.

Finalization is idempotent by session ID; repeated ticks/restarts do not append another charge. Adjustment IDs are idempotent and reference the original record. Rates and final financial records are immutable through application operations. This is an append-only application history, **not a tamper-proof ledger** against an operator editing its private files. The JSON backend is a single-process prototype, not a production financial database. Identity remains the Phase 6 eight-hour development credential model; there is no production signup, identity renewal or team billing authority. Production requires transactional storage/queue, backup/restore drills, access separation and independent reconciliation.

Workspace limits reserve full requested lifetime before provisioning, under the same lock as session creation. Phase 6 global runtime/concurrency/idle limits remain an additional company-cost guard. An 80% reservation/incurred warning is visible in billing status. Cap, trial depletion, delinquency, cancellation, suspension or unsafe clocks block new admission; the worker queues affected active sessions for termination. Paid usage requires a recent successful payment-state check (two-minute freshness window). Funded trial usage can proceed without Stripe while local metering is healthy. Payment operations have bounded timeouts; errors retain intents and fail closed when paid-state freshness expires.

Clock rollback never decreases high water usage. An uncertain clock or unexpectedly long interval beyond expiry plus a one-minute cleanup allowance produces a **review record that cannot be invoiced**, reserves exposure and stops the session. An operator can resolve it using independently verified provider duration; the original review remains. Forward jumps, delayed provider absence, service outage and provider billing lag can exceed a prospective reservation. **No perfect hard spending cap is promised.** Provider-side TTL is requested, not empirically guaranteed; inventory/cleanup outages and external resources may still incur costs.

A state-write/audit failure poisons admission and control. The next reconciliation still attempts termination of known owned provider resources without requiring a financial write. It cannot falsely finalize usage or claim durable cleanup succeeded. Restore/inspect the state and reconcile the provider before resuming. No active session automatically becomes free because the process restarted. Provider inventory and ownership-label reconciliation remain from Phase 6; actual provider cost reconciliation is an operator task pending real provider tests.

## Stripe sandbox integration

Official sources inspected: [Go SDK and recommended client](https://github.com/stripe/stripe-go), [Checkout setup mode](https://docs.stripe.com/payments/save-and-reuse?platform=web&ui=checkout), [Checkout creation](https://docs.stripe.com/api/checkout/sessions/create?lang=go), [draft invoice creation](https://docs.stripe.com/api/invoices/create?lang=go), [invoice items](https://docs.stripe.com/api/invoiceitems/create?lang=go), [manual finalization](https://docs.stripe.com/api/invoices/finalize?lang=go), [idempotency](https://docs.stripe.com/api/idempotent_requests), [webhooks](https://docs.stripe.com/webhooks), and [sandbox testing](https://docs.stripe.com/testing). SDK license is MIT, included in third-party notices. E2B commercial permission/provider acceptance gates from Phase 6 still apply independently of Stripe.

The server accepts only a server-side `sk_test_` key. It rejects live/restricted keys, live config mode, live SDK objects/events and connected-account events. No runtime switch enables production. No raw card endpoint, subscription/product creation, invoice send/pay, automatic collection or live charge operation exists. SDK network retries are disabled; durable intent keys handle explicit retries. Redirects/proxy inheritance and SDK payload logging are disabled; provider errors are sanitized. Credentials remain in server environment only.

An operator supplies test credentials through their environment (never paste them into Git, commands, issue reports or this document), plus `STRIPE_WEBHOOK_SECRET` and an operator-controlled `AGENT_LINUX_BILLING_RETURN_URL`. Serve the configured state with:

```sh
agent-linux cloud serve --dev --state-dir /private/tmp/agent-linux-billing-demo --stripe-sandbox
# Separate terminal, using the official Stripe CLI in the SAME sandbox:
stripe listen --events checkout.session.completed,checkout.session.expired,invoice.paid,invoice.payment_failed,invoice.voided,invoice.marked_uncollectible,customer.deleted \
  --forward-to http://127.0.0.1:8787/v1/stripe/webhook
# Configure the listener's signing secret in the server environment, then restart the server.
agent-linux billing setup --config /private/tmp/agent-linux-billing-demo/alice.json \
  --request-id 2123456789abcdef0123456789abcdef
agent-linux billing status --config /private/tmp/agent-linux-billing-demo/alice.json
```

The setup request queues a durable job. Read status after several worker ticks to obtain its private hosted Checkout URL. Enter **only Stripe test card data in Stripe's hosted page**, never our CLI. A redirect alone is not accepted as success: the worker retrieves the current sandbox customer, Checkout session and expanded SetupIntent and validates association/status. This avoids trusting a user-controlled success redirect. Payment setup does not itself charge anything.

`billing portal --config FILE --request-id ID` queues a portal session for that authenticated workspace's associated customer. It may require test portal configuration in the Stripe Dashboard; unavailable configuration yields a visible retrying job rather than a fake link. There is no team/subscription cancellation UI. `billing cancel --config FILE --yes` cancels cloud account access and queues its active machines for deletion; it does not erase finalized usage or existing invoices. Resume is operator-only and requires fresh payment reconciliation.

Webhook verification uses the official SDK on exact bounded raw bytes and validates timestamp tolerance/API version. Future signature clocks are also rejected. Persist the minimal receipt before returning 2xx; process current associated Stripe objects later. Duplicate event IDs are deduplicated, and current object retrieval prevents an old payment-failure event overriding a paid invoice. An event arriving before a lost customer/checkout reply remains pending until association recovers. Unknown associations are not silently declared processed. Failed payment/current overdue or uncollectible invoice blocks admission and queues shutdown; paid/void current state can clear the invoice failure. Customer deletion suspends the account.

## Operator workflow and recovery

Operator edits require the **server stopped**, the exact private namespace, exclusive lock, explicit action and `--yes`. Provider TTL continues during maintenance; keep it brief and restart the same server/provider to reconcile. Offline admin commands do not call Stripe or providers.

```sh
# Review billing status --json to select the exact workspace; stop server first.
agent-linux billing invoice --state-dir STATE --workspace WORKSPACE \
  --request-id UNIQUE_ID --yes
# Add --finalize when deliberately queuing an open SANDBOX invoice.
agent-linux billing suspend --state-dir STATE --workspace WORKSPACE --yes
agent-linux billing resume --state-dir STATE --workspace WORKSPACE --yes
agent-linux billing policy --state-dir STATE --workspace WORKSPACE --policy REVIEWED_POLICY.json --yes
agent-linux billing adjust --state-dir STATE --workspace WORKSPACE --request-id CREDIT_ID \
  --reference usage-SESSION_ID --micros -10000 --reason service_credit --yes
agent-linux billing resolve --state-dir STATE --workspace WORKSPACE --request-id RESOLUTION_ID \
  --reference usage-SESSION_ID --resolved-seconds VERIFIED_SECONDS --yes
```

The reviewed policy JSON uses `trialMicros`, `spendingCapMicros`, `concurrency`, `maxSeconds`. Increasing trial is an intentional additional allocation, not a reset of prior consumed credit. Policy/account admin changes are retained in bounded before/after audit records. Rates change with `billing configure` and a new effective version; existing sessions retain their pinned version.

Invoice worker steps are separately committed: customer association, isolated draft (`send_invoice`, `auto_advance=false`, excluding unrelated pending items), attached usage line, and optional manual finalization with collection still disabled. It verifies customer/job metadata, currency and total before finalizing. No email send or payment API is invoked. Review a draft and finalize/pay manually in the Stripe sandbox Dashboard if it was queued without `--finalize`; changing an existing request's parameters is rejected. Hosted invoice links appear when Stripe returns them.

Stripe v1 may discard idempotency keys after 24 hours. Jobs older than 23 hours are held for operator review; they are not blindly replayed. Inspect the sandbox by job/workspace metadata and request logs, reconcile any already-created customer/draft/line, and recover the private state through a reviewed maintenance change before retrying. A production recovery command with independent authorization is still needed. Retry the same recent request ID, not a new ID. Failed webhooks remain durable pending work; SDK errors are retried on later passes, while Stripe retries failed delivery. Use official CLI/Dashboard resend after correcting signing/version/inbox problems. Configure a snapshot destination matching the pinned SDK API version (`2026-08-26.dahlia` for v86.4.2).

Local adjustments are accounting entries, **not payment refunds or credit notes**. Already invoiced credits carry into the next usage invoice; refunds/credit notes for existing Stripe invoices must be reviewed and performed explicitly in the sandbox Dashboard, then recorded as an adjustment without double-crediting. Production refund/tax/retention workflows require further design and authorization.

No automatic financial-history pruning or account erasure is implemented. Bounds: 128 metered session reservations, 512 ledger entries, 256 payment jobs/account admin records and 1,024 minimal webhook receipts, plus a total state-size guard below 1 MiB. Admission stops before exhaustion; never delete records to reset trial/caps. Export and securely archive a complete epoch, verify against provider/Stripe records and create a deliberately new development namespace. Production archival, jurisdiction-specific retention/deletion policies and scheduled reconciliation are release gates. Alerts presently appear as status warnings/job error codes and generic reconciliation stderr; external paging/email alert delivery is not implemented.

## Editable financial model

Edit [cost-model.json](../examples/cost-model.json), then run `python3 scripts/cost_model.py --config examples/cost-model.json`. All inputs are illustrative assumptions, including processing fees; none are validated provider quotes or measured economics. It covers CPU, RAM, ephemeral storage lifetime, streaming/other egress, provider overhead, monthly database/control plane allocation, trial and idle/orphan fractions, support, fixed/percentage processing fees, average invoice size and target gross margin. Low/base/high scenarios show sensitivity to utilization and streaming. CPU and RAM rates assume separate billing, so replace both consistently for an all-inclusive provider. Output computes required revenue under the assumptions, **not a finalized retail price**. Taxes, discounts, legal/accounting costs and regional/provider contract details require further inputs before any offer.

## Acceptance status

Passing: Go race tests for metering/SDK/HTTP integration, strict signing and replay tests, clock/rate/rounding/reservation cases, payment retry/failure/out-of-order cases, multi-workspace/cancellation, disk-failure fail-closed cleanup, and a zero-charge simulated CLI acceptance. SDK HTTP tests inspect real official request encoding against a local fixture and ensure no send/pay endpoint is called. Existing local desktop tests remain passing.

Pending: actual Stripe sandbox Checkout completion, live test-mode webhook forwarding, failed-payment/portal/invoice flow on a Stripe account, provider-cost reconciliation and real E2B enforcement. No Stripe test credentials were present. To accept, configure sandbox credentials, complete setup using hosted test fields, create/stop separately authorized provider sessions (or clearly labeled simulated arithmetic), explicitly queue a sandbox invoice, exercise paid/failed/void states through the sandbox Dashboard, confirm billing status and shutdown behavior, replay events, restart the server and inspect the ledger. Record actual SDK/API/account mode/results and verify no live objects/charges/resources were created. Production activation remains unavailable regardless of these tests.
