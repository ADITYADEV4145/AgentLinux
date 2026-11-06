# 0014: atomic sandbox usage ledger and manual Stripe invoices

Accepted for Phase 7 development, 2026-11-05.

Use the official Stripe Go SDK v86.4.2 with test keys only. Store usage/reservations, immutable rate references, final records/adjustments, payment intents and minimal webhook receipts atomically with session lifecycle under the existing exclusive server lock. Local use remains free; cloud pricing/trials require explicit reviewed configuration. Meter first confirmed desktop readiness through confirmed termination, round seconds once, reserve prospective net exposure and carry sub-cent totals forward.

Use Checkout setup mode and explicit isolated usage invoices, not a subscription or unvalidated Stripe meter product. No live switch, automatic collection, charge/send operation or raw card handling. Current-object reconciliation handles unordered webhook delivery; durable intents use stable idempotency keys and hold old ambiguous jobs for review. This is simpler to audit during the prototype, but manual invoice operations, single-process JSON limits and pending sandbox-account acceptance prevent a production billing claim.

Clock uncertainty produces non-invoiceable review records, and local metering failures close admission while owned cleanup is still attempted. Caps limit product admission rather than guaranteeing provider spending. Production requires transactional storage/queue, provider commercial authorization, operational reconciliation, retention/recovery controls and separately authorized activation. See [full policy, workflows and missing acceptance](../billing.md).
