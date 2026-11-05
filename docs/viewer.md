# Live desktop viewer

`agent-linux view` consumes the **actual Linux guest's** scrot PNG capture through the common machine interface. It never captures or controls the Mac desktop. The first terminal target is **Kitty 0.50.0**, using direct Kitty graphics-protocol PNG chunks. Other terminals, redirected output, tmux/screen, failed capability queries and missing terminal pixel dimensions fall back to an authenticated live loopback web viewer.

**Verification status (Chrome 156.0.8078.80 fallback; Kitty 0.50.0 PTY/protocol only):** real guest streaming, MCP concurrency, live browser fallback and reconnect/pause behavior passed on macOS 26.5. Kitty 0.50.0 capability/PNG acknowledgement, actual PTY resizing and cleanup were exercised. **Visible Kitty graphics and the terminal video demo remain pending:** the available Mac was locked with its display asleep, and graphics were absent from internal captures. That is not counted as a passed visual test or a proven terminal compatibility claim. An unlocked, awake Mac is required for the remaining checklist below; if graphics remain blank then, the terminal renderer still needs investigation.

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

## Tests and remaining acceptance

`make build test check` includes capability/layout/chunking/exit-key tests, invalid PNG/size checks, bounded capture/cache/concurrency, disconnect/reconnect, terminal cleanup on cancellation/output error, CLI/help validation, web authorization/Host/Origin/cache behavior and bounded lock waits. These use machine/terminal doubles and handler-level HTTP fixtures, not a simulated VM presented as hardware evidence.

Real local browser acceptance (requires optional test-only `websocket-client==1.9.0`, Chrome, and an already started **disposable** fixture VM/server):

```sh
python3 scripts/viewer_web_acceptance.py --cli bin/agent-linux \
  --lima /absolute/path/to/limactl --state-dir "$TEST_STATE" \
  --viewer-output "$PRIVATE_VIEWER_OUTPUT" --evidence artifacts/new-web-test \
  --chrome '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' --restart
```

The script restricts restart to agentlinux-viewer-acceptance-* namespaces, uses a temporary Chrome profile, tests a real official MCP client changing the guest, records real page PNGs, rejects unauthorized requests, checks pause and reconnect, and cleans its own browser profile. It does not delete a VM it did not create. Its CDP listener is test-only loopback, never part of the product. Remote research/browser tools cannot access this loopback URL; this test uses local Chrome.

Remaining **unlocked-Mac terminal checklist**, still pending:

1. Start the guest and open Kitty 0.50.0 (official binary, not a multiplexer). Run view in that terminal.
2. Visually confirm the Linux panel/windows are present in the terminal, not merely a LIVE status. Capture a recording scoped to that test window with appropriate local permissions.
3. Use a real MCP client to open/type in a guest application; observe those exact changes in Kitty.
4. Resize wide/tall/small; confirm aspect ratio, status placement and no clipping/ghost images. Actual 120×40 → 80×30 PTY geometry was tested; its visible image fit was not.
5. Exit with q and Ctrl+C; run normal shell input, compare terminal settings, then repeat with SIGTERM and a failure path. Real q/SIGTERM persistent termios and canonical input checks passed; Darwin's transient PENDIN bit clears on ordinary input and is separately recorded.
6. Confirm VM remains running/ready, stop it explicitly, and record the tested host/guest/Kitty versions and active CPU/RSS/latency/image-update counts.

See [verification](verification.md) for measured observations and the exact blocked test. Optional `scripts/viewer_terminal_probe.py` runs only inside an isolated test Kitty window and records TTY state; it does not certify visual output by itself.

Official sources consulted before implementation: [Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/), [Kitty binary installation](https://sw.kovidgoyal.net/kitty/binary/), [Kitty remote control](https://sw.kovidgoyal.net/kitty/remote-control/), [official Kitty 0.50.0 release](https://github.com/kovidgoyal/kitty/releases/tag/v0.50.0). Official Go x/term v0.40.0 source is pinned in go.mod and its BSD license retained in third_party; x/sys remains v0.41.0.
