# Roadmap

Updated 2026-11-05. Commit after every completed phase, reviewing staged code/docs for secrets and personal data; no AI contributor/co-author attribution. Paid resources, publishing and deployments require separate authorization.

## Phase 0 — Complete

Product specification, source-backed architecture comparisons, local/cloud boundaries, guest trust model, license inventory and validation plan. Original commit afaa74b remains intact. Research facts are documentation/source observations, not hardware tests.

## Phase 1 — Local lifecycle and guest communication

Implemented the user's narrower Phase 1 scope: CLI doctor/up/status/stop/logs, private configuration/state, CPU/RAM/disk validation, pinned image download or local QCOW2 input, idempotent provisioning, generated keys and strict application SSH pinning, systemd non-sudo guest service, framed hello/health/probe, honest readiness, interrupted-state reconciliation, locking and bounded sanitized logs.

Real Mac boot, guest operation, stop/restart, identity/session changes, persistence and wrong-key rejection have evidence. Automated race tests and vet cover protocol, state failures, paths, resources, parsing, timeouts, output limits and config/image safety. scripts/acceptance.py reproduces a smaller lifecycle acceptance test and cleans only its owned temporary machine. Full desktop acceptance from the original Phase 0 roadmap has **not** happened and is no longer a Phase 1 exit claim.

Corrections from real evidence: compressed provisioning under a 3 MiB budget; selected-field Lima inventory; explicit loopback SSH fallback. See ADR 0005. Exact final verification/cleanup status is recorded in verification.md.

## Phase 2 — Real desktop/control slice (complete)

Implemented managed Xfce/Xvfb/Chromium, current PNG capture, recent-frame coordinate input, Unicode guest clipboard paste, fixed-app launch, common local machine interface, and arbitrary guest exec with cgroup timeout/disconnect/output cleanup. Added direct verification CLI commands and a reproducible fresh-VM acceptance script. Real UI, exact saved UTF-8 text, browser, mouse, stale frames, crashes and reboot/persistence passed. Host pointer/clipboard metadata remained unchanged. See verification.md and ADR 0006 for the limitations and corrections discovered on hardware.

Not claimed: Wayland/multiple displays, broad application paste parity, a strong per-command malicious-code sandbox, implicit Phase 1 upgrades, MCP or live viewing. No paid infrastructure was allocated.

## Phase 3 — MCP and agent integration (complete)

Implemented official Go SDK v1.8.0 stdio, ten real local tools, strict schemas, structured/image/error outputs, observed-frame input, deadlines/cancellation and immediate-busy serialization. Added agent instructions and configuration printing for Codex, Claude Code and VS Code without changing global settings. Official SDK protocol tests, multiple supported protocol revisions, real stdio/VM acceptance and actual Codex screenshot → input → verify acceptance passed. Claude Code and VS Code remain explicitly untested as agent integrations. See verification.md and ADR 0007.

The private broker and persistent ownership/takeover lease remain deferred beyond the view-only Phase 4: existing file locks plus per-server serialization satisfy this slice. No nonfunctional cloud endpoint is advertised.

## Phase 3.5 — Complete desktop, apps and maintenance (complete)

Extend the working Debian/Xfce stack: launcher/files/settings targets, focused existing windows, two real distro browsers, a small development baseline, explicit guest package installation/updates, persistence documentation and a Go/Rust decision. No language rewrite, host sharing, privileged MCP installer, cloud backend or live terminal viewer. A clean-image real Mac run passed launcher/file management/settings/terminal, exact Unicode guest clipboard, both browsers, real MCP, package/network failure and persistence checks. Exact implementation and real acceptance verdict is in verification.md; tested/untested browser architectures are in browsers.md.

Phase 4 inputs remain actual guest scrot PNGs at 1280×800 scale 1 and the existing X11/MCP control contract. Phase 4 below adds rendering, resize/cleanup and measured observations; direct human input/takeover remains deferred. Keep Go for the first viewer unless profiling identifies a concrete reason for a narrow native/Rust component.

## Phase 4 — Terminal viewer (complete on the tested Mac/Kitty configuration)

Implemented Kitty direct PNG rendering, active query/real pixel sizing, raw terminal cleanup, bounded capture/backpressure/unchanged-frame suppression, and secure live loopback fallback. View-only human observer; agents control through MCP. The fallback and actual VM/MCP changes/restart/pause/auth checks passed. Actual Kitty 0.50.0 visible Linux/MCP changes, wide/tall/tiny resizing, VM stop/restart reconnect, q/Ctrl+C/SIGTERM cleanup and unchanged VM lifecycle passed on macOS 26.5. A private sampled test-window recording and reusable opt-in harness preserve that evidence; see viewer.md and verification.md.

Future work: optional human input with ownership/takeover, measured optimizations, broad terminal/host support matrix, and distributor signature/installer/SBOM hardening. No fixed-FPS or production-ready claim. No release publication without authorization.

## Phase 5 — Local hardening and developer release candidate (complete at tested developer scope)

The user's Phase 5 scope is local product hardening, superseding the earlier planned cloud phase. Implemented an explicit isolation/trust audit, bounded one-way file copies, stopped-machine identity/confirmation guards for reset/deletion, private offline bundle installation/atomic activation/uninstall, versioning, original/third-party notices, contribution/security guidance and a first-run/troubleshooting matrix. Real installed-bundle guest copy/MCP/view/persistence/reset/delete and tools uninstall passed; packages were built locally with no release/tag publication or cloud allocation. Automated boundary tests and actual timings are recorded. Clean-Mac/Gatekeeper acceptance, wider authenticity/signing/reporting infrastructure and full image distribution review remain explicit future gates. See release.md and verification.md for exact results and limits.

## Desktop quality — GNOME integration gate (recommended next local scope)

Ubuntu 26.04.1 LTS ARM64/GNOME 50 is the recommended modern full-desktop target after comparison with Fedora 45/GNOME 51 and the current Debian/Xfce guest. A separate Mac VM proved real Wayland capture, core app windows, physical-key/modifier input and Unicode clipboard paste. No production guest swap has occurred. Complete the managed-session, software-store, stable capture/input adapter, lifecycle and real MCP/Kitty/web acceptance in desktop-quality.md before changing defaults. Keep current Debian support during migration; do not optimize away the full desktop solely to reduce resources.

## Future managed cloud pilot (not started)

No-cost provider/license/contract review first. Evaluate one fixed region/provider for real isolation, auth, quotas, private viewing, persistent/ephemeral profiles, TTL, cleanup and metering. Only allocate paid resources after explicit authorization. Local/cloud operations share logical contracts with capability differences; no fake cloud implementation or silent local fallback. Cross-workspace denial, ambiguous allocation, orphan cleanup and storage retention must be tested before public use. Billing activation remains separate.

## Outstanding risks

NAT reaches host/LAN; offline/egress-restricted modes are not validated. Lima bootstrap management SSH is trusted and weaker than application pinning. Provisioning data retains guest host-key material in private state. Detached image signatures and broad host compatibility are unverified. MCP, desktop/control and a live view-only viewer are implemented. Stronger hostile-guest security, distribution compliance and performance claims require further evidence.

Next action: before wider binary distribution, run the clean-Mac checklist, establish a monitored private reporting channel and choose an authenticated release/signing process. Optional human input needs an ownership design; any cloud pilot starts only under separately chosen scope. End every phase with actual outcome, key files, commands, automated/hardware/skipped verification, limits, git status/commit, exit verdict and exact next action.
