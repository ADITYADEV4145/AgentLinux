# Local hardening verification

## Phase 0 baseline — 2026-11-13

Starting published commit: `190c38d`. Original checkout access was restored;
uncommitted Stripe work was inspected and preserved separately. No planning
clone replaced this checkout. Local-only scope: no payments or deployment.

Hardware: Apple M4 Pro, 12 logical CPUs, 24 GiB RAM; macOS 26.5 (25F71),
ARM64. Go 1.26.9, Lima 2.2.1/VZ, Kitty 0.50.0. Disposable guest: Debian
13 ARM64, Xfce/X11 1280×800, guest helper 0.6.0, protocol 2.0,
2 vCPU / 4 GiB RAM / 16 GiB disk. The personal workstation was not modified.
The smaller test disk reflects available host space, not a changed default.

`scripts/measure_local.py` measures an explicitly selected ready test VM.
It never creates or deletes a machine. Mutating workloads require an explicit
`--actions` flag and a disposable `/private/tmp/agentlinux-hardening-*`
namespace. Private captures, logs, capability URLs and machine IDs stay in
ignored evidence; only aggregate observations appear here.

Baseline: 10 CLI screenshot calls, median **404.7 ms**, sampled p95/max
**1026.3 ms**. Three Unicode text actions to changed PNG: **1752.3, 1796.3,
1804.9 ms**. This detects changed image bytes, not successful semantic task
completion or terminal paint latency. Viewer-only 8.684 s run: 0.290 s user
CPU + 0.067 s system CPU, 57,802,752 bytes peak RSS. Six guest captures,
zero capture errors. Subprocesses, guest, Lima, browser and terminal resources
are excluded. Ten samples cannot establish a population tail guarantee.

Reproduce with built binaries and a newly created evidence directory:

```sh
python3 scripts/measure_local.py --cli "$PWD/bin/agent-linux" \
  --lima "$PWD/.tools/lima/bin/limactl" \
  --state-dir /private/tmp/agentlinux-hardening-EXPLICIT-FIXTURE \
  --evidence "$PWD/artifacts/local-hardening/NEW-RUN" --actions
```

Baseline browser viewer exited on SIGINT within the harness deadline and
left the VM running. Native terminal cleanup, paint latency and comparative
connection-reuse measurements remain Phase 1 acceptance work.

## Progress

- Landing export integrated, unified Manrope typography and badge visibility
  change committed separately in `190c38d`; actual desktop/mobile inspection.
- Phase 0 measurement complete. Existing full repository checks passed at the
  site baseline; additional viewer edits are deliberately outside that baseline.
- Phases 1–5 remain in progress until their checks and hardware evidence are
  recorded. No broader reliability or smoothness claim is made.

## Phase 1 implementation and bounded comparison — 2026-11-14

Viewer-owned capture SSH channels retain the same pinned host key, guest
machine identity, hello and nonce checks. Each response also validates boot,
service session and PNG metadata. Only screenshots use this channel; mutations
remain one-shot and are never replayed. A canceled/failed request destroys the
stream; a later observation reconnects. Recycle after 48 captures or 45 seconds
idle, below the guest's 64-request/75-second limits. Each operation is bounded
by six seconds and parent cancellation. `view --capture-transport oneshot`
retains the diagnostic path; standalone `screenshot` is unchanged.

Terminal capture now runs separately from the interaction loop, with one capture
in flight and a single replaceable latest-update slot. Input is processed before
rendering; the reader recognizes exit even while a backend call is waiting.
Two fixed Kitty image IDs alternate: transmission and placement of the replacement
precede deletion of the previous image. Both IDs are freed during cleanup.
Existing cross-process invalidation and demand-driven browser capture remain.

Same disposable host/guest and three synthetic Unicode text actions, matching
one-shot control versus persistent transport (one short run per mode):

| Observation | One-shot control | Persistent |
| --- | ---: | ---: |
| Median guest capture request, 6 samples | 359.6 ms | 293.4 ms |
| Maximum capture request | 371.3 ms | 350.2 ms |
| Median action → changed PNG, 3 samples | 1764.8 ms | 1694.2 ms |
| Viewer user + system CPU | 0.358 s / 8.074 s | 0.328 s / 7.752 s |
| Viewer peak RSS | 56,852,480 bytes | 60,588,032 bytes |

Persistent metrics recorded one connection opened, five reuses and zero failures.
This sample suggests lower capture overhead; it does not establish broad
performance, animation FPS, terminal paint latency, statistically robust tails,
or successful semantic task completion. Warm CLI screenshots remain one-shot
(~396–398 ms median), as intended. The small RSS difference is reported without
attributing it to a specific cause. CPU/RSS exclude SSH, VM and terminal.

