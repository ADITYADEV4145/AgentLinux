# Product specification

Desktop quality direction (2026-11-05): Ubuntu 26.04.1 LTS ARM64/GNOME 50 is the recommended next full-desktop profile. A separate Mac feasibility VM produced actual Wayland screenshots and guest-only keyboard/Unicode clipboard evidence. The default remains Debian/Xfce until the complete GNOME/MCP/viewer acceptance gate passes; see desktop-quality.md.

Phase 5 local RC status (2026-11-05): local desktop/control and official-SDK MCP are implemented. New recipes add a complete Xfce app set, Chromium plus Firefox ESR, a small development baseline and explicit host-operator guest package maintenance. Live view-only Kitty 0.50.0 rendering and the browser fallback are verified on one Mac, including actual MCP-driven guest changes, resize, reconnect and terminal cleanup. Bounded explicit file copies, guarded reset/delete and offline local tool packaging/installation harden the developer workflow; no release is published. See release.md, security.md and viewer.md for the current surface and limits. An actual Codex agent operated Chromium through screenshots and guest input. Claude Code/VS Code setup is documented but their agent compatibility is untested. Cloud and direct human viewer input remain future scope; see [MCP setup](mcp.md) and [verification](verification.md).

Phase 0 specification, updated 2026-11-05. Phase 2 adds actual guest desktop capture/input and bounded arbitrary guest commands to local lifecycle. Phase 3 implements MCP and Phase 4 adds live view-only rendering; broader cloud/human-interaction portions of the full MVP below remain targets. README documents the commands that actually exist.

## User, problem, and positioning

For developers using MCP-compatible agents who need a real browser and desktop without giving computer-use tools their host desktop. Bring your own agent and model. The local product must work without a hosted account or subscription. A future managed service sells compute, storage, and operational/team features.

A terminal alone cannot show what a desktop agent is doing. Running the agent's visual tools directly on a personal desktop risks interference with private apps, files, and input. Provide a separate Linux desktop with an observable lifecycle and an observe → act → verify API.

## First compelling demo

On an Apple silicon Mac, create a new Linux machine, show its desktop in Kitty, and connect an MCP client. The agent opens an offline test page in guest Chromium, clicks a text field, enters `Hello, café — 日本語 👋`, submits the form, and screenshots the result. The user watches, pauses agent input, takes control, then hands control back. Stop and restart the machine; a file in the guest survives and the old session ID is rejected. The Mac's pointer and clipboard remain unchanged. This is a planned acceptance scenario, not a completed demo.

The first engineering test is smaller: boot → guest helper → screenshot → click/type → verify → stop. That smaller path and MCP integration now work; terminal viewing remains future work.

## Local MVP

- One named persistent machine at a time; one active desktop session and one input owner per machine. Use opaque IDs internally.
- Explicit creation, start, status, screenshot, input, stop, and confirmed deletion. Closing a viewer leaves the machine running and says so.
- Debian 13 ARM64 cloud image, minimal Xfce, Xvfb virtual X11 display, Chromium with its sandbox enabled. No physical monitor or macOS screen permissions required by design.
- Go CLI, local broker, and stdio MCP adapter use the same application operations. MCP is bound to a selected machine; an agent cannot silently switch to another machine.
- Kitty PNG graphics with bounded refresh; optional interactive terminal input after capture is validated. Local browser canvas fallback supports viewing and manual input.
- No host mounts, shared host clipboard, host credentials, forwarded SSH agent, host device access, or automatic service-port forwarding. Resource and disk checks precede downloads.
- Interactive creation explains networking and asks the user to choose isolated/offline operation or networked operation with host/LAN reachability risk. Noninteractive creation requires an explicit network choice. Provisioning network access is disclosed separately.
- Structured JSON status/errors, readiness probes, bounded logs, clear recovery instructions. No agent text or screenshots in default logs.

Offline mode will be advertised only after verifying that the pinned runtime can disable external guest networking while retaining the SSH control channel. Until then, refuse `--network=off` with `UNSUPPORTED_CAPABILITY`; do not label NAT as isolation.

## Future cloud MVP

After local acceptance: one region, one provider, authenticated users/workspaces, per-machine authorization, short-lived viewer sessions, explicit resource limits, idle shutdown, hard lifetime, cleanup reconciliation, and measured usage. Preserve disk across stop/start only where the provider can implement that guarantee; report unsupported persistence before allocating anything. No cross-backend disk migration or live migration. Cloud is unimplemented in Phase 0, and its commands must return unsupported until a real adapter exists.

## Non-goals

An AI model, chat interface, arbitrary host execution, Intel guest emulation, multiple displays, GPU acceleration, audio/video conferencing, host clipboard sync, host project mounts, Wayland parity, tmux/SSH terminal graphics compatibility, video frame-rate guarantees, enterprise security claims, billing implementation, multi-region scheduling, Kubernetes orchestration, and a custom hypervisor or codec.

