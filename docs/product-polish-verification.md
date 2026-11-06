# Product polish and console verification

Follow-up to Phase 8, 2026-11-05 US local time (some guest/browser UTC timestamps are 2026-11-06). This is a development implementation, not authorization to publish or charge users.

## Implemented

- Interactive bare `agent-linux` dispatches the existing start/resume + human viewer path. Noninteractive bare invocation remains help. Existing explicit `start` flags remain supported.
- Private atomic cross-process mutation revision resets viewer static backoff. No host desktop or clipboard fallback.
- Authenticated identity/workspace fleet inventory, immutable per-session client selection, explicit fresh-deployment concurrency two, with existing lifecycle/financial admission limits retained.
- Responsive embedded original landing page and loopback console. Real backend create/list/stop/usage routes, separate live web viewers and existing input leases. No fabricated live cloud fleet, account login or raw card form.
- Proposed $0.59/$1.19 catalog plus repeatable scenario stress evaluation. No configured billing rates, Stripe products or production flags are changed by this catalog.

## Automated checks

`make build test check`: host/guest builds, complete Go race suite, installer tests, cloud worker tests, cost-model tests, Go vet and whitespace checks. New coverage verifies console capability/Host/Origin/query checks, empty state, no credential leakage, owner-scoped list, two simultaneous sessions and rejected third, independent viewer capability and actual provider PNG route, explicit stop confirmation, viewer cleanup, invalid bodies, revoked identity, cross-process invalidation, and noninteractive/help parsing. Provider tests here use a simulated adapter, not real cloud compute. HTTP test listeners require the development test environment to allow loopback sockets.

## Actual Mac and guest acceptance

Host: Apple M4 Pro, 12 logical CPU cores, 24 GiB RAM, macOS 26.5 (25F71). Reviewed Lima 2.2.1/VZ. A fresh, uniquely named `/private/tmp` namespace was used; the existing personal workstation was not selected.

Ran the real equivalent of:

```sh
agent-linux start --state-dir "$NEW_DISPOSABLE_ROOT" --lima /absolute/reviewed/limactl --web --view-only
agent-linux screenshot --state-dir "$NEW_DISPOSABLE_ROOT" --lima /absolute/reviewed/limactl --output /absolute/private/desktop.png
agent-linux open --state-dir "$NEW_DISPOSABLE_ROOT" --lima /absolute/reviewed/limactl editor
agent-linux type --state-dir "$NEW_DISPOSABLE_ROOT" --lima /absolute/reviewed/limactl 'Agent Linux: hello, café 日本語 👋'
```

Observed download of the 337,510,400-byte pinned Debian image, provisioning, actual guest handshake, desktop readiness and a live 1280×800 PNG. Browser inspection showed the real Xfce desktop and Mousepad containing the ASCII/Unicode string. The separate CLI guest mutations produced a private viewer revision; the already-open viewer displayed the changed guest. Closed the viewer with Ctrl+C, explicitly stopped the selected machine and used guarded delete with its exact machine ID. The delete plan included `viewer-change.json`; no other VM namespace was selected.

This was one fresh source-build/web-viewer run. The bare-no-flags interactive branch was not independently booted against the user's default namespace. Native Kitty resize/input/cleanup and actual Codex/MCP acceptance retain their earlier Phase 8 evidence; they were not rerun in this follow-up. No new latency/FPS/CPU benchmark claim is made. Existing measurements in `performance.md` remain hardware-specific prior observations, not measurements of these changes.

## Browser/console acceptance

Used the first-party browser control surface against localhost only. Tested 1440×1000, 552×752 and 390×844 viewports. Landing and console rendered without horizontal page overflow at measured desktop/mobile widths. Narrow layout stacks pricing and exposes a compact workspace navigation. Verified real DOM text, explicit simulation/pricing labels, machine creation dialog, two separate development sessions, readiness updates and disabled create at concurrency two. Server integration tests verify stop authorization and viewer capture without irreversible browser actions. Temporary viewport override was reset.

Private screenshots under ignored `artifacts/product-polish/` include landing desktop/mobile, console desktop/mobile, a real guest PNG and real guest Unicode viewer result. They are not added to Git or uploaded to a media host. The landing's code-native desktop picture is labeled an illustration. Attempting a separate Chrome extension preview hit a browser-control error; successful layout and action testing used the in-app browser. No Chrome extension acceptance claim is made.

## Remaining gates

No paid compute, public website, release artifact upload, live billing or production payment product was created. The multi-session cloud run used the fake provider; no real multi-VM cost/isolation/performance evidence exists yet. Public authentication/TLS/database/abuse controls, provider terms/permission, real cloud acceptance, validated profit/egress policy, actual Builder profile, signed distribution and authorization remain launch gates. These changes do not make a substantiated market-leadership claim. Persistent capture and improved terminal rendering remain the next performance experiments documented in the comparison.
