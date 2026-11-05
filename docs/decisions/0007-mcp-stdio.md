# ADR 0007: Official Go MCP SDK and one-machine stdio adapter

Date: 2026-11-05. Status: accepted and implemented for Phase 3.

Use official `modelcontextprotocol/go-sdk` v1.8.0 and its stdio transport, adapting the existing `machine.Machine` contract. Keep guest protocol 2.0 independent of negotiated MCP versions. No cloud stub, public HTTP endpoint, lifecycle tool, or implicit VM startup. The client launches a process bound to one existing private namespace.

Use strict tool-specific JSON schemas, structured result envelopes plus text, PNG image content with metadata, required recent observed frame IDs, per-call deadlines, propagated cancellation, and an immediate-busy gate. Existing machine locks also cover CLI/other MCP processes. SDK input framing is capped; diagnostic stdout is prohibited. Typed backend failures remain coded tool errors; unknown host details are withheld.

The Phase 0 broker is deferred: the tested file lock plus per-server gate satisfies this phase's serialized operations with fewer moving parts. This does not provide a viewer/human ownership lease, fair queuing, reconciliation of ambiguous mutations, or persistent multi-client sessions. Those remain explicit Phase 4 work. No automatic action retry is introduced to hide disconnects.

Client instructions guide observe/act/verify but do not enforce intent or defend against prompt injection. A trusted client receives arbitrary guest access, including network reachability under NAT. Authentication remains the local process/user boundary plus existing pinned guest SSH. A hosted transport would require separate identity, session authorization, network and viewer security; it is not exposed here.

Consequences: fast integration with real existing clients and protocol conformance maintained upstream; an additional dependency/license inventory; powerful guest access delegated to client policy; recent-frame expiry can require recapture when an agent is slow. Compatibility is claimed only where actual evidence exists. See [MCP contract/setup](../mcp.md) and [verification](../verification.md).
