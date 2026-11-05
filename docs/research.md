# Research notes and primary sources

Researched 2026-11-04 (America/Chicago). Sources were retrieved through TinyFish search/fetch; no code was executed from upstream projects. Dynamic branch/release pages describe their retrieved state, not a promise they stay unchanged. Recheck exact artifacts before installing or redistributing. `batch_create` was not exposed in the available tools, so multi-URL content retrieval used fetch_content's URL array; no paid browser automation or research jobs were launched.

**Evidence vocabulary:** Verified means documentation/source content was retrieved, or a local command was run. It does not mean the described technology worked on this Mac. Decision means our chosen path. Validation means an unproven compatibility/performance/security requirement. The [architecture](architecture.md) and [verification](verification.md) keep those distinct.

## R1 — Apple virtualization

- [GUI Linux sample, official Markdown](https://developer.apple.com/documentation/virtualization/running-gui-linux-in-a-virtual-machine-on-a-mac.md)
- [WWDC22: Create macOS or Linux virtual machines](https://developer.apple.com/videos/play/wwdc2022/10002/)
- [VZVirtioSocketDevice, official Markdown](https://developer.apple.com/documentation/virtualization/vzvirtiosocketdevice.md)

Verified: Apple documents native-architecture GUI Linux, EFI/device configuration, and VM display interaction through VZVirtualMachineView. Apple silicon requires an ARM guest image. VZ socket APIs provide host/guest connections without specifying an application authentication scheme. The GUI sample's default deployment target is macOS 14; this is not our tested minimum. JavaScript-only documentation initially yielded a shell page; the official Markdown endpoint provided readable content.

Decision: consume VZ through Lima first. Validation: ARM64 image boot, resource configuration, plain-mode vsock control and distribution requirements. Do not infer Linux hardware graphics acceleration from Apple's macOS guest demo.

## R2 — Lima behavior and trust defaults

- [Plain mode](https://lima-vm.io/docs/config/plain/)
- [FAQ](https://lima-vm.io/docs/faq/)
- [SSH usage](https://lima-vm.io/docs/usage/ssh/)
- [Pinned v2.2.1 default schema/template](https://github.com/lima-vm/lima/blob/v2.2.1/templates/default.yaml)
- [Pinned cidata provisioning implementation](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/cidata/cidata.go)
- [Upstream discussion of strict host-key checking](https://github.com/lima-vm/lima/issues/3775)

Verified: normal templates can share the host home; plain mode disables mounts, dynamic forwarding and several integrations, but keeps provisioning, base user/keys and static forwards. The template documents SSH-over-vsock defaults for VZ Linux, global overrides, agent forwarding options and proxy propagation. Reviewed cidata setup reads macOS proxy settings separately from process-environment propagation. Official SSH examples relax localhost host-key authentication; we must not treat generated SSH config as our security policy. The issue is supporting context, not a guarantee an upstream fix exists.

Decision: private per-machine runtime state, plain mode, explicit restrictive configuration and independent application host-key pinning. Validation: inspect actual resolved config and process arguments, guest mounts, listeners, credentials/environment and first boot/restart.

## R3 — Runtime versions, images and Tart

- [Lima releases](https://github.com/lima-vm/lima/releases) — retrieved page marks 2.2.1 latest stable and 2.3.0-beta.1 prerelease.
- [Lima v2.2.1 Debian 13 image manifest](https://github.com/lima-vm/lima/blob/v2.2.1/templates/_images/debian-13.yaml) — dated ARM64 genericcloud URL/digest and SSH/LLMNR workaround; mutable daily fallbacks also present.
- [Tart quick start](https://tart.run/quick-start/) — official search result identifies Linux/macOS support and common lifecycle commands.
- [Current Tart LICENSE](https://github.com/cirruslabs/tart/blob/main/LICENSE) — retrieved raw content is FSL-1.1-ALv2, including competing-use restrictions and a future Apache grant.
- [Historical Tart license announcement](https://tart.run/blog/2023/02/11/changing-tart-license/) — describes an older Fair Source transition, not authority for current source.

Decision: start with Lima 2.2.1; do not select Tart as the unrestricted core dependency. Version numbers are candidate pins, not installed/validated versions. Validate image checksum signatures and compatibility before using it. No Tart license text is adopted for our work.

## R4 — Network boundary

- [Lima user-mode network](https://lima-vm.io/docs/config/network/user/)

Verified: documented user-mode guest IP is not host-accessible by default, while the host gateway/loopback is guest-accessible through host.lima.internal; DNS can involve host resolution. This page includes QEMU-specific mechanics, so exact VZ reachability is an explicit experiment. Decision: no claim that NAT or disabled mounts isolates host/LAN services; require explicit network choice and report observed policy.

## R5 — Debian and browser availability

- [Debian 13 release announcement](https://www.debian.org/News/2025/20250809)
- [Official Debian cloud image index](https://cdimage.debian.org/cdimage/cloud/)
- [Debian release-note issues](https://www.debian.org/releases/stable/release-notes/issues.html) — official search result flags ARM64 cloud/UEFI boot issues; investigate against selected image.
- [Chromium package for trixie](https://packages.debian.org/trixie/chromium) — retrieved package table includes ARM64, sandbox recommendation and extensive dependencies.

Decision: Debian 13 ARM64, avoiding an assumption that Ubuntu desktop browser packaging is equivalent. Validation: exact image boots on VZ, Xfce packages install, browser sandbox remains enabled and Unicode fonts render. No Debian image downloaded in Phase 0.

## R6 — Headless display

- [X.Org Xvfb manual](https://www.x.org/releases/current/doc/man/man1/Xvfb.1.xhtml)

Verified: Xvfb implements an X server in memory without physical display/input hardware; screen dimensions/depth are configurable. Decision: create one managed Xvfb display, not automate a host window. Validation: Xauthority isolation, session services, capture, browser software rendering and recovery after display death.

## R7 — X11 input

- [xdotool README](https://github.com/jordansissel/xdotool/blob/main/README.md)
- [xdotool COPYRIGHT](https://github.com/jordansissel/xdotool/blob/main/COPYRIGHT)
- [xdotool changelist](https://github.com/jordansissel/xdotool/blob/main/CHANGELIST)

Verified: xdotool uses X11/XTEST; upstream warns about Wayland/XWayland, and the COPYRIGHT contains BSD-3-Clause-style terms. Decision: a fixed X11 session plus bounded exec arguments. Validation: pointer mapping, held modifiers, paste destinations and toolkit behavior; no assumption of Wayland parity.

## R8 — Wayland alternative

- [XDG RemoteDesktop portal](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.RemoteDesktop.html)
- [XDG ScreenCast portal](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.ScreenCast.html)

Verified: RemoteDesktop documentation describes session creation, permission dialog, device grants, integration with screencasting, EIS/libei input and persistence tokens. Decision: defer a compositor-specific Wayland adapter. Validation: a future headless compositor must actually implement required portals and a usable consent lifecycle; these interfaces alone do not prove unattended support.

## R9 — Capture and literal text dependencies

- [scrot COPYING](https://github.com/resurrecting-open-source-projects/scrot/blob/master/COPYING)
- [xclip COPYING](https://github.com/astrand/xclip/blob/master/COPYING)

Verified: scrot's permission text includes an acknowledgement requirement; xclip includes GPLv2 text. E2B's official desktop recipe uses scrot and xdotool [R16], providing prior art rather than a compatibility test. Decision: scrot capture, guest-only xclip paste; inspect Debian package copyright and exact installed command behavior in Phase 1. No code copied from either project.

## R10 — First terminal protocol

- [Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/)
- [Kitty protocol extensions](https://sw.kovidgoyal.net/kitty/protocol-extensions/)

Verified: PNG/direct data, chunking, image placement, pixel/cell sizing and capability-query mechanisms are documented. Decision: local Kitty, direct PNG frames, negotiated input features, bounded refresh. Validation: actual terminal version, display size/scaling, resizing, query timeouts and cleanup. The existence of a graphics protocol is not evidence of video performance.

## R11 — Other terminal protocols

- [iTerm2 inline-image protocol](https://iterm2.com/documentation-images.html)
- [iTerm2 feature reporting](https://iterm2.com/feature-reporting/)

Verified: OSC 1337 image/file transfer and multipart framing are documented; the page also documents Kitty graphics support. Feature reporting includes Sixel support reporting. Decision: defer both OSC 1337 and Sixel paths rather than promise untested parity. Validation: future terminal-specific placement, transport limits and input compatibility.

## R12 — MCP SDKs

- [Official SDK catalog](https://modelcontextprotocol.io/docs/2026-07-28/sdk)
- [Go SDK README](https://github.com/modelcontextprotocol/go-sdk)
- [Go SDK releases](https://github.com/modelcontextprotocol/go-sdk/releases) — retrieved latest release is v1.8.0.
- [Go SDK v1.8.0 go.mod](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/go.mod) — Go 1.25.0 requirement and dependency versions.
- [Go SDK LICENSE](https://github.com/modelcontextprotocol/go-sdk/blob/main/LICENSE) — explicit MIT/Apache transition notice; pinned v1.7.0 license also retrieved, v1.8.0 requires release audit before incorporation.
- [Official Python SDK](https://github.com/modelcontextprotocol/python-sdk)
- [Official TypeScript SDK](https://github.com/modelcontextprotocol/typescript-sdk)

Verified: Go and Python are listed Tier 1, Swift Tier 3 in the retrieved catalog. TypeScript repository documents stable v2 packages and support overlap with v1; catalog omission must not be read as abandonment. Python README documents 3.11+ and v2 migration. Go release notes describe 2026-07-28 plus older revision support and transport hardening. Decision: Go SDK v1.8.0 as initial pin; Python/TypeScript are alternatives, not additional mandatory runtimes. Validation: real client negotiation, malformed messages, cancellation and all license/transitive notices at pinned revision.

## R13 — MCP transport/version compatibility

- [2026-07-28 transport specification](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports)
- [2025-03-26 older transport specification](https://modelcontextprotocol.io/specification/2025-03-26/basic/transports)

Verified: current spec defines stdio and Streamable HTTP and describes differences from older session/initialize semantics. Decision: use official SDK compatibility handling, not a handwritten initialize response or unsupported claims of universal client compatibility. Local stdio has no public HTTP auth surface. Future cloud calls go through our authenticated HTTPS API; an internet-facing MCP endpoint would require a separate current authorization-spec review.

## R14 — terminal-mac

- [Project README](https://github.com/govijr/terminal-mac)
- [MIT LICENSE](https://github.com/govijr/terminal-mac/blob/main/LICENSE)
- [Inspected tree at ac787d577de58a3804d32c34b73af453e83f77d8](https://github.com/govijr/terminal-mac/tree/ac787d577de58a3804d32c34b73af453e83f77d8)
- [Mailbox.swift at inspected commit](https://github.com/govijr/terminal-mac/blob/ac787d577de58a3804d32c34b73af453e83f77d8/Sources/TerminalMacCore/Mailbox.swift)
- [DesktopRenderer.swift at inspected commit](https://github.com/govijr/terminal-mac/blob/ac787d577de58a3804d32c34b73af453e83f77d8/Sources/TerminalMac/DesktopRenderer.swift)
- [AgentServer.swift at inspected commit](https://github.com/govijr/terminal-mac/blob/ac787d577de58a3804d32c34b73af453e83f77d8/Sources/TerminalMac/AgentServer.swift)

Verified source reading, not runtime testing: Swift/VZ macOS guest; helper in guest; virtiofs file mailbox validates paths and bounds files; renderer keeps newest pending frame and supports shared-memory Kitty/direct PNG/iTerm output; MCP implementation manually handles requests. README describes ScreenCaptureKit/input and guided guest setup plus pending clean-Mac acceptance. Product lessons: separate desktop, same helper for viewer and agents, observe/act/verify, visible readiness, bounded stale work. Decisions differ: Linux/X11, SSH rather than host directory mailbox, official SDK, simple PNG before optimization. No code or assets reused; performance and security statements are not adopted.

## R15 — vmette and Vitrine

- [vmette](https://github.com/chamuka-inc/vmette), [MIT license](https://github.com/chamuka-inc/vmette/blob/main/LICENSE)
- [Vitrine by cioufong](https://github.com/cioufong/vitrine), [Apache-2.0 license](https://github.com/cioufong/vitrine/blob/main/LICENSE)

Verified project documentation: vmette describes a VZ-backed microVM sandbox and persistent guest Xvfb desktop driven over vsock with VNC viewing; it distinguishes its tools from the agent host's other capabilities. Vitrine describes Apple silicon VM support and a guest-agent stream/input design including X11 and native host window integration. These README capabilities are not independent acceptance evidence. Use their guest-boundary and session ideas as prior art, not latency/safety guarantees. Their scope exceeds our first slice. No implementation reused.

## R16 — E2B

- [Official desktop template](https://docs.e2b.dev/template/examples/desktop)
- [Sandbox persistence](https://docs.e2b.dev/sandbox/persistence)
- [E2B repository LICENSE](https://github.com/e2b-dev/E2B/blob/main/LICENSE)

Verified docs: desktop recipe uses Ubuntu/Xfce, Xvfb, x11vnc/noVNC, scrot and xdotool; persistence includes pause/resume and explicit deletion, with filesystem-only variants and operational failure behavior. The example includes permissive X/VNC flags we must not copy as safe defaults. Repository LICENSE is Apache-2.0; this does not establish the license of every infrastructure repo or grant hosted-service rights. Decision: candidate managed backend; validation needed for region, ARM64, network enforcement, authenticated viewing, resource limits, cleanup and persistence mapping.

## R17 — Daytona

- [Computer use](https://www.daytona.io/docs/en/computer-use/)
- [VNC access](https://www.daytona.io/docs/en/vnc-access/)
- [Repository README](https://github.com/daytonaio/daytona)
- [Repository root inventory](https://api.github.com/repos/daytonaio/daytona/contents)

Verified docs: computer use exposes screenshot/input/status and starts Xvfb, Xfce, x11vnc and noVNC. Current root inventory returned README/assets only, while README describes a much larger codebase. LICENSE and LICENSE.md fetches returned 404; search found older AGPL references. Therefore exact current code/license availability remains unresolved and is explicitly a cloud-selection gate. Documentation of a feature is not proof of service limits, isolation implementation or entitlement to redistribute code. No license or self-hosting guarantee is inferred.

## R18 — Deferred viewer dependencies

- [noVNC license inventory](https://github.com/novnc/noVNC/blob/master/LICENSE.txt)
- [TigerVNC](https://github.com/TigerVNC/tigervnc), [license](https://github.com/TigerVNC/tigervnc/blob/master/LICENCE.TXT)

Verified: noVNC core MPL-2.0, with other licenses on HTML/CSS/fonts/images/vendor portions; TigerVNC describes GPL distribution. Decision: optional later alternatives, not bundled now. Audit selected files and versions if adopted.

## Research limits

No VM runtime, guest desktop, provider account, terminal graphics session, MCP client integration or image download was exercised. Search/fetch cannot establish runtime behavior, minimum hardware, source authenticity beyond retrieval, service entitlement, or full license compliance. Package and exact-release license audits remain distribution gates. Hosted claims, provider regions/prices/quotas and mutable branch licenses must be reverified at implementation time. No project was copied or benchmarked.
