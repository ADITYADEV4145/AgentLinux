# 0013: gated cloud prototype and explicit human ownership

Accepted for Phase 6 development, 2026-11-05.

Keep Lima/VZ and the tested Debian/Xfce workstation locally. Add `start` to compose actual boot/readiness with an interactive viewer. Kitty is the terminal target; unsupported terminals get the secured loopback web viewer. Human and agent input are serialized through an expiring ownership lease, with explicit F2 handoff. Viewing remains concurrent. This adds a predictable interaction model but agents cannot mutate while a human retains control.

Put cloud sessions behind an authenticated development API implementing the same machine interface. Default to an unmistakably simulated provider for free deterministic tests. Implement an optional E2B Desktop SDK worker, enabled only by an operator; never silently allocate cloud resources. E2B commercial terms require clarification/permission before a hosted product. An EC2 VM per session remains the alternative when image/region/commercial control outweighs operational cost. See [research, security boundaries and pending acceptance](../cloud.md).

Cloud sessions are ephemeral with confirmed deletion, conservative runtime reservations and bounded TTL/idle/concurrency. No billing, production identity, public deployment, provider-tested isolation, durable cloud home or local/cloud image parity is claimed. Capability discovery removes unsupported tools. A simple authenticated PNG proxy reuses the viewer; public VNC links and premature video optimization are avoided.
