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

The private broker and persistent ownership/takeover lease are deferred to Phase 4: existing file locks plus per-server serialization satisfy this slice. No nonfunctional cloud endpoint is advertised.

## Phase 3.5 — Complete desktop, apps and maintenance (complete)

Extend the working Debian/Xfce stack: launcher/files/settings targets, focused existing windows, two real distro browsers, a small development baseline, explicit guest package installation/updates, persistence documentation and a Go/Rust decision. No language rewrite, host sharing, privileged MCP installer, cloud backend or live terminal viewer. A clean-image real Mac run passed launcher/file management/settings/terminal, exact Unicode guest clipboard, both browsers, real MCP, package/network failure and persistence checks. Exact implementation and real acceptance verdict is in verification.md; tested/untested browser architectures are in browsers.md.

Phase 4 inputs remain actual guest scrot PNGs at 1280×800 scale 1 and the existing X11/MCP control contract. Live rendering, terminal input/resize/cleanup, measured throughput and human/agent takeover are still Phase 4 work. Keep Go for the first viewer unless profiling identifies a concrete reason for a narrow native/Rust component.

## Phase 4 — Terminal visibility and local MVP hardening

Kitty first, bounded PNG refresh and real keyboard/mouse mapping; authenticated local browser fallback; human takeover; terminal mode cleanup. Measure actual performance/scaling/backpressure. Establish a host/runtime support matrix, recovery/update behavior and installer/uninstall plan. Add distributor signature verification and complete artifact/image license/SBOM/notices review before distribution. No release publication without authorization.

## Phase 5 — Managed cloud pilot (not started)

No-cost provider/license/contract review first. Evaluate one fixed region/provider for real isolation, auth, quotas, private viewing, persistent/ephemeral profiles, TTL, cleanup and metering. Only allocate paid resources after explicit authorization. Local/cloud operations share logical contracts with capability differences; no fake cloud implementation or silent local fallback. Cross-workspace denial, ambiguous allocation, orphan cleanup and storage retention must be tested before public use. Billing activation remains separate.

## Outstanding risks

NAT reaches host/LAN; offline/egress-restricted modes are not validated. Lima bootstrap management SSH is trusted and weaker than application pinning. Provisioning data retains guest host-key material in private state. Detached image signatures and broad host compatibility are unverified. MCP and desktop/control are implemented; no live viewer exists yet. Stronger hostile-guest security, distribution compliance and performance claims require further evidence.

Next action after Phase 3.5: implement measured terminal visibility with explicit human/agent ownership and takeover, while preserving the tested guest and MCP contracts. End every phase with actual outcome, key files, commands, automated/hardware/skipped verification, limits, git status/commit, exit verdict and exact next action.
