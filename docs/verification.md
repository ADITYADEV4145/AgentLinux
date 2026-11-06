# Verification evidence

Phase 0 inspection: 2026-11-04. Phase 1 verification: 2026-11-05 (America/Chicago).

## Phase 5.5 — Evidence-based workstation selection (2026-11-05)

Selected **Debian 13.7 ARM64 / Xfce 4.20**, recipe **workstation-5.5.0**; retain existing VZ/X11/MCP/viewing stack and sealed old machines. [ADR 0012](decisions/0012-agent-workstation.md) supersedes the GNOME-first recommendation. [Workstation report](workstation.md) records official sources, exact versions, candidate/task matrix, resource scopes, genuine-image observations and reproducible commands.

Initial tree was clean at 58ac142; GitHub had no open issues. Inspected instructions, README, implementation/tests, architecture/roadmap/verification, provisioning, helper/MCP, viewer and packaging. All experiments used new private `agentlinux-phase55-*` namespaces; the default/personal VM path was never controlled. No paid infrastructure, release, deployment or binary/image publication.

Actual Apple M4 Pro/12-CPU/24-GiB macOS 26.5/VZ guests: equal 4 CPUs/8 GiB/40 GiB, 1280×800, same Debian image/kernel/Firefox/Chromium and deterministic offline task. Final polished Xfce **2/3 complete MCP runs**, including one recorded initial Chromium launch failure before a successful explicit retry; repeatable Plasma **2/2**. Both passed exact ASCII/CJK/emoji save/guest clipboard, GUI folder creation, terminal keyboard command, both browser forms, semantic drag/scroll and focus switching. Three actual warm-ready starts each were ~8.8 s. Xfce/Plasma post-task memory pressure ~1,054/1,368 MiB and sparse disk ~3.48/4.37 GiB; these are small samples, not certified minima or universal rankings. No whole-stack CPU or display-FPS claim.

Failures were retained: early test driver mishandled empty stdout; manual KDE needed KIO runtime writes and reverted to Xfce on stored provisioning replay, invalidating its restart numbers; a fresh repeatable research recipe corrected this. First wallpaper was root-only readable; new provisioning explicitly sets non-secret SVG 0644. An extra editor launch opened an untitled tab during a file-open transition; the corrected client waits for the requested title before typing and keeps the exact byte oracle. Chromium's initial fresh-profile launch failure remains unexplained, visible and not automatically replayed. Do not hide failed runs or infer universal reliability from two passes.

Real **Codex CLI 0.164.0-alpha.11.1** used 10 guest MCP calls and five screenshots to navigate and accurately describe the actual page. This is a model-driven run, separate from deterministic official-SDK tests and unit doubles. Mac pointer/clipboard-change counter were unchanged in read-only observations around that run. Actual **Kitty 0.50.0** displayed MCP-driven changes, resized narrow/wide/tall/tiny/restored, reconnected over VM restart and restored terminal state on q/Ctrl+C/SIGTERM; VM remained ready. Native images inspected, 51 sampled window captures/43.97-s recording. Main Go viewer: 1.47 CPU seconds/47.03 s, ~67.5-MiB max RSS, excludes terminal/SSH/VM/recording. Five expected capture errors occurred during injected restart/contention. No direct human guest input is implemented.

Settings rendered; fontconfig resolved Noto Sans/Mono. Another stop/up preserved all saved document hashes and a deliberately customized font, retained machine identity and changed guest session; restored initial font afterward. `make build test check` passed; new first-run recovery/preservation tests use a fake settings daemon, while the opt-in workstation test uses the actual SDK/VM. Existing protocol, paths, resource bounds, installer, concurrent/restart and terminal cleanup coverage remains. Final doc/secret/staging checks and phase commit/CI are reported at completion.

Known limits: one development Mac, no clean-Mac/wider terminal/application matrix, unresolved initial Chromium launch, no IME/audio/GPU/multi-display certification, no native Wayland production backend/accessibility-tree tool, view-only viewer, no cloud. Explicit copies, guarded reset/delete, non-sudo helper, no host mounts/clipboard and NAT host/LAN reachability remain unchanged. Private evidence is ignored under artifacts/phase55; cleanup removes only the test namespaces' disks/credentials and leaves ownership markers/locks. No personal VM/data migration is performed.

**Cleanup complete:** all five recorded disposable machines were stopped/deleted after exact identity/ownership checks; only marker/lock remained, with disks and credentials gone. The private Kitty process/socket was closed. Final hardware exit: chosen workstation passed real guest, MCP, model, viewer and persistence checks on this Mac, with the stated initial-browser limitation.

## Earlier modern desktop quality evaluation — 2026-11-05 (historical, superseded by Phase 5.5)

**Earlier recommendation: Ubuntu 26.04.1 LTS ARM64 / Ubuntu GNOME 50. Superseded by ADR 0012; the feasibility observations below remain valid.** Official release/support/ARM64/browser sources, Fedora 45/GNOME 51 comparison, exact installed app versions and migration acceptance are in [desktop-quality.md](desktop-quality.md). This is an evaluated next target, not a claim that the RC already supports GNOME through MCP/viewer.

Created a separate private Lima 2.2.1/VZ namespace on the same ARM64 macOS 26.5/24-GiB Mac: 4 vCPUs, 8 GiB RAM, 40 GiB disk, no mounts, host clipboard/agent forwarding or application-port rules. Used the dated Ubuntu ARM64 cloud image from 20260927, SHA-256 63a93bd5a8d76e33b15ceb5daa3657bd79be804748051ab178e643b0f5da22e7, cross-checked with Ubuntu's dated checksum list and Lima's pin; detached image signatures were not independently verified. Installed the full ubuntu-desktop package with recommendations and normal supported apps, not a stripped window manager. Actual /etc/os-release reported **Ubuntu 26.04.1 LTS**, running kernel **7.0.0-34-generic**, native arm64. GNOME Shell **50.1-0ubuntu1.5**, Mutter **50.1-0ubuntu2.5** ran as a non-sudo guest user on a headless Wayland virtual monitor.

Observed limits/failures were retained: initial cold package/Snap provisioning exceeded Lima's boot-script wait and returned DEGRADED/error after **674.8 s** while installation continued. Waited for the explicit installation marker and clean dpkg audit before launching GNOME; VM process existence was never treated as desktop readiness. Initial save-dialog/arbitrary-Unicode-keysym tests did not preserve expected text. A corrected test used actual physical keys/modifiers and the guest clipboard; Text Editor's final newline was accounted for. A command-line data URL initially produced a blank browser capture; pasting/navigating the URL through guest UI visibly rendered the test page. These earlier attempts are not counted as passed compatibility checks.

A real guest-only prototype used GNOME session D-Bus/PipeWire with Mutter's private version-specific capture/input interfaces. Real PNGs showed Ubuntu's desktop/dock, launcher/overview, Files, Ptyxis terminal, Settings and Firefox. Physical `a`/Shift+A plus save produced the expected bytes. The clipboard route preserved **café — 日本語 👋** and the file contents matched exactly including the editor newline. Move/click/held-button motion/scroll methods were acknowledged, but complete semantic gesture/button/layout/IME tests remain pending. One actual GNOME PNG was accepted by the existing Go `protocol.Frame.Validate` with matching 1280×800/scale-1 test metadata: image-format compatibility only, not a real protocol/MCP connection to GNOME.

The full desktop metapackage on the cloud image did not seed App Center. Installed Canonical snap-store **0+git.5534399d**, revision 1469, channel 2/stable (9.9 s after shared runtimes existed), and GNOME Software **50.0-1** through normal Snap/APT. App Center's initial window was blank and activation was inconsistent; GNOME Software opened a metadata refresh view. GUI catalog/auth/install flow is **not verified**. Native Files/Text Editor/Settings/Ptyxis and Firefox rendered; installing a package is not proof every included app is usable. Standalone shell user activation/app registration and a fully managed GNOME session remain production work.

Software-rendered headless session: no /dev/dri GPU device. A point-in-time guest sample before additional software-manager installs reported ~2.79 GB used/~5.51 GB available and GNOME Shell ~528 MiB RSS. Native ARM64 works, but no GPU-accelerated, Fedora-relative, motion-FPS or end-to-end latency advantage is established. Do not convert short individual PNG operation times into a display FPS claim.

An actual stop/start took **5.63 s / 16.98 s** and preserved the saved Unicode document. That 16.98 s is OS/Lima startup, **not desktop-to-agent readiness**: the transient standalone compositor did not auto-start. Managed-session autostart/recovery, complete GUI-store behavior, stable API/grant design and real MCP + Kitty/web tests on the new GNOME guest are still required before a default switch. Existing Debian guest/MCP/viewer results remain separate valid evidence. Fedora was researched from official sources, not booted; some Fedora media pages returned stale/beta cached metadata, so no final Fedora image/digest or Mac benchmark is claimed.

Original guarded research code is in experiments/gnome/probe.py (external guest GI/GStreamer dependencies, no new Go module); it refuses macOS/non-VM/unmarked/root/other-GNOME-major execution and requires private new guest-home output. Actual guarded guest run passed the core input checks. Python compilation/help and refusal on macOS before output/desktop access passed; no mock is represented as hardware. Report flags distinguish core input from unverified production/gesture/store integration. No production code/image/default was changed, no upstream GNOME implementation was copied, and no paid resource/public image/release was created.

Private evidence: ignored artifacts/desktop-quality includes original startup/degraded result, package/browser inventory, successive failed/corrected probe outputs, real guest PNGs, persistence and cleanup. After matching the opt-in ownership marker and the single expected VZ instance name/directory, confirmed stopped/deleted and removed only the evaluation's private state/keys/cache. The production/default namespace was not touched. Screenshots are real guest captures, not mockups or host-desktop images. Commit source/docs under owner identity only, excluding private evidence and AI attribution.

