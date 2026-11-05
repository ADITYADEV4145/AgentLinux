# ADR 0009: Kitty direct PNG with a view-only loopback fallback

Date: 2026-11-05. Status: implemented; Kitty visual acceptance pending an unlocked/awake test Mac.

Keep the existing guest PNG source and common machine interface. Add a Go view-only terminal consumer using official Kitty direct PNG chunking, active capability probing, real pixel/cell sizing, bounded writes and deferred termios/alternate-screen cleanup. Reuse one image ID and suppress unchanged host-to-terminal PNG transfers. Do not read host files through terminal graphics commands or forward terminal input to the guest.

Unsupported environments receive a real HTTP polling viewer on ephemeral 127.0.0.1, authenticated with a random fragment-delivered capability/bearer header, strict Host/Origin/CSP/no-store boundaries, one shared bounded capture cache, and no input/command endpoints. This fits Xvfb better than a native VZ display that does not own the Xvfb screen. A native app would add platform/UI work without improving the existing capture contract.

Synchronous capture/render provides backpressure and no old-frame queue. Static-image hashing/ETags reduce display transfers, not guest capture/SSH bytes; adaptive idle cadence and no background web capture limit work. Full frames, subprocess capture and per-request SSH remain explicit costs. Guest calls wait at most two seconds for an existing machine lock to avoid routine viewer/MCP collisions. No lifecycle retry, input replay or long transaction lease is introduced.

Direct human control is deferred: a read-only observer can coexist with MCP without mixed input. Persistent ownership/takeover is required before adding human typing/mouse; do not silently treat terminal intercepted shortcuts as guest input. No custom codec, Rust rewrite, broker daemon, cloud endpoint or paid deployment is introduced.

The actual browser fallback and real VM/MCP/restart path were verified. Kitty's protocol/PNG acknowledgement, PTY geometry/cleanup were tested, but the locked/asleep Mac did not permit a trustworthy visible desktop recording. Blank internal graphics captures are not a passed terminal test, and the implementation needs the explicit visual checklist before Phase 4 is declared fully accepted. See [viewer contract](../viewer.md) and [verification](../verification.md).
