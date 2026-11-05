# Implementation roadmap

Phase 0 • 2026-11-04. Only Phase 0 is complete. Later phases describe work to do, not implemented features. Each phase ends with a buildable tree, appropriate checks, updated verification evidence and a local commit reviewed for secrets/personal files. No automated AI contributor or co-author attribution. Pushes, releases, paid allocation and public deployment are separate actions.

## Phase 0 — Research and specification (complete)

Outcome: product scope, runtime/desktop/channel/viewer choices, local/cloud API boundaries, source-backed comparisons, license inventory, risks and real-Mac acceptance plan. Deliverables: README, product, architecture, research, roadmap, verification and four ADRs. No executable implementation or infrastructure.

Exit: a concrete local path is selected; remaining uncertainty is a set of bounded validation experiments with failure decisions rather than an unspecified architecture. Cloud contracts distinguish unsupported features and persistent/ephemeral profiles. No paid resources or deployment. Documentation checks and commit evidence are recorded in verification; hardware claims remain unproven.

## Phase 1 — Prove the guest desktop on a real Mac

Implement only the smallest working path:

1. Initialize Go module with official MCP Go SDK candidate v1.8.0 only when needed, validated toolchain pin, original-code LICENSE, dependency inventory, build commands and baseline CI. Review exact dependency licenses/transitives; do not vendor unrelated projects.
2. Implement `doctor` read-only checks and resource plan: native architecture/macOS/runtime version, virtualization availability, CPU/RAM/disk, image size/expansion/headroom, network prerequisites. Unknown resource availability must not silently pass.
3. Create a private per-machine Lima namespace, explicit plain/VZ config and verified dated Debian ARM64 image. Validate resolved mounts, proxy values, credentials and forwarding. Pin selected runtime/image after compatibility test.
4. Provision authenticated Xvfb, Xfce, browser, fonts, capture/input tools and root-owned Go helper under a separate non-sudo desktop user. Generate/pin guest host key through owned provisioning; establish strict SSH application channel. Validate root/bootstrap boundaries.
5. Minimal CLI create/start/status/screenshot/input/stop on one owned disposable machine. Use reviewed guest test fixtures rather than an arbitrary exec tool. Run the first E2E scenario in verification. Do not build terminal graphics before a valid screenshot and verified input exist.

Exit: three consecutive real boot/control/stop runs, PNG correctness and Unicode task evidence, host pointer/clipboard unchanged, no host mounts/agent credentials, bounded failure behavior, successful stop. Build/unit checks cover config restrictions, request/image bounds, coordinate mapping, argument injection and timeout/cleanup. Runtime/image/host details and performance observations are recorded. No VM startup or minimum-RAM claims without these results.

If 13 GiB free is insufficient, complete source/build/schema work and report image/VM testing blocked until a user-selected volume or additional free space is available. Do not delete user data or quietly shrink requirements to make a test fit. If VZ boot or strict SSH enrollment fails, preserve logs, test a deliberate pinned-image/config correction and revise ADR before changing runtime. Offline networking remains unsupported until proven.

## Phase 2 — Coherent lifecycle and MCP

Implement broker/private Unix socket, atomic machine/operation state and reconciliation, API schemas, readiness, IDs/errors, capabilities, session/frame generations, input leases, forced guest command, cancellation and bounded deduplication. Add official SDK stdio tools bound to one selected machine. Keep stdout protocol-only. Reject unknown capabilities, stale sessions, unauthorized ownership and malformed data before sending actions.

Exit: real MCP client observes/clicks/types/verifies the demo fixture, older protocol negotiation tested through SDK, stop/restart persistence verified, guest/helper/broker crash and unknown-action-outcome paths exercised. Contract tests use explicit test doubles for lifecycle failure cases; they do not count as VM evidence. CLI examples become actual documented commands only when passing. No cloud success placeholder.

## Phase 3 — Visible and interactive desktop

Implement Kitty capability/pixel queries, bounded PNG refresh, keyboard/mouse/paste, shortcut palette and terminal cleanup. Add authenticated loopback browser fallback sharing the same frame/input operations. Human takeover preempts agent input; viewer exit does not stop the VM. Record exact Kitty/browser versions.

Exit: watch/control full demo in Kitty and browser; unsupported terminal/redirected stdout falls back cleanly; test resize/Retina mapping, letterboxing, slow renderer/backpressure, Ctrl+] and signal cleanup, disconnect/restart, token expiry, Origin/CSRF denial and two viewers competing for control. Publish only measured frame rate/latency and known keyboard/accessibility limitations.

