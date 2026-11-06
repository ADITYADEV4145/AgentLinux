# Contributor roadmap

Start with [CONTRIBUTING](../CONTRIBUTING.md), [architecture](architecture.md) and [current verification](verification.md). Prefer small fixes with evidence; preserve the common machine interface and honest capability/error handling.

| Priority | Useful work | Evidence of completion |
| --- | --- | --- |
| Local preview | Clean supported Mac/account walkthrough, additional Apple silicon/macOS versions | Recorded install/doctor/boot/agent/viewer/copy/stop/reset/delete/uninstall; unrelated data unchanged; exact versions/limitations |
| Reliability | Interrupted provisioning/download recovery, display restart/focus and concurrent input usability | Meaningful failure tests plus disposable real-VM reproduction; no blind replay or fake readiness |
| Accessibility | Keyboard-only web use, focus/labels/contrast, IME/shortcuts, terminal navigation | Actual client/terminal/browser matrix; preserve guest-only input and explicit ownership |
| Images | Review Debian pins/packages and guest updates, reproducible recipe changes | Integrity/provenance/license review, saved version inventory, real screenshot/input/browser/MCP/viewer and persistence tests |
| Packaging | Authentic distribution, signing/notarization and upgrade recovery | Approved identity/credentials, clean-Mac/quarantine tests, reproducible artifacts and notices; no automatic public release |
| Cloud prototype | Provider acceptance and enforceable network/cleanup tests | Separately authorized short pilot, exact resource/template/region, no remaining resources, commercial permission and cost observations |
| Billing foundation | Actual Stripe sandbox acceptance, transactional store/queue, reconciliation/recovery | Real test-mode observations with zero live objects/charges; preserved immutable history, ownership/caps and fault tests |
| Operations | Private reporting, monitoring/runbooks, retention, support/legal review | Named owners and exercised controls rather than a checklist marked complete by assumption |

Do not present a new provider/UI as working without evidence, copy an external service's marketing claims, add live billing activation, or introduce a host-input fallback. Follow the [launch sequence and authorization list](launch-readiness.md) before any publication/deployment action.