## Phase 5 — Local hardening and unpublished developer RC (2026-11-05)

**Local developer RC 0.5.0-rc.1 is implemented, with real installed-bundle/VM acceptance and explicit distribution limits.** Continued from clean pushed Phase 4 commit 236b15f (CI 42063417310 succeeded). This phase supersedes the earlier Phase 5 cloud plan. No release/tag/upload, paid resource, public deployment, global tool installation or shell-profile change was made.

### Audit and implemented boundaries

Audited effective Lima template validation, private environment/keys/SSH, systemd privileges, X11 clipboard/control, viewer listeners, guest protocol parsing, lifecycle locks/state and packaging. No host mounts/devices/clipboard/agent forwarding were added. Explicit NAT still permits possible host/LAN access; no untested offline/firewall mode is advertised. Same-user host processes/runtime/bootstrap trust and malicious-guest limits are documented in security.md, alongside official Lima plain/static-forwarding behavior. No independent security/penetration certification is claimed.

Added operator-only copy-in/out: one regular file up to 4 MiB, 96-KiB chunks, guest-home confinement, fresh upload directories, checksum/size/order checks, symlink/hardlink/special-file rejection and atomic no-overwrite host output. Archives remain opaque; no host extraction, live share or new MCP filesystem tool. Guest helper 0.5.0 advertises files on additive protocol 2.0; old supported 0.3.5/0.2.0 operations remain accepted without disk upgrades.

Added explicit stopped-machine reset/delete with exact machine ID, removal plan, intentional --yes, ownership/path checks, known-entry confinement and no forced shutdown. Same lock remains held through deletion/reset, with marker/lock inode preserved and interrupted deleting stage retained even through status. Reset downloads/verifies/reprovisions and rotates identity/keys; it is not a snapshot or byte-identical apt state. Only selected private state is cleaned.

Added version reporting, sibling-helper diagnostics, stderr download/stage/heartbeat progress, normalized local host/helper/docs/notices packaging, dependency inventory, offline manifest installer, atomic activation, separate tool uninstall, contribution/security/issue guidance and a concise README path. Packaging selects repository notice/docs files rather than sweeping ignored local files, and rejects linked/outside inputs. No remote executable fetcher or fabricated publisher authenticity. Apple codesign reported **Signature=adhoc, TeamIdentifier=not set** on the tested Go-built CLI; no Developer ID signing or notarization is configured/performed.

### Automated verification and real Mac workflow

`make build test check` passed: both binaries, all race-enabled Go tests, five Python installer/packaging tests, vet and whitespace. Meaningful new coverage includes unsafe paths, links/special/oversized/empty files, ordering/checksum/cancellation and malformed guest chunks, partial-output removal, wrong-ID/running-machine/unexpected-entry deletion refusal, confined cleanup, reset identity, interrupted deletion/status, unowned/missing-state rejection, installed launcher discovery, bundle tampering, manifest traversal, failed activation preserving current, symlinked package inputs, permissions/unexpected-data uninstall refusal. Existing protocol/stale-state/timeout/restart/PNG/host-config tests remain. Unit doubles are not VM evidence. Linux CI adds cross-build and unpublished local bundle/manifest checks; it cannot boot VZ.

Host: ARM64 macOS 26.5 (25F71), 12 logical CPUs/24 GiB RAM, Go 1.26.1, Python 3.12, external reviewed Lima 2.2.1. This is the **existing developer Mac**, not a clean Mac/user account. Used a fresh private tools prefix and fresh disposable 2-vCPU/4-GiB/24-GiB Debian 13 ARM64 machine, pinned 20261001-2618 base, desktop-3.5.0 recipe, actual hello guest 0.5.0/protocol 2.0, real Xfce/Xvfb. A new kernel/point-release inventory was not captured here; earlier Debian 13.7/kernel observations remain separate historical evidence.

1. Built a private explicitly dirty development bundle, installed without sudo/network/shell edits and invoked its launcher. First doctor exposed a real symlink/helper-discovery failure; fixed discovery to the actual immutable executable directory, added a regression test, rebuilt/reinstalled, and doctor then passed.
2. Installed CLI downloaded and SHA-512-verified **337,510,400 bytes (~322 MiB)**, provisioned a new guest, and emitted ready only after actual service/desktop handshake. Observed progress/heartbeat on stderr; JSON stdout stayed parseable.
3. Copied **432,000 UTF-8 bytes** in/out across multiple chunks and compared exact bytes. Overwrite, host symlink, guest symlink and traversal attempts failed. Source and a separate unrelated sentinel remained unchanged. Captured a real guest PNG. Guest audit reported **UID 1001, groups [1001], sudo exit 1**, no virtiofs/9p/Users mounts, and only the fixed guest environment keys (no host secret/proxy environment).
4. Real official MCP Go SDK v1.8.0 stdio client exercised the installed binary and guest. Live fallback in isolated headless **Chrome 156.0.8078.80** visibly showed the MCP-controlled Chromium UTF-8 page. Token/Origin/Host rejection, pause and fragment removal passed. Viewer exit left VM ready. Kitty visual testing was not repeated in Phase 5; its actual Phase 4 evidence remains valid for that renderer.
5. Delete preview without --yes, wrong ID and running-machine deletion were refused. Stop/up retained the original machine ID and guest marker. Reset returned a new ID/real ready guest; the marker was gone. Explicit stopped-machine deletion left **only marker + lock**, with VM disk/config/cache/keys removed. No user default VM was controlled.
6. Real tools-only uninstall removed launchers/releases and left VM namespace marker/lock unchanged, retaining only the tools marker/lock. Original development bundle was restored solely for the final clean-commit upgrade check. Unit failure injection separately verified tampered/copy/activation failures do not switch current.
7. After the full VM run, code review tightened interruption recovery and plan entry reporting. A **synthetic interrupted-state fixture using the actual CLI and absent runtime**, not a simulated real runtime failure, verified status preserves deleting/absent, up refuses resume, and explicit selected-ID deletion finishes cleanup. Additional reset-input/symlink and reserved upload-name guards are automated checks; the full provision/copy/MCP/reset run was not repeated for those small guard changes.

Private evidence is ignored under artifacts/phase5: installed first-run failure/success, setup progress, acceptance1 reports/images, isolation audit, signing observation, interrupted-state check and uninstall observation. The successful real guest/browser PNGs were inspected. The reusable scripts/rc_acceptance.py requires an explicitly named disposable fixture and machine ID; --reset/--delete intentionally authorize that fixture's destruction. It never creates a VM or chooses a personal/default state directory. No model-based agent acceptance was repeated (official SDK was real); earlier Codex evidence is separate.

| Operation | Observed elapsed time on this Mac/network |
| --- | --- |
| Fresh download + provisioning + real ready | 192.923 s |
| 432,000-byte copy-in / copy-out | 2.815 s / 2.386 s |
| Stop / persistent restart to ready | 3.866 s / 8.783 s |
| Full clean reset to new ready guest | 163.864 s |
| Selected stopped-VM deletion | 0.813 s |
| Tools-only uninstall | 0.072 s |

These are single-run hardware/network-specific wall times, not performance guarantees. Default preflight requires 30 GiB free (24-GiB logical disk plus 6-GiB conversion/headroom); sparse physical usage grows and was not independently benchmarked. Apt packages/repos can change, so provisioning is recipe-repeatable, not disk-byte reproducible.

### Reproducibility, final artifact check and limits

Two independent private development packages at the same review checkpoint produced **byte-identical 5,201,917-byte archives** (repro-check.json); both were honestly marked dirty and included the embedded base commit. Packaging/notices guards and final docs were subsequently refined. The clean committed-source build, repeated archive comparison, previous-development-prefix upgrade, doctor/version and tools-only uninstall are the final local artifact check, recorded separately in artifacts/phase5/final-check.json; no artifact is uploaded. A manifest hash validates local integrity, not publisher authenticity through a compromised common channel. The documented current authenticity path is reviewed/pinned source and local build; an authenticated binary release process is future work.

No clean-Mac/quarantined-download/Gatekeeper acceptance, full offline host startup, network-isolation enforcement, broader terminal/Mac matrix, independent security review, formal complete SBOM/image redistribution clearance or monitored private-report SLA is claimed. Private reporting API lookup returned 404, so availability is unestablished; SECURITY.md provides a no-sensitive-details fallback and calls out the owner task. The precise clean-Mac prerequisite → install → doctor → boot → MCP/view → copy/persistence/reset/delete → uninstall checklist is in troubleshooting.md.

**Exit verdict:** functioning local developer RC path, explicit trust/limitation docs and guarded destructive operations are present and tested at the stated scope. Wider binary distribution remains a separate decision. Original code stays Apache-2.0; exact module notices/inventory accompany local artifacts, with no bundled Lima/Kitty/browser/image. Commit under the owner identity, no AI attribution/private evidence/secrets, and create no public release.

## Phase 4 completion — 2026-11-05 (current)

**Phase 4 passed on the tested Mac/Kitty configuration.** Continued from pushed commit 3191b65 (CI run 42012902307 succeeded). The unlocked Mac permitted actual graphics inspection: scoped native capture and Kitty screenshots visibly showed the Linux Xfce desktop, not just a LIVE label. The failed locked-display attempt below remains historical evidence.

### Real acceptance and correction

Same host/guest stack as the initial Phase 4 report: ARM64 macOS 26.5 (25F71), 12 logical CPUs/24 GiB RAM, Lima 2.2.1/VZ, disposable 2-vCPU/4-GiB Debian 13.7 ARM64 guest (kernel 6.12.111+deb13-cloud-arm64), desktop-3.5.0/helper 0.3.5/protocol 2.0, Xfce/Xvfb 1280×800, Kitty 0.50.0. No personal/default machine or browser profile was changed.

