# Cloud release checklist — NOT cleared

This is a release gate, not a certification or security audit. Local source-preview readiness does not clear any hosted-service gate. No paid provider resource, internet deployment, Stripe account payment, live billing, legal approval or public release was created by this review.

| Area | Implemented / evidence | Required before a paid/public pilot |
| --- | --- | --- |
| Identity | Random private development bearer credentials, eight-hour expiry/revocation; loopback development server | Production identity/OIDC, renewal/recovery, session credential lifetime and revocation, account lifecycle and operator authorization; no public signup exists |
| Tenant authorization | Owner/workspace check on every session route; cross-user/API tests; authenticated billing is workspace-derived | Real multi-tenant deployment tests, independent review, access separation and audit/alert operations |
| Session isolation | Local VZ guest and controlled guest service tested; E2B adapter exists and provider describes microVM isolation | Written provider/commercial permission where needed; real region/template/architecture/image validation, adversarial cross-session tests, quota and teardown evidence |
| Secrets | No host mounts/clipboard/agent credentials by default; provider/test keys server environment only; no raw cards or payload logging | Production secret manager/rotation/access controls, worker compromise plan, backups and incident credential revocation |
| Network | No public raw VNC links; authenticated bounded PNG proxy; E2B private/link-local CIDRs requested | Actual ingress/egress/DNS/IPv6/metadata enforcement tests; production TLS ingress; no network-isolation guarantee from an untested option |
| Abuse | Bounded messages/rates/queues/concurrency/duration; no implicit cloud allocation | Abuse account controls, provider acceptable-use approval, budget-funded trial policy, response ownership and controls outside application process |
| Resource/spending limits | Durable reservations, TTL/idle stop, caps, cancellation/suspension and fail-closed accounting tests | Provider-enforced limits/alarms, outage cleanup drills and cost reconciliation; explain lag/out-of-band charges rather than promise a perfect cap |
| Metering | Rate-pinned atomic JSON usage/adjustment history, estimates/finals, confirmed-stop semantics and restart tests | Transactional durable store/queue, archival, backup/restore, independent provider invoice reconciliation, operational dispute handling |
| Payments | Official pinned Stripe SDK; live keys/mode rejected; setup/manual invoices/portal and signed minimal inbox; local fixture tests | Hosted sandbox Checkout, forwarding/replay/payment-failure/paid/void/portal tests on an account; tax/refund/credit-note workflow; separate explicit production authorization; no live switch is implemented |
| Monitoring/alerts | Structured local diagnostics, bounded audit history, status warnings/error codes | Metrics, operational dashboards/paging, alerts on orphan/cost/cleanup/backlog/state loss, named on-call and tested runbooks |
| Cleanup/persistence | Owned-label inventory reconciliation; ephemeral cloud stop; local guarded reset/delete; simulation/failure tests | Actual provider kill/TTL/orphan and storage/egress cleanup observations; service outage/restart drills; provider retention/deletion confirmation |
| Retention/deletion | Bounded private prototype records; no silent financial pruning; guest ephemeral policy documented | Jurisdiction-specific policy, user/account deletion/export, archived financial history, secure backups and provider data lifecycle |
| Incident/support | Draft triage/containment procedure and repository security instructions | Monitored private reporting route, named owner/support channel, response expectations, restoration/disclosure exercises; no SLA currently promised |
| Legal/privacy | Original code license/notices present; provider commercial restrictions identified; external model data flow disclosed | Appropriate professional review of provider/commercial permissions, terms, privacy/data processing, tax/consumer/refund/retention requirements before offering service |

## Suggested incident procedure (prepared, not an operating service)

1. Record the affected deployment/session/workspace IDs, source/tool/provider versions and sanitized timestamps. Preserve private state/audit backups and cloud inventory; do not post tokens, screenshots or user content.
2. Suspend affected accounts, block new allocations, and queue owned active sessions for termination. During ledger failure, use the documented owned cleanup path and verify provider absence independently. Never run global deletion against unrelated resources.
3. Revoke/rotate affected credentials and stop billing activation. Determine whether cloud resources, guest data, host access or payment associations were exposed; do not infer safety from a process exiting.
4. Reconcile provider resources/costs and Stripe current objects against immutable records/intents before reopening. Review uncertain clocks/jobs; never replay an old ambiguous payment creation blindly.
5. Coordinate private remediation/disclosure with the repository owner and affected parties under reviewed obligations. Set public support/incident commitments only once a staffed process exists.

These steps have not been exercised against real managed-provider/Stripe incidents. See [billing recovery](billing.md) and [security reporting](../SECURITY.md) for current mechanisms and missing pieces.
