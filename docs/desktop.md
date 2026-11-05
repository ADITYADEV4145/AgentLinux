# Linux desktop and Phase 4 handoff

Phase 3.5 keeps Debian 13 (trixie), ARM64, Xfce and one authenticated Xvfb X11 display. The dated base remains genericcloud 20261001-2618, checksum-pinned in [manifest](../images/debian/manifest.json). New machines use recipe `desktop-3.5.0`, helper 0.3.5 and guest protocol 2.0. Exact observed distribution point release, kernel, applications, timings and disk usage are recorded in [verification](verification.md); the base build date is not a claim that its packages stay current.

## What existed and what changed

Phases 1–3 already implemented actual VZ boot/lifecycle, Debian/Xfce/Xvfb, Chromium, Mousepad, xfce4-terminal, Thunar and settings packages, scrot capture, XTEST input, Unicode guest clipboard paste, arbitrary non-sudo guest commands and the official MCP SDK adapter. Phase 2 and Phase 3 have real hardware evidence; the successful pushed Phase 3 CI run is 41995229635. File/settings packages existed, but fixed-app control exposed only editor/terminal/browser, and their GUI usability was not established by that earlier browser demo.

Phase 3.5 adds explicit launcher/files/settings/Firefox/Chromium launch targets, focusing existing windows, Firefox ESR, desktop icons/menu utilities, a small development baseline, explicit operator package maintenance and a versioned recipe. Readiness now requires the X server, window manager, panel and desktop process, rather than a bare window manager. Existing screenshot/input/exec/MCP paths are reused. No host screen/pointer/clipboard APIs or shared filesystem are introduced.

```sh
make build test check
# Supply --lima /absolute/path/to/limactl before positional arguments if not on PATH.
bin/agent-linux up --local --network=nat
bin/agent-linux open launcher
bin/agent-linux open files
bin/agent-linux open settings
bin/agent-linux open terminal
bin/agent-linux open editor
bin/agent-linux open chromium
bin/agent-linux open firefox
bin/agent-linux screenshot --output /tmp/agent-linux-desktop.png
bin/agent-linux browsers list --json
bin/agent-linux mcp-config --client codex
bin/agent-linux stop
```

The PNG output path must be new; existing files are not overwritten. The image and MCP client expose the actual graphical computer. Phase 4 adds a view-only Kitty implementation (0.50.0 visually verified on macOS 26.5) and a verified live local web fallback; see [viewer.md](viewer.md). `browser` remains an explicit alias for Chromium; `firefox` launches Firefox ESR. Chrome and Edge are never substituted. The panel's Applications menu and Application Finder provide ordinary guest desktop launching; Thunar manages files, xfce4-terminal runs Bash, Mousepad edits files, and Xfce Settings configures the guest. Guest home directories include Desktop/Documents/Downloads/Projects.

## Keep the existing distribution and display model

| Choice | Specific benefit | Cost / evidence limit |
| --- | --- | --- |
| Debian 13 + Xfce/X11 (selected) | Working ARM64/VZ/automation path; signed apt repositories, both distro browsers; small component recipe | Security updates require maintenance; apt versions evolve; X11 apps share screen/input; no benchmark says it is fastest |
| Ubuntu ARM64 + Xfce | Broad packages and similar X11 tools | Would require cloud-init/image and browser packaging revalidation; no measured benefit here |
| Fedora + a Wayland desktop | Maintained ARM64 packages and modern desktop integration | XTEST/scrot/xclip are not equivalent native-Wayland controls; compositor/portal permissions require a separate implementation and tests |
| Alpine/minimal custom image | Potential smaller base | musl/package/browser differences and more provisioning work; no measured complete-desktop advantage |

Debian documents full support through August 2028 followed by a reduced-architecture LTS term through June 2030; individual package/security status must still be checked. The package recipe is repeatable, not byte-reproducible without repository snapshots. We retain the working ARM64 image and measure the actual guest rather than infer speed from the operating system name. The second browser increases image size; optional Node/build profiles avoid including those larger dependency trees by default.

## Display, input and clipboard

The root-installed desktop launcher runs as `agent`, creates Xauthority in an owner-only guest runtime directory, starts Xvfb `:1` at 1280×800×24 with no TCP listener, starts a guest D-Bus session and Xfce, then publishes a fresh generation only after desktop components exist. systemd restarts the desktop on failure. Live health checks detect missing components; actions fail visibly while unavailable. A service/desktop restart invalidates frames; a VM restart creates new boot/session identity while preserving disk data.

scrot captures **guest** PNGs; maximum 8 MiB, scale 1, origin top-left, one display. XTEST/xdotool sends input only to that authenticated guest X server. Each MCP input requires an observed screenshot ID, valid for 10 seconds and the same desktop generation/session. Observe → act → screenshot → verify remains mandatory. Client preview resizing does not change the original coordinate system.

Unicode text entry temporarily owns the **guest** clipboard and sends Ctrl+V (Ctrl+Shift+V for terminals). Normal applications can also copy/paste inside Linux; exact Unicode copy/paste must be verified, not inferred from dispatch. The temporary paste owner can replace a previous guest clipboard value. Applications may reject paste, and clipboard data owned only by an application may disappear when that application exits. No persistent clipboard history or host clipboard sync is added. X11 applications are not mutually isolated; Wayland capture/input permissions are a distinct future design, not an interchangeable switch.

## Local and future cloud

Local support is ARM64 Linux on Apple silicon through Lima 2.2.1/VZ. Cloud is still unimplemented. A future ARM64 guest could reuse package choices but needs provider-specific boot/network/identity provisioning; x86_64 needs a separately trusted amd64 base and guest binary. Browser availability differs (notably Edge); [browser compatibility](browsers.md) distinguishes architecture from actual tests. Neither cloud architecture is validated here.

Move source files and ordinary documents through a future explicit export/import facility or user-managed guest networking. Do not promise portable executables, identical browser profiles, transferable encrypted credentials, VM disk compatibility, live migration, or application parity. Persistent machines retain their own disk; provider ephemeral sessions require a separate lifecycle contract. No cloud stub, host mount or automatic data transfer is added.

## Phase 4 inputs (handoff; current viewer status is in viewer.md)

- Real Xfce desktop and apps, authenticated guest X11 capture/input and real MCP operations.
- Current source: bounded scrot PNG frames, 1280×800, scale 1, one display, top-left coordinates; generation/session/freshness rules already defined.
- First terminal: Kitty direct, chunked PNG graphics, bounded refresh; see ADR 0003. Start from screenshot refresh and measure achieved rate before promising motion. No guest-supplied escape sequences or host paths should be interpreted by a viewer.
- Remaining work: live rendering, terminal input mapping, resize/Retina behavior, bounded queues/backpressure, terminal restoration, human/agent ownership/takeover, and authenticated browser fallback.
- Likely costs: scrot process/PNG encode, SSH handshake per call, image validation/serialization, terminal transfer and browser CPU/RAM. Measure each stage, bytes/frame, p50/p95 latency, host/guest RSS and CPU. A few screenshot samples are not an FPS or minimum-hardware guarantee.

Official references: [Debian release/support](https://www.debian.org/releases/trixie/), [Xfce session](https://docs.xfce.org/xfce/xfce4-session/start), [Applications menu](https://docs.xfce.org/xfce/xfce4-panel/applicationsmenu). See [updates/persistence](../images/debian/README.md), [development tools](development.md), [Go/Rust decision](decisions/0008-stack-and-desktop.md) and [verification](verification.md).
