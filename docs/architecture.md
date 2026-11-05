# Architecture

Phase 0 • 2026-11-04 • Decisions below specify intended implementation. The Phase 0 design below is retained as the target architecture; current implementation and hardware evidence are distinguished in the snapshot below. Research facts and links are in [research.md](research.md); experiments and evidence are in [verification.md](verification.md).

## Phase 2 implementation snapshot (2026-11-05)

Implemented and hardware-tested: managed Xfce/Xvfb, Mousepad/Chromium, fresh 1280×800 PNG capture, guest-only XTEST mouse/keys and UTF-8 clipboard paste, a shared local machine interface, and arbitrary non-sudo guest argv execution with per-request cgroup cleanup. CLI computer commands are real; no MCP, continuous viewer or cloud backend is present. Phase 1 now has a successful GitHub CI run, and its private-tmp/home/AF_UNIX-only restrictions were corrected for the desktop.

Protocol 2.0/service 0.2.0 negotiates exact versions, bounds requests to 256 KiB/responses to 16 MiB/PNG to 8 MiB, and advertises capabilities separately from current desktop readiness. Frame IDs bind capture/input to session, generation and a 10-second observation window. The non-sudo helper and separate desktop unit share only guest resources. Exec uses Linux clone3 atomic cgroup placement and kill/empty-scope cleanup; unsupported delegation fails rather than claiming cancellation succeeded. This is lifecycle management, not a malicious-guest containment claim.

Actual evidence includes saved ASCII/CJK/emoji text and visible browser PNGs, detached-child timeout cleanup, disconnect/output limits, desktop/helper crashes, stale frame rejection, VM restart/persistence and unchanged host pointer/clipboard counter. Phase 2 defaults are the earlier desktop experiment values (2 vCPU/4 GiB/24 GiB), not a supported minimum. Sources/revisions and limits are in [verification](verification.md), [protocol](guest-protocol.md) and [ADR 0006](decisions/0006-real-desktop-control.md). Existing Phase 1 disks are preserved; use a fresh namespace rather than an implicit migration.

## Phase 1 implementation snapshot (2026-11-05)

Implemented: Go CLI (`agent-linux`), locked/atomic local state, Lima 2.2.1 VZ backend, Debian 13 ARM64 image with checksum pin, systemd guest service on a private Unix socket, restricted forced SSH bridge through a loopback host endpoint, strict application host-key pinning, hello/health/probe protocol 1.0, lifecycle reconciliation and bounded sanitized logs. The final build is not a graphical desktop. No broker daemon, MCP SDK/server, viewer, cloud adapter or desktop automation is present yet.

The shipped service version is 0.1.0. One Mac has actual boot/probe/stop/restart evidence; see verification.md. Phase 1 uses 2 vCPUs/2 GiB/16 GiB logical disk as its default experiment config, not a measured minimum. Original 4 GiB/24 GiB desktop resource proposals below remain unvalidated future desktop targets. The observed runtime does not need a separate QEMU installation for the selected QCOW2 conversion.

The backend uses the permitted loopback SSH fallback, not vsock. Input templates embed a stripped, compressed guest helper and are capped at 3 MiB after Lima was observed truncating input at 4 MiB. Inventory uses selected lifecycle fields, avoiding Lima JSON scanner failure on large embedded config. Original state schemas/API design below are future targets; actual Phase 1 protocol uses exact version 1.0 and 32-hex random machine/session IDs. See [ADR 0005](decisions/0005-phase1-evidence.md) and [guest-protocol.md](guest-protocol.md).

Apache-2.0 is adopted in LICENSE. The only external Go dependency is go.yaml.in/yaml/v3 v3.0.4 (MIT/Apache component licenses retained). MCP is deferred and has no dependency yet. Image integrity uses the reviewed SHA-512/HTTPS sources; detached signature validation remains a release gate. No distribution clearance or broad security guarantee is claimed.

## Recommended first slice

```text
MCP client --stdio--> agentlinux mcp ----+
CLI -----------------------------------+--> local Go broker (private Unix socket)
Kitty viewer --------------------------+       |
Browser viewer --loopback HTTP/WS-------+       +--> Lima 2.2.1 / Apple VZ
                                               |     Debian 13 ARM64 disk
                                               +--> SSH exec, preferably over vsock
                                                     nonprivileged Go guest helper
                                                     Xvfb :1 + Xfce + Chromium
                                                     scrot / xdotool / xclip
```

The broker owns lifecycle, input ownership, request ordering, and frame metadata. Each frontend uses the same application service. Lima provides VM management; it is not our cloud API. The guest helper controls only its fixed X display. For cloud, a provider adapter replaces VM lifecycle and guest transport underneath the same logical operations. No cloud implementation is included now.

