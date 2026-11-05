# Roadmap

Updated 2026-11-05. Commit after every completed phase, reviewing staged code/docs for secrets and personal data; no AI contributor/co-author attribution. Paid resources, publishing and deployments require separate authorization.

## Phase 0 — Complete

Product specification, source-backed architecture comparisons, local/cloud boundaries, guest trust model, license inventory and validation plan. Original commit afaa74b remains intact. Research facts are documentation/source observations, not hardware tests.

## Phase 1 — Local lifecycle and guest communication

Implemented the user's narrower Phase 1 scope: CLI doctor/up/status/stop/logs, private configuration/state, CPU/RAM/disk validation, pinned image download or local QCOW2 input, idempotent provisioning, generated keys and strict application SSH pinning, systemd non-sudo guest service, framed hello/health/probe, honest readiness, interrupted-state reconciliation, locking and bounded sanitized logs.

Real Mac boot, guest operation, stop/restart, identity/session changes, persistence and wrong-key rejection have evidence. Automated race tests and vet cover protocol, state failures, paths, resources, parsing, timeouts, output limits and config/image safety. scripts/acceptance.py reproduces a smaller lifecycle acceptance test and cleans only its owned temporary machine. Full desktop acceptance from the original Phase 0 roadmap has **not** happened and is no longer a Phase 1 exit claim.

Corrections from real evidence: compressed provisioning under a 3 MiB budget; selected-field Lima inventory; explicit loopback SSH fallback. See ADR 0005. Exact final verification/cleanup status is recorded in verification.md.

## Phase 2 — Guest desktop/control slice (not started)

Provision Xvfb/Xfce/Chromium under the non-sudo guest account. Add bounded PNG capture, fixed-display XTEST input, guest-only Unicode paste modes, frame/session generations and action errors. Test actual browser typing, coordinates, input cleanup, service/display restarts and the host pointer/clipboard boundary. Keep guest input implementation out of the host executable path. No arbitrary execution tool without a separate trust/timeout/output policy.

Exit: actual screenshot → action → screenshot task passes on the Mac; Unicode/focus limitations and resource measurements documented. Recheck image/package licenses and browser sandbox. Commit tested work and update docs.

## Phase 3 — Shared API, MCP and observable lifecycle

Implement the broker/private Unix socket and shared application operations, stable error/capability schemas, input ownership/takeover and operation reconciliation. Add the official maintained MCP SDK with explicit version pin only when integrating it. Validate a real client, older supported protocol negotiation, cancellation, stale frames and service failure. Do not count backend doubles as VM evidence.

## Phase 4 — Terminal visibility and local MVP hardening

Kitty first, bounded PNG refresh and real keyboard/mouse mapping; authenticated local browser fallback; human takeover; terminal mode cleanup. Measure actual performance/scaling/backpressure. Establish a host/runtime support matrix, recovery/update behavior and installer/uninstall plan. Add distributor signature verification and complete artifact/image license/SBOM/notices review before distribution. No release publication without authorization.

## Phase 5 — Managed cloud pilot (not started)

No-cost provider/license/contract review first. Evaluate one fixed region/provider for real isolation, auth, quotas, private viewing, persistent/ephemeral profiles, TTL, cleanup and metering. Only allocate paid resources after explicit authorization. Local/cloud operations share logical contracts with capability differences; no fake cloud implementation or silent local fallback. Cross-workspace denial, ambiguous allocation, orphan cleanup and storage retention must be tested before public use. Billing activation remains separate.

## Outstanding risks

NAT reaches host/LAN; offline/egress-restricted modes are not validated. Lima bootstrap management SSH is trusted and weaker than application pinning. Provisioning data retains guest host-key material in private state. Detached image signatures and broad host compatibility are unverified. No graphical desktop/MCP/viewer exists yet. Stronger hostile-guest security, distribution compliance and performance claims require further evidence.

Next action after Phase 1: implement and validate the guest desktop/capture/input slice on the established VM/service path. End every phase with actual outcome, key files, commands, automated/hardware/skipped verification, limits, git status/commit, exit verdict and exact next action.
