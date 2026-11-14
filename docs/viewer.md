> Current Phase 8 acceptance includes native Kitty and Chrome real guest human widgets; see [performance](performance.md) and [verification](verification.md). Earlier phase notes below are historical.

# Live desktop viewer

`agent-linux view` consumes the **actual Linux guest's** scrot PNG capture through the common machine interface. It never captures or controls the Mac desktop. The first terminal target is **Kitty 0.50.0**, using direct Kitty graphics-protocol PNG chunks. Other terminals, redirected output, tmux/screen, failed capability queries and missing terminal pixel dimensions fall back to an authenticated live loopback web viewer.

**Verified configuration:** Kitty **0.50.0** and Chrome **156.0.8078.80** fallback on Apple silicon macOS **26.5 (25F71)** with the actual Lima 2.2.1/VZ Debian/Xfce guest. Real screenshots of the Kitty test window show Linux, MCP-driven Chromium changes, aspect-preserving wide/tall resize, tiny-window guidance, unavailable state and reconnection. q, raw Ctrl+C and SIGTERM restore terminal input and leave the VM ready. A private sampled test-window video is recorded. This establishes one tested configuration, not support for every terminal/Mac or a full-motion performance claim. Earlier locked-display blank captures remain documented as unsuccessful historical evidence.

## Start, view and exit

```sh
agent-linux up --local --network=nat --lima /absolute/path/to/limactl
agent-linux view --lima /absolute/path/to/limactl
# Force the working live fallback:
agent-linux view --web --lima /absolute/path/to/limactl
# Optional: minimum cadence and a NEW private metrics file:
agent-linux view --interval 500ms --metrics /tmp/viewer-metrics.json --lima /absolute/path/to/limactl
```

Use the same `--state-dir /absolute/private/state` on up/view/MCP if nondefault. `view` requires an existing machine; it does not create, start, stop, delete, or silently switch one. A stopped machine is shown as unavailable and the viewer retries until the user starts it. `view --help` lists options and limitations.

In Kitty, **q, Q or Ctrl+C exits**. SIGTERM also unwinds cleanup. The viewer uses an alternate screen, hides/restores the cursor, saves/restores raw terminal settings and original output flags, and deletes only its own image. No shell profile/global terminal settings are edited. SIGKILL, power loss, or destruction of the terminal itself cannot guarantee cleanup. After an externally killed process, the usual terminal `reset`/`stty sane` recovery may be necessary. Escape/control-key replies are consumed, never forwarded to Linux.

In fallback mode, open the private clickable `http://127.0.0.1:<port>/#<capability>` printed by the CLI. **Ctrl+C closes the server**; closing the browser page stops its capture requests but leaves the server and VM available. Pause/hidden pages stop requesting new frames. Reloading a stripped URL loses the in-memory capability; reopen the original printed URL. Neither exit path stops the VM. Use `agent-linux stop` explicitly when done.

## Watch-only interaction model

This version is deliberately **view only**. Mouse, typing, clipboard shortcuts and terminal keys other than local exit are not guest control. Use MCP or existing guest-only CLI input commands in another terminal. Agents still use screenshot → act → screenshot → verify with original 1280×800 coordinates and recent frame IDs. Resizing a viewer does not change guest coordinates. No host pointer/clipboard fallback is present.