- An actual official Go MCP SDK v1.8.0 client connected to the real stdio server while Kitty view was active. It captured the guest, opened Thunar/Chromium, typed a UTF-8 data page and verified the guest browser title. The terminal visibly showed “MCP controls this Linux guest” plus café/Japanese/emoji. Central image pixels changed substantially; the status line alone could not pass that check. A model-based Codex/Claude run was not repeated here.
- Real window sizes were 120×40 → 80×30 → 140×24 → 70×44 → 18×4 → 120×40. macOS/Kitty clamped some requested row counts; the harness now records requested and actual geometry instead of requiring an impossible exact request. Manual inspection confirmed preserved guest aspect ratio on wide/tall layouts and no leftover old placement. Tiny-window inspection exposed a clipped status: fixed by prioritizing RESIZE/connection state and q exit guidance over the machine ID. One-cell/one-row output is also bounded to avoid wrap/invalid row addressing.
- Stopping only the fixture cleared the image and showed NOT_READY. Starting it waited for the real guest handshake; the scoped recording's final frame visibly shows the Linux desktop again. The first reconnect PNG snapshot contained only the LIVE text; it is retained, not presented as graphics proof. The final visible recording frame establishes reconnection. The opt-in harness now waits for actual guest pixels after reconnect/resizing as well as initialization.
- q, raw Ctrl+C, and SIGTERM each returned exit 0, restored persistent termios flags, accepted ordinary canonical input, and left the same machine ready. Immediate stty strings differed only by Darwin's transient PENDIN bit; after ordinary input they matched exactly. Error-path cleanup is additionally covered by automated output-failure/cancellation tests.
- Private evidence: ignored artifacts/phase4-resume/acceptance4 contains the successful corrected run, report/metrics, original scoped PNGs and terminal-demo.mp4. The recording contains **34 actual window captures over 47.966 s**, encoded with measured timestamp gaps. It is a sampled visual record, not terminal paint FPS or full-motion video. The separate initial native window screenshot also displayed Linux correctly. No whole-host desktop, pointer or host clipboard was captured/controlled.

The reusable scripts/viewer_terminal_acceptance.py restricts testing to an explicitly selected disposable fixture and isolated Kitty socket. It never creates/deletes the VM and closes only its own windows. Two earlier fixture runs passed the complete workflow (one before, one after the tiny-status correction). Later harness refinements were driven by preserved failures: a status read hit legitimate MACHINE_BUSY, so only that read now has bounded retry; another visibility check cropped away the icons on the mostly black Xfce desktop, so its crop was corrected and checked against retained real/blank images. Those latest harness refinements were not followed by another full hardware run after the Mac relocked. The production viewer acceptance and recorded successful run remain valid. An asleep-display flag had remained true even during successful unlocked capture; neither that flag nor a capability acknowledgement substitutes for visible evidence.

### Measured scope and automated checks

Successful corrected terminal run: **51.720 s**, 22 capture attempts, 15 valid guest frames, 4 changed PNG hashes, 7 capture errors during contention/deliberate unavailability. Attempt-duration median **464.7 ms**, maximum **2002.1 ms**, including failed/busy attempts; these are backend capture/request timings, not screen-to-terminal paint latency. Guest captures totaled **579,165 PNG bytes**; display uploads totaled **325,514 PNG bytes** (including resize retransmission). Actual guest-to-host captures still transfer full PNGs.

Go viewer counters: **70,598,656 bytes (~67.3 MiB) peak RSS**, **1.106060 s user + 0.312363 s system CPU**, about 2.74% of one CPU averaged over the run, including static/resize/reconnect periods. These exclude SSH subprocesses, Lima, guest, Kitty/GPU and recording overhead. No animation benchmark, maximum throughput, end-to-end paint-latency or broad hardware/resource guarantee follows. The earlier live fallback measurements below remain valid.

`make build test check` passed with the status fix: both binaries, all race-enabled Go tests, vet and whitespace. Subsequent `make test check` passed after adding bounded read-only retry to the hardware MCP test. New unit checks cover tiny/narrow/degenerate status output; Python syntax compilation and retained-real/blank image classification checks passed. Automated terminal/backend doubles remain distinct from the actual Mac results above.

### Exit verdict

Real guest rendering, bounded streaming, usable resizing/status, normal/interrupt cleanup, live secure fallback and a truthful end-to-end sampled demo satisfy Phase 4 for Kitty 0.50.0 on this Mac. Direct human viewer input remains optional/deferred; MCP is the control path. Other terminals/multiplexers, broad Mac compatibility, optimized video, ownership leases and cloud remain unclaimed. No paid infrastructure, public deployment or global terminal configuration was created. Cleanup completed after matching the original machine identity: the disposable VM was explicitly stopped/deleted, its private state/keys removed, and the isolated Kitty process/socket exited. Ignored artifacts/phase4-resume/cleanup.json records this. VM readiness before explicit cleanup confirms viewer exit did not stop it. Source/docs only are committed under the repository owner's identity, without AI attribution or private evidence.

## Phase 4 initial results — 2026-11-05 (historical; visual gate was pending)

**Implementation exists and the real live web fallback passed. Phase 4 is not yet fully accepted: visible Kitty rendering and a truthful terminal video still need an unlocked, awake Mac.** Began from clean pushed Phase 3.5 commit 9d5bb7b; checks run 42000580834 succeeded. Inspected the common machine/capture/locking paths and official Kitty graphics/install/remote-control documentation before choosing direct PNG and a live loopback fallback.

### Implemented behavior and automated checks

Added `view` with active Kitty query, real pixel/cell sizing, aspect-aware placement, direct <=4096-byte base64 chunks, one image ID, bounded/cancelable writes, alternate-screen/raw-mode cleanup, identity/status/age/footer and q/Ctrl+C exit without VM shutdown. No viewer input reaches guest or host. The fallback is a real live PNG/ETag poller with a random capability, bearer headers, loopback-only listener, Host/Origin/CSP/no-store protection, bounded handlers/cache and pause/hidden-page behavior. One synchronous capture, adaptive static backoff and no work without web requests avoid a frame queue. Guest call lock waits are bounded to two seconds to reduce viewer/MCP collisions; lifecycle/MCP semantics and guest protocol remain otherwise unchanged.

`make build test check` passed locally: builds, all `go test -race ./...`, vet and whitespace. New tests cover capability detection/query confirmation, cell/pixel layouts and small/unknown sizes, chunk reconstruction and escape isolation, exit-key parsing, cache/digest suppression, invalid/oversized PNGs, bounded single capture/cancellation, terminal disconnect/reconnect/resize and cleanup on output error, CLI/help/errors, HTTP authorization/Origin/Host/ETags/errors and bounded lock wait/cancellation. Handler/terminal/machine doubles are unit tests, not real desktop evidence. Shell/browser fixture source compilation was checked separately. The only new production Go module is official x/term v0.40.0 with its retained BSD notice; no Kitty code/assets were incorporated.

### Actual Mac, guest and client evidence

Host: ARM64 macOS 26.5 (25F71), 12 logical CPUs/24 GiB RAM; real disposable Lima 2.2.1/VZ Debian 13 ARM64 guest using the unchanged desktop-3.5.0 recipe, helper 0.3.5/protocol 2.0, 2 vCPUs/4 GiB/24 GiB disk. No user default machine/disk was reset. Official Kitty **0.50.0** was downloaded into ignored local tools, digest checked against the official release SHA-256 cc01acf6db19a1730ef60b3e7cf0f6300feb51199bf17e9cff7ee12d28d74e0c. It was not globally installed/bundled. No signature/audit claim follows from that hash check.

- **Live fallback passed twice**, including automatic unsupported/non-TTY fallback rather than only explicit --web. Used host **Chrome 156.0.8078.80** headless with a temporary isolated profile, test-only loopback CDP and websocket-client 1.9.0. No personal browser profile/account/credentials were used. Real page PNGs visibly showed the guest Xfce/Chromium desktop and changed MCP page, not a mock application. Native host browser GUIs and Safari were not tested.
- A real official Go MCP SDK v1.8.0 CommandTransport client connected to the actual MCP binary **while the viewer was connected**, captured the guest, opened Thunar, controlled Chromium with keys/text, verified the changed page/window, and checked command error/timeout results. A model-based Codex/Claude run was not repeated in this phase; Phase 3's Codex evidence remains separate.
- Real network requests without a token, with a foreign Origin, or with an unexpected Host were rejected. The browser removed the fragment after reading it. Pause stopped capture counts, and capture stayed stopped after the browser exited. Requests do not start/stop the VM. One run deliberately stopped and restarted only the owned VM while viewing: the page cleared its image/showed unavailable status and returned to live after the actual guest handshake. CLI/server closure left VM lifecycle unchanged.
- Kitty's actual PTY returned the graphics capability response, and a separate minimal PNG protocol fixture returned a successful image acknowledgement. The live viewer performed real guest captures and graphics transmissions. Resizing through the socket-scoped test control changed the real PTY from **120×40 to 80×30**. q and SIGTERM returned exit 0, restored persistent termios/output state and allowed normal canonical input. Darwin sets a transient PENDIN bit on raw→canonical; recorded exact strings differed only by that bit immediately after exit and matched exactly again after normal input. That distinction is retained, not hidden as a false exact immediate equality.
- **Visual terminal result remains pending.** Internal Kitty screenshots showed the status/footer but blank graphics. The same happened with a minimal official-protocol fixture despite a PNG OK acknowledgement. A read-only OS check reported both `screenLocked=true` and `mainDisplayAsleep=true`; scoped native window capture could not create an image. No broad Mac desktop capture, unlock, permission change or display-wake action was performed. This does not prove that the renderer is visually correct or that every blank-image cause is resolved. An async request for an awake/unlocked test session was made while independent checks continued. Verify the visible image/resize/MCP demo after unlocking, and investigate further if graphics remain blank.

