# Local measurements — Phase 8

Observed on 2026-11-05 (America/Chicago; guest UTC crossed into November 6). These are small development-machine samples, not a comparative benchmark, clean-Mac measurement, frame-rate promise or cloud result.

## Hardware and method

Apple **M4 Pro, 12 logical CPUs, 24 GiB RAM**, macOS **26.5 (25F71)**; Lima **2.2.1**, Apple VZ; Kitty **0.50.0**, Chrome **156.0.8078.80**, Codex CLI **0.164.0-alpha.11.1**. Guest: Debian **13 ARM64**, pinned `20261001-2618` genericcloud base, `workstation-5.5.0` Xfce recipe, helper **0.6.0**, protocol **2.0**, single **1280×800 X11/Xvfb** display, **2 CPUs / 4 GiB RAM / 24 GiB logical disk**, NAT. Repository packages can evolve; apt disk bytes and installed package versions are not bit-for-bit pinned. No model performance or every-app compatibility claim follows from this demo.

The main run used **Go 1.26.9**, an unpacked publisher-unsigned private preview bundle (`dirty:true`, based on commit `b797430` plus Phase 8 working changes), installed into a new temporary user-owned prefix. `scripts/launch_acceptance.py` created only its own disposable state, used the actual installed CLI/helper, exercised the real model and actual terminal/browser viewers, then reset/deleted that VM and uninstalled those tools. The existing workstation stayed outside the test namespace. The final source-commit bundle is built/verified separately; signed/downloaded clean-Mac artifact acceptance remains pending.

The public Debian base image was **already cached and checked against the compiled SHA-512**. “Fresh setup” here means a new VM disk and fresh guest provisioning/APT downloads, not an uncached image download, prerequisite installation, source compilation or fresh Mac account. Caches/network and an existing workstation can affect results. All timing uses host `time.monotonic()` around the named operation; readiness requires actual guest/desktop checks. No cloud provisioning was attempted or measured.

## Main Go 1.26.9 run

| Operation | Runs | Observed seconds | Boundary |
| --- | ---: | --- | --- |
| Unpacked bundle install | 1 | 0.10 | Verify/copy/activate local files; excludes build/download/prerequisites/quarantine |
| Fresh guest setup | 1 | 147.02 | New guest through desktop readiness; verified cached base, network APT |
| Warm startup | 3 | 8.73, 8.73, 8.80; median **8.73** | Stopped VM through actual guest/desktop readiness; persistence sentinel checked each time |
| Screenshot | 5 | 0.538, 0.559, 0.538, 0.522, 0.525; median **0.538** | New CLI process → bridge/real PNG → output file, without a viewer competing |
| Viewer action to visible change | 1 | **2.33** | Guest files-app open request → changed central Kitty image; includes app startup, capture and 200-ms polling/window-capture overhead |
| Real Codex form demo | 1 | 39.08 | Client startup/model work/MCP: 13 completed guest calls, five screenshots; actual $300.00 form result verified |
| Reset | 1 | **137.48** | Explicit stopped-machine destruction/reprovision/readiness, cached external base retained; marker absent/new machine ID afterwards |
| Stop after reset | 1 | 4.40 | Request through runtime-confirmed stopped state |
| Selected deletion | 1 | 0.93 | Guarded deletion; only marker/lock remain |
| Tool uninstall | 1 | 0.11 | Only temporary launchers/releases; unrelated sentinel unchanged |

No “instant” or “fastest” claim. Reset is reprovisioning, not a snapshot. Three runs cannot establish a percentile/SLA, and the 200-ms viewer poll limits timing resolution. The files-app visual change is deliberately larger than a cursor/status blink; it is not an isolated transport or input-to-photon measurement.

## Resource and viewer observations

Main terminal viewer ran **51.75 seconds**, consuming **2.07 CPU-seconds** (user + system) with **55 MiB maximum RSS**. These are the Go viewer process only: **exclude Kitty, recording, SSH, Lima/VZ, guest CPU/RAM and other host apps**. The sampled Lima supervisor was about **38.94 MiB RSS / 0.4% CPU** at one moment; that is not total virtualization memory or whole-product CPU. No full-stack CPU profile or sustained workload stress test was performed.

Guest `MemTotal - MemAvailable` was about **461 MiB** before browser tasks and **636 MiB** after the budget task, each one point sample; this is kernel availability accounting, not summed process RSS or a minimum-memory recommendation. The VM is allocated 4 GiB. `du -sk` reported about **3.31 GiB** of allocated blocks under the selected private state after the task (APFS shared extents need not be exclusive physical usage); exclude the shared public base cache, compiler/module caches and tool bundle. A 24-GiB logical disk can grow, and preflight still requires 30 GiB free.

The terminal hub recorded **25 capture attempts, 25 valid frames, eight changed frames and zero capture errors** during that mostly static session. Backend PNG bytes were about 1.84 MB; terminal display bytes about 0.71 MB. Deduplication reduces unchanged terminal repaint/transmission; it does not imply guest capture is free or that every backend transfer is a delta. Adaptive 1–5-second polling, bounded latest-frame work and workload/contention determine behavior. These counts are not a maximum video FPS measurement.

Chrome fallback reported 19 client capture-age samples: median **1 ms**, range **0–1,004 ms**, measured from the server's **capture-completion timestamp to response-header receipt** on the same Mac clock. This explicitly **excludes capture time and browser paint** and is not end-to-end desktop latency. Web image updates, ownership, click, Unicode typing and saving were verified separately against a real guest file; pause stopped capture. No host pointer or host clipboard API was used.

## Earlier comparison and limits

An earlier separate Go **1.26.1** preview run passed the same native real-agent/lifecycle test before the patch update: fresh cached-base setup 137.39 s; warm starts 8.81/8.66/8.73 s; screenshots 0.520–0.536 s; viewer change 2.58 s; reset 128.51 s. It did not include the new web human test. Do not pool these into a compiler-speed comparison: downloads, workload, observer timing and run order are uncontrolled.

Both real Codex runs used 13 completed MCP calls/five screenshots and observed the actual form result. The retained raw evidence is private under ignored `artifacts/phase8/hardware1` and `hardware2`, including failures/warnings, per-operation output, timestamps and process metrics. The native recording samples the owned Kitty window approximately every half-second with original timing, not the whole Mac desktop or full-motion output. No benchmark/recording is fabricated. Model/client warnings remain in private logs; success is grounded in guest title/pixels and file oracles, not just client exit 0.

Missing: uncached first-image download, clean/quarantined install, broad Mac/terminal/browser/IME matrix, full-stack CPU/power/resource peaks, long-duration concurrency/load, real cloud provisioning/egress/cost and hosted Stripe measurements. [Launch gates](launch-readiness.md) and [known issues](known-issues.md) retain these limits.