The viewer reads the same guest session that agents operate. Each capture or guest action holds the existing machine lock; guest calls now wait up to two seconds (within the caller's deadline) before returning MACHINE_BUSY. The viewer has no input sequences to interleave. MCP's own conflicting-call rejection remains. There is no human input lease, takeover UI or multi-client transaction guarantee yet. Direct human interaction should be added only with an explicit ownership design. A high capture rate can evict recent guest frame references; recapture on STALE_FRAME, never blindly replay mutations.

## Rendering, resizing and bounded work

The guest remains one X11/Xvfb display: **1280×800 PNG, scale 1, top-left origin**, up to 8 MiB. Kitty support is actively queried rather than inferred solely from TERM. Actual TIOCGWINSZ pixel/cell dimensions determine an aspect-preserving placement with status/footer rows reserved; no cell aspect ratio is guessed. The renderer sends base64 chunks of at most 4096 bytes using direct transmission, one fixed image ID, quiet responses and no cursor advancement. It never asks the terminal to read a guest-provided host path. On resize the cached image is re-placed/retransmitted for the new layout; very small terminals show a resize status.

One capture at a time, one cached image, no frame queue. Backend deadlines are six seconds, terminal output deadlines five seconds, cleanup writes one second. Web shutdown closes request contexts and waits up to three seconds for canceled capture/subprocess cleanup before claiming it completed. Slow terminal output backpressures capture; an output failure exits through cleanup rather than accumulating frames. Identical PNG hashes are not re-uploaded to Kitty; HTTP ETags produce body-free 304 replies. Static screens gradually reduce capture cadence to five seconds; changed images reset it. This can add several seconds before noticing a change after idle. The interval flag is a minimum capture interval, **not achieved FPS**.

No connected browser requests means no web capture worker. Paused/hidden pages stop new requests; an already running request may finish. The web server limits connections/handlers and shares the same one-frame cache. Blob URLs are revoked when replaced. Metrics retain at most 256 timing samples. PNG input is size/dimension/format validated before output. Disconnected/invalid captures show coded status and do not masquerade as current frames; browser frames are cleared on errors, terminal busy status explicitly labels the last capture age. Reconnection does not restart the VM or replay guest actions.

This first implementation still transfers full PNGs from the guest for every actual capture, even if the host later finds the image unchanged. The guest service/capture/SSH round trips, repeated PNG validation, encoding/base64, terminal bandwidth and browser memory are costs to measure. Future work can add negotiated guest-side conditional frames, persistent transport, damage tracking or a measured codec. No full-motion, zero-resource, minimum-hardware or fixed-FPS claim is made.

## Local fallback boundary

A fresh 256-bit capability authorizes image/metrics requests. It is printed only as the explicit local connection URL; no access log is enabled. The browser reads the URL fragment, removes it from the address/history entry, and holds it in memory, not cookies or persistent storage. Frame requests use a bearer header. Bind address is hardcoded **127.0.0.1**, port allocated by the OS; no LAN/cloud listener option exists. Host and Origin checks reject DNS rebinding and cross-origin use. CSP denies embedding and external scripts/resources, responses are no-store/nosniff, and referrers are disabled. The page uses textContent and validated PNG blobs; no guest HTML/escape sequences are interpreted.

This is a trusted local-user/process boundary, not protection against malware/root/browser extensions on the Mac. Anyone deliberately given the private URL receives read access to the selected guest screen. The web viewer exposes no command/input/lifecycle endpoint. Future hosted viewing requires separate HTTPS, user/session authorization and infrastructure isolation; this loopback listener is not a cloud implementation.

## Tests and hardware acceptance

`make build test check` includes capability/layout/chunking/exit-key tests, invalid PNG/size checks, bounded capture/cache/concurrency, disconnect/reconnect, terminal cleanup on cancellation/output error, CLI/help validation, web authorization/Host/Origin/cache behavior and bounded lock waits. These use machine/terminal doubles and handler-level HTTP fixtures, not a simulated VM presented as hardware evidence.

Real local browser acceptance (requires optional test-only `websocket-client==1.9.0`, Chrome, and an already started **disposable** fixture VM/server):

```sh
python3 scripts/viewer_web_acceptance.py --cli bin/agent-linux \
  --lima /absolute/path/to/limactl --state-dir "$TEST_STATE" \
  --viewer-output "$PRIVATE_VIEWER_OUTPUT" --evidence artifacts/new-web-test \
  --chrome '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' --restart
```

The script restricts restart to agentlinux-viewer-acceptance-* namespaces, uses a temporary Chrome profile, tests a real official MCP client changing the guest, records real page PNGs, rejects unauthorized requests, checks pause and reconnect, and cleans its own browser profile. It does not delete a VM it did not create. Its CDP listener is test-only loopback, never part of the product. Remote research/browser tools cannot access this loopback URL; this test uses local Chrome.

The complete terminal hardware checklist passed on the configuration above. Reproduce against a dedicated disposable VM (`TEST_STATE` must resolve directly under `/private/tmp` with an `agentlinux-viewer-acceptance-*` name). Start an isolated Kitty instance with a private socket; no personal terminal or global settings are altered:

```sh
/absolute/path/to/kitty --config NONE --listen-on "unix:$PRIVATE_SOCKET" \
  -o allow_remote_control=socket-only -o shell_integration=disabled \
  -o confirm_os_window_close=0 python3 -c 'import time; time.sleep(3600)'
# In another terminal, with the disposable VM already ready:
python3 scripts/viewer_terminal_acceptance.py --cli bin/agent-linux \
  --lima /absolute/path/to/limactl --state-dir "$TEST_STATE" \
  --kitten /absolute/path/to/kitten --socket "$PRIVATE_SOCKET" \
  --evidence artifacts/new-terminal-test --restart \
  --ffmpeg /absolute/path/to/ffmpeg
```

Use a private parent directory for the socket. Test-only dependencies: Pillow, Go, Kitty 0.50.0; ffmpeg is optional and only encodes the scoped recording. The harness creates only its own windows on that socket, runs the official SDK against the real MCP process/guest, requires visible guest pixels and a changed central image region (status text alone cannot pass), resizes, checks tiny-window guidance, optionally stops/restarts only the selected fixture, then checks q/Ctrl+C/SIGTERM restoration and VM identity/readiness. It does not create/delete the VM or terminate an unrelated Kitty instance. Close the isolated seed window and explicitly stop/delete your fixture afterward.

The OS can clamp requested cell sizes; the report records requested and actual geometry. Observed sizes were 120×40, 80×30, 140×24, 70×44 and 18×4. Wide/tall captures preserve the original 1280:800 aspect ratio; tiny windows show RESIZE and q exit guidance rather than a clipped identity. Darwin's transient PENDIN bit differs immediately after raw→canonical restoration; persistent termios flags restore, normal canonical input works, and exact stty strings match after that input. The report records all three observations separately.

The recording samples only the test window with a 500 ms pause between captures (actual rate is lower when capture work is slow), retains actual sample timestamps and encodes variable-duration frames. This is evidence of visible changes, not measured terminal paint FPS or full-motion recording. Initial internal GPU snapshots occasionally contained only text; the test explicitly waits for actual guest pixels. On the successful unlocked run, an OS display-asleep flag still reported true, so that flag alone is not a reliable blocker or graphics-health check. See [verification](verification.md) for timings/resource counters and retained failure history.

Official sources consulted before implementation: [Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/), [Kitty binary installation](https://sw.kovidgoyal.net/kitty/binary/), [Kitty remote control](https://sw.kovidgoyal.net/kitty/remote-control/), [official Kitty 0.50.0 release](https://github.com/kovidgoyal/kitty/releases/tag/v0.50.0). Official Go x/term v0.40.0 source is pinned in go.mod and its BSD license retained in third_party; x/sys remains v0.41.0.

## Phase 6 interactive mode (2026-11-05)

`agent-linux start` boots/resumes local Linux and opens an interactive viewer; `view --interactive` attaches to an already running machine. Plain `view` remains view-only. Local `start` defaults to NAT and a new 2-CPU/4-GiB/24-GiB machine; existing machines retain their configuration. Startup readiness is a real guest/desktop handshake. No implicit reset or cloud allocation occurs.

Kitty 0.50.0 is the initial terminal. Interactive controls: ordinary typing, arrows/navigation/control letters, SGR mouse clicks (left/middle/right), scroll, bracketed Unicode paste; F2 takes/gives input ownership, F3 switches paste mode, Ctrl+] exits and leaves the VM running. Ctrl+C is guest input, and q types q. Mouse coordinates are terminal cell centers mapped through the actual aspect-correct image placement; bars do not send actions. Continuous drag and full shortcut/IME support are pending. The existing agent drag operation remains available.

The viewer saves and restores terminal termios, alternate screen/cursor and queried mouse/bracketed-paste modes. Abrupt SIGKILL cannot execute cleanup; terminal reset may be needed. Agent screenshots remain available during human ownership; agent mutations receive INPUT_OWNED. Ownership is renewed every five seconds, expires after fifteen, and is revoked on desktop generation change. Reacquire explicitly after an error/restart; uncertain mutations are never replayed.

The interactive web fallback provides Take control/Give agent control, text entry, terminal-paste selection, shortcut input, image clicks and wheel scrolling. It uses the same authenticated loopback/frame path, does not read navigator.clipboard, and has no host shell or lifecycle endpoint. Browser keyboard input uses explicit fields rather than claiming complete native desktop keyboard behavior. Cloud viewers use `--cloud-config FILE`; simulated sessions are labeled and remote expiry is shown. See [cloud boundaries](cloud.md) and [verification](verification.md).

Real Phase 6 acceptance passed in Kitty 0.50.0 on the documented development Mac: human PTY typing/Unicode/mouse, explicit MCP handoff, resize, Ctrl+] exit, canonical input and original mouse/paste mode restoration. Web human widgets have automated validation/auth tests; full real-browser input acceptance is still pending. [Exact evidence and timings](verification.md).

## Local hardening capture transport

Local viewers default to a viewer-owned persistent pinned SSH capture channel.
Use `--capture-transport oneshot` for the original diagnostic transport. Capture
failures are visible; the next observation reconnects, without replaying input.
Connections recycle below guest request/idle limits. CLI screenshots remain
one-shot. Native capture is asynchronous with one latest update; two fixed Kitty
image buffers preserve the previous placement until its replacement is submitted.
The new native painting path still requires fresh visual acceptance; see
[hardening evidence](local-hardening-verification.md). Closing either viewer
leaves the machine running.