Private evidence is in ignored artifacts/phase4 (web-test, web-auto-test, terminal-final, terminal-q, diagnostics and metrics). Failed/blank terminal captures are not presented as a successful terminal recording. `scripts/viewer_terminal_probe.py` records real TTY behavior; it cannot certify visible graphics. `scripts/viewer_web_acceptance.py` reproduces the real browser/MCP checks with an isolated profile and optional explicitly scoped test-VM restart.

### Hardware-specific observations

In the final automatic-fallback run, the browser loaded **4 changed PNG images in 21.086 s**, about **0.190 image-load events/s** during a mostly static task with a deliberate pause. This is not a motion benchmark, screen-paint FPS or maximum achievable throughput. Client request median was **347.4 ms**; individual requests included contention waits up to about 1.75 s. Eight valid guest captures totaled **292,723 PNG bytes** and the viewer sent **137,859 PNG bytes** to clients (unchanged/cache responses avoided about 53% of those PNG body bytes). Guest→host SSH still transfers full captured PNGs; no delta codec or guest-side conditional frame is claimed.

The automatic-fallback Go process recorded **55,001,088 bytes (~52.5 MiB) peak RSS** and **0.394659 s user + 0.104998 s system CPU** over **234.521 s**, much of it idle after the browser closed. These are viewer-process counters, excluding SSH/Lima/guest/browser/GPU costs; not an active-load or zero-resource claim. A separate Kitty PTY run recorded **56,819,712 bytes (~54.2 MiB) peak RSS**, **2.172509 + 0.612770 CPU seconds** over **119.977 s**, with 38 valid captures and 3 changed images. Its locked/asleep graphical display prevents interpreting that as visible graphics/FPS performance. Capture/request timing samples and transferred-byte counts are bounded and available through `--metrics`; client counters distinguish image-load events from capture count.

### Exit verdict and remaining exact action

Implemented real guest content, bounded view-only streaming, terminal cleanup paths and a working secure live fallback. Automated checks and the browser/VM/MCP acceptance passed; terminal PTY/protocol/resize/cleanup checks passed. **Do not mark the entire phase complete yet:** unlock/awake the Mac, run Kitty view, visibly verify real Linux/MCP changes and aspect/resize behavior, record only that test window, then confirm normal/error/interrupt cleanup and unchanged VM state using the checklist in [viewer.md](viewer.md). Other terminals/multiplexers, human input/takeover, optimized video, cloud and production support/performance remain unclaimed.

Cleanup completed: viewer servers and temporary browser profiles closed, the socket-scoped Kitty test windows/app terminated, the selected VM was confirmed stopped and deleted only after matching its original machine identity, and its private state/keys were removed. Closing the viewers before that explicit cleanup had left the VM ready, as recorded in post-viewer-status.json. Interrupting a live web capture also exited cleanly, drained the active capture, finished its request thread and wrote metrics. Final commit/remote CI are reported after pushing the implementation. No paid resources, public service deployment, billing, package/image publication or global terminal configuration changes were made.

## Phase 3.5 results — 2026-11-05 (historical)

**Implementation and real Mac acceptance passed.** Inspected the Master Prompt, supplied project instructions, clean Phase 3 git state, Phase 0–3 design/evidence, image manifest/template, desktop adapter, guest protocol, CLI and MCP implementation before extending the existing architecture. Phase 3 was committed/pushed as f78109d and GitHub checks run 41995229635 succeeded; this phase does not infer earlier completion from mocks. No default user machine existed in the inspected default namespace, and no user disk was reset or migrated.

### Before and after

Already present: Debian 13 ARM64/VZ lifecycle and pinned SSH, Xfce/Xvfb, Chromium, Mousepad, terminal, Thunar/settings packages, screenshots/XTEST/Unicode paste, bounded arbitrary guest exec and official MCP. The earlier fixed-app control exposed only editor/terminal/browser and did not establish a complete file/settings/second-browser workflow.

Added: explicit launcher/files/settings/Chromium/Firefox launch/focus, stricter panel/desktop readiness, Firefox ESR, GVFS/tumbler/accessibility and desktop utility packages, Git/SSH/curl/Python development baseline, optional Node/npm/build profiles, browser inventory and explicit guest-admin package refresh/install/upgrade. Guest agent remains non-sudo; no privileged MCP installer or network listener. New recipe desktop-3.5.0/helper 0.3.5, same protocol 2.0; old 0.2.0 accepted without rewriting its sealed disk/template. Legacy acceptance is covered by version/contract tests; a legacy disk was not newly booted in this phase. Go is retained, with [ADR 0008](decisions/0008-stack-and-desktop.md) recording the Go/Rust/platform reasoning.

### Automated verification

`make build test check` passed: host CLI and static ARM64 helper, all race-enabled Go tests, vet and whitespace checks. New tests cover package-name/option/path/URL/shell-fragment rejection, fixed guest-admin argv/runtime limits, architecture-specific browser inventory and no vendor substitution, profile/CLI parsing, missing installed packages, app routing/focus, missing panel readiness, recipe contents/non-sudo policy, legacy service-version acceptance, maintenance routing/unsupported backend and machine-lock exclusion. Existing framing, Unicode/key/PNG, timeouts, restarts, exec cleanup and official MCP protocol tests remain. Shell syntax checks and Python compilation passed. No new Go dependency was introduced.

Mocks/fake runners validate boundaries and failure paths; they are not evidence of virtualization or application usability. The real opt-in official SDK test was extended to open Thunar through MCP on new guests, while retaining legacy capability handling. Optional Node/npm/compiler profiles were not installed/executed in the real guest; full apt upgrade, broad distro/host compatibility, Chrome, Edge and Wayland remain untested here.

### Exact observed environment

Host: Apple silicon ARM64, macOS 26.5 (25F71), 12 logical CPUs, 24 GiB RAM. VM: Lima 2.2.1/VZ, 2 vCPUs/4 GiB RAM/24 GiB logical disk, NAT with host/LAN reachability. Base: Debian genericcloud ARM64 20261001-2618, original reviewed SHA-512. Actual running distribution: **Debian GNU/Linux 13.7 (trixie)**, kernel **6.12.111+deb13-cloud-arm64**, dpkg architecture **arm64**. Display: Xfce 4.20 components on Xvfb/X11, 1280×800, scale 1. Guest helper 0.3.5, protocol 2.0; MCP official Go SDK v1.8.0 client over actual stdio.

| Installed component | Observed package version |
| --- | --- |
| Chromium | 156.0.8078.79-1~deb13u1 |
| Firefox ESR | 153.6.0esr-1~deb13u1 |
| Xfce session / Thunar | 4.20.2-2 / 4.20.2-1+deb13u1 |
| Terminal / App Finder / Settings | 1.1.4-1 / 4.20.0-2 / 4.20.1-1 |
| Mousepad / Xvfb | 0.6.3-1 / 2:21.1.16-1.3+deb13u4 |
| Git / OpenSSH client | 1:2.47.3-0+deb13u1 / 1:10.0p1-7+deb13u4 |
| curl / Python / python3-venv | 8.14.1-2+deb13u7 / 3.13.5-1 / 3.13.5-1 |
| GVFS / tumbler / at-spi2-core | 1.57.2-2+deb13u1 / 4.20.0-1 / 2.56.2-1+deb13u2 |
| jq (installed during acceptance) | 1.7.1-6+deb13u4 |

These versions describe one observed install, not permanent pins or universal browser certification. Chromium and Firefox are preinstalled; jq was added explicitly for the test. Node.js/npm, build-essential/pkg-config/Python headers are optional profiles. Chrome is optional/untested (current official documentation includes Linux ARM); Edge ARM Linux is unavailable in reviewed sources. Both x86_64 vendor builds and any cloud guest remain untested; see [compatibility table](browsers.md).

### Real end-to-end run and observations

Ran `scripts/full_desktop_acceptance.py` on a **fresh** VM after correcting the issues below; completed successfully with a read-only host-observation fixture. Full private evidence is in ignored artifacts/phase35-release; earlier failure/debug/resumed evidence is retained separately, not counted as a fresh pass. The completed resumed debug run also passed, but its subsecond ready check is explicitly not cold provisioning evidence.

1. Obtained the pinned base, provisioned real packages and waited for guest handshake plus X11/WM/panel/desktop readiness.
2. Opened Application Finder, Thunar, Xfce Settings and terminal; captured their actual windows. Created a real folder through Thunar's keyboard UI. Typed a shell command in the terminal and verified the guest-created file.
3. Opened Mousepad, loaded a new test file, typed ASCII plus café/CJK/emoji, copied and pasted inside the guest, saved, and compared **exact UTF-8 bytes** to the expected duplicated text. Visually inspected the resulting PNG. No host clipboard was used.
4. Opened Chromium and Firefox ESR separately, navigated each to https://example.com and checked the **active target browser's window class and page title**, not just any matching window. Inspected the Firefox PNG and its real page text. No browser account sign-in or unofficial build was used.
5. Installed jq through the explicit host CLI guest-admin helper and ran it. A deliberately nonexistent package returned PACKAGE_UNAVAILABLE. Temporarily blocked only APT via a test-owned guest proxy config, observed REPOSITORY_UNAVAILABLE instead of stale-index success, then removed that config. The general guest/control network stayed usable.
6. Used the real official MCP library's CommandTransport to launch the actual MCP binary, check status, capture frames, open Thunar, open/control Chromium with keys/text, verify a changed PNG and guest window title, and verify nonzero guest command/timeout results. This phase did **not** rerun a model-based Codex/Claude integration; the separate successful actual Codex run remains Phase 3 evidence.
7. Executed real Git and Python/venv/SSL import checks plus an HTTPS curl request; recorded the complete dpkg inventory, OS/kernel, memory and disk data. SSH client installation/version was inventoried, not tested against an external personal account.
8. Compared Mac pointer coordinates and clipboard **change counter only** around UI/MCP operations: unchanged. Contents were never read; this fixture does not control the Mac. Product source still has no host screen/input/clipboard path.
9. Stopped and restarted the actual VM. Machine ID stayed stable, boot/session changed, desktop became ready, saved Unicode/terminal files and installed jq persisted, and Firefox reopened. Captured the restarted desktop. No snapshot/reset/rollback was assumed.
10. Confirmed stopped and removed only the identity-checked test VM/private state/keys. All failure/debug VMs were likewise removed; no acceptance VM is intentionally left running. Screenshots/events/reports are private ignored evidence, not committed user data.

