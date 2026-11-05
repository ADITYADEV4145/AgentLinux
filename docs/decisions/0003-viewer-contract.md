# ADR 0003: Kitty first; common operations, explicit capabilities

Status: accepted for the first implementation.

Use Kitty's direct, chunked PNG graphics transport in one tested local terminal configuration. Start with bounded screenshot refresh. A local browser canvas uses the same broker frames and input operations. Do not transmit terminal escape sequences, HTML, URLs, or host file paths supplied by the guest.

Model a persistent machine separately from an ephemeral desktop session. Backend-neutral lifecycle and desktop operations use versioned schemas; capabilities describe persistence, input modes, networking, viewing, limits, and architecture. Unsupported cloud operations fail before allocation. MCP protocol negotiation is delegated to the official Go SDK and is separate from our API versioning.

Cost: lower initial refresh than optimized video and no native VM window fallback for an Xvfb desktop. The benefit is one observable desktop and the same input/coordinate semantics for viewers and agents. No implied local/cloud feature parity.

Evidence: [architecture contract](../architecture.md), [sources R10–R13](../research.md).
