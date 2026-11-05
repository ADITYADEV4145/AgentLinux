# ADR 0005: Phase 1 implementation corrections

Status: accepted from real implementation evidence, 2026-11-05.

Use the approved Go/Lima/VZ/Debian path, but scope Phase 1 to lifecycle and minimal guest communication per the user's phase request. Desktop capture/input, viewers and MCP are not implemented yet. The minimal service supports hello/health/probe, not arbitrary shell execution.

- **Provisioning size:** real Lima 2.2.1 truncated a >4 MiB input template. Initial validation refused to boot that malformed config. Strip the static guest binary, gzip/base64 it into multiline YAML, and reject templates >3 MiB before runtime creation. No new image codec/converter was built.
- **Inventory:** `limactl list --json` failed with `bufio.Scanner: token too long` when including embedded provisioning. Use a fixed Go output template selecting only lifecycle fields. No key/config blobs are returned to our parser or logs.
- **Channel:** use the approved loopback SSH fallback explicitly in Phase 1, keeping application SSH config/pinning independent from Lima's generated relaxed SSH configuration. Vsock is deferred, not claimed broken or tested. NAT and host/LAN reachability remain explicit limitations.
- **Image trust:** reviewed SHA-512 pin cross-checked with Debian's HTTPS checksum list and Lima's pinned manifest; image signatures are not automated. Keep independent signature verification as a release gate rather than claim it ran.
- **Image format:** host checks only bounded QCOW2 header metadata for virtual capacity and prohibited host-file references; Lima's established converter handles the disk. No QEMU installation was required by the tested Lima/VZ path.

Real Mac evidence: verified guest handshake/probe, graceful stop/restart with stable machine/new boot/session IDs, guest-disk sentinel persistence, and wrong pinned host key rejection. See verification.md for versions and limits. These corrections do not imply desktop readiness or local/cloud parity.