Measured on that clean run: cold CLI up/download/provision-to-ready **108.209 s**; stopped-VM up-to-ready **7.090 s**. Five serial screenshot calls: **0.3742, 0.3576, 0.3562, 0.3557, 0.3599 s**, median **0.3576 s**. The downloaded QCOW2 was **337,510,400 bytes (~322 MiB)**; raw VM disk logical size **25,769,803,776 bytes (24 GiB)**, allocated **3,493,568,512 bytes (~3.25 GiB)** after the test. Guest root filesystem used about 3.10 GB; `/usr` about 2.21 GB. At the recorded observation, MemAvailable was 2,875,788 kB of 4,016,264 kB, including reclaimable cache. These are one-run observations with warm host caches/network-specific provisioning, not minimum-resource/FPS/tail-latency promises.

### Failures, limits and Phase 4 verdict

An initial Thunar first launch exceeded the earlier eight-second visible-window wait even though actual windows appeared later. Guest diagnostics also showed missing thumbnail/accessibility services. Added standard GVFS/tumbler/accessibility packages and a bounded 20-second app-start wait; subsequent fresh file-manager launch/folder creation passed. A test initially clicked a fixed left-side pixel while Mousepad was placed on the right, causing input to hit Thunar. Corrected the test to focus the actual editor and verify its window title/screenshot. Reusing an externally modified open file then triggered a real save-confirmation dialog; debug tests now use new files and reset only their own graphical/helper sessions. None of those failed attempts was reported as successful typing or saved data.

No underlying desktop/MCP blocker remains for Phase 4 at this tested development scope. Phase 4 still must implement Kitty rendering, bounded streaming/refresh/backpressure, terminal input/resize/cleanup and human/agent ownership. Current source is scrot PNG on the defined single X11 display. Screenshot subprocess/PNG work, SSH startup and serialization/terminal bandwidth are likely costs; keep Go and measure before considering a rewrite/native optimization. No complete live viewer was built or advertised.

Known limits: one Mac/configuration; no Wayland/multiple displays; paste destination/IME limitations; no host/LAN egress isolation under NAT; no automatic old-disk migration, snapshots, reset, ephemeral mode or rollback; no independent browser sandbox/security or distribution-license certification. Security maintenance is explicit and documented, not automatically guaranteed by restarting. Package jobs can outlive a disconnected host wait within their guest runtime bound; inspect before retrying. No paid infrastructure, public deployment, billing or package/image publication was created. Phase 3.5 exit criteria are met with these limitations. Commit/push and remote CI are reported after the phase commit; local checks above do not imply remote execution.

Reproduce with a new private evidence directory (requires real Apple silicon Mac, Go 1.26.1, pinned Lima 2.2.1 and the reviewed image/network prerequisites):

```sh
make build test check
swiftc scripts/observe_host.swift -o /private/tmp/agent-linux-observe-host
python3 scripts/full_desktop_acceptance.py --cli bin/agent-linux \
  --lima /absolute/path/to/limactl --evidence artifacts/my-phase35-run \
  --observe-host /private/tmp/agent-linux-observe-host
```

The optional debug-only keep/resume flags apply solely to explicitly selected isolated test namespaces; they are not a product migration/reset feature. Normal acceptance creates and cleans its own VM. See [desktop](desktop.md), [browser installation](browsers.md), [development](development.md), and [image update/persistence](../images/debian/README.md).

## Phase 3 results — 2026-11-05 (historical)

**Implemented official-SDK MCP stdio; real SDK client and real Codex agent acceptance passed on the actual VM.** Phase 3 began with a clean Phase 2 checkout. No Phase 2 guest migration was needed. Official MCP specification/SDK and client documentation were consulted before implementation; sources and exact setup are in [mcp.md](mcp.md). ADR 0007 records the bounded adapter and explicit broker/ownership deferral.

### Automated checks

`make build test check` passed: host CLI and static Linux ARM64 helper, `go test -race ./...`, `go vet ./...`, whitespace checks. The new tests use the **real official Go MCP SDK client**, not a handwritten MCP mock. In-memory transports test initialize/list/call, tool schema validation/unknown fields, coordinate/key/cwd/timeout rejection, exact Unicode routing, PNG image blocks and metadata without duplicate PNG bytes, invalid/oversized frames, typed unavailable/disconnected/unsupported/stale/uncertain failures, command exit/error data, concurrency/busy gate, per-call deadline, client cancellation and gate release, debug redaction, and configuration printer/error stdout behavior. Initialization/status were tested explicitly under protocol revisions 2025-03-26, 2025-06-18, 2025-11-25 and 2026-07-28.

Machine doubles exercise the MCP adapter only; they are **not** claimed as virtualization evidence. Existing guest/protocol/desktop/lifecycle tests remain. The real hardware test skips explicitly unless AGENTLINUX_ACCEPT_* environment variables are set. Maximum image validation is bounded; no throughput, broad client compatibility, or hostile-client denial-of-service certification is claimed.

### Real Mac and stdio acceptance

Host: ARM64 macOS 26.5 (25F71), 12 logical CPUs, 24 GiB RAM. Disposable VM: Lima 2.2.1/VZ, Debian 13 ARM64 build 20261001-2618, 2 vCPUs/4 GiB/24 GiB logical disk. Guest helper 0.2.0, guest protocol 2.0. MCP SDK v1.8.0; real SDK client negotiated MCP 2026-07-28. Guest image provenance remains the reviewed manifest; no new image was redistributed.

1. Booted a fresh disposable VM through the actual CLI and waited for the real handshake plus managed desktop readiness.
2. Ran opt-in `TestRealStdioGuest` using official `mcp.CommandTransport` launching the actual `agent-linux mcp` binary. It initialized, checked live VM status, captured PNG plus frame metadata, opened guest Chromium, sent CTRL+L, Unicode-containing page text and ENTER with observed frame IDs, then captured the changed display. Guest `xdotool` found the visible expected browser title through the MCP guest command tool. PNGs were visually inspected: actual Linux panel, Chromium, expected page text and Unicode were visible.
3. Verified a nonzero guest command preserved exit/stderr data in an error result, and a timed-out sleep returned COMMAND_TIMEOUT with confirmed command cleanup. Re-ran this real stdio test after the final adapter/schema/binary changes; passed again.
4. Ran installed **Codex CLI 0.164.0-alpha.11.1**, already authenticated via ChatGPT, against that real VM using temporary per-invocation MCP config. No credentials were read/copied into the repo. Host sandbox stayed read-only; only six named guest desktop tools were exposed/approved for that test, guest exec excluded. No global client settings were edited.
5. The recorded Codex events show ten completed MCP calls: status, five screenshots, open browser, CTRL+L, type https://example.com, ENTER. No host shell or guest command call occurred. The final PNG visibly shows Chromium at example.com with tab title “Example Domain” and the page's multilingual explanatory text. Codex's final answer correctly described that observed screen. Original before/after screenshots and event stream are retained in ignored private `artifacts/phase3/codex-approved/`; deterministic SDK evidence is in `artifacts/phase3/`. These generated files contain machine-specific data and are not committed.
6. An initial Codex invocation with approval_policy=never and no per-tool approvals correctly refused its first status tool. That was a client policy failure, not successful guest acceptance. After checking official Codex configuration documentation, the bounded test explicitly approved only the named guest tools for that invocation. The checked-in opt-in script reproduces this setup.

Claude Code 2.1.318 was installed but **signed out**; no Claude agent/image/tool acceptance was run. VS Code Copilot was **not tested**. Their configuration examples were checked against official docs and printer tests; compatibility is not claimed from those examples. No new Mac pointer/clipboard measurement was run in Phase 3: guest routing is unchanged, product code has no host control/clipboard path, and Phase 2's actual unchanged-counter evidence remains separately recorded.

### Scope, cleanup and exit verdict

Phase 3 implements all required real local operations through the common interface. Capabilities/readiness remain live guest observations; unavailable operations fail visibly. Stdio has no listener and stdout contains only protocol traffic. Concurrent calls get immediate MACHINE_BUSY; machine file locks cover other processes. A 70-second request ceiling, bounded framing/images/guest output and propagated cancellation are implemented. Agent guidance is not a security boundary; connected clients receive powerful guest access and NAT host/LAN reachability remains.

The broker, persistent human/agent takeover leases, fair queuing, live viewer, cloud, distribution, paid resources and deployments remain outside this phase. Client/model image support varies. Recent frame expiry can require recapture when model reasoning is slow. Hardware evidence covers one Mac/configuration, not a support matrix. Cleanup completed: confirmed the disposable VM stopped, checked its machine identity against the original creation result, deleted only its private Lima instance, and removed its owned state/keys. Evidence was retained privately under ignored artifacts/phase3. No acceptance VM is left running. All Phase 3 local exit criteria passed, including actual agent acceptance. GitHub CI is checked after pushing the phase commit and reported separately; local checks above are not represented as remote CI execution.

## Phase 2 results — 2026-11-05 (historical)

