# ADR 0002: One guest X11 desktop and SSH control

Status: accepted for the first implementation.

Use Debian 13 ARM64, minimal Xfce on an authenticated Xvfb display, Chromium, scrot PNG capture, xdotool XTEST input, and guest-only xclip text paste. A nonprivileged Go helper reads bounded JSON messages over a host-initiated SSH exec channel. No shared host filesystem and no guest-initiated host RPC.

Prefer SSH over Lima's vsock transport where the pinned release supports it; a loopback SSH TCP endpoint is a documented fallback, never a LAN listener. Host-key pinning and dedicated keys are mandatory regardless of transport. Do not add raw vsock framing and key distribution before a working desktop.

X11 gives predictable capture/input support; every client in that desktop can observe and inject input. Wayland would require a selected compositor plus portal/PipeWire/EIS support and permission lifecycle testing. It is deferred rather than treated as interchangeable. Unicode uses guest clipboard paste into tested applications; unsupported targets report an error. No host clipboard synchronization.

Cost: a software-rendered desktop, weak intra-desktop isolation, clipboard-dependent text entry, and a protocol that must bound untrusted replies. Validate these with real applications before claiming support.

Evidence: [architecture](../architecture.md), [sources R2, R5–R9](../research.md).
