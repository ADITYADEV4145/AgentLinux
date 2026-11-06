# Known issues and launch gates

This list describes the current source preview. It is not a claim that a security audit, clean-Mac review or hosted service exists. Phase 8 checked the GitHub open issue list and found none; this document is a prepared backlog, not remotely created issues.

| Scope | Issue / limitation | Impact and next step |
| --- | --- | --- |
| Broader local launch | Only one existing developer Mac is available; clean/quarantined/new-account install and other Mac versions are not accepted | Complete the [clean-Mac checklist](troubleshooting.md) before broad binary distribution; source-preview users need the exact prerequisites |
| Distribution | No publisher signing/notarization, authentic binary download/release process or OS image redistribution approval is established | Keep source/local builds; obtain explicit publication/signing authorization and review artifact provenance/licenses |
| Security/support | Durable monitored private reporting availability is unconfirmed (Phase 8 read-only API check returned 404); no independent audit/SLA | Owner must establish/verify a private route and support/incident responsibility before broader launch |
| Input | X11 applications share a guest session; lease only coordinates cooperating frontends | Do not describe X11/lease as an adversarial app security boundary |
| Terminal | Kitty 0.50.0 is the narrow tested target; limited legacy shortcuts, no continuous human drag/IME or tmux/screen guarantee | Expand an explicit versioned terminal/input matrix; use the documented browser fallback |
| Browser fallback | Chrome-only observed scope; broader Safari/accessibility/physical keyboard/IME coverage is pending | Verify focus, labels, contrast and keyboard-only input across browsers; report current human-widget acceptance in verification |
| First setup | Pinned image/APT downloads can stall/fail; no auto-latest fallback or full offline/isolation mode | Keep bounded deadlines and diagnostics; measure truly uncached/new-machine networking separately; never bypass integrity to make a demo pass |
| Updates | Existing disks retain sealed helpers/recipe; reset is destructive and reprovisions | Document explicit maintenance/export/reset; do not silently migrate or call reset a snapshot |
| Data transfer | One regular file up to 4 MiB, no overwrite or automatic archive extraction | Suitable for bounded demo/project transfers, not live host project synchronization |
| Automation reliability | Initial Chromium fresh-profile launch failed in an earlier run; root cause was not established | Failure remains in historical evidence; observe/retry deliberately, never silently claim success or auto-replay uncertain actions |
| Cloud | E2B provider, network/metadata enforcement, region/template/cost and commercial permissions remain unaccepted | No cloud demo or paid/public offering until the [cloud gates](cloud-release-checklist.md) are cleared |
| Billing | Real Stripe-hosted sandbox completion/portal/invoices/webhook forwarding have no credential-backed acceptance | Finish test-mode acceptance with configured credentials; no live switch or automatic collection is provided |
| Billing storage/ops | Single-process bounded JSON ledger/intents, manual recovery/archival and no external alerts | Transactional store/queue, reconciliation, backup/restore, retention, on-call and privacy/tax/refund review are production gates |
| Performance | Small hardware-specific samples and cached-image provisioning | No instant/fastest/video-FPS/whole-product CPU claim; read [measurement boundaries](performance.md) |

Fixed during Phase 8: stale README/MCP ownership/cloud claims, confusing sequential first-demo instructions, invalid viewer metrics destinations checked only after VM startup, missing support examples/scripts in the local bundle, and an older Go patch pin. Those fixes do not clear the pending gates above.