**Actual guest desktop/control and bounded guest execution implemented; real Mac acceptance passed.** The working tree was clean at phase start. Authenticated GitHub inspection found no open issues. Phase 1's GitHub `checks` run 41982081761 succeeded for commit 7128b09. No Phase 1 blocker was hidden: private tmp/home/AF_UNIX-only restrictions were changed where they prevented the real desktop, and the protocol was versioned explicitly.

Host remains ARM64 macOS 26.5 (25F71), 12 logical CPUs, 24 GiB RAM. The VM used Lima 2.2.1/VZ, Debian 13 genericcloud ARM64 build 20261001-2618, 2 vCPUs, 4 GiB RAM and 24 GiB logical disk. This is one tested configuration, not a measured minimum. Guest helper 0.2.0, protocol 2.0. Desktop packages observed on the fresh run:

| Package | Observed Debian version |
| --- | --- |
| Chromium | 156.0.8078.79-1~deb13u1 |
| Mousepad | 0.6.3-1 |
| scrot | 1.12.1-1 |
| xclip | 0.13-4+b2 |
| xdotool | 1:3.20160805.1-5.1 |
| xfce4-session | 4.20.2-2 |
| Xvfb | 2:21.1.16-1.3+deb13u4 |

### Automated checks

`make build test check` passed locally: host/static ARM64 guest builds, `go test -race ./...`, `go vet ./...` and whitespace checks. Tests now cover input bounds/button/key validation, international UTF-8 preservation, key translation, PNG metadata/size/corruption, fresh sequences, unavailable/changed/expired displays, observed-frame routing, common machine operation routing to a guest backend double, separate request bounds, malformed protocol, guest exec argv/cwd/env, exit/stdout/stderr, timeout/cancellation/output overflow and cleanup paths. Existing lifecycle/locking/image/config safety tests remain. Host CLI dependency inspection confirms it does not link the desktop/guestexec/guest implementation packages.

Desktop/backend doubles and non-cgroup process fixtures are **unit tests**, not virtualization evidence. The Linux command cgroup/clone3 path was exercised separately on the actual VM below. The Python acceptance script compiles; its runs below used real VMs, not a simulated desktop. GitHub Phase 2 CI result is reported after push; local success is not represented as remote execution.

### Real Mac acceptance

Ran iterative checks, then **two complete fresh-VM passes** of checked-in `scripts/desktop_acceptance.py` with normal provisioning. Both passed; the final post-review build is recorded under local ignored `artifacts/phase2-release/`, with the earlier full run under `artifacts/phase2-final/`:

1. Booted and checked service plus Xvfb geometry, Xfce window manager and current generation. `ready` was emitted only after actual managed desktop readiness.
2. Captured a real Linux desktop PNG (1280×800, scale 1), visually inspected it, and validated dimensions/format/metadata. No host display capture was used.
3. Opened Mousepad through the guest fixed-app operation, opened a guest file using keys/text, clicked, typed ASCII and `café — 日本語 👋`, and saved through Ctrl+S. Reading the guest file verified **exact UTF-8 text**, not merely a dispatch acknowledgement. The editor PNG visibly showed both lines.
4. Opened Chromium without `--no-sandbox`; entered a self-contained offline data page and captured the changed guest screen. No claim of external-site browsing or independent sandbox audit follows from this test.
5. Dispatched double/right-click, drag, scroll, key and move on the actual display. Guest pointer query confirmed final pixel position exactly. Visual screenshots changed; not every input acknowledgement is claimed to prove a semantic application outcome.
6. Optional read-only `observe_host.swift` fixture compared Mac pointer coordinates and the **clipboard change counter only**, before/after guest UI actions. Both were unchanged on the complete run and on a separate iterative run. Clipboard contents were never read, nor were host pointer/clipboard values changed. This fixture is test-only; product code has no Mac input/clipboard/screen API. Source/dependency audit found no such control path.
7. Verified guest stdout/stderr separately and exit 7, cwd/environment handling, bounded output flood termination, timeout of a command with a `setsid` child, actual disappearance/non-running state of that detached child, and confirmed emptied cgroup cleanup. Ctrl+C closed the control channel; the host reported OUTCOME_UNKNOWN rather than fabricated completion, and a subsequent guest check confirmed the command was no longer running.
8. Killed the guest X server and observed automatic graphical-session recovery with a new generation. An old frame ID was rejected with STALE_FRAME. Killed the helper and observed service recovery with a new session ID. Stopped/restarted the VM, verified new boot/session/display identity and persistent saved text.
9. The complete script stopped/deleted only its manifest-owned VM/state. The separate development VM was also explicitly stopped, checked Stopped, and removed with its generated keys and state. No user/default machine or global Lima resources were deleted. Local ignored runtime/build caches and known demo screenshots remain; no test VM is intentionally left running.

### Failures and corrections from real tests

- A one-request xclip owner did not reliably survive clipboard format/data negotiation; dispatched input initially saved an empty file. The bounded multi-request guest clipboard window plus exact file verification corrected this. A helper restart also correctly closed its owned editor; subsequent tests reopened it rather than treating missing focus as successful typing.
- xdotool `mousemove --sync` timed out on an already-current coordinate in this guest despite the documented behavior. Movement now avoids that option and verifies the resulting pointer coordinates within a bounded query loop. Repeated same-coordinate clicks are an acceptance regression step.
- Phase 1's service sandbox settings were unsuitable for X11/home/networked apps. The corrected guest units share authenticated guest X11 resources and explicitly allow the required guest paths/networking; none of these changes exposes a host mount or host desktop API.

### Limits and reproducibility

One display/X11 only; no Wayland, live viewer or MCP. Clipboard paste is application-dependent and dispatch must be verified; arbitrary slow/IME/password fields are not guaranteed. Commands run as the non-sudo guest agent and can alter/read its data and invoke guest services. Cgroup cleanup is lifecycle management, not a security boundary against deliberately malicious guest code manipulating delegated resources. No per-command RAM/CPU quota or rollback guarantee is claimed. NAT host/LAN reachability and non-automated image signature verification remain. Existing Phase 1 disks are preserved; automatic guest/protocol migration is not implemented.

```sh
make build test check
# Optional read-only Mac metadata observation (not needed by the product):
swiftc scripts/observe_host.swift -o /private/tmp/agent-linux-observe-host
python3 scripts/desktop_acceptance.py \
  --cli ./bin/agent-linux --lima /absolute/path/to/limactl \
  --evidence ./artifacts/new-desktop-test \
  --observe-host /private/tmp/agent-linux-observe-host
```

Use a **new** evidence directory; it is not overwritten. The script creates its own temporary machine namespace, requires network for initial apt provisioning, and cleans only its own confirmed stopped machine. If shutdown/cleanup is unconfirmed it preserves state and reports the failure. A host observation change can be external user activity and must be investigated rather than assumed to be caused by guest input. Software/OS support beyond this recorded configuration, distribution compliance, and the complete three-run product demo remain future validation gates.

Phase 2 exit criteria: actual guest effects, consistent capture/input coordinates, visible failure states and no required host control path are met at this development scope. No paid resources, billing, public deployment or unrelated UI were created. Commit/push and CI state are reported in the completion response; staged files are reviewed for secrets/personal data, with no AI attribution. Next: MCP/client integration and input ownership on this tested guest contract.

## Phase 1 results — 2026-11-05

**Actual local implementation and actual VM acceptance passed.** This section records Phase 1; the Phase 0 inspection below is historical. Desktop/MCP/viewer acceptance is still pending.

Host: Apple silicon arm64, macOS 26.5 build 25F71, 12 logical CPUs, 24 GiB RAM; hardware virtualization query returned 1. Free space was rechecked after cleanup and exceeded the conservative creation budget. Go 1.26.1; workspace-local Lima 2.2.1 from its official ARM64 release archive, checked against release SHA256SUMS. No global runtime install or privileged network helper was added.

Guest: Debian GNU/Linux 13 (trixie), genericcloud ARM64 build 20261001-2618, kernel `6.12.111+deb13-cloud-arm64`. Image SHA-512 matched the compiled pin and Debian's HTTPS manifest; no detached signature verification was performed. Service 0.1.0, protocol 1.0, 2 vCPUs/2 GiB RAM/16 GiB logical disk, plain VZ with NAT and no inherited host mounts. No desktop packages were installed.

### Automated checks actually run

- `make build`: host command and static Linux ARM64 helper built successfully.
- `make test`: `go test -race ./...` passed. Tests cover CLI parsing/errors; protocol framing, duplicate/unknown/malformed/oversized messages and versions; handshake/session association; fixed probe; arbitrary-operation rejection; disconnect and idle timeout; state transitions, interrupted startup and missing disk; unconfirmed shutdown; file locks/atomic state, symlinks/path traversal; resource budgets; effective config and QCOW2 safety; image mirror policy; provisioning-size regression; log redaction; subprocess-group deadline/output bounds.
- `make check`: `go vet ./...` and whitespace check passed. These are local checks, not a GitHub Actions run. Linux CI config was added but not executed remotely.
- Python acceptance script passed on a real Mac and compiled locally. The shell subprocess timeout test exercises a host-side test fixture only; **there is no guest shell-execution API** and no claim that guest arbitrary-command limits were implemented/tested.

### Real Mac observations