Choose Go for the host CLI/broker, terminal renderer, and guest helper: one language, static ARM64 guest binaries, existing JSON/HTTP/PNG tooling, and an official MCP SDK. Use macOS OpenSSH through an explicit argument vector, not a shell command assembled from user/guest data. HTML/JavaScript is limited to the local browser viewer. A direct Swift helper is reserved for a measured need that Lima cannot satisfy, not a parallel initial implementation.

## VM approaches

These are engineering estimates, not benchmarks. Apple documents ARM-native GUI Linux, EFI boot, virtual graphics/input devices, and VZVirtualMachineView [R1]. No assumption of Linux Metal/GPU acceleration follows from macOS guest graphics examples.

| Approach | First desktop / installation | Maintenance and provisioning | Controls / distribution | Cloud fit / license |
| --- | --- | --- | --- | --- |
| Direct Swift/VZ | More initial work: image boot, installation, firmware state, signing and process lifecycle | Own all image/provision/recovery tooling; native view available for a virtual graphics display | Explicit CPU/memory/device configuration; own entitlements, signing/notarization, packaging | Guest helper reusable; VZ itself is Mac-only and governed by Apple SDK/platform terms |
| Lima/VZ integration, selected | Existing cloud-image and SSH provisioning path; separate pinned executable initially | Maintain a small CLI adapter and restrictive template; upstream handles VM runtime | CPU/memory/disk settings; own diagnostics and image verification; Apache-2.0 runtime | Backend contract reusable; cloud is a separate adapter, not remote Lima |
| Tart integration | Existing OCI image workflow and Linux support, native VM UI | Existing lifecycle/images but tool-specific guest setup | Resource flags and packaged runtime; current FSL commercial competing-use restriction | Licensing conflicts with an unrestricted default; not selected |
| Hybrid: Swift VM helper + Go product | Faster only if a proven helper fits exactly; otherwise two builds and IPC boundary | Own helper plus integration, image and release work | Most direct device control, extra signed artifact | Reuse guest/API; future option only after a concrete Lima blocker |

Lima is the shortest credible path without adopting a new VM manager's full implementation. vmette already demonstrates a guest desktop/vsock approach; Vitrine demonstrates richer VM/window integration. Both are relevant prior art, but integrating either now adds another product-specific runtime contract whose distribution/maintenance needs validation. Their advertised timings are not our measurements [R14, R15].

### Host assumptions and resources

Initial development target: native Apple silicon macOS with virtualization available; the inspected machine is macOS 26.5 ARM64. The supported minimum macOS version is **unset** until tested against the chosen Lima binary and guest. No Intel/translation fallback. Go SDK v1.8.0 declares Go 1.25.0; use the inspected Go 1.26.1 toolchain initially, then pin the validated toolchain and module checksums in Phase 1 [R12].

Starting **experiment**, not a minimum or performance promise: 2 vCPUs, 4 GiB guest RAM, 24 GiB logical sparse disk, 1280×800×24 display. Measure actual allocation and growth; sparse capacity is not free physical space. The inspected host has only about 13 GiB free. Before image download, measure host memory/available disk and image expanded size plus temporary conversion/package-install overhead; refuse if the operation cannot leave headroom. Do not automatically delete caches or user files. Use a separately selected volume or ask the user to free space if the budget does not fit.

### Lima configuration requirements

Use a private per-machine LIMA_HOME inside project-owned state under the user's application data, directory mode 0700, not the user's existing Lima namespace. Refuse unexpected global override files or mounts. Validate the resolved configuration before **every** start; a YAML field alone is not evidence of isolation. Set a generic bootstrap username/home/comment rather than inheriting the host username or profile information. Pin Lima 2.2.1 as the initial compatibility candidate, verify release artifact checksums, and reject unknown versions until tested [R2, R3].

Required intent (exact serialization validated in Phase 1): `vmType: vz`, `arch: aarch64`, `plain: true`, no mount template/base, empty mounts, no copyToHost, no additional/block devices, no Rosetta, no container runtime, no audio/microphone, no shared clipboard. Explicitly disable `ssh.loadDotSSHPubKeys`, `ssh.forwardAgent`, `ssh.forwardX11`, proxy environment propagation, and arbitrary static port forwards. Explicitly clear lowercase and uppercase ftp/http/https/no_proxy settings as well: the reviewed cidata code still reads macOS system proxies when process proxy propagation is false [R2]. Plain mode disables dynamic forwarding, mounts, containerd, Rosetta and agent forwarding, but static forwards and base SSH setup remain possible [R2]. Reject unexpected static forwards too. Use product-owned keys, no access to the user's SSH config/agent. Do not mutate other Lima instances or globally install privileged networking helpers.

