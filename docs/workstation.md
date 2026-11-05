# Phase 5.5 workstation decision and acceptance

2026-11-05. **Selected: Debian 13.7 ARM64 / Xfce 4.20 (xfce4-session 4.20.2-2), workstation-5.5.0.** Retain and polish the working desktop after actual guest/MCP, repeated task/restart and model/viewer acceptance. This supersedes the earlier GNOME-first direction in ADR 0011. Appearance alone is not an automation capability.

## Candidates and verified mechanisms

| Candidate | Support and ARM64 | Capture/input on a virtual display | Integration and maintenance |
| --- | --- | --- | --- |
| Debian 13 / Xfce 4.20 | Debian full support to August 2028, reduced-architecture LTS to June 2030; native ARM64 packages | Managed authenticated Xvfb `:1`, scrot PNG, XTEST/xdotool, guest xclip | Existing lifecycle, common interface, MCP and Kitty; no GPU requirement; X11 apps share session trust |
| Ubuntu 26.04.1 LTS / GNOME 50 | Native ARM64 desktop media; Main maintenance to May 2031 (coverage varies for Universe/vendor apps) | Wayland virtual monitor; PipeWire capture and compositor-authorized input needed | Prior actual Mac prototype used private Mutter D-Bus, not the production backend. Managed GNOME session, maintained grants/GRD transport, full gesture/restart/MCP/viewer acceptance remain unimplemented |
| Debian 13 / Plasma 6.3.6 | Native ARM64 Plasma, KWin/X11 and normal Debian browsers | Research adapter uses the same Xvfb/scrot/XTEST/guest clipboard mechanisms | Real VM candidate; KIO required a guest-runtime permission correction. No production profile or migration is shipped |
| Current upstream Plasma 6.8 | Released October 2026; distribution/package qualification is separate | Wayland session; Xwayland does not make whole-desktop scrot/XTEST work | X11 session startup retired. The Debian 6.3 result must not be presented as 6.8/Wayland support |

