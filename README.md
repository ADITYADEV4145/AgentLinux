# Agent Linux

Give your agent a Linux computer. Run it on your Mac or in the cloud.

**Status: Phase 0 specification complete; no runnable product yet.** Agent Linux is a working name. Local development targets Apple silicon Macs. Cloud support is a future phase, not an available service.

The recommended first implementation uses Lima with Apple's Virtualization.framework, Debian 13 ARM64, an Xfce desktop on Xvfb, and a Go CLI/MCP server. A guest helper handles screenshots and input over SSH. Kitty is the first terminal viewer target; a local browser viewer is the fallback. These are design decisions, not tested compatibility claims.

- [Product and proposed commands](docs/product.md)
- [Architecture, contracts, and dependency review](docs/architecture.md)
- [Research and primary sources](docs/research.md)
- [Implementation phases](docs/roadmap.md)
- [Verification evidence and first Mac experiment](docs/verification.md)
- [Decision records](docs/decisions/)

No installer, VM image, build target, or CLI is provided in this phase. Host folders, host clipboard, SSH agent forwarding, and host devices must remain disabled by default. Network access is a separate boundary: a VM with networking can reach services outside the guest.

Apache-2.0 is recommended for original project code; adoption and the LICENSE file are deferred to the implementation phase. Third-party components retain their own licenses. No upstream implementation or assets have been copied into this repository.