Prefer the dated Debian 13 genericcloud ARM64 image referenced by Lima 2.2.1 (20261001-2618); verify against the distributor's signed checksum manifest before adopting it. Lima's template provides a SHA-512 digest, but that is not independent signature verification. Copy only the selected image URL/digest and required reviewed compatibility workaround into our own manifest; do not inherit its host-mount template or unpinned `daily/latest` fallback. If unavailable or incompatible, fail and deliberately select another dated image. Debian ARM64 UEFI issues and the template's SSH-over-vsock/LLMNR workaround make a real boot test essential [R3, R5].

Provision through reviewed idempotent scripts/cloud-init. Install minimal Xfce components, Xvfb, xauth, dbus, Chromium plus its sandbox package, scrot, xdotool, xclip and fonts for tested Unicode. Create an unprivileged `agent` desktop user distinct from the Lima bootstrap administrator. Start desktop units under that user, persist guest home on the VM disk, and install a root-owned, non-writable guest helper. The bootstrap admin remains trusted management access, never exposed by MCP. Record package versions, image digest, provision revision, and logs; do not claim byte-reproducible apt installs without a repository snapshot.

### Networking

MVP networked mode is outbound NAT, no bridged LAN attachment or automatic guest service publication. NAT does **not** deny access to host or LAN services. Lima documents host reachability through its user-mode gateway; exact VZ behavior must be tested rather than inferred from QEMU documentation [R4]. Disabling host DNS integration is not a firewall. Report `networkIsolation: none` for NAT mode.

Creation requires explicit network choice. Explain that package installation needs provisioning network access and distinguish that from subsequent runtime access. Test offline VZ operation while preserving SSH-over-vsock; advertise it only if confirmed. A later restricted mode requires actual host/runtime/provider egress enforcement for IPv4, IPv6, DNS, host gateway, LAN and metadata destinations. Guest nftables alone cannot constrain a compromised guest root, so it is not a strong host-network boundary. Do not claim hostile-guest safety from this MVP network configuration.

## Host/guest channel

| Channel | Benefits | Risks / decision |
| --- | --- | --- |
| Raw virtio socket | No IP route required; Apple supports host/guest port streams | Device access is not application authentication; own Swift/VZ plumbing and protocol lifecycle. Defer raw use |
| SSH, selected | Proven authentication, encryption, exec streaming, established Lima integration; can ride vsock | Dedicated management credentials and host-key pinning required; no agent/X11/reverse forwarding |
| Restricted virtiofs mailbox | No network listener; simple offline exchange | Exposes host filesystem surface, symlink/race/disk exhaustion hazards, polling; rejects no-host-mount default |
| Guest HTTPS service | Natural remote transport | Adds certificate/bootstrap and listener exposure; cloud can use a private authenticated route later |

Use one host-initiated persistent SSH process per active guest session, running only a fixed installed helper command. Prefer Lima's SSH-over-vsock, supported by current defaults for VZ Linux; test in the pinned plain configuration. If unavailable, use only the generated loopback SSH endpoint, show its presence in diagnostics, and retain all SSH authentication. Never broaden to `0.0.0.0` to fix connectivity. Read runtime-generated SSH endpoint metadata as untrusted bounded data and constrain paths to the owned runtime directory.

Bootstrap trust: generate a unique guest SSH host key on the host for each new machine and install it through reviewed root-only cloud-init/data provisioning, then reload guest sshd before declaring readiness. Pin the public key in the host-owned known_hosts under a stable HostKeyAlias tied to machine UUID; require StrictHostKeyChecking=yes and NoHostAuthenticationForLocalhost=no on application connections. Keep bootstrap files containing private key material only in mode-0700 runtime state, never source control/logs; remove temporary copies when provisioning finishes, and account for retained cidata copies until VM deletion. Never reuse this guest host key as a host credential. Validate that provisioning/restarts do not regenerate or overwrite it. This is a concrete Phase 1 implementation/compatibility gate, not tested functionality.

Lima's own bootstrap SSH does not promise this strict verification; its documented examples relax localhost host checks [R2]. Treat the initial runtime/bootstrap path as part of the trusted local VM manager and avoid application traffic until our key is installed and verified. Prefer vsock so a reused loopback port is not the bootstrap destination. If the pinned runtime cannot install the key reliably, fail closed and revise enrollment before enabling the application, rather than silently weakening verification. A mismatched key on restart is an error; reinstall requires a new machine identity.