Xfce documents [4.20 as its current stable desktop](https://www.xfce.org/download); its [Wayland roadmap](https://wiki.xfce.org/releng/wayland_roadmap) explicitly does not promise current X11 feature parity.

Official sources checked during this phase: [Debian lifecycle](https://www.debian.org/releases/trixie/), [Debian Plasma/ARM64](https://packages.debian.org/trixie/plasma-desktop), [KWin/X11](https://packages.debian.org/trixie/kwin-x11), [Plasma 6.8 retirement of X11 sessions](https://kde.org/announcements/plasma/6/6.8.0/), [Ubuntu ARM64 desktop and requirements](https://ubuntu.com/download/desktop), [Ubuntu GNOME/Wayland release notes](https://documentation.ubuntu.com/release-notes/26.04/summary-for-lts-users/), [Ubuntu support](https://ubuntu.com/about/release-cycle/). Ubuntu lists 6 GB RAM/25 GB disk, not a measured Agent Linux minimum. Fedora Workstation 45/GNOME 51 and its approximately 13-month lifecycle are covered by the earlier [quality research](desktop-quality.md); it was not booted here.

GTK/Qt accessibility information could supplement screenshots: [KDE's AT-SPI testing workflow](https://community.kde.org/Selenium) describes a separate D-Bus/WebDriver stack, and [Xfce accessibility settings](https://docs.xfce.org/xfce/xfce4-settings/accessibility) enable assistive technologies. Toolkit bridges, app coverage, browser accessibility activation, stable element identity, bounding boxes and safe actions need independent evaluation. **No accessibility tree or DOM tool is exposed by our current MCP server.** Changing theme cannot add that capability.

## Repeatable workstation recipe

`workstation-5.5.0` retains the checksum-pinned Debian 13 ARM64 cloud base and signed Debian APT repositories. Exact package versions move with repository updates: this is a versioned provisioning recipe, not a bit-identical image. The base provenance, digest and update process remain in [images/debian](../images/debian/README.md). No new redistributed VM/browser image is published.

The complete app set remains Xfce session/window manager/desktop/panel/Application Finder/Settings, Thunar plus GVFS/thumbnail support, xfce4-terminal/Bash, Mousepad, sandbox-enabled Chromium and Firefox ESR. Readable Noto/CJK/emoji and DejaVu fonts, in-guest clipboard, Git, OpenSSH client, curl/TLS, Python/venv, less/nano/manuals, APT/dpkg and explicit `web-dev`/`build` profiles are included. See [browser matrix](browsers.md) and [developer tools](development.md). Additional supported Debian apps can be installed through explicit operator package maintenance; the desktop user/MCP service is not given sudo.

New guests receive Debian-packaged Greybird GTK/window decoration, Adwaita icons, Noto Sans/Mono fonts, an original restrained geometric wallpaper, click-to-focus and one labeled bottom panel with Applications, running windows, tray, clock and Show Desktop. No extensions, proprietary art, Apple branding, compositor effects or screen-lock daemon are added. Greybird is the [Shimmer project's Xfce suite](https://github.com/shimmerproject/Greybird), dual GPL-2.0-or-later/CC-BY-SA-3.0-or-later; use Debian's maintained package and exact copyright inventory, not an unpinned theme download. Existing Adwaita/fonts retain their own notices; review all package terms before image redistribution.

Defaults run once as the non-sudo guest user before the graphical session; a completion marker is written only after successful settings initialization. A failed setup retries at next service start. Subsequent boots preserve theme/font/panel preferences; the managed resolution and disabled compositing remain enforced. Existing sealed VM templates/disks are not rewritten, upgraded or migrated. To try the recipe, deliberately select a **new** private state namespace. Stop/up preserves guest files, browser profiles, package installs and preferences. Software security updates require explicit `packages refresh`/`packages upgrade`; boot is not an automatic update.

## Comparison method and limits

Use separate disposable namespaces; same Mac, Debian base/kernel, 4 CPUs/8 GiB/40-GiB logical disk, 1280×800 scale 1, same installed Chromium/Firefox versions and offline HTML fixture. Timed comparison runs keep the other test VM stopped. Host workload is not otherwise controlled; samples are small. The first manually modified Plasma fixture retained unused Xfce packages and reverted on restart; exclude it from final performance results. The repeatable Plasma recipe removes Xfce session/app packages and verifies actual Plasma/KWin processes after every restart.

`TestRealWorkstation` is an opt-in **official Go MCP SDK stdio client**, not an AI model. It checks status and PNG dimensions, launcher, GUI folder creation, a terminal keyboard command, exact saved ASCII/CJK/emoji with in-guest copy/paste, Chromium and Firefox forms, semantic drag/scroll through fixture event/title oracles, pointer coordinates, right/double-click dispatch, app focus switching and fresh screenshots before actions. Oracles run only in the guest. Right/double-click acknowledgements are not claimed as a full semantic gesture suite. Reports include completed steps and all screenshot request durations; incomplete runs never count as a pass. Synthetic tests do not count as VM evidence.

The initial harness mishandled an omitted empty stdout field and panicked after folder creation. Corrected empty-output decoding and explicit successful-completion reporting; that attempt is excluded from desktop reliability counts. Plasma's first actual suite failed: Dolphin displayed a KIO socket error. Granting its helper write access to the existing guest-owned desktop runtime directory corrected this candidate-only integration fault, without exposing host resources. Retain the failure; do not infer an inherent Dolphin defect or hide integration cost. The manual candidate also reverted to Xfce after Lima reapplied its sealed provisioner; those restart numbers were invalidated and a fresh repeatable recipe was tested instead.

Measured results, genuine image inspection, model acceptance and cleanup are recorded below and in [verification](verification.md). A screenshot request duration includes capture/serialization/SSH/MCP overhead; it is not screen-to-terminal latency or an FPS claim. `MemTotal - MemAvailable` estimates memory pressure without counting reclaimable cache as committed workload. Process RSS double-counts shared pages; `%CPU` from `ps` is lifetime-averaged, not an idle interval benchmark.

## Human viewer interaction and next implementation

The Kitty and local web viewers are **view-only**. `q`/Ctrl+C exits the viewer without stopping the VM; those keys do not type into Linux. Terminal resizing changes placement only, never guest coordinates. Both viewer capture and MCP use the common machine interface and bounded cross-process operation lock. That serializes individual operations but cannot protect a multi-action human typing/drag sequence from an agent.

A reliable interactive slice needs an explicit machine-wide input lease before enabling terminal mouse/paste. Implement an ownership record under the existing lock with owner nonce, monotonic expiry/generation and explicit takeover/release; lifecycle/restarts revoke it, all MCP/CLI/viewer mutation entry points enforce it, observation stays available. Human takeover pauses agent mutations with a typed ownership error, never queues/replays them. Parse bounded SGR mouse and bracketed UTF-8 paste, translate terminal placement back into source pixels, reject letterboxes/stale frame generations, release modifiers/buttons on every completion/disconnect/interrupt. Start with click and paste; test lease contention/crash/expiry, resize during drag, Unicode, SSH loss and terminal cleanup before adding shortcuts/IME. Existing viewers remain functional meanwhile. No interactive or ownership security guarantee is claimed now.

## Security and cloud boundary

No host home/project mounts, clipboard sharing, SSH-agent forwarding, user credentials or host pointer/device control are introduced. Unicode paste uses only the guest clipboard and can replace its previous contents. Explicit bounded regular-file copy and guarded stopped-machine reset/delete retain their existing contracts. NAT can reach host/LAN services; it is not network isolation. The host runtime/bootstrap and X11 guest session are trusted, and a VM is not a perfect boundary against malicious code. See [security](security.md).

Future cloud work can reuse the PNG/coordinate/input/argv contract with an appropriate provider guest transport and managed session. ARM64 Debian packages and x86_64 cloud packages/browser availability differ; no x86 image has been tested here. A richer Wayland option requires its own capture/input/readiness capability implementation, not a theme switch. Measure workload RAM/CPU and allocate cloud machines accordingly; no resource-cost or parity promise follows from local samples. No cloud backend, paid resources, billing, public deployment or release is created.

## Reproduce the hardware task suite

This test writes files, opens apps, types a shell command **in Linux**, and changes
browser/editor state. Use a newly created disposable VM only. It does not create,
reset or delete a machine, and never chooses a default state directory.

```sh
make build test check
WORKSTATION_STATE=$(mktemp -d /private/tmp/agentlinux-phase55-review-XXXXXXXX)
bin/agent-linux up --local --network=nat --cpus 4 --memory 8 --disk 40 \
  --state-dir "$WORKSTATION_STATE" --lima /absolute/path/to/limactl
mkdir -m 700 /tmp/agentlinux-new-evidence
AGENTLINUX_WORKSTATION=disposable \
AGENTLINUX_ACCEPT_CLI="$PWD/bin/agent-linux" \
AGENTLINUX_ACCEPT_ROOT="$WORKSTATION_STATE" \
AGENTLINUX_ACCEPT_LIMA=/absolute/path/to/limactl \
AGENTLINUX_ACCEPT_EVIDENCE=/tmp/agentlinux-new-evidence \
GOCACHE="$PWD/.cache/go-build" GOMODCACHE="$PWD/.cache/go-mod" \
go test -v ./internal/mcpserver -run '^TestRealWorkstation$' -count=1 -timeout=10m
```

Use a new evidence directory for each run and inspect PNGs plus the JSON report.
For Plasma, build the [research adapter](../experiments/plasma/README.md) first and
use its CLI/helper to create the new fixture; do not replace production source or
manually patch a persistent VM. After testing, stop only this fixture, inspect its
machine ID and explicit deletion plan, then deliberately delete it if desired.
Normal unit tests skip this opt-in test; they cannot boot VZ or certify a desktop.

Development failures retained separately: the first wallpaper was unreadable because the root provisioning umask made it 0600; fresh provisioning now sets the non-secret SVG to 0644. One early polished test showed entered text in a new untitled editor tab while the requested file tab remained empty. Removed a redundant editor launch during the file-open transition and now wait for the requested filename before typing; the exact saved-byte oracle is unchanged. The final fresh Xfce candidate also returned APP_UNAVAILABLE on its first Chromium launch. A bounded diagnostic launch and explicit retry succeeded; the first failure remains in the acceptance counts and its root cause is not established. No automatic replay or sandbox-disabling workaround was added.

## Actual results on the development Mac

Apple M4 Pro, 12 logical CPUs, 24 GiB RAM, ARM64 macOS 26.5 (25F71), Lima 2.2.1/VZ, Go 1.26.1. This is the existing development Mac, not a clean account or hardware matrix. Both final candidates actually reported Debian 13.7 and kernel 6.12.111+deb13-cloud-arm64. Same Chromium **156.0.8078.79-1~deb13u1** and Firefox ESR **153.6.0esr-1~deb13u1**; final Xfce session **4.20.2-2**, Greybird **3.23.3-1**, experimental Plasma/KWin **6.3.6-1**. Fontconfig resolved actual Noto Sans and Noto Sans Mono files.

| Observation | Existing Xfce | Polished Xfce (selected) | Repeatable Plasma/X11 |
| --- | --- | --- | --- |
| Complete isolated MCP task runs | 1/1 (another earlier run passed during candidate package installation) | 2/3; first Chromium launch failed, explicit retry and two later runs passed | 2/2; manual prototype failures/reversion excluded and retained separately |
| Successful task suite elapsed | 77.75 s | 76.88 / 78.28 s | 83.93 / 85.72 s |
| Screenshot request median per successful run | 0.483 s | 0.479 / 0.495 s | 0.498 / 0.512 s |
| Three warm starts to actual readiness | 8.83 / 8.81 / 8.71 s | 8.88 / 8.80 / 8.89 s | 8.91 / 8.88 / 8.85 s |
| Memory pressure immediately after readiness | 383 MiB | 420 MiB | 600 MiB |
| Post-task memory pressure | 1,088 MiB | 1,054 MiB | 1,368 MiB |
| Sparse VM disk physical allocation | 3.46 GiB | 3.48 GiB | 4.37 GiB |
| Logical VM disk | 40 GiB | 40 GiB | 40 GiB |

Memory rows are individual `MemTotal - MemAvailable` samples, not proven minimum requirements, settled-idle averages or complete application RSS sums. All candidates have the same allocation. Original-baseline client had one extra editor activation/screenshot; final candidate runs use the same corrected sequence (61 screenshots each). Task times include frequent observation, SDK/SSH and saving evidence, not AI reasoning time. Small differences are not statistical proof of a general speed ranking. No whole-stack CPU advantage is claimed: the observed Lima/VZ process snapshots were ~137/152/140 MiB RSS and lifetime `%CPU` 0.1/0.7/0.5 respectively; ha.pid and vz.pid named the **same** process, so do not sum them. Process RSS does not represent all guest/virtualization memory.

Fresh base download/provisioning/ready took 83.52 s for the original baseline, 227.69 s for the final polished guest and 226.88 s for the repeatable Plasma guest. These included different network/host/cache conditions, sometimes overlapping provisioning; **not a fair cold-start ranking**. The base download remains 337,510,400 bytes (~322 MiB); APT packages/storage grow. Final timed task/warm-start runs kept all other comparison VMs stopped. Defaults remain 2 CPUs/4 GiB/24 GiB (previously verified working Xfce configuration); the new recipe's comparison/acceptance profile is 4 CPUs/8 GiB/40 GiB. No new lower-resource acceptance or minimum-hardware claim is made.

The chosen guest also passed:

- Authenticated **Codex CLI 0.164.0-alpha.11.1**, real agent decisions over MCP: 10 completed guest-tool calls, five screenshot results, public browser navigation and a final description grounded in the inspected PNG. No host shell/guest command tools were enabled for that run. Read-only Mac pointer coordinates and clipboard-change counter were identical before/after; clipboard contents were not read. [Official stdio configuration](https://developers.openai.com/codex/mcp) and [ephemeral noninteractive mode](https://developers.openai.com/codex/non-interactive-mode) were rechecked. This separate public-page model demo is not the offline comparison fixture or a universal agent compatibility test.
- **Kitty 0.50.0**, actual native image inspected while real MCP changed the guest; 80×30, 140×25, 70×45, 18×5 and 120×40 resize requests (macOS clamped three heights), unavailable/reconnected states over an actual VM stop/start, q/Ctrl+C/SIGTERM exits with canonical terminal restoration and the VM still ready. Recording has 51 sampled terminal-window images over 43.97 s, not a video-FPS measurement.
- Main viewer process over 47.03 s: user+system CPU 1.47 s, max RSS 67.5 MiB; 21 capture attempts, 16 valid frames, five changed frames. Five capture errors occurred during deliberate restart/contention. These counters exclude SSH, Lima/VZ, Linux, Kitty and recording overhead. PNG requests during this competing/restart run ranged about 0.32–2.00 s, including failed/busy attempts; no measured screen-to-terminal latency or full-motion claim.
- Actual Settings window, exact saved Unicode document hashes across another stop/up, preserved deliberately changed font preference, stable machine identity/new guest session, and restoration of the initial font. This last stop/ready was 3.39/8.87 s. Guest clipboard ownership is ephemeral; saved documents persist.
- `make build test check`: real builds, all Go race tests, Python installer tests, vet and whitespace. New automated tests execute first-run shell logic with a fake settings daemon and verify failure/retry plus preservation. Existing malformed protocol/frame, routing, bounds, concurrency, terminal cleanup and destructive-path tests remain; none substitutes for hardware.

Private ignored `artifacts/phase55` retains raw successes/failures, classifications, inventories, metric JSON, genuine Linux PNGs, model events and the sampled Kitty recording. Images are not mockups. Cleanup verifies only these disposable namespaces lose their disks/keys; default/personal machines were never opened, reset, deleted or migrated. No claim of an unrelated-user-filesystem hash audit is made. No release/tag/image upload, public deployment, paid infrastructure or billing activation occurred.

**Decision:** [ADR 0012](decisions/0012-agent-workstation.md). Retain a complete current-stable Xfce workstation, improve its visual defaults, save modest RAM/disk and keep one proven production control stack. Plasma passed and remains a credible research alternative; no inherent reliability superiority is proven. GNOME's larger integration gap remains explicit. Next investigate cold-profile Chromium launch reliability, then implement input ownership/human viewer interaction; qualify any Wayland replacement with the same acceptance rather than assuming parity.

Cleanup completed for all five Phase 5.5 fixtures after matching each original machine ID and ownership marker. Only each namespace’s marker/lock remains; VM disks, runtime configuration and credentials were removed. Private screenshots/reports/recording were retained deliberately. The isolated Kitty process/socket was also closed. No default/personal namespace was controlled.
