# Agent Linux

**Give your agent a Linux computer.**

Run a real Debian/Xfce desktop on an Apple silicon Mac, connect an agent through MCP, and watch or operate Linux in Kitty or a secure local browser viewer. Chromium, Firefox, a file manager, terminal, editor, launcher and settings are included.

**Available now:** a source-built local developer preview, tested on one development Mac. **Cloud:** a development API/adapter prototype; the default provider is explicitly **SIMULATED**, and the optional real provider remains unaccepted. **Billing:** sandbox foundation only, with no live activation path. The full promise—“Run it on your Mac or in the cloud”—is a product direction, not a claim that a hosted service is available.

No public binary/image release, independent security audit, notarization, production cloud service or finalized retail prices are claimed. [Launch readiness and blockers](docs/launch-readiness.md).

## Requirements and install

- Apple silicon Mac with hardware virtualization. macOS **26.5 (25F71)** on an M4 Pro is tested; macOS 14+ is a target, not a verified minimum.
- **Go 1.26.9**, Python **3.10+**, Git/macOS developer command-line tools, OpenSSH, and external **Lima 2.2.1**. `make` pins the Go patch toolchain; it may fetch that compiler through Go's authenticated toolchain mechanism if absent. [Official Go downloads](https://go.dev/dl/) and [Lima's exact release](https://github.com/lima-vm/lima/releases/tag/v2.2.1). A newer runtime is not silently accepted.
- At least **30 GiB free** for the default 2-CPU/4-GiB-RAM/24-GiB logical disk. Sparse usage grows. Initial setup downloads a pinned **337,510,400-byte (~322 MiB)** Debian image plus changing signed Debian packages; Internet is required.
- For terminal graphics, **Kitty 0.50.0** is the tested target. Install it deliberately using [upstream instructions](https://sw.kovidgoyal.net/kitty/binary/). Other terminals receive a live, authenticated loopback web-viewer URL.

```sh
git clone https://github.com/ADITYADEV4145/AgentLinux.git
cd AgentLinux
make build test check
./bin/agent-linux doctor --lima /absolute/path/to/limactl --json
```

If Lima 2.2.1 is on PATH, omit `--lima`. Otherwise use that flag consistently, **before positional arguments**, for each local VM command. No system package manager, sudo, shell-profile edit or prerequisite installation is hidden in our tool.

Optional private tool installation, from a reviewed committed checkout:

```sh
python3 scripts/package.py --output dist/reviewed-build
python3 scripts/install.py \
  --bundle "$PWD/dist/reviewed-build/agent-linux-0.5.0-rc.1-darwin-arm64" \
  --prefix "$HOME/.local/share/agent-linux-tools"
```

Use the printed executable path or deliberately configure PATH yourself. The bundle is publisher-unsigned/ad-hoc and **not notarized**. The installer verifies local file hashes and switches versions atomically; it never fetches binaries. Checksums alone do not prove publisher authenticity. [Packaging, upgrades and uninstall](docs/release.md).

## Quick start: two terminals

Terminal A, in Kitty (or open the private browser URL printed by the fallback):

```sh
./bin/agent-linux start --view-only --lima /absolute/path/to/limactl
```

This creates/resumes a persistent local machine, waits for the real desktop handshake, and keeps the viewer open. It does not create cloud resources. Leave it running while using terminal B:

```sh
./bin/agent-linux mcp-config --client codex --lima /absolute/path/to/limactl
```

Review the printed configuration and add it to your chosen client using [agent setup](docs/mcp.md). The printer changes no client settings. The `mcp` command itself is a stdio endpoint launched by the client; it is not a chat prompt or viewer. Codex and the official MCP SDK have real local acceptance evidence. Claude Code/VS Code instructions are documentation-based and their model/tool flows are untested here.

Ask the connected agent:

> Check my Linux machine. Open its browser, visit https://example.com, capture the result, and describe what you actually see. Use screenshots before and after actions.

For a reproducible offline desktop task, use the included [project-budget form and demo script](docs/demo.md). [Measured methods/results](docs/performance.md) distinguish real agent runs, deterministic SDK tests and simulations.

For human control instead, use plain `start` or `view --interactive`. Type, paste Unicode, click and scroll on Linux; **F2** hands input between you and the agent. **Ctrl+]** exits the interactive terminal viewer; Ctrl+C and q go to Linux. Plain `view`/`start --view-only` are observers and exit with q/Ctrl+C. Closing any viewer leaves Linux running. Web controls use explicit Take control/text/shortcut buttons. [Exact controls and limits](docs/viewer.md).

Stop deliberately in terminal B:

```sh
./bin/agent-linux stop --lima /absolute/path/to/limactl
./bin/agent-linux up --local --lima /absolute/path/to/limactl  # resume guest files
```

## Data, cost and cleanup

Local use has no Agent Linux usage fee; it consumes your Mac's CPU/RAM/disk/network. Your connected model/client may have its own charges and retention policies. Screenshots, guest commands and tool output go to that client/model. Use a disposable guest and synthetic data for demos.

Default state is `~/.local/share/agent-linux/local`; `--state-dir` selects another private namespace. Stop preserves guest data; reset/delete are destructive and require the exact selected machine ID. No snapshot or instant-reset promise:

```sh
./bin/agent-linux status --json
# Stop first; inspect the plan before intentionally adding --yes:
./bin/agent-linux delete --machine-id MACHINE_ID
# Or permanently reset the stopped selected VM:
./bin/agent-linux reset --machine-id MACHINE_ID --yes
```

Keep using the same nondefault `--state-dir` and `--lima` when applicable. [Guarded lifecycle/uninstall](docs/release.md). Uninstalling tools does not remove VM data or stop running machines. [One-way file copy](docs/file-transfer.md) supports bounded regular files, no overwrite or automatic archive extraction; no host project mount is needed.

Cloud sessions are explicitly selected with private `--cloud-config FILE` for MCP/viewer, or `cloud up` for an opted-in provider. There is no automatic local-to-cloud fallback. Prototype cloud sessions are ephemeral, owned/authenticated, TTL/idle limited and discard guest data on confirmed stop. [Cloud scope and provider/commercial gates](docs/cloud.md). Sandbox `billing status --config FILE` shows configured rates, trial, estimates, reservations and history; live keys/automatic collection are rejected. Hosted Stripe sandbox acceptance is still pending credentials. [Billing policy](docs/billing.md), [editable cost assumptions](examples/cost-model.json).

## Compatibility and limitations

| Component | Verified scope | Limits / pending |
| --- | --- | --- |
| Local VM | Apple M4 Pro, macOS 26.5, Lima 2.2.1 / Apple VZ | One development Mac; clean-Mac/other Apple silicon/macOS matrix pending; Intel/Linux/Windows local hosts unsupported |
| Linux workstation | ARM64 Debian 13, Xfce 4.20, helper 0.6.0, protocol 2.0 | One 1280×800 X11/Xvfb display; no Wayland/multi-display; old disks keep their recipe/helper |
| Browsers/apps | Debian Chromium/Firefox ESR, Thunar, terminal, Mousepad, settings/launcher | Package versions evolve with signed repository updates; vendor Google Chrome ARM64 is not silently substituted |
| Terminal | Kitty 0.50.0; real native images, resize and restoration tested | No tmux/screen, full-motion FPS, IME composition or continuous human drag guarantee |
| Web fallback | Chrome 156 view-only and real guest click/Unicode/save widgets tested | Broader browser, accessibility and physical input matrix pending; see current acceptance in [verification](docs/verification.md) |
| Agents | Real Codex CLI and official Go MCP SDK | Image-capable client/model needed; other client integrations remain unverified |
| Cloud / payments | Implemented prototype and local SDK/simulation tests | No real-provider acceptance, public service, production identity/database or Stripe-hosted sandbox completion |

Input uses fresh original PNG coordinates, top-left, scale 1. Dispatch acknowledgement is not application success; observe again. Arbitrary command execution is **inside the guest**, with access to guest data/network. Human ownership serializes cooperating frontends and is not an isolation boundary between X11 applications or hostile same-UID host processes.

No host home/project mount, clipboard sharing, pointer control, SSH-agent forwarding or device sharing by default. The host process uses your ordinary file permissions; provisioning/explicit package maintenance uses guest administration. **NAT can reach host/LAN services; it is not network isolation.** Image integrity is a reviewed compiled SHA-512 pin; detached signature validation is not automated. The VM reduces risk and is not a certified malicious-code sandbox. [Security boundaries](docs/security.md).

## Help and contribute

Start with `doctor`, `status --json`, bounded `logs --backend` and [troubleshooting](docs/troubleshooting.md). Never post keys, VM state, project data or unreviewed screenshots in issues. [Security reporting](SECURITY.md) explains the private-contact gap that must be resolved before broader distribution.

[Architecture](docs/architecture.md) · [Verification](docs/verification.md) · [Contributor roadmap](docs/contributor-roadmap.md) · [Contributing](CONTRIBUTING.md) · [Known issues](docs/known-issues.md).

Original repository code, including the development control plane and adapter code, is **Apache-2.0**: [license](LICENSE). External runtimes, SDKs, browsers and guest packages retain their own [licenses/notices](third_party/README.md). No Lima/Kitty/compiler/browser/VM image or operated hosted service is bundled. A future managed service may be paid; its availability, pricing and any separately developed proprietary components must be disclosed explicitly. Open-source adapter code does not make an external provider's infrastructure open source.