Use a separate product-generated SSH key. After provisioning authorize its desktop-control access through a forced command executed as `agent`, disabling PTY, forwarding, agent forwarding and user rc processing. Provisioning admin keys are separate and inaccessible to agents. No ambient SSH config, host environment, host home paths, or credentials are sent. SSH authentication authorizes guest access; it does not make guest responses trustworthy. The host exposes no guest-callable control service over this channel.

### Guest wire protocol v1

Independent of MCP. UTF-8 newline-delimited JSON, one message per line, escaped newlines in strings; no banners/debug output on stdout. Stderr is bounded and sanitized. A hello exchanges supported API major/minor, machine binding, fresh session/display generation, capabilities and limits. No common major → `VERSION_MISMATCH` and close. Reject duplicate fields, malformed types, unknown action types, invalid UTF-8 and oversized messages. Ignore documented optional additive fields only after major-version agreement.

Request: `{apiVersion:"1.0", requestId, sessionId, method, params, timeoutMs}`. Response echoes request/session IDs and contains exactly one `result` or `error`. Supported methods: status, screenshot, input, cancel, releaseInput. PNG is base64 in screenshot replies for the first slice. No guest response may name a host path to read/write or supply a command to execute. Guest helper paths/display/Xauthority are fixed by installation, never request parameters.

Initial safety limits, tunable after measurements: 16 MiB JSON line, 8 MiB encoded PNG, maximum image dimensions 4096×2160 and 9 million pixels, 64 KiB UTF-8 text action, 16 queued input actions, 64 KiB retained stderr, one capture in flight and one pending viewer frame. Limit decoded image memory before allocation, reject decompression bombs/truncated PNG, and limit child-process runtime/output. Validate request sizes before parsing fully. Reject excess requests with `RESOURCE_EXHAUSTED`, do not buffer without bounds.

Default action/capture timeout 5 seconds; text paste 15 seconds; handshake 10 seconds. Boot/readiness has a separately configurable 180-second diagnostic deadline; provisioning defaults to 20 minutes with stage reporting and cancellation. These are budgets, not speed claims. Kill owned child process groups on timeout. Use host/guest monotonic duration budgets, not synchronized wall clocks. Serialize input; a bounded viewer stream must not starve it. Use separate SSH stream/process if measurements reveal frame head-of-line blocking.

Cache completed action request IDs and results within a session (bounded 256 entries, 5 minutes). A reconnect never automatically retries a mutation with unknown outcome. Return `OUTCOME_UNKNOWN`, reacquire current session and screenshot. Cached deduplication does not promise exactly-once execution across guest crashes. On link loss cancel pending work, release held inputs if possible, mark session disconnected, and reconnect with backoff capped at 5 seconds; no implicit infinite start/provision retries.

## Desktop capture and input

Systemd runs `Xvfb :1 -screen 0 1280x800x24 -nolisten tcp -auth <guest-owned-cookie>` and an Xfce session under a private D-Bus session. Do not use `-ac`. Disable Xfce compositor/locking for the managed demo session; expose lock/login/session failure as not-ready if encountered. Xvfb is a real virtual X server, not a physical VZ console [R6]. All browser apps run on this single display; Chromium runs as `agent` with its sandbox enabled, never `--no-sandbox`.

First capture adapter runs scrot against fixed DISPLAY/XAUTHORITY and reads a private guest temporary PNG with size/time limits and cleanup. Prefer stdout if the pinned package supports it. It never writes to a host mount. This intentionally favors proven tools over a new X11 capture engine. If measurements require improvement, evaluate XGetImage/XShm plus XDamage dirty regions before selecting video codecs.

xdotool provides XTEST pointer/button/key actions only on this X11 session [R7]. Pass validated arguments directly to exec, never interpolate text into shell. Always release modifiers/buttons in finally/timeout handling; guest helper startup also clears known held state. A hard crash can still leave an X server state until reset; report degraded state and offer session restart rather than asserting cleanup succeeded.

**Text and keys differ:** text is literal UTF-8 pasted through a guest-only CLIPBOARD selection via xclip, then a tested paste chord. Clear the temporary selection after the paste handshake/timeout; do not expose read-clipboard or host synchronization. Guest apps can read that text. Chromium and the demo editor are supported targets only after testing. Terminal apps may need Ctrl+Shift+V rather than Ctrl+V; require an explicit paste mode. Password fields, unusual apps and IMEs may reject paste; return `TEXT_INPUT_UNSUPPORTED` or require user verification, never claim text landed merely because a chord was sent. Key chords use a finite documented enum (Ctrl/Alt/Shift/Super and named keys); macOS Command is not silently translated to guest Ctrl. Do not promise arbitrary Unicode from keycodes.