## Phase 4 — Local MVP hardening and distribution

Measure CPU/RSS/disk/capture/viewer behavior across at least two recorded Apple silicon configurations if available; derive a supported host/version/resource matrix. Test image integrity failures, package install interruption, disk pressure, shutdown timeout and upgrade/recovery without data loss. Validate offline mode or document it unsupported; add network restriction only with actual enforcement/evidence.

Produce reviewed installation/build/uninstall paths, checksums, SBOM, third-party notices, image package/source compliance and signed/notarized artifacts where applicable. A development build is not a publicly published release. Retain no host grants and no telemetry by default. No release/package publication without authorization.

Exit: local acceptance reproducible on a clean Mac, accurate diagnostics, resources/version minimums justified by measurements, no outstanding critical data-loss/control-path defects, release artifact audit complete. If only one hardware configuration is available, document that limit instead of claiming broad support.

## Phase 5 — Cloud adapter validation (future; authorization required for paid tests)

First do no-cost contract/SDK/license review: region, architecture, image customization, auth/viewing, resource hard limits, stop/persistence, TTL, lookup/idempotency and cleanup behavior for E2B/Daytona. Select one provider only after evidence. Add a real adapter, OIDC/workspace authorization, private viewing gateway, usage events and resource reconciliation. Reject unsupported profile requirements before allocation.

Paid experiments and public endpoints require explicit authorization before execution. Test identity/cross-workspace denial, token expiry/revocation, concurrent control, partial create, provider outage, idle/hard lifetime, persisted stop/restart and verified cleanup. Compute TTL and storage retention are distinct. No billing activation or multi-region orchestration.

Exit: one-region pilot passes common contract where advertised, records capability differences/limits and measured costs, and reconciles all experiment resources to deleted or intentionally retained ownership records. Update roadmap honestly if a provider cannot support the contract.

## Major risks and validation ownership

| Risk | Earliest experiment / phase | Decision on failure |
| --- | --- | --- |
| Disk capacity or unavailable RAM facts | Host preflight, Phase 1 | Refuse allocation; complete independent source work; request storage/hardware prerequisite |
| ARM64 UEFI image or plain-mode VZ/vsock mismatch | Boot + strict SSH enrollment, Phase 1 | Deliberate pinned image/runtime correction; reopen ADR before replacing runtime |
| Lima leaks mounts, proxy credentials, forwards, identity metadata | Resolved config + guest/runtime inspection, Phase 1 | Fail closed, remove inherited config; never waive mount/credential default |
| NAT reaches host/LAN | Probe only owned test endpoints, Phase 1/4 | Explicit limitation and consent; no restricted-network label without enforcement |
| Unicode paste/X11 focus failure | Real Chromium/editor/terminal fixtures, Phase 1/2 | Restrict advertised paste modes; surface errors and require verification |
| Guest parser/image resource exhaustion | Bounds/malformed cases, Phase 1/2 | Reject/drop and disconnect; never allocate unbounded buffers |
| Duplicate actions after timeout/reboot | Lost acknowledgement + generation tests, Phase 2 | OUTCOME_UNKNOWN; fresh screenshot, no automatic mutation retry |
| Viewer is too slow or changes coordinates | Retina/resize/backpressure measurements, Phase 3 | Reduce refresh; on-demand mode; profile before optimizing codec |
| Weak desktop accessibility | Keyboard/zoom/status testing, Phase 3 | Document raster limitation; plan AT-SPI capability separately |
| License or redistribution restrictions | Exact dependency/image review, Phase 1/4 | Replace dependency or satisfy obligations before distribution |
| Provider capabilities differ from local | No-cost review then authorized tests, Phase 5 | Report unsupported features; no fake parity or unapproved costs |

## End-of-phase reporting

Report actual outcome, key files, runnable commands, automated results, real hardware/VM results, skipped tests and reasons, limits/risks, git status and commit, exit-criteria verdict and exact next action. Keep docs accurate after scope changes. Commit only the completed phase's reviewed files; do not commit runtime state, credentials, recordings or personal paths. Keep API-changing phases compatible with a documented version policy. Do not mark later phases complete from mock-only results.

Next action: on Phase 1 authorization, implement read-only host/resource preflight and restrictive Lima manifest validation before downloading any image or installing a runtime.
