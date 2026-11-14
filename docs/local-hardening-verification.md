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