**Coordinates:** canonical actions use integer pixels in an identified screenshot, top-left origin, `0 <= x < width`, `0 <= y < height`. For a returned scaled screenshot width Ws/height Hs of a guest Wg/Hg, map pixel centers as `floor((x+0.5)*Wg/Ws)` and analogously for y, clamped only for floating-point rounding. Reject out-of-range inputs rather than silently clamp user mistakes. Metadata includes guest dimensions, image dimensions, displayId, scaleX/scaleY, displayGeneration, sessionId, frameId and capturedAt. MVP single display and scale 1; no dynamic guest resolution changes. Terminal/browser resize letterboxes the image and recalculates offsets; it does not resize the guest.

Viewer pointer coordinates first subtract the displayed-image offset and account for canvas CSS/device pixel scale, then map to canonical frame pixels. Ignore clicks in letterbox margins. Reject unknown frame IDs, images older than the input freshness budget (initially 10 seconds), mismatched session or generation with `STALE_FRAME`; require a fresh screenshot after any display restart. Layout may change even during that budget, so action acknowledgement never proves semantic success.

On guest boot, helper/display restart, or reconnection after lost identity, broker issues a new session UUID and revokes control leases/old frames. Machine ID and persistent disk remain unchanged. Input cannot escape to the host: host code never calls macOS input APIs and never accepts guest instructions to invoke them. Verify unchanged host pointer/clipboard through a manual acceptance check; no host Screen Recording or Accessibility permission should be requested.

Wayland is a later, separate adapter. X11 capture/XTEST does not provide general Wayland capture/input; XWayland is not a substitute for whole-desktop control. A selected Wayland compositor would need tested ScreenCast/PipeWire and RemoteDesktop/libei/EIS support, portal permissions/restoration, and headless session behavior [R8].

## Viewer

First supported target: local Kitty on macOS, exact version recorded during Phase 3. Direct chunked PNG Kitty graphics over the PTY avoids shared-memory file lifetimes and filesystem-based image loading. Use capability query with bounded timeout, real pixel/cell size detection, fixed image IDs and cleanup. Do not infer support solely from TERM. Initial scheduled refresh 2 frames/second maximum, plus on-demand screenshots; actual achieved rate unknown. Skip identical captures, keep newest frame, bound capture rate and drop stale frames. Stop polling when no viewer watches. No full-motion video claim [R10].

Terminal viewer initially watch-only; interactive input is a Phase 3 exit requirement. Use Kitty keyboard protocol where negotiated, SGR mouse, bracketed paste, an explicit shortcut palette for intercepted shortcuts, and documented Ctrl+] exit. On exit/signals restore raw mode, mouse reporting, alternate screen and graphics state. On disconnect display frame age/reconnecting status and disable input. Guest window titles/logs are escaped as text, never emitted as terminal control sequences.

If capability query fails or stdout is redirected: print status and `view --web` instructions; no raw escape noise. No tmux, remote SSH session, iTerm2 or Sixel support promise initially. iTerm2 OSC 1337 has different framing/file-transfer semantics; Sixel is another terminal-specific protocol with its own detection/placement behavior. Neither is needed to validate one viewer. A newer iTerm2 may also implement Kitty graphics, but that alone is not a tested support claim [R11].

Fallback: a small local web page on loopback displays broker PNG frames on a canvas and sends the same typed input operations through an authenticated WebSocket. No guest HTML is rendered. Use a cryptographically random one-use bootstrap token in URL fragment, exchange to an HttpOnly SameSite cookie, remove fragment, apply restrictive CSP, validate Origin/Host and a CSRF token on writes; no CORS wildcard. Token expires in 60 seconds; viewer authorization lasts at most 15 minutes and is scoped to machine/session and view/control permissions. Never log tokens or put them in query strings. Limit frame/connection counts; closing a tab releases its control lease. Browser HTTP is loopback-only, not a public API.

A VZ native window would show the virtual hardware console, **not our Xvfb desktop**. It is therefore not a valid automatic fallback. A dedicated native raster viewer could reuse our frames but adds a second UI stack. noVNC is proven and useful if we adopt RFB/TigerVNC later; starting a separate VNC input path now would bypass common control leases unless carefully integrated, so defer it.

## Local/cloud contract v1

Application API v1 is distinct from MCP's version and the guest wire version. Initially internal Go types plus JSON schemas; add HTTP endpoints without changing semantics for cloud. Do not create an abstraction for unsupported runtime devices.

