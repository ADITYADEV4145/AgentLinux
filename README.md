# Agent Linux

An agent-ready Linux desktop on an Apple silicon Mac, with human control and a development cloud backend.

**Local developer release candidate 0.5.0-rc.1:** real Debian/Xfce desktop, Chromium/Firefox, guest-only screenshots/input/commands, official-SDK MCP, a Kitty viewer and authenticated live loopback fallback. Tested on one development Mac; not a production/security certification. No binary release or VM image is published by this phase.

**Phase 5.5 workstation:** retain **Debian 13 / Xfce 4.20**, with a new `workstation-5.5.0` recipe: Greybird, readable fonts, a clear panel and original wallpaper, plus the full application set. [Actual Xfce/Plasma comparison, GNOME research, measurements and limitations](docs/workstation.md). Existing machines keep their sealed recipe; use a new namespace to try the changes.

## Install and first demo

Prerequisites: **Go 1.26.1**, **Python 3.10+**, Git/macOS developer tools, external **Lima 2.2.1**, OpenSSH, Apple silicon, and at least **30 GiB free** for the default VM (**46 GiB** for the measured 4-CPU/8-GiB/40-GiB workstation profile). macOS **26.5 (25F71)** is tested; macOS 14+ is a development target. Initial creation downloads a pinned **337,510,400-byte (~322 MiB)** Debian image plus changing apt packages; Internet is required. No offline network-isolation mode is offered.

```sh
git clone https://github.com/ADITYADEV4145/AgentLinux.git
cd AgentLinux
make build test check
# From a reviewed, committed source tree; no upload or public release:
python3 scripts/package.py --output dist/rc-build
python3 scripts/install.py \
  --bundle "$PWD/dist/rc-build/agent-linux-0.5.0-rc.1-darwin-arm64" \
  --prefix "$HOME/.local/share/agent-linux-tools"
# Use the printed path or deliberately add it to PATH yourself. No shell settings are changed.
```

The bundle is publisher-unsigned (Go may add an ad-hoc signature), **not notarized**. The installer verifies a local manifest, stages a version and switches atomically; it never downloads executables or requests sudo. Checksums alone do not prove publisher authenticity. The supported initial distribution path is reviewed source/local build; see [release, upgrade, reset and uninstall](docs/release.md).

For the remaining examples, `agent-linux` means the installed executable's full path or a PATH entry you deliberately configured. Developer builds can use `bin/agent-linux` directly.

```sh
agent-linux version --json
agent-linux doctor --lima /absolute/path/to/limactl --json
agent-linux start --lima /absolute/path/to/limactl
# Linux boots/resumes and opens the interactive viewer. F2 gives control to your agent.
# Ctrl+] exits the viewer; Linux stays running. Ctrl+C goes to Linux.
# In another terminal, print configuration to review/install in your chosen client; no global edits:
agent-linux mcp-config --client codex --lima /absolute/path/to/limactl
agent-linux view --lima /absolute/path/to/limactl  # optional separate view-only observer
# Connect the agent using docs/mcp.md, then ask it to open the guest browser and verify what it sees.
# Separate view-only viewer: q/Ctrl+C exits; the VM stays running.
agent-linux stop --lima /absolute/path/to/limactl
agent-linux up --local --lima /absolute/path/to/limactl  # resumes persistent guest data
```

Kitty **0.50.0** is the visually verified terminal; unsupported terminals use the live loopback web viewer. Chrome **156.0.8078.80** view-only fallback is tested; new human web controls have automated routing tests, with real-browser interaction acceptance pending. [Viewer behavior and actual demo evidence](docs/viewer.md), [MCP setup and tested clients](docs/mcp.md), and [first-run troubleshooting/compatibility](docs/troubleshooting.md) state precise limits. There are no fabricated screenshots or performance claims.

If limactl is on PATH, omit `--lima` from these and subsequent commands. Otherwise provide it **before positional arguments** on every command that contacts the VM:

```sh
agent-linux status --json
agent-linux view          # Kitty target; automatic live web fallback
agent-linux view --web    # verified live loopback fallback
agent-linux screenshot --output /tmp/agent-linux.png
agent-linux open launcher
agent-linux open files
agent-linux open settings
agent-linux open editor
agent-linux click --button left --clicks 2 300 250
agent-linux type "hello, café — 日本語 👋"
agent-linux key CTRL+S
agent-linux open browser  # Chromium
agent-linux open firefox
agent-linux browsers list --json
agent-linux packages install jq
agent-linux packages install --profile web-dev
agent-linux move 850 600
agent-linux drag 700 50 720 70
agent-linux scroll 800 600 0 3
agent-linux exec -- /usr/bin/uname -a
agent-linux exec --cwd /tmp --env DEMO=value --command-timeout 2s -- /bin/sh -c 'printf "%s\n" "$DEMO"'
agent-linux logs --backend
agent-linux stop
```

`exec` runs argv **inside Linux**, never on the Mac. Granting machine control access allows arbitrary commands as the guest `agent` user and access to guest data. JSON exec results contain separate base64 stdout/stderr plus exit/timeout/output/cleanup status; human output escapes guest terminal controls. A shell is used only when explicitly requested inside the guest. Long-lived GUI demo apps should use `open`; exec cleans up its process scope.

Screenshots are PNG, 1280×800, scale 1, top-left coordinates. Input uses a fresh frame automatically unless `--frame-id` selects a recent observed frame (10-second lifetime). Input acknowledgements mean dispatched, not task completed. Text uses only the **guest** clipboard; some applications reject paste. One display and X11 only; no Wayland or full-motion performance claim.

