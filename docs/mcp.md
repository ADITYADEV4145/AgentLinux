# MCP and agent clients

Phase 3, researched and implemented 2026-11-05. The stdio server uses the official Go MCP SDK v1.8.0, not a handwritten JSON-RPC implementation. It adapts `internal/machine.Machine` to a single existing local machine. No cloud or lifecycle tool is advertised. Start the VM yourself first:

```sh
make build test check
bin/agent-linux up --local --network=nat --lima /absolute/path/to/limactl
bin/agent-linux mcp --lima /absolute/path/to/limactl
```

The last command is a stdio protocol endpoint, not an interactive terminal. Clients launch it as a subprocess. Use absolute binary/runtime paths. `--state-dir /absolute/private/state` selects an existing Phase 2-compatible machine; omit it for the normal default. Phase 3 does not require a guest upgrade from Phase 2. Phase 3.5 new launch targets require helper 0.3.5; the host also accepts legacy 0.2.0 and does not silently upgrade it. Inspect live capabilities; old guests may reject newer app names. Closing MCP cancels outstanding operations and closes SSH subprocesses; it **does not stop the VM**. Use `agent-linux stop` when finished.

## Tool contract

Every result includes `{ok, code, message?, data?}` as `structuredContent`, mirrored in a text block for clients that use text results. Application failures set `isError: true`; JSON-RPC/method/protocol errors remain SDK protocol errors. Each tool advertises an input schema and the result envelope schema. Unknown fields are rejected. Read-only annotations identify status and screenshot; other tools retain conservative side-effect defaults. Annotations and agent instructions are advisory, not authorization boundaries.

| Tool | Arguments | Successful data |
| --- | --- | --- |
| `computer_status` | none | Real lifecycle state, guest identity/version, live capabilities/readiness |
| `computer_screenshot` | none | Frame metadata plus a separate SDK `image/png` image content block |
| `computer_move` | frameId, x, y | Guest input acknowledgement |
| `computer_click` | frameId, x, y; optional button, clicks | Left/middle/right, 1 or 2 clicks |
| `computer_drag` | frameId, x, y, toX, toY; optional button | Guest input acknowledgement |
| `computer_scroll` | frameId, x, y, dx, dy | Wheel steps, -100..100; positive right/down |
| `computer_type` | frameId, text; optional pasteMode | UTF-8 through guest clipboard; standard or terminal paste |
| `computer_key` | frameId, key | ENTER, TAB, arrows, F1..F12, etc.; CTRL/ALT/SHIFT/SUPER combinations |
| `computer_open` | app: browser/chromium/firefox/editor/terminal/files/settings/launcher | Fixed guest app launch/focus; screenshot still needed to verify |
| `computer_run_command` | argv; optional cwd, env, timeoutMs, outputLimit | Guest exit status, separate base64 stdout/stderr, timeout/cancellation/truncation/cleanup indicators |

Images are current guest PNGs, 1280×800, scale 1, origin top-left, one display. Metadata includes `id`, `sessionId`, desktop `generation`, sequence, guest UTC capture time, width/height/scale/format. PNG bytes appear once in image content, never duplicated in structured metadata. The original pixel coordinate system applies even if a client displays a resized preview. An image-capable client/model is required for visual reasoning; a text-only client can inspect metadata/commands but cannot claim to have seen the desktop.

Input **requires** an observed screenshot ID, unlike the CLI's optional automatic capture. Guest validation expires it after 10 seconds, helper/VM restart, or a changed graphical session. Capture again on `STALE_FRAME`; do not blindly replay actions after timeout/disconnection. Screenshots after actions are the evidence of application behavior; dispatch is not proof of insertion/navigation. Unicode/paste limitations are inherited from the actual X11 adapter; there is no host clipboard/pointer fallback.