| Operation | Semantics |
| --- | --- |
| capabilities | Backend availability, architecture, supported operations/actions, network modes, persistence modes, viewer formats, limits and versions |
| machine.create(spec, idempotencyKey) | Allocate an owned machine; return machine + asynchronous operation; reject unsupported requirements first |
| machine.list/get | User/workspace-scoped records; desired and observed state, readiness, resources, last error/time |
| machine.start/stop | Start or graceful stop; stop preserves disk where persistence is advertised; operation tracks completion |
| machine.delete | Explicit irreversible removal of owned disks/resources; idempotent, never conflated with stop |
| operation.get/cancel | Progress and outcome; cancellation is best effort, does not roll back completed actions |
| session.open/get/close | Attach a desktop session; close detaches, does not stop machine |
| frame.capture | PNG plus trusted broker metadata after guest validation |
| input.send | Typed input against session/frame plus valid input lease |
| control.acquire/release | One writer; human takeover preempts agent; view remains shared |

No common shell, mount, snapshot, pause, GPU, or arbitrary TCP forwarding operation in v1. Cloud providers may expose optional snapshot/pause/export capabilities later; absent capability is an error, never success with no effect. Local disk persistence is a required local MVP capability. Cloud stop/start persistence is either mapped to a tested provider snapshot/disk mechanism or rejected before creation if requested. Ephemeral cloud machines must advertise `persistence: none` and explain stop data loss; they cannot impersonate the persistent profile.

Machine IDs: random UUIDv4, never recycled, scoped by backend authority + workspace; names are mutable aliases and not authorization. Keep provider instance IDs private. Session IDs: random UUIDv4 per attached desktop generation, invalidated on restart; not a credential. Frame IDs and request IDs are opaque and bounded. Lifecycle operation IDs persist independently of CLI processes. Local JSON state is atomically replaced under a file lock, mode 0600; reconcile after broker crash with actual Lima instance identity before taking action. Never delete an instance because its name merely matches a prefix.

Machine state transitions: `creating -> stopped -> starting -> running -> stopping -> stopped`; `creating/starting/stopping -> error` on confirmed failure; observed `unknown` on unreachable runtime, not assumed stopped. Running has separate readiness `booting | provisioning | desktop_ready | degraded`. `running` alone does not authorize input. `deleting -> deleted` only after cleanup confirms no owned resources remain; from stopped/error/unknown require runtime reconciliation and explicit delete authorization. Sessions: `connecting -> ready -> disconnected | closed`; disconnected sessions cannot accept input. Desired state and observed state are separate, with lastObservedAt.

Mutations return `{operationId, state}` and status contains progress stage plus error; retries with the same idempotency key must not allocate a second machine. Persist operation intent before allocation, and tag resources with machine/operation/workspace IDs for crash recovery. Input has session-limited deduplication, not lifecycle idempotency. Stop is idempotent once stopped; force-stop is a separate explicit option and can lose data. Delete does not remove logs/artifacts outside recorded owned paths and refuses symlink/path escapes.

Errors contain `code`, actionable `message`, `requestId`, `retryable`, optional `retryAfterMs`, and redacted details. Stable codes: INVALID_ARGUMENT, NOT_FOUND, UNAUTHENTICATED, PERMISSION_DENIED, UNSUPPORTED_CAPABILITY, VERSION_MISMATCH, RESOURCE_EXHAUSTED, IMAGE_VERIFICATION_FAILED, HOST_KEY_MISMATCH, NOT_READY, CONTROL_BUSY, STALE_SESSION, STALE_FRAME, TEXT_INPUT_UNSUPPORTED, DEADLINE_EXCEEDED, TRANSPORT_LOST, OUTCOME_UNKNOWN, BACKEND_UNAVAILABLE, CLEANUP_PENDING. Retryable does not authorize replay of an input action. Standard HTTP mappings for cloud: 400/401/403/404/409/413/429/503/504; MCP wraps application failures as tool errors with structured code/details. Non-JSON CLI mode emits actionable stderr and nonzero exit; JSON mode emits one structured result, no progress mixed into stdout.

Capabilities report schema version, supported API major/minor, backend, guest arch/display server, action enums, text paste modes, persistence, resource ranges, max frame/text/request sizes and observed network mode. `supported` is separate from `available/ready`. Major mismatch fails; additive minor fields remain optional. Negotiate MCP using official Go SDK v1.8.0, which documents support through 2026-07-28 and older revisions; do not hand-roll initialization or echo arbitrary client versions [R12, R13]. Test at least one current real client and an older negotiated revision. Local MCP uses stdio (stdout protocol only, stderr sanitized diagnostics); remote cloud MCP can remain a local adapter over our authenticated HTTPS API initially, avoiding a separate public MCP auth surface.

### MCP SDK alternatives

