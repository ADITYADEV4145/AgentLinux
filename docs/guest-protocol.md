# Phase 1 guest protocol

Implemented version: `1.0`. Guest binary version: `0.1.0`. This is internal guest communication, not MCP. Desktop/MCP and the broader Phase 0 API remain future work.

Transport: host-initiated SSH stdio, forced bridge to a mode-0600 guest Unix socket. Only `agent` owns that socket. The service runs under systemd at boot; SSH is not used to spawn arbitrary commands. There is no added guest TCP/HTTP listener.

UTF-8 newline-delimited JSON, at most 16 KiB **including newline**, 64 requests per connection, four concurrent connections, 15-second idle/read/write deadlines. Reject malformed UTF-8/JSON, duplicate object keys, unknown fields and oversized frames. Broken/oversized framing closes the connection. A partial frame at EOF does not execute. No debug output on stdout.

Request fields:

```json
{"version":"1.0","id":"00000000000000000000000000000001","machineId":"00000000000000000000000000000002","method":"hello"}
```

`id` and `machineId` are 32 hex characters; generated identities are random 128-bit values, not credentials. First successful hello returns guest/protocol versions, machine ID, service-session ID, Linux boot ID, OS/architecture, uptime, and allowed operations. Version must match exactly; unknown revisions fail `VERSION_MISMATCH`. Machine identity mismatch terminates the connection. Host also verifies response request/machine IDs, envelope, version, Linux ARM64 architecture and guest binary version.

After hello, `health` and `probe` require `sessionId`. `probe` additionally accepts a 1..128-byte nonce and returns its SHA-256 plus health information. The host verifies that digest as a small actual guest-side computation before readiness. This is not an authentication protocol or a security attestation. SHA-256 execution adds evidence beyond process startup; SSH host-key authentication supplies transport identity.

Responses contain version and matching id and **exactly one** of `result` or `error`. Errors have stable `code` and a generic actionable `message`. Implemented guest codes: INVALID_MESSAGE, INVALID_ARGUMENT, VERSION_MISMATCH, IDENTITY_MISMATCH, HANDSHAKE_REQUIRED, STALE_SESSION, UNSUPPORTED_OPERATION. The host also reports HOST_KEY_MISMATCH and deadline/interruption failures.

Health/probe are read-only, fixed operations. **No command execution endpoint exists**, so arbitrary commands, argv, environment, host/guest paths, stdin streams and executable requests are rejected. There is no command stdout/stderr/exit-code API to misrepresent. Provisioning is a separate trusted management script, not an agent request. Adding command execution later requires its own allowlist, process-group timeout/cancellation/output tests and a documented trust decision.

The client has an 8-second attempt deadline within the command's overall deadline, closes/kills its owned SSH subprocess group on cancellation, and does not reuse old connection framing. Connection loss closes the bridge/handler; fixed operations have no background child processes to abandon. Systemd restart generates a fresh random service-session ID; reboot also changes boot ID. Old session IDs fail. Machine ID and disk persist. Reconnect starts a fresh hello; no readiness is cached without checking the guest again. Protocol bounds, wrong versions, disconnects and idle timeout have unit tests; real boot/reboot and pin failure have hardware evidence in verification.md.
