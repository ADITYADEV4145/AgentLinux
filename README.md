# Agent Linux

Give your agent a Linux computer. Run it on your Mac or in the cloud.

**Status: Phase 1 development implementation.** Local VM creation, startup, actual guest handshake/probe, status, graceful shutdown and sanitized logs are implemented and tested on one Apple silicon Mac. Desktop control, terminal/browser viewing, MCP and cloud remain unimplemented. Agent Linux is a working name.

Requirements: Apple silicon, development target macOS 14+, Go 1.26.1 to build, and Lima **2.2.1**. Only macOS 26.5 has real acceptance evidence so far; this is not a broad support or minimum-resource guarantee.

```sh
make build
make test check
bin/agent-linux doctor --lima /absolute/path/to/limactl --json
bin/agent-linux up --local --network=nat --lima /absolute/path/to/limactl
bin/agent-linux status --lima /absolute/path/to/limactl --json
bin/agent-linux logs --backend
bin/agent-linux stop --lima /absolute/path/to/limactl
```

If `limactl` is on PATH, omit `--lima`. Creation-only resource options: `--cpus=2 --memory=2 --disk=16` (GiB for memory/disk). Default private state: `~/.local/share/agent-linux/local`; `--state-dir=/absolute/private/path` selects an isolated machine namespace. `up` resumes it without creation flags after interruption or shutdown. `--help` and per-command `--help` describe limits and deadlines. JSON failures have nonzero exit status.

The guest service is read-only and reports **service readiness**, not a graphical desktop. There is no arbitrary command-execution tool. `stop` preserves the disk. Closing a CLI does not imply the VM stopped; use status/stop. No force-stop or deletion CLI is provided yet.

No host mounts, shared host clipboard, host credentials, forwarded SSH agent/X11, or device access. Application control uses restricted SSH over a loopback endpoint and a guest-only Unix socket, with unique generated keys and pinned guest host identity. **NAT can reach host/LAN services; it is not network isolation.** The initial image download is checked against a reviewed SHA-512 pin; detached image signatures are not automated.

- [Provisioning, image provenance and local prebuilt images](images/debian/README.md)
- [Guest protocol and limits](docs/guest-protocol.md)
- [Product specification](docs/product.md)
- [Architecture](docs/architecture.md) and [Phase 1 corrections](docs/decisions/0005-phase1-evidence.md)
- [Verification evidence](docs/verification.md), [roadmap](docs/roadmap.md), [research](docs/research.md)
- [Third-party licenses and redistribution limits](third_party/README.md)

Original code is Apache-2.0; third-party components retain their licenses. No runtime/images, secrets, machine state or private recordings are committed. No paid resources, hosted service or public release has been created.