1. Downloaded/checksummed the selected Debian QCOW2, created a VZ disk with Lima, booted Linux and completed the actual strict-SSH hello and nonce SHA-256 probe. `ready` was emitted only after both checks. Service reported Linux/arm64, matching machine/session identity and protocol.
2. Inspected the test guest through trusted bootstrap administration: Debian/kernel as above, systemd service active, desktop/service account `agent` UID/GID 1001 with no supplementary groups. `findmnt` found no virtiofs, 9p or SSHFS mounts. Application channel's SSH key was restricted to the bridge.
3. Graceful `stop` returned only after runtime reported Stopped. Restart preserved the machine ID, changed Linux boot ID and service-session ID, and completed a fresh handshake/probe. A sentinel created inside the guest persisted across shutdown/restart. Initial sentinel read failed because the bootstrap admin lacked access to the agent-owned file; a privileged test-only read confirmed persistence. This was a test-permission issue, not a missing file.
4. Replaced the test known_hosts entry temporarily with a different public key: status failed with HOST_KEY_MISMATCH and did not report ready. Restored the correct entry, confirmed ready again, then stopped the VM.
5. Ran the checked-in `scripts/acceptance.py` against a fresh machine using the final helper and an existing verified image. It passed real up/status/stop/restart checks, stable machine/new boot/session checks, and removed its own test resources. Observed first readiness was **10.71 seconds with cached image** on this one run. This is not a cold-download benchmark or a performance guarantee.
6. Separately removed both earlier specifically identified stopped development instances and their generated host/control keys, image caches and private state after checking owner marker, machine IDs and runtime Stopped state. No global delete/prune was used. No VM created by this phase remains running. Workspace-local runtime/build caches remain ignored, with no guest credentials in tracked files.

### Failures that improved the implementation

The first network request followed Debian's official HTTPS mirror redirect, which the initial allowlist rejected. The allowlist now includes that documented mirror, while checksum pinning and HTTPS remain mandatory. An uncompressed embedded executable exceeded Lima's observed 4 MiB template input limit and was truncated; validation prevented boot. Compressed multiline provisioning with an explicit 3 MiB cap fixed it. Lima's ordinary JSON inventory hit `bufio.Scanner: token too long` on embedded config; a fixed selected-field format resolved it without returning config/key blobs. See ADR 0005. Failed attempts were not recorded as successful acceptance.

### Constraints and risks

One Mac/runtime/image combination was exercised. macOS 14 is a development target, not a tested minimum; Intel, other host versions and cloud were not tested. NAT host/LAN reachability remains a limitation, and no host/LAN network scan was performed. Vsock/offline networking, distributor signature validation, installed release/notarization, image redistribution compliance, physical-device access, desktop capture/input, Unicode UI behavior, MCP/client compatibility and viewers remain untested/unimplemented. A VM is a risk-reduction boundary, not a malicious-code security guarantee. The guest bootstrap admin and Lima runtime remain trusted, including upstream management SSH behavior. No screen recording/accessibility grants were requested, and no host desktop input API exists in this phase.

### Reproduce safely

```sh
make build test check
bin/agent-linux doctor --lima /absolute/path/to/limactl --json
python3 scripts/acceptance.py --cli ./bin/agent-linux --lima /absolute/path/to/limactl
```

The acceptance script creates a dedicated temporary root, uses explicit NAT, stops/restarts a real VM, and deletes only its own stopped machine. It does not use a personal default machine. It can reuse an existing copy of the pinned QCOW2 with `--image /absolute/path/image.qcow2`. On unconfirmed shutdown/deletion it preserves the state and reports its location. More extensive protocol failure tests use explicit in-process/backend doubles and do not constitute hardware evidence.

### Phase 1 exit verdict

Local creation/start/actual readiness/shutdown: met. Guest service/protocol: met. Relevant automated checks: passed. Real hardware validation: completed with the limits above. Paid resources, public deployments, package publication and billing: none. Staged review passed across 41 text files: no generated/private paths, binaries, actual private-key blocks, common credential-token patterns, private home paths/email or AI attribution trailers. These checks and content review are not a universal secret-detection guarantee. Phase-end commit follows that review; the final response gives the commit hash and clean/dirty status. Nothing was pushed unless separately reported. Next action: guest desktop/capture/input slice, not cloud or UI polish.

## Historical Phase 0 evidence

## What was actually done

The initially empty workspace was connected to the supplied GitHub repository before this phase. At Phase 0 start, `git status --short --branch` reported main with no commits and no tracked/untracked project files. No AGENTS.md or CLAUDE.md existed in the inspected workspace/ancestor locations; supplied user instructions remain authoritative.

Read-only local observations:

| Check | Observed result | What it does not prove |
| --- | --- | --- |
| `uname -m` | arm64 | Exact chip generation or virtualization availability |
| `sw_vers` | macOS 26.5, build 25F71 | Compatibility with any uninstalled VM runtime |
| `xcode-select -p` | Xcode application developer directory selected | Signed VZ app builds/runs successfully |
| `xcrun swift --version` (initial inspection) | Swift 6.2.1 | A Swift implementation was compiled |
| `go version` (initial inspection) | go1.26.1 darwin/arm64 | Go module/dependency build success |
| `git --version` (initial inspection) | 2.55.0 | Remote branches contain code |
| `df -h .` (repeated in Phase 0) | About 13 GiB available; data volume 97% used | Sufficient space for guest/image/conversion/provisioning |
| Tool lookup | Homebrew, Node/npm, Python, Go, Rust available | Any dependency has been installed for this project |
| Runtime lookup | Lima, Tart, QEMU, Docker absent from PATH in initial inspection; Lima still absent in Phase 0 | Apps cannot exist outside PATH |
| `command -v kitty` | No PATH entry | Kitty app is not installed elsewhere or terminal graphics are unsupported |
| CPU/memory sysctl attempt (initial inspection) | Sandbox denied query | Host capacity is known |

Research used TinyFish search and multi-URL fetch on official project docs, source/license files and release pages. [research.md](research.md) records links, retrieved facts and gaps. No provider account/API credential was accessed. No code, images, binaries, or installers were downloaded into this repository. TinyFish tools' batch_create was unavailable; no browser automation jobs were created.

## Phase 0 checks

Documentation validation checks local relative links, required deliverables, fenced-code balance, trailing whitespace and accidental sensitive-file patterns. Secret review checks staged content for private keys, common credential token formats, private email, user home paths and attribution trailers. These are limited checks plus content review, not a security certification or a comprehensive secret-scanning guarantee.

Results and final commit state are recorded below after execution. No application code exists, so there are no product builds, unit/contract tests, MCP integration tests or VM acceptance results in Phase 0. No mocks/fakes were run either. No paid resources, public deployment, package publication or billing activation occurred.

## First real Mac end-to-end experiment (planned, not run)

Prerequisites: Phase 1 implementation; validated runtime/image license and checksum; available memory and enough disk for the measured provisioning budget; read-only `doctor` pass. Install only the deliberately selected Lima release, without privileged networking helpers. Record Mac chip/CPU/RAM, macOS, Go/Lima versions, exact guest image URL/digest, packages, logical/physical disk sizes and network choice. Do not assume today's 13 GiB free suffices.

1. Create an isolated test root with `mktemp -d`; put per-machine runtime/state there. No user home/project mounts. Record every resource UUID/path created in a cleanup manifest. Use a small deterministic HTML form bundled with the test in the guest; no internet task or credentials needed after provisioning.
2. Render and inspect the effective Lima config: plain VZ ARM64, empty mounts, no proxy values/host certificates, no SSH agent/X11 forwarding, no static service forwarding, no inherited user metadata beyond a generic guest identity. Verify the checksum/signature chain, then boot/provision and measure stage durations and disk growth. Package repository access during provisioning is explicit.
3. Verify guest host key enrollment and strict application SSH pinning. Inspect guest mounts/environment, Xauthority, service UID, absence of sudo for desktop user and host-home/SSH-agent exposure. Test a deliberate host-key mismatch: application connection must fail. Only trusted test harness/admin inspection performs these checks; no agent host shell tool is added.
4. Start Xvfb/Xfce and guest Chromium with browser sandbox intact. Wait for separate display/helper readiness. Capture a PNG; decode with dimension/byte limits and inspect desktop content. Record capture metadata/latency and evidence outside the repository. Empty/error PNGs do not pass readiness.
5. Through CLI's application path, screenshot, click a deterministic field and enter the mixed Unicode test string from product.md. Submit via key chord, recapture and verify exact DOM/text with an in-guest test oracle. The oracle is test-only and cannot substitute for visual input or claim the screenshot proves the text value. Confirm changed pixels in the expected region and unchanged out-of-bounds behavior. Manually verify host pointer/clipboard before/after without logging their contents.
6. Send an invalid coordinate, unknown action, malformed/oversized reply fixture and expired session/frame reference. Expect typed errors; no host command or filesystem effect. Kill/restart the guest helper/display, interrupt a write acknowledgement and disconnect SSH; reject old generations, release inputs where possible, report unknown outcome and require observation instead of duplicate actions.
7. Write a sentinel **inside the guest** using the reviewed test fixture. Stop gracefully, confirm runtime stopped, start again, and verify sentinel persistence/new session identity. An old session input must fail. Inject stop timeout: report still-running/unknown rather than success, and require explicit force-stop.
8. Measure host/guest CPU/RSS, actual disk allocation, cold provision/warm ready time and screenshot/input latencies. Probe guest access only to owned host/LAN test listeners and document observed network behavior. Do not scan the user's network. Test offline mode separately only if runtime supports it; otherwise record unsupported.
9. Delete only the test machine's manifest-owned resources after checking UUID/paths and avoiding symlink traversal. Confirm no owned VM/process/tunnel remains. Retain redacted evidence deliberately outside source control, or delete only evidence this test created. Never run global Lima prune/delete commands. If cleanup fails, report paths/IDs and CLEANUP_PENDING; do not mark the experiment passed.

First acceptance: complete steps 1–9 three consecutive times on a recorded Mac configuration. Early bring-up may run the smaller boot → frame → action → frame → stop loop first; it does not satisfy the full gate. Failure artifacts and manual observations must accompany results.

## Original Phase 0 future-test plan (historical; current results above supersede it)