`make build test check` covers both binaries, race-enabled Go tests, Python
installer/worker/cost-model checks, vet and whitespace. New doubles test channel
reuse/recycling, nonce/boot/session mismatch, failure without replay, reconnect
on the next observation, cancellation, bounded buffer IDs and replacement order,
slow capture with responsive resize/cancellation and terminal cleanup.

**Hardware gate still pending:** the native computer-use tool refused access to
Kitty, so this change's actual double-buffer painting and native exit/resize
acceptance were not visually repeated. Earlier Kitty acceptance remains historical
and does not validate the new render path. On an unlocked supported Mac, open a
new Kitty window, run `view` for a disposable namespace, use guest MCP input,
resize wide/tall, exercise Ctrl+] in interactive mode (q in view-only), compare
`stty -g` before/after normal exit and interrupts, then check `status` remains
ready. Repeat with `--capture-transport oneshot`. No claim of complete Phase 1
hardware acceptance is made until that checklist is recorded.

## Phase 2 — Named machines and local fleet (2026-11-14)

Implemented private versioned catalog, central CLI selection, short generated
namespaces, explicit existing-namespace registration, identity-pinned viewer/MCP
controllers/configuration, atomic startup reservations and configurable aggregate
budgets. Default machine path and explicit state-directory workflows remain.
Local dashboard is now the default without a cloud config; authenticated local
routes support inventory/readiness, asynchronous bounded startup, create, explicit
stop and on-demand interactive viewers. No cloud implementation is fabricated.

Same M4 Pro / macOS 26.5 / Go 1.26.9 / Lima 2.2.1 / guest 0.6.0 configuration:

- Two actual disposable Debian/Xfce desktops ran concurrently: 2 vCPU / 4 GiB
  each; one existing test disk 16 GiB, one newly provisioned 8 GiB. Existing test
  namespace registration moved no disk and preserved its identity. The personal
  workstation was not started, stopped, reset or modified.
- Both passed distinct machine/service identities, real desktop readiness,
  separate 1280×800 guest PNGs and GUI-saved `café 日本語` text read back through
  each selected guest's command channel. Different per-machine expected strings
  prevented cross-routing from passing.
- Additional 2-vCPU / 8-GiB start refused before VM creation: requested aggregate
  6/10 vCPU and 16/12 GiB. No third VM was booted. The failed creation's private
  metadata remains inspectable. A UI bug initially counted an absent/error machine
  as active; fixed absent runtime labeling and known-active allocation display.
- Real local dashboard showed two ready machines and the enforced 10-vCPU /
  12-GiB defaults. Open selected the research guest; its real browser viewer
  displayed the saved Unicode editor contents. Human Take control / Type text /
  handoff were exercised. Explicit dashboard Stop was independently verified by
  CLI as stopped, then dashboard Start reached the actual readiness handshake.
  A browser automation click timed out around stop confirmation; runtime status
  and refreshed UI established that stop had completed, so it was not replayed.
- Harness failures are retained privately: empty stdout is JSON null, not always
  base64 text; `type` uses a bounded temporary clipboard owner and does not promise
  clipboard persistence; opening an existing editor need not focus it; a save-key
  acknowledgment can precede filesystem completion. Successful verification used
  screenshot-based focus and read-only result checks after the observed save.
  These are not claims that unattended agent recovery is finished.

Automated checks: catalog/name/path validation, unique identities/paths, socket
length and permissions, simultaneous startup reservations, host RAM reserve,
running allocations, unknown-runtime fail-closed behavior, central selection and
conflicts, preserved guest argv, controller identity changes, admission before VM
creation, dashboard authentication, asynchronous startup deduplication, explicit
stop and no unready viewer creation. `make build test check` passed, including all race-enabled Go tests, Python
checks, vet and whitespace validation; `node --check` also passed.

Private screenshots, machine state and capability URLs remain ignored. Native
Kitty visual re-acceptance from Phase 1 remains pending; no new terminal graphics
performance claim follows. Mobile creation dialog and desktop inventory were visually checked with no horizontal overflow.
Saved Unicode files survived the explicit research stop/restart. Both disposable
test VMs were explicitly stopped after acceptance, freeing their RAM; disks remain
private for later recovery tests. Disk checkpoints/helper upgrade, capture worker/full
pointer gestures and observation/recovery hardening are subsequent phases.
