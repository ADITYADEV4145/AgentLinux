# ADR 0010: Local developer RC with explicit copies and identity-guarded deletion

Date: 2026-11-05. Status: implemented; no public release authorized.

Keep the VZ/Lima/X11/Go stack and explicit NAT limitation. Offer no host mounts or claimed network-isolation mode. Transfer one bounded regular file over the existing pinned application channel; archives stay opaque. This gives developers a project/result path without a live host filesystem connection, at the cost of a 4-MiB file limit and manual archive preparation/extraction.

Preserve disks on stop/up. Reset/delete require the stopped machine's exact ID plus explicit --yes after a removal plan. Hold the same lifecycle lock through confirmation/recheck/delete/reset; keep the lock inode/marker and a recoverable deleting stage. Reset reprovisions and rotates identity, so it is slower and less reproducible than a snapshot; no snapshot is claimed. Unknown top-level state data is refused.

Build an unpublished source/developer RC: deterministic host/helper/docs/notices bundles, no bundled runtime/browser/image, offline installer with stage/verify/atomic current activation and separate tools-only uninstall. No binary fetcher, publisher signing/notarization credentials or checksum-as-authenticity claim. Original code stays Apache-2.0; inventory and exact upstream notices travel with the bundle. Clean-Mac/Gatekeeper acceptance, a maintained private-report channel and broader distribution/source-obligation review remain explicit limits before wider distribution.

See [release](../release.md), [isolation audit](../security.md) and [verification](../verification.md).