## One command, human and agent control

After installing prerequisites, run **`agent-linux start`** in Kitty 0.50.0. It starts or resumes the persistent local workstation, waits for the real guest/desktop handshake and opens an interactive viewer. Unsupported terminals show a secure local web viewer URL; open that URL in a browser and use Take control. No cloud machine is created by this command. First startup downloads Linux/packages; subsequent startup keeps your guest files. NAT can reach host/LAN services.

In the terminal, type normally, click or scroll on the desktop, and use bracketed paste for Unicode text. **F2** gives control to the agent or takes it back; both can observe, but only the input owner can mutate. **Ctrl+]** exits without stopping Linux. Ctrl+C and `q` are guest input. F3 selects standard/terminal paste if automatic detection is unsuitable. Legacy terminal encoding limits shortcuts; continuous mouse drag, IME composition and complete desktop shortcut parity are not implemented in the human viewer. Agents retain the existing guest drag operation. `start --view-only` and plain `view` are observers.

The web fallback uses explicit text/shortcut fields and click/scroll controls; it never reads the host clipboard. Ownership coordinates cooperating clients and expires after disconnect; it is not a security boundary against guest applications or same-UID host processes. See [controls and limitations](docs/viewer.md).

## Development cloud prototype

[Phase 6 setup, provider research and pending acceptance](docs/cloud.md). The authenticated API/remote machine/MCP/viewer paths are implemented. The default provider is visibly **SIMULATED** and creates no Linux VM. An optional E2B adapter requires explicit operator opt-in and remains untested against the paid provider. Production identity, public deployment, live billing and persistent cloud homes are not implemented. E2B service terms require commercial permission review before a hosted offering.

## Sandbox billing foundation

[Phase 7 billing policy, setup, operator recovery and pending acceptance](docs/billing.md). Local use stays free. Cloud pricing/trial allocation requires reviewed configuration; there are no finalized retail prices. The durable usage ledger, workspace reservations/caps, immutable rate versions, trial accounting and usage history are implemented. `agent-linux billing status --config FILE` shows readable amounts, with `--json` for automation.

The official Stripe Go SDK supports sandbox customer/Checkout setup, portal links, explicit manual usage invoices and verified durable webhooks. Live mode/keys and automatic collection are disabled. SDK/signature tests use a local API fixture; actual Stripe-hosted Checkout remains **pending because test credentials were unavailable**. No customer was charged or paid service published. [Editable cost assumptions](examples/cost-model.json) and `python3 scripts/cost_model.py` provide estimated scenarios, not measured economics or retail prices.

## File transfer and lifecycle

Default private machine state: `~/.local/share/agent-linux/local`. `--state-dir` selects an absolute, private application namespace, never home/project itself. `stop` preserves data; `up --local` resumes. No host home/project is mounted and host clipboard sharing is disabled. Guest input/Unicode paste uses the Linux clipboard only.

```sh
agent-linux copy-in --source /absolute/reviewed-project.tar --json
agent-linux copy-out --source /home/agent/results.txt --output /absolute/new-results.txt --json
agent-linux status --json
# Stop first. Without --yes, only prints a removal plan and exits with confirmation required:
agent-linux delete --machine-id MACHINE_ID
# Review plan and use the exact ID from status. These permanently lose all guest data:
agent-linux reset --machine-id MACHINE_ID --yes
# Or delete instead of resetting:
agent-linux delete --machine-id MACHINE_ID --yes
```

Copy is one-way, one regular file up to **4 MiB**, with path/link/size/checksum checks, no overwrite and no automatic archive extraction. See [file transfer](docs/file-transfer.md). Reset reprovisions a new guest from the pinned image/current recipe and rotates identity/keys; it is not a snapshot or instantaneous restore. Deletion/reset require explicit stopped-machine identity confirmation and never delete files outside the selected owned state. [Uninstall](docs/release.md) removes only the tools; it leaves VM data unless you explicitly delete the machine first.

## Trust and limitations

No host mounts, shared host clipboard, SSH-agent/X11 forwarding, host device or host desktop control. Application communication is pinned SSH over loopback to a guest Unix socket. **NAT can reach host/LAN services; it is not network isolation.** The host process runs with your user's file permissions. Bootstrap guest administration and explicit package maintenance use guest root; MCP's service is non-sudo. X11 apps share the guest session, and guest execution is powerful. Image integrity uses a reviewed compiled SHA-512 pin; detached image signature validation is not automated.

Read the [isolation audit and malicious-guest limits](docs/security.md). No independent security audit, production certification, clean-Mac matrix, signed/notarized download, Wayland/multi-display support, human viewer input/takeover, video FPS guarantee or functioning cloud backend is claimed. Claude Code/VS Code setup remains untested as agent integrations; Codex and official-SDK evidence is recorded in [verification](docs/verification.md). This phase uses a real new guest on an existing development Mac; the missing clean-Mac checklist is explicit.

Original code: [Apache-2.0](LICENSE). External tools/guest packages retain their own terms; [third-party notices](third_party/README.md) and linked dependency inventory accompany local packages. No Lima/Kitty/browser/VM image is bundled. [Contributing](CONTRIBUTING.md), [security reporting](SECURITY.md), [architecture](docs/architecture.md), [roadmap](docs/roadmap.md), and [verification](docs/verification.md).
