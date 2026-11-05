# Guest computer protocol 2.0

Service version **0.2.0**, Linux ARM64. This is the implemented internal protocol, not MCP. Version 1.0 guests fail explicitly; there is no silent fallback, implicit disk replacement or automatic migration. Preserve a Phase 1 machine and use a fresh `--state-dir` for Phase 2. New CLI `stop` can still stop a valid old runtime without a guest handshake.

## Channel, readiness and capabilities

Host-initiated restricted SSH stdio bridges to `/run/agent-linux/control.sock` (0600, parent 0700). Per-machine keys and strict pinned host-key verification remain. SSH does not become an arbitrary host/guest shell endpoint: commands are structured requests to the non-sudo guest service. Lifecycle `ready` now requires a real service handshake/probe **and** a live managed X11 display, expected geometry, window manager and generation marker. Status distinguishes an unavailable desktop from a ready guest service.

`internal/machine.Machine` is the common interface: Status/capabilities, Screenshot, Input, Exec, and optional fixed-app Open. `machine.Local` routes these through the locked local lifecycle controller to Lima/SSH. No cloud implementation exists. Status returns supported operations, input/paste modes, one display, PNG/top-left/scale 1, and limits; `desktop.ready` reports current availability, not a guessed supported-feature flag.

## Framing and identity

UTF-8 newline-delimited JSON. Maximum request: **256 KiB including newline**, checked before transmission and by the guest reader. Maximum response: **16 MiB**; maximum PNG: **8 MiB**. Invalid UTF-8, duplicate keys, unknown fields, broken framing and oversized messages fail safely. Four active connections, at most 64 requests per connection, 75-second connection read timeout (command limit is 60 seconds), bounded response-write deadlines. Stdout contains protocol only.

Requests carry `version`, random `id`, `machineId`, `method` and, after hello, `sessionId`. IDs are random 128-bit values represented as 32 hex characters; they are not credentials. Hello binds the connection and returns versions, machine, boot and service-session identities. Health/probe are fixed read-only operations; probe computes SHA-256 of a bounded nonce. Responses echo version/request ID and contain exactly one result/error. All results include machine/protocol/session association. Error code/message text is sanitized before human terminal output.

Restarting the helper changes service-session ID and clears observed-frame metadata; reboot changes boot ID too. Restarting the graphical session changes its independent generation marker. Reconnect performs a fresh handshake. Desktop/exec mutations are sent once and never automatically replayed. A lost acknowledgement is OUTCOME_UNKNOWN, even when cancellation was requested; observe before retrying.

## Screenshots and input

One Xvfb X11 screen, **1280×800 physical guest pixels**, top-left origin, no scaling (scale=1), PNG. Other resolutions/multiple displays are not supported. Capture runs scrot against fixed guest DISPLAY=:1/XAUTHORITY. No old PNG is cached or returned as current. Each frame includes random ID, service-session ID, display generation, sequence and guest UTC capture time. Generation is checked before/after capture. Host validates size, PNG structure and matching dimensions before writing a new output file (0600, no overwrite).

Input requires an observed frame ID, valid service session, current display generation and age <=10 seconds measured with the guest's monotonic clock. At most 16 frame metadata records are retained. `--frame-id` uses an explicit screenshot reference; direct CLI input without it captures a fresh frame immediately before dispatch. This convenience does not guarantee that application layout remains unchanged between observation and action. Frame IDs from a restart or expired observation fail STALE_FRAME.

- Move/click/drag/scroll coordinates are integers: 0<=x<1280 and 0<=y<800. Out-of-range inputs fail; no silent clamping or host fallback.
- Buttons: left/middle/right; clicks: one or two. Drag is interpolated with bounded duration and button release. Scroll deltas are wheel steps, -100..100; positive dy=down and positive dx=right.
- Motion avoids the observed repeated-coordinate `xdotool --sync` hang. It sends motion, then verifies actual guest pointer coordinates with bounded queries.
- Key combinations use unique CTRL/ALT/SHIFT/SUPER plus documented letters/digits/F1–F12/ENTER/TAB/ESC/BACKSPACE/DELETE/HOME/END/PAGEUP/PAGEDOWN/arrows/SPACE. Command/CMD is not silently mapped to Linux Ctrl. Invalid keys fail. Modifiers and mouse buttons are released after operations, including failure/cancellation paths where the display is reachable.
- Text is literal UTF-8 up to 64 KiB without NUL. Guest-only xclip ownership is kept alive through format/data negotiation and a bounded paste window, then terminated. Standard paste is Ctrl+V; terminal mode is Ctrl+Shift+V. No host clipboard synchronization or reading. Paste acceptance varies by application (password fields/slow consumers/IME restrictions); dispatch acknowledgement is **not proof of insertion**. Verify via a new screenshot or application state. Actual Mousepad ASCII/CJK/emoji round-trip passed; arbitrary applications are not promised identical behavior.

X11 clients in this guest can observe/inject into the same desktop. Wayland/portal/PipeWire control is unimplemented. Clipboard contents are briefly exposed to other guest X11 clients. Service restart closes its owned GUI apps; the desktop itself is a separate managed unit. Open(editor|terminal|browser) provides a bounded visible-window check for the first demo; Chromium is not launched with `--no-sandbox`.

## Guest execution and trust grant

Control credentials grant the ability to run arbitrary **guest** commands as the non-sudo `agent` account, access guest files, and operate its desktop. They do not grant host shell execution. No argv string is interpreted as a host shell command. A shell is used only if explicitly requested, e.g. `exec -- /bin/sh -c '...'`; the shell runs in Linux.

Exec carries an argv array (1..128 values, total <=64 KiB), clean absolute guest cwd (default /home/agent), <=32 environment overrides (valid names, values <=8 KiB), timeout 1..60000 ms, and output limit 1..262144 bytes **per stream**. Default CLI timeout is 20 seconds, output 64 KiB/stream. Baseline environment is fixed guest PATH/HOME/user/UTF-8 locale/display/authority/D-Bus settings, not inherited host credentials. A basename resolves against the requested guest PATH; relative executable paths with slashes are rejected. Stdin is EOF; no PTY or streaming stdin in this version.

Results provide distinct stdout/stderr bytes, exit code, duration, timeout/cancellation/truncation and cleanup confirmation. JSON encodes byte streams as base64 (null means empty); human CLI output shows UTF-8 while escaping terminal control codes, preventing guest OSC clipboard/terminal commands from executing on the Mac. CLI propagates ordinary nonzero guest exits (1..125); timeout is 124, and uncertain cleanup/cancellation/output-limit failure is nonzero.

Linux clone3/CgroupFD places the process directly into an owned delegated cgroup before execution, avoiding a start-then-move fork race. Timeout, disconnect cancellation, output overflow and normal completion call cgroup.kill and remove the emptied scope, including ordinary forked/setsid children. Unsupported delegation fails EXEC_UNAVAILABLE; there is no weaker silent fallback. Output overflow terminates rather than buffering indefinitely. Ctrl+C cancels the host request and closes SSH; guest EOF detection cancels the request and its scope. No separate cancel message is implemented.

This is lifecycle cleanup, not a new security sandbox or rollback mechanism: command effects persist, and deliberately malicious guest code may tamper with its own delegated runtime or invoke other guest services. No per-command CPU/RAM quota is claimed; VM allocations remain the resource boundary. Systemd limits the service's writable system paths; home/tmp are usable, sudo and implicit host mounts are absent. NAT can reach host/LAN services. A VM reduces exposure but does not guarantee containment of malicious code or network attacks.