## Proposed CLI (not available yet)

```sh
agentlinux doctor
agentlinux machine create demo --backend=local --network=nat
agentlinux machine start demo --wait
agentlinux machine status demo --json
agentlinux view demo --terminal=kitty
agentlinux view demo --web
agentlinux screenshot demo --output=./guest.png
agentlinux control demo --owner=human
agentlinux control demo --owner=agent
agentlinux mcp --machine=demo
agentlinux machine stop demo
agentlinux machine delete demo --confirm=demo
```

The future deletion CLI must enforce confirmation; the MCP desktop server does not expose deletion. `create` reports the resource plan, image source, estimated download size when known, and network implications before changes. Proposed cloud syntax is `--backend=cloud --workspace=<id> --region=<supported-region>`; it must never silently fall back to local or create paid resources without separate authorization.

## Agent workflow

The user creates/starts a machine outside MCP and launches `agent-linux mcp --state-dir /absolute/private/state`. The client lists tools, reads `computer_status` capabilities and current session, and requests `computer_screenshot`. Each image includes frame ID, session ID, display generation, dimensions, and capture time in accompanying structured metadata. The separate click/move/drag/scroll/type/key tools require the observed frame reference. Observe again to verify outcomes. A completed input call means input was dispatched, not that the task succeeded.

Implemented tools: `computer_status`, `computer_screenshot`, `computer_click`, `computer_move`, `computer_drag`, `computer_scroll`, `computer_type`, `computer_key`, `computer_open`, `computer_run_command`; exact schemas and limits are in [mcp.md](mcp.md). Guest commands are arbitrary argv as the non-sudo guest user. No arbitrary host shell, model calls, lifecycle deletion, credential transfer, or implicit uploads. Unsupported tools/actions fail explicitly. Screens and web pages are untrusted content, including instructions visible to the model. Adding this MCP server does not disable an agent client's other host tools; users must configure those separately if they want guest-only execution.

## Trust and usability

The host CLI/MCP process and VM runtime are trusted; the proposed persistent broker is deferred. The guest, agent commands, guest filenames, images, network content, and all guest output are untrusted. The guest desktop account has no sudo rights; provisioning uses a separate administrative identity. X11 clients within that desktop share access to its input and screen: guest apps are not mutually isolated.

The VM reduces exposure; it does not guarantee containment of malicious code or network attacks. Networked guests may reach the host/LAN and exfiltrate guest data. Destructive actions inside the guest are still destructive. Host-side API access is equivalent to controlling the selected guest and must not be exposed on the LAN.

The future viewer must implement human takeover that revokes the agent input lease and rejects agent writes with `CONTROL_BUSY`. Keyboard-only takeover/exit controls, a visible owner indicator, text status, retry instructions, and non-color-only error indicators are required. Always restore terminal modes after exit or interruption. Do not require a mouse for setup. Provide browser zoom and explicit text entry for Unicode/IME use; browser/terminal raster views alone do not provide screen-reader access to guest applications. AT-SPI accessibility tooling is a later capability, not an MVP guarantee.

## Failure behavior

| Failure | Required product response |
| --- | --- |
| Unsupported host/runtime, missing terminal graphics | Explain prerequisite; offer browser viewer without pretending graphics work |
| Insufficient disk, memory, or failed image verification | Refuse start/download; keep existing disks intact |
| Slow apt mirror or interrupted provisioning | Expose stage and bounded diagnostics; retry idempotently, never reinstall an existing disk automatically |
| VM running but helper/display unavailable | Report degraded readiness; reject input and show repair steps |
| Dropped control link or lost action acknowledgement | Report uncertain outcome; screenshot before retrying a non-idempotent action |
| Guest reboot/display restart/stale frame | Invalidate old session/display references; require fresh screenshot |
| Unsupported Unicode paste destination or key | Return explicit limitation; never type a substitute silently |
| Viewer closed, process crash, host sleep | Do not infer VM stopped; reconcile runtime state on reconnect |
| Stop timeout | Report machine still running/unknown; require explicit force-stop choice |
| Delete failure/cloud cleanup lag | Keep deleting state and resource ownership records; retry cleanup without false success |

## Measurable success

Acceptance requires the scripted offline form task and persistence test to pass in three consecutive runs on a recorded real Mac configuration. Record cold provisioning and warm readiness time, screenshot round-trip p50/p95, frame age, input dispatch latency, CPU/RSS, disk growth, bytes transferred, and viewer achieved refresh rate. Record failures as well as successes. No published minimum hardware requirement, startup-time claim, or frame-rate promise until measured. The terminal viewer should remain responsive while frames are dropped under load; exact performance gates are selected from Phase 1/3 measurements.
