# Agent Linux

Give your agent a Linux computer. Run it on your Mac or in the cloud.

**Phase 3 development implementation:** a real Debian ARM64 desktop on Apple virtualization, guest screenshots/input, bounded guest commands, and local lifecycle control. Tested on one Apple silicon Mac. An official-SDK MCP stdio server exposes the local machine to agents. Live terminal/browser viewers and cloud remain future work. Agent Linux is a working name.

Build requirements: Go 1.26.1, Apple silicon Mac, and Lima **2.2.1**. macOS 26.5 has actual acceptance evidence; macOS 14+ is a development target, not a tested compatibility guarantee.

```sh
make build test check
bin/agent-linux doctor --lima /absolute/path/to/limactl --json
bin/agent-linux up --local --network=nat --lima /absolute/path/to/limactl
```

If limactl is on PATH, omit `--lima` from these and subsequent commands. Otherwise provide it **before positional arguments** on every command that contacts the VM:

```sh
bin/agent-linux status --json
bin/agent-linux screenshot --output /tmp/agent-linux.png
bin/agent-linux open editor
bin/agent-linux click --button left --clicks 2 300 250
bin/agent-linux type "hello, café — 日本語 👋"
bin/agent-linux key CTRL+S
bin/agent-linux open browser
bin/agent-linux move 850 600
bin/agent-linux drag 700 50 720 70
bin/agent-linux scroll 800 600 0 3
bin/agent-linux exec -- /usr/bin/uname -a
bin/agent-linux exec --cwd /tmp --env DEMO=value --command-timeout 2s -- /bin/sh -c 'printf "%s\n" "$DEMO"'
bin/agent-linux logs --backend
bin/agent-linux stop
```

`exec` runs argv **inside Linux**, never on the Mac. Granting machine control access allows arbitrary commands as the guest `agent` user and access to guest data. JSON exec results contain separate base64 stdout/stderr plus exit/timeout/output/cleanup status; human output escapes guest terminal controls. A shell is used only when explicitly requested inside the guest. Long-lived GUI demo apps should use `open`; exec cleans up its process scope.

Screenshots are PNG, 1280×800, scale 1, top-left coordinates. Input uses a fresh frame automatically unless `--frame-id` selects a recent observed frame (10-second lifetime). Input acknowledgements mean dispatched, not task completed. Text uses only the **guest** clipboard; some applications reject paste. One display and X11 only; no Wayland or full-motion performance claim.

Default private machine state: `~/.local/share/agent-linux/local`; `--state-dir` selects another absolute private namespace. Initial experiment defaults: 2 vCPUs, 4 GiB RAM, 24 GiB logical disk, with host capacity/headroom checks. These are not measured minimum requirements. `stop` preserves the disk; closing a CLI does not imply stopped. Resource/image flags are creation-only. **For a Phase 1 machine, preserve its disk and use a fresh state directory for Phase 2; automatic protocol/guest migration is not implemented.**

No host mounts, shared host clipboard, SSH-agent/X11 forwarding, host device or host desktop control. Application communication is pinned SSH over loopback to a guest Unix socket. **NAT can reach host/LAN services; it is not network isolation.** X11 apps share the guest desktop, and guest execution is powerful. Image integrity uses a reviewed SHA-512 pin; detached signature validation is not automated.

- [MCP tools, client setup and agent workflow](docs/mcp.md)
- [Guest protocol, capabilities and limits](docs/guest-protocol.md)
- [Image provenance and provisioning](images/debian/README.md)
- [Real hardware evidence and acceptance commands](docs/verification.md)
- [Architecture](docs/architecture.md), [product target](docs/product.md), [roadmap](docs/roadmap.md), [research](docs/research.md)
- [Third-party licenses](third_party/README.md)

Original code is Apache-2.0. The screenshot adapter uses scrot; installed packages retain their own licenses. Generated runtimes, VM keys/state and acceptance screenshots are not committed. No paid cloud resources, billing, hosted service or public deployment has been created.