- Phase 2: real MCP client discovery/schema/negotiation, screenshot image + metadata, action errors, human ownership, persistence and broker crash reconciliation. Test doubles cover backend failures only and are labeled separately.
- Phase 3: real Kitty graphics/keyboard/mouse with exact version, Retina/resize mapping, bounded refresh/backpressure and terminal restoration; browser token/Origin/CSRF/expiry and takeover checks. Do not report 2 fps scheduling as 2 fps achieved.
- Phase 4: clean-Mac install/uninstall/recovery and measured compatibility/resource matrix; license/SBOM/distribution review and image updates. No hardware support matrix is established yet.
- Phase 5: provider contract, authentication/workspace isolation, stream authorization, usage reconciliation, hard TTL/idle cleanup and retained-storage deletion, only with authorization for paid resources.

## Exit-criteria assessment

- Concrete local first-slice implementation path: met at design level (Lima plain/VZ, Debian ARM64 X11, SSH helper, bounded PNG/control). Hardware/strict key provisioning compatibility remains an explicit Phase 1 gate, not an architectural choice left unspecified.
- Local/cloud boundary: specified; cloud is not implemented and no feature parity is claimed.
- Risks/assumptions/validation: documented, including network reachability, disk pressure, X11 trust, Unicode paste, protocol/image bounds and licenses.
- No paid resources/public deployment: met.
- Phase commit: required after documentation checks; no implementation work is included.

## Phase 0 final check record

Documentation check passed for all 10 Markdown files: required deliverables exist, relative links resolve, code fences balance, no trailing whitespace or private home-path/email patterns were found. Staged checks also passed across 11 allowed text files: git diff --cached --check, credential/private-key/token patterns, private paths/email patterns and attribution-trailer checks. These are bounded pattern checks, not proof that every possible secret format is detectable. No product tests were run. Commit attribution uses the repository owner’s GitHub handle and noreply address rather than a private email, with no AI attribution trailers. The completion report gives the commit hash and post-commit git status; this file cannot embed its own final commit hash without changing that hash.

## Phase 6 development verification (2026-11-05)

Scope: development cloud prototype plus single-command local startup and human input. No paid provider API, public deployment or billing integration was exercised. Tests using `fake-development` are simulations, never evidence of cloud Linux.

Automated: `make test check` runs Go race tests, vet, installer checks and offline Python SDK doubles. New tests cover authenticated HTTP machine operations through an official MCP client, capability filtering, every cross-user session route, concurrent duplicate creation, lost allocation reply, inventory recovery/orphans, provider outage/deadline, resource rejection/confirmed deletion, revocation, TTL/idle/budget enforcement, private credential paths, corrupted identity state, input ownership/restarts, incremental Unicode/bracketed paste, terminal-cell mapping and authenticated web input. Python doubles verify exact SDK create metadata/idempotency/TTL/network flags and compile the embedded guest runner without importing a real provider SDK. They do not validate a provider's promises.

A separate actual CLI loopback smoke test ran `cloud init → serve → up → status → stop`, observed authenticated ready/stopped state and removed its newly generated private test credentials. Provider was SIMULATED and no resource was created. Official SDK-over-HTTP integration is covered by `TestOfficialMCPClientThroughDevelopmentCloud`; this is different from the existing real local stdio MCP acceptance.

Real Mac observations: Apple M4 Pro, macOS 26.5 (25F71), Lima 2.2.1/VZ, Kitty 0.50.0; disposable new 2-CPU/4-GiB/24-GiB Debian ARM64/Xfce workstation, helper 0.6.0, protocol 2.0. Repeated runs of `scripts/interactive_terminal_acceptance.py` demonstrated one `start` command booting the actual VM and obtaining the desktop/input lease, real terminal images, exclusion of agent mutations during human ownership, exact editor ASCII/Unicode/emoji file bytes, q as text, automatic terminal paste, guest shell operation and Ctrl+C remaining guest input, then terminal SGR click mapped through actual placement and Unicode browser form submission. These were programmatically injected PTY events in an isolated Kitty window, not a physical human or an AI-model completion. No Mac pointer or clipboard was used.

Early failures are retained privately in ignored `artifacts/phase6`: the first run exposed status-lock starvation (changed to bounded wait); a driver oracle needed to poll an asynchronously created file; later human-path passes exposed an acceptance assumption about navigating a browser handed over fullscreen. The revised MCP test observes the guest fullscreen state and exits it before using browser chrome. Guest activation now verifies actual keyboard focus. The final hardware run below passed the revised acceptance; the earlier failures remain recorded rather than being discarded.

Repeat on a supported unlocked Mac, after `make build`:

```sh
python3 scripts/interactive_terminal_acceptance.py \
 --cli "$PWD/bin/agent-linux" --lima /absolute/reviewed/limactl \
 --kitty /absolute/kitty.app/Contents/MacOS/kitty \
 --kitten /absolute/kitty.app/Contents/MacOS/kitten \
 --evidence "$PWD/artifacts/phase6/interactive-new"
```

The script creates its own `/private/tmp/agentlinux-phase6-acceptance-*` namespace, waits for real readiness, tests human/MCP handoff, resizes, verifies terminal restoration and VM survival, then stops/deletes only that selected disposable VM. Evidence contains local paths and stays untracked. First-boot timings include variable image/APT downloads and are observations, not startup or frame-rate promises. A clean-Mac installer walkthrough, physical keyboard/IME coverage, real paid cloud tests, provider commercial approval, and production remote ingress remain pending. See [provider acceptance checklist](cloud.md).

Final real-Mac result: **PASS**, private evidence `artifacts/phase6/interactive6/report.json` and `mcp.txt`. Fresh local VM used the pinned public Debian base already cached and hash-verified; remaining provisioning used normal package downloads. Boot through desktop image and human ownership took **139.96 seconds** on this machine in this run, not a general timing guarantee. Guest helper 0.6.0, protocol 2.0, actual Linux ARM64, one 1280×800 X11 display. Human editor/terminal/browser actions passed, then the official SDK stdio MCP test passed in **30.03 seconds** against the same guest while the terminal viewer remained open. The agent observed `_NET_WM_STATE_FULLSCREEN`, sent guest F11, navigated and verified the changed browser title/screenshot. Focus diagnostics confirmed the actual Chromium guest window.

Resize to 80×30 and Ctrl+] exit passed. Canonical input worked afterwards; all persistent termios values matched. Darwin's transient PENDIN bit cleared on the next input, after which exact `stty -g` matched. DECRQM queries confirmed modes 1000/1006/2004 were reset before and after. The VM remained ready and the input lease was absent after viewer exit. The test subsequently stopped and deleted its exact owned VM; only the ownership marker/lock remain. The existing `workstation55-demo` VM was not controlled, stopped or migrated. One interrupted later download attempt created no VM; its known partial download and namespace state were separately cleaned through guarded deletion.

Human web input has automated authorization/validation/routing coverage; the newly added browser control widgets have not had a real-Linux browser interaction acceptance run. Earlier Chrome 156 view-only fallback acceptance remains valid for observation. Continuous human drag/IME and physical-keyboard coverage are pending. No provider-dependent result is inferred from any local or simulated test.

## Phase 7 metering and billing verification

Implemented on the existing development Mac with Go 1.26.1 and official Stripe Go SDK v86.4.2. No Stripe key, webhook signing secret or return URL was configured in the environment. No Stripe account calls, live objects, card collection, payments, paid cloud resources, public deployments or release publication were attempted.

`make build test check` passes Go race checks across the repository, installer tests, offline E2B worker tests, cost-model tests, vet and whitespace checks. New billing tests cover exact seconds/cost rounding, sub-cent invoice carry, immutable effective rates, failed-before-ready zero billing, restart/idempotent finalization, trial release/exhaustion, workspace concurrency/caps, negative credits, clock rollback/review/resolution, durable payment intent steps/retry/retention, payment failures and current-state reconciliation for delayed/out-of-order receipts. Signed webhook tests use the official SDK's test signer and verifier, test tampering/stale/future/live/unsubscribed payload rejection and replay deduplication. This is not evidence of Stripe delivering an event.

Official SDK HTTP tests exercise actual request serialization/response decoding against a local fixture for customer association, setup-only Checkout, expanded SetupIntent, isolated draft/item/manual finalization and no charge/send API call. Cloud tests cover authenticated account isolation, confirmed-stop accounting/restart, cancellation/blocked admission, minimal durable inbox privacy and disk-write failure followed by owned resource cleanup. No test double is labeled a running cloud Linux machine or hosted Stripe flow.

The opt-in `scripts/billing_acceptance.py --cli /absolute/agent-linux` executes the real CLI/loopback API against the explicitly SIMULATED provider, configures illustrative test rates, observes readiness through final stop, verifies final trial-covered usage and isolated workspace history, and exercises guarded cancellation/denied admission. It creates and deletes only its own private development credentials/state. The local desktop/human-control implementation from Phase 6 is unchanged; no new real-Mac desktop performance claim is made by the billing work.

`python3 scripts/cost_model.py` evaluates editable low/base/high assumptions. Independent tests check the gross-margin cash equation, egress sensitivity and invalid margin rejection. Its output is an estimate from illustrative inputs, not actual provider or payment unit economics.

**Pending acceptance:** actual Stripe-hosted sandbox Checkout completion, Stripe CLI webhook delivery, real account failure/paid/void/portal/invoice recovery, provider invoice reconciliation and production retention/recovery/transactional-storage controls. Exact setup and checklist are in [billing](billing.md). Live billing is unavailable in code, and the real cloud provider gates remain from Phase 6. Do not claim those exit requirements passed merely because fixture tests are green.

Phase 7 CLI acceptance result: **PASS**, `python3 scripts/billing_acceptance.py --cli "$PWD/bin/agent-linux"`; reported provider SIMULATED, zero Stripe calls, zero paid resources, with all generated fixture credentials/state removed. Stripe API snapshot schema required by the pinned SDK is `2026-08-26.dahlia`. Exact indented state-size preservation and non-billable provider profile mismatch are also covered by the final suite.
