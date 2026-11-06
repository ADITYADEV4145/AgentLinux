# Phase 8 product review and launch decision

**Recommendation: prepare a narrow local source-preview launch for informed Apple silicon developers. Do not activate or advertise a hosted paid service.** This report prepares a decision; it does not authorize or perform publication.

The product direction is “Give your agent a Linux computer. Run it on your Mac or in the cloud.” Today the verified implementation is local Linux on a development Mac. Cloud and sandbox billing code exist, but real-provider and hosted Stripe acceptance are pending. The README now says that directly.

## End-to-end review and fixes

| Journey | Review result / change |
| --- | --- |
| First install | Source build and private offline bundle install are real. External Go/Python/Lima/Kitty prerequisites are explicit; no hidden sudo/shell changes. Package now includes the public image recipe metadata, examples, MCP instructions and cost-model/simulated billing helper referenced by bundled docs. No OS image is bundled. |
| First startup | Persistent local defaults 2 CPUs/4 GiB RAM/24 GiB logical disk, actual guest/desktop handshake. `start` validates an invalid relative/existing metrics destination before starting a VM. Network download/clean-Mac caveats are visible. |
| Agent connection | Two-terminal quick start uses a view-only observer, then configuration in another terminal. No instruction to type chat into stdio. Real Codex and official SDK evidence are distinguished from documentation-only clients. |
| Viewing/human input | Kitty and secure loopback browser paths retained; ownership/exit keys explained. Agent input is denied while a human owns it. New actual widget acceptance is reported with its precise scope, not inferred from unit tests. |
| Common failures | Troubleshooting covers missing runtime/helper, resource/path/permissions, interrupted download, readiness, stale frame/lease, busy operations, viewer recovery and cloud/billing limits. No host input fallback, daily image fallback or uncertainty replay is introduced. |
| Stop/reset/cleanup | Installed-tool review checks persistence across warm boot, clean reset/new identity, exact selected deletion, VM survival after viewer exit and tools uninstall preserving an unrelated sentinel. Reset reprovisions, not a snapshot. |
| Mode selection | Local default never allocates cloud. Private cloud config is explicit; prototype/simulation and missing capabilities stay visible. No cloud clip is included. |
| Ownership/limits | Existing owner/workspace/auth/rate/TTL/idle and billing reservation tests retained; not a production identity or provider hard-cap claim. |
| Usage/billing | Human/JSON CLI, estimate/final/trial/rate/cap/history semantics and recovery docs available. Test-only SDK/signatures are tested; hosted Stripe is pending. No final retail price. |
| Documentation | README reorganized for first use and current matrix; stale no-ownership/no-cloud text corrected. Historical verification remains labeled rather than rewritten. Launch materials, known issues, contributor work and cloud checklist are linked. |
| Reliability/performance | Go branch patched from 1.26.1 to 1.26.9 using an authenticated toolchain download and exact build pin, with tests and hardware revalidation. Methods, sample counts and cache/observer boundaries are in performance.md. |

## Ready for a separate local preview decision

The source repository and unpublished local bundles have a reproducible path through build/install/doctor/real Linux/agent/viewer/stop/reset/delete/uninstall on the documented development Mac. Original code and notices are present, with no host mounts/clipboard sharing by default and explicit NAT/guest trust limits. Prepared local demo uses a synthetic offline project-budget form and an actual image-capable agent; observed application result is verified independently.

This supports an **informed source-based developer preview**, not a broad compatible production binary claim. No clean-Mac, Intel/Windows/local Linux host, security-audited hostile-code sandbox, full-motion desktop video, functioning hosted provider or paid product claim is ready. [Known issues](known-issues.md) lists release-blocking and scope limitations.

## Exact evidence and costs

See [verification](verification.md) for executed checks and [performance](performance.md) for hardware, versions, fresh/warm/screenshot/viewer/reset/resource measurements and limitations. Installed review uses its own private state/tools/Kitty/browser profiles; it does not control the existing workstation, host pointer or host clipboard. Raw recordings/model events and private links remain ignored and require review before sharing. Cloud provisioning has not been measured because no real cloud environment was exercised.

Local software use has no Agent Linux fee; Mac resources/downloads and the chosen model/client may cost money. The [editable model](../examples/cost-model.json) currently estimates total operating cost **before payment fees** of roughly **$0.23 / $0.72 / $4.98 per billable machine-hour** across its low/base/high assumptions. CPU/RAM/storage/egress/provider overhead, trial/orphan waste, control-plane/database and support allocation are included. Percentage/fixed processing fees and a 60% target margin yield required-revenue estimates of about $0.69 / $2.01 / $13.49 per hour; these are **not retail prices or measured economics**. Replace all inputs with account/region/provider quotes and actual workload utilization before any financial offer. [Billing policy and model limits](billing.md).

## Suggested launch sequence

1. Review this report, known issues, draft notes/post/demo and private captures. Keep the initial scope local and source-based. Obtain explicit approval for any announcement, release/tag or uploaded media.
2. Establish a monitored private security/support contact and run a clean supported Mac/account walkthrough. Validate any distributed/quarantined artifact and signing policy before broad binary downloads.
3. Invite a small technical local preview only after approval. Collect reproducible sanitized issues and expand the compatibility/keyboard/browser matrix; do not infer universal performance from this Mac.
4. Separately resolve provider commercial permission, region/template, network/isolation/cleanup/abuse and operational/legal gates. Authorize a bounded provider pilot with an accepted budget and cleanup lag, then measure actual cost and provisioning.
5. Complete Stripe **sandbox** account acceptance and accounting/recovery/retention work. Test credentials/configuration are missing; sandbox validation is already in the requested scope and is not permission to activate live payments.
6. Seek a fresh explicit decision for a public/paid cloud pilot only after its checklist is cleared. Production identity/database/operations and a reviewed live-billing implementation would be new work; no live switch exists today.

## Authorization boundaries

Already authorized and performed: reversible source/docs/tests, isolated local fixture creation/reset/deletion and temporary tool installation/uninstall, read-only repository checks, phase commit/push under the owner's identity. No AI co-author or private credentials are added.

Still requires a separate user decision: publish release/tag/binaries/image or upload/link public demo media; post launch materials or change repository description/settings; enable private reporting/contact integrations; use signing/notarization accounts/credentials; provision paid provider resources/deploy internet infrastructure with an agreed budget; create live payment products, activate charges/subscriptions or offer a paid service. Any engagement/fee for professional legal/privacy review is also an owner decision. Do not interpret this prepared checklist or the source commit as approval for those actions.