Command argv executes as the guest `agent` user, not on macOS. Defaults: /home/agent, 20-second guest timeout, 64 KiB output per stream. Maximum: 60 seconds, 256 KiB per stream; bounded environment/argv and clean absolute cwd are enforced by the common protocol. No implicit shell. Explicit `/bin/sh -c` grants guest shell semantics. Nonzero exit, command timeout/cancellation and unconfirmed cleanup return an error **with the bounded execution data preserved**. Guest output is untrusted; do not render arbitrary decoded terminal control sequences on the host. Arbitrary guest execution may access or modify all agent-owned guest data and use its network. Cgroup cleanup is not an adversarial sandbox.

## Limits, errors and concurrency

The SDK handles initialize/version negotiation, framing, list/call, cancellation and connection cleanup. It supports revisions through 2026-07-28, including older supported revisions; the guest wire protocol remains independently versioned 2.0. No raw MCP traffic or SDK wire debug logger is enabled.

Inbound stdio lines are capped at 256 KiB, with SDK JSON nesting limits. Oversized framing closes the transport; the client must reconnect. Arguments are also bounded and schema-validated before reaching the backend. Guest PNG maximum is 8 MiB and response maximum 16 MiB. Clients must accommodate base64 image framing (~11 MiB at the maximum), rather than assume small text-only responses. Requests have a 70-second server ceiling and propagate client cancellation to backend/SSH/guest cleanup. A shorter client deadline can leave an uncertain outcome: observe before retrying.

Each server allows one in-flight machine operation; a concurrent request returns `MACHINE_BUSY` without waiting or replay. Existing cross-process machine locks also protect against another MCP process or CLI. There is no persistent human/agent ownership lease yet: serial access is enforced, and clients must coordinate their workflow. No broker daemon is necessary for this slice. A future live viewer needs explicit takeover/ownership beyond this lock.

Codes include `INVALID_ARGUMENT`, `TIMEOUT`, `CANCELED`, `NOT_READY`, `DISCONNECTED`, `DESKTOP_UNAVAILABLE`, `STALE_FRAME`, `STALE_SESSION`, `VERSION_MISMATCH`, `UNSUPPORTED_OPERATION`, `MACHINE_BUSY`, `OUTCOME_UNKNOWN`, `COMMAND_FAILED`, `COMMAND_TIMEOUT`, `CLEANUP_UNCONFIRMED`, and bounded-frame errors. Backend-specific typed codes/context are preserved. Unknown host errors become `BACKEND_ERROR` with a private-diagnostics hint so paths/command secrets are not echoed. `OUTCOME_UNKNOWN` never means a mutation did not occur. Status may return an error state with its diagnostics even when status observation itself succeeds.

`--debug` writes only tool names, codes and durations to stderr. It never logs arguments, full commands, environment, returned output or image data. Stdout belongs exclusively to MCP. Screenshots/results necessarily go to the connected client/model; its storage and provider privacy policy apply. Connecting the server grants guest access; stdio relies on the trusted local user/process boundary. There is no public listener. NAT still reaches host/LAN services.

## Agent instructions

The exact guidance shipped in the initialize instructions is [instructions.txt](../internal/mcpserver/instructions.txt): status first, screenshot → act → verify, recent original-pixel coordinates, no blind retries, guest-only commands, ask for missing intent/credentials/consequential external actions, and treat guest content as untrusted data. These instructions are not a security boundary and do not replace client permissions.

## Client setup and evidence

The printer writes a snippet to stdout and never edits user/global settings:

```sh
bin/agent-linux mcp-config --client codex --lima /absolute/path/to/limactl
bin/agent-linux mcp-config --client claude --lima /absolute/path/to/limactl
bin/agent-linux mcp-config --client vscode --lima /absolute/path/to/limactl
```

Pass the same `--state-dir` used for `up` if nondefault. Review the printed absolute paths before adding a configuration. Keep user-specific generated snippets outside Git.

**Codex:** actual agent acceptance is recorded in [verification.md](verification.md). Add the printed TOML table to your chosen Codex configuration, or use its official command:

```sh
codex mcp add agent_linux -- /absolute/path/to/agent-linux mcp --lima /absolute/path/to/limactl
codex mcp list
```