The official Python SDK offers concise server authoring and mature transport support but adds a Python runtime/package environment to the host distribution. TypeScript's current v2 server/client packages provide stdio/HTTP and auth helpers, but add a JavaScript runtime and packaging stack. Both remain credible maintained choices, with migration/version support checked at their exact release. The official Go SDK is selected because it covers the needed protocol transports and lets the host and guest use one build toolchain; Swift's SDK would not remove the need for guest/Linux code and is listed at a lower maintenance tier in the current SDK catalog [R12, R13]. Do not select a language based solely on SDK tier or assume all clients implement the newest spec.

The local Unix socket lives in a private runtime directory, socket mode 0600, with same-user peer verification. Any process with that user's privileges may control the guest: there is no protection from a compromised host account. MCP never exposes a raw broker socket path to guest content. A stdio adapter exiting does not kill the broker/VM; explicit stop remains required.

## Future cloud: one region, managed provider first

Design, not deployment. Prefer a provider-backed pilot (evaluate E2B desktop and Daytona computer use) over operating KVM hosts initially [R16, R17]. Do not choose a provider from startup-time marketing. Require a fixed selectable region, lifecycle lookup after ambiguous create, resource tagging, hard TTL, authenticated private access, known isolation boundary, quotas, export/retention semantics, image compatibility, and a cleanup API. Validate ARM64 availability; a provider with only x86-64 needs an explicitly different guest capability/image recipe, never an ARM compatibility claim.

| Managed provider | Own VM infrastructure |
| --- | --- |
| Faster pilot, existing SDK/viewing and lifecycle; provider handles host operations | Choose KVM-capable workers, distro/image format, network policy, scheduler and streaming gateway |
| Provider pricing, data handling, architecture, service limits, semantics and lock-in | Own kernel/hypervisor patching, capacity, placement, noisy-neighbor limits, disk snapshots and incident response |
| Must map its auth/persistence/view APIs without exposing account-wide keys | Greater control but much more operational burden; Firecracker/KVM is an option to evaluate, not chosen implementation |

Single-region shape: TLS API + OIDC identity provider; workspace/membership/role tables and machine/operation records in PostgreSQL; queue/reconciler for provider calls; private viewing gateway; object storage for explicitly retained artifacts/snapshots; provider sandbox fleet. Keep provider credentials in managed secret storage, never clients/guests. Validate OIDC issuer, audience, expiry and membership on each API operation. Database row ownership and workspace RBAC gate every machine operation; provider IDs are not authorization. Distinguish reader/controller/admin. Enforce one active input lease, with user takeover policy.

Viewer gateway issues short-lived session-scoped view/control credentials after authorization and rechecks revocation/expiry. TLS/WSS streams provider frames or broker PNGs; raw VNC/provider ports are never public. If using provider noVNC, its input must pass our control-lease policy or be disabled while agent control is active. Bound bandwidth, connection count and frame dimensions. Session restart invalidates credentials. API tokens require audience/workspace/action restrictions; do not forward them to guest apps.

Meter requested vs allocated vCPU/RAM, provider-confirmed running intervals, persistent storage bytes/time, and streaming bytes with event IDs, timestamps and reconciliation. Usage ledger is not an invoice or custom billing platform; payment integration is deferred. Prevent double-counting retried events, use provider-confirmed transitions and reconcile missing stops. Display estimated vs finalized usage separately. Enforce allocation quotas before creation, plus hard resource limits in provider/host infrastructure rather than app promises.

Proposed pilot policy: explicit 15-minute input/agent-work idle deadline plus 60-minute hard session lifetime, adjustable by workspace within provider limits. Passive viewing does not reset idle; active authorized work renews a bounded lease. Warn before shutdown; pause only if snapshot persistence is tested, otherwise explain deletion/loss for ephemeral machines. Configure a provider hard timeout so API failure cannot leave compute running indefinitely. TTL expiry stops compute; stored snapshots still need a separate retention policy and explicit deletion. E2B documents persistent paused state requiring explicit kill; never assume pause cleans up storage [R16].

A reconciler compares database records with tagged provider resources, handles partial creation and duplicate callbacks, retries stop/delete with backoff, and surfaces CLEANUP_PENDING until verified. Tombstones retain ownership and audit metadata, not guest secrets. Delete must remove compute, snapshots/volumes, viewer credentials and retained artifacts according to policy; audit evidence notes failures. Test API outage, orphan allocation, expired credentials, quota exhaustion, and cross-workspace denial before paid/public use. Backups, retention, lawful data handling and incident operations need a separate hosted-service review.

## Dependencies and licensing

No runtime dependency is installed/vendored in Phase 0. Recommend Apache-2.0 for original code, schemas, image recipes and viewers, with LICENSE adoption in Phase 1. License labels below are source observations, not a complete redistribution clearance. Review exact releases, transitive dependencies, notices and image package copyrights before shipping binaries or images.

