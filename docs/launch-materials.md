# Launch materials — DRAFT, not published

These are reviewable text drafts. No repository description/settings, release/tag, social post, uploaded recording, public deployment or paid activation was changed by preparing them. The repository's source commit is separate from approving a product launch.

## Proposed GitHub description

Agent-ready Linux desktop for Apple silicon Macs, with MCP control and a terminal viewer. Local developer preview; cloud backend in development.

## Draft release notes

Unpublished developer candidate; CLI version 0.5.0-rc.1 plus the reviewed Phase 8 commit. Phase numbers are implementation milestones, not a claim that a versioned release exists.

- Real local Debian 13 ARM64/Xfce workstation using Lima 2.2.1 and Apple's Virtualization framework. Chromium/Firefox ESR, file manager, terminal, editor, settings and launcher.
- Guest-only screenshots, mouse/keyboard/Unicode, constrained guest command execution, bounded file copy and guarded persistent lifecycle/reset/delete.
- Official-SDK stdio MCP with status/capabilities and screenshot → act → verify workflow. Real Codex acceptance is distinct from deterministic SDK tests.
- One-command `start`, native Kitty observation/human control, explicit F2 ownership handoff, safe viewer exit and authenticated loopback browser fallback. Limited human shortcuts/IME/drag remain documented.
- Development cloud HTTP/session adapter, explicit SIMULATED default, gated E2B integration and sandbox-only Stripe metering foundation. No functioning hosted offering, real-provider acceptance or hosted Stripe completion claim.
- Phase 8 fixes: current README/two-terminal agent flow, corrected stale capability/ownership docs, preflight viewer-output validation, bundle examples/cost-model data, clearer first MCP instructions and Go 1.26.9 patch pin.
- [Measured methods/results](performance.md), [known issues](known-issues.md), [security boundaries](security.md), and [cloud launch gates](cloud-release-checklist.md).

Unsigned/source-built, not notarized; one development Mac is tested. No clean-Mac or broad compatibility certification. No binary/image/public release is included with this draft. Existing guest disks retain their installed helper/recipe; explicit reset is destructive, not a migration or snapshot.

## Draft launch post

**Give your agent a Linux computer—starting with your Mac.**

Agent Linux is an open-source developer preview that runs a real Linux desktop on an Apple silicon Mac. Connect an MCP agent, let it operate the guest, and watch the desktop in Kitty or a secure local browser viewer. You can take a human turn and hand control back.

Our reproducible demo asks an agent to complete a project-budget form in the Linux browser, verify the displayed result and report it. It uses synthetic local data, not a staged screenshot, fake cloud or customer transaction. Exact observations and limitations accompany the demo.

Local source builds are the current path. The cloud API and sandbox billing are foundations in development; there is no hosted service to buy or finalized cloud price. The longer-term promise is “Run it on your Mac or in the cloud.” We are not claiming that future offering is available today.

Original code is Apache-2.0. Bring your own reviewed Lima/Kitty and agent client; model usage follows that client's terms/costs. The VM has no host project/home mount or host clipboard by default, but its NAT network can reach host/LAN services. This is not an independently audited hostile-code sandbox.

We welcome contributors for clean-Mac acceptance, terminal/browser accessibility, image updates and lifecycle reliability. See the compatibility matrix, demo instructions, measurements and known issues before trying it. No testimonials, user counts, benchmark superlatives or guarantees are implied.

## Demo and recording draft

Use [the short script](demo.md). Publish only a reviewed real capture whose source hardware/versions, run date and scope are stated. The Phase 8 capture is sampled from an isolated Kitty window with original frame timestamps; it is not a full-desktop screen recording, a measured video FPS demonstration or proof of cloud performance. Keep raw paths, viewer tokens, screenshots of personal data, model events and logs private. Approval is still needed before uploading/linking public media.

## Open-source versus paid boundary

The code actually present in this repository—including local CLI/guest service, MCP/viewer, development control plane, E2B adapter and sandbox billing foundation—is Apache-2.0 except attributed third-party components. External managed provider infrastructure and the agent/model services are governed by their own terms; they are not made open source by our adapter. No hosted service exists in this repository's operation today. A future managed offering may be paid, and any proprietary components or hosted-only features must be identified honestly when they exist. No paid team plan is advertised without real team features.