The add command explicitly modifies client settings; the AgentLinux printer does not. Normal interactive use should retain the user's MCP approval choices. `approval_policy="never"` can deny tools requiring approval; it is not permission to operate the guest. The opt-in acceptance script explicitly approves only six named desktop tools for that temporary disposable-machine invocation, keeps the host sandbox read-only, excludes guest exec, and ignores user configuration/rules. It does not alter persistent approvals. Current Codex per-tool settings are documented in the official configuration reference below.

**Claude Code:** official setup syntax checked; the installed 2.1.318 client was signed out, so model/tool/image compatibility is **not tested here**. Use:

```sh
claude mcp add --transport stdio agent_linux -- /absolute/path/to/agent-linux mcp --lima /absolute/path/to/limactl
```

Alternatively review the printer's `mcpServers` JSON for a project `.mcp.json` and follow Claude Code's trust/permission prompts. Use `/mcp` to inspect the connection. Then try the demo prompt below and verify image handling; installing a config is not acceptance evidence.

**VS Code Copilot:** documentation-based setup, **not executed/verified here**. Use “MCP: Add Server” → Command (stdio), select the absolute binary plus `mcp` arguments and review workspace trust/tool approvals. For the documented VS Code-native workspace format, the printer emits `servers` JSON for `.vscode/mcp.json`. That differs from the portable `mcpServers` format; do not paste it into the wrong file. Current docs also describe portable/global configurations; use the format your installed client supports. A working Copilot entitlement/model with image-tool support is still required.

Demo prompt: “Check my Linux machine, open its browser, visit https://example.com, and tell me what you see. Use screenshots before and after actions.” No universal client compatibility is claimed.

## Reproduce acceptance

Requires the same real Mac/Lima/image prerequisites as [verification.md](verification.md). Use a dedicated state namespace for test data. The Go test connects the actual binary over stdio using the official MCP library; without its environment variables it skips explicitly.

```sh
make build test check
# Choose a NEW private absolute STATE path; use it consistently below.
bin/agent-linux up --local --network=nat --state-dir "$STATE" --lima "$LIMA"
mkdir -m 700 "$EVIDENCE"  # NEW private directory
AGENTLINUX_ACCEPT_CLI="$PWD/bin/agent-linux" \
AGENTLINUX_ACCEPT_ROOT="$STATE" AGENTLINUX_ACCEPT_LIMA="$LIMA" \
AGENTLINUX_ACCEPT_EVIDENCE="$EVIDENCE" \
GOCACHE="$PWD/.cache/go-build" GOMODCACHE="$PWD/.cache/go-mod" \
go test -v ./internal/mcpserver -run '^TestRealStdioGuest$' -count=1
# Optional: requires an already authenticated Codex CLI; EVIDENCE/codex must not exist.
python3 scripts/mcp_agent_acceptance.py --cli bin/agent-linux \
  --lima "$LIMA" --state-dir "$STATE" --evidence "$EVIDENCE/codex"
bin/agent-linux stop --state-dir "$STATE" --lima "$LIMA"
```

The deterministic test visits an offline data page and verifies its visible guest window title, as well as guest timeout/nonzero-command results. The Codex test visits example.com. Review its recorded tool events/screenshots and grounded answer; process exit zero alone does not prove success. The manual commands preserve the stopped disk; remove only your own test namespace through the reviewed lifecycle cleanup procedure. Never delete a user's default VM to rerun a test.

## Official sources consulted before implementation

- [MCP tools specification, 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/server/tools): schemas, image/structured results, tool errors and annotations.
- [Official Go SDK v1.8.0 README](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/README.md), [releases](https://github.com/modelcontextprotocol/go-sdk/releases), and pinned module source: supported transports, initialization, client testing, cancellation and framing bounds.
- [Codex MCP](https://developers.openai.com/codex/mcp/), [configuration reference](https://developers.openai.com/codex/config-file/config-reference), [agent approvals](https://developers.openai.com/codex/agent-approvals-security): client launch, tables, per-tool approval behavior.
- [Claude Code MCP](https://code.claude.com/docs/en/mcp): stdio registration, scopes and permissions.
- [VS Code MCP servers](https://code.visualstudio.com/docs/copilot/customization/mcp-servers): command setup, configuration formats and trust.