| Component | Why / initial selection | License observation and action |
| --- | --- | --- |
| Apple Virtualization.framework | Native virtualization through Lima | Apple SDK/platform terms, not an open-source dependency to relicense; signing/distribution validation required [R1] |
| Lima 2.2.1 | Local VM and cloud-init/SSH lifecycle | Apache-2.0; retain LICENSE/NOTICE and inspect release SBOM/transitives [R2, R3] |
| Go toolchain + standard library | CLI/broker/helper and PNG/HTTP | BSD-style; retain required notices in distributed artifacts; exact toolchain audit in Phase 1 |
| MCP Go SDK v1.8.0 | Maintained MCP transport/schema implementation | Transition text says Apache-2.0 for new/consented code and MIT for unconsented older contributions; preserve applicable notices, review pinned tag and transitives [R12] |
| OpenSSH host/guest | Authenticated guest transport | Multiple BSD-style and other component notices; inspect shipped binary/package copyright |
| Debian 13 ARM64 + Linux | Guest image and packages | Aggregate of licenses, kernel GPL-2.0; preserve per-package copyright, source obligations and redistribution/trademark requirements; not covered by our license [R5] |
| Xvfb/Xorg and Xfce | Display/session | Mixed X11/MIT-style and component GPL/LGPL licenses; package-by-package review pending |
| Chromium + sandbox + fonts | Browser and test text rendering | BSD-style core plus many third-party licenses, codec/font obligations; record all installed packages/licenses |
| scrot | First PNG capture adapter | MIT-style text with extra acknowledgement language; preserve exact COPYING and acknowledgement, not just generic MIT [R9] |
| xdotool | XTEST input | BSD-3-Clause-style COPYRIGHT reviewed; retain notices [R7] |
| xclip | Guest-only Unicode paste | GPLv2 COPYING; package headers determine exact expression; redistribute with required source/notice compliance [R9] |
| Kitty | First terminal; protocol implemented independently | User-installed terminal, not bundled; Kitty GPLv3 project license and spec/example reuse terms need review before copying code [R10] |
| noVNC / TigerVNC, deferred | Possible later remote viewer/display stack | noVNC core MPL-2.0 with separately licensed assets; TigerVNC GPLv2 distribution; review exact components if selected [R18] |
| E2B / Daytona, candidates only | Future provider adapters | E2B repository LICENSE Apache-2.0 does not cover all infrastructure/service terms; Daytona current LICENSE paths failed retrieval despite older AGPL references. No blanket license conclusion or incorporation until exact artifact reviewed [R16, R17] |
| terminal-mac / vmette / Vitrine | Research only, no copied code/assets | MIT / MIT / Apache-2.0 files reviewed. Any future reuse requires retained upstream notices and file-level audit [R14, R15] |
| Tart, rejected default | VM comparison | Current main LICENSE FSL-1.1-ALv2, competing-use restriction, Apache future grant after two years per version; historical Fair Source pages are not current license authority [R3] |

Pin actual guest package versions and source image/checksums in an image manifest after compatibility tests; do not freeze an insecure browser indefinitely. Updates are deliberate tested image revisions with rollback, not unreviewed mutable tags. Build a dependency/license inventory before distribution. The core will remain usable locally without a cloud account; a future hosted scheduler, auth, metering or team service may be separately licensed/private. No claim that uncreated hosting code is open source.

## Proposed repository structure

Only README/docs exist now; this is the implementation layout, not stub modules to create in Phase 0.

```text
cmd/agentlinux/           CLI, MCP entry, broker entry
cmd/agentlinux-guest/     Linux-only helper
internal/app/            shared operations and input ownership
internal/api/            versioned DTOs, errors, capability validation
internal/backend/lima/   owned runtime config, lifecycle and reconciliation
internal/guest/          SSH transport and bounded guest protocol
internal/desktop/        Linux capture/input adapters
internal/viewer/kitty/   terminal rendering/input and cleanup
internal/viewer/web/     loopback authenticated canvas viewer
internal/state/          atomic local machine/operation records
api/v1/                  JSON schemas and examples
images/debian/           reviewed provisioning, package/image manifest
scripts/                 build and isolated hardware acceptance harness
tests/contract/          backend behavior
tests/e2e/               real guest/client fixtures
docs/decisions/          short ADRs
```

A future `internal/backend/cloud/` is created only with a real provider adapter. Keep host and guest builds distinct so a macOS binary cannot accidentally execute guest input code. Build scripts must work before adding packaging polish; CI without VZ runs unit/contract checks and explicitly skips hardware tests. Planned phase gates and technical experiments: [roadmap.md](roadmap.md), [verification.md](verification.md).
