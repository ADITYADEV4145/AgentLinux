# ADR 0015 — Stopped disk checkpoints and journaled recovery

Accepted for local hardening, 2026-11-14.

Pinned Lima 2.2.1's VZ snapshot create/apply/delete/list methods return
`errUnimplemented` ([official implementation](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/driver/vz/vz_driver_darwin.go#L603)).
Use application-managed copies of the verified raw writable disk and EFI state,
plus corresponding sealed configuration/state metadata, while runtime confirms
Stopped. Do not market these as memory snapshots or machine clones.

APFS copy-on-write clones reduce initial copying; integrity checks remain proportional
to logical data size. A portable sparse fallback is bounded and conservatively checks
space. Restrict restore to the original identity/VZ instance and compatible hardware/
image contract. Limit to three private checkpoints; deletion is explicit.

Prepare rollback and target copies before any swap. Sync a journal before installation;
resume atomically swapped files or explicitly roll back after interruption. Keep
rollback data until a fresh authenticated readiness handshake, with a separate durable
receipt for recoverable cleanup. No implicit stop/restart/restore/reset is permitted.

Helper upgrade is a separate checkpoint-backed explicit action. Only replace its
embedded binary payload, keep original/effective provisioning/seal coherent, and verify
the installed digest after an explicit boot. Unsupported templates/containers require
an explicit correction, not an automatic reset. Expose administrative operations only
through CLI and the authenticated local dashboard, not autonomous MCP tools.
