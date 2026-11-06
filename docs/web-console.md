# Local website and development cloud console

The original responsive landing page and console are embedded in the Go binary. No Node build, remote fonts, analytics, registration or public deployment is needed.

```sh
agent-linux dashboard
```

Open the printed private localhost URL, then **Open console**. With no config, the console displays a helpful empty state and never invents machines or usage. The website's terminal desktop is explicitly labeled an **illustration**, not a VM screenshot. Local Linux still starts through `agent-linux` (interactive TTY) or `agent-linux start`; the browser console's machine fleet is cloud-only in this version.

For a fresh simulated, disposable development deployment (use an actual unused `/private/tmp` path):

```sh
agent-linux cloud init --dev --state-dir /private/tmp/agentlinux-cloud-review --endpoint http://127.0.0.1:8787 --max-per-owner 2
agent-linux cloud serve --dev --state-dir /private/tmp/agentlinux-cloud-review --listen 127.0.0.1:8787
# Another terminal:
agent-linux dashboard --cloud-config /private/tmp/agentlinux-cloud-review/alice.json
# Non-browser account inventory:
agent-linux cloud list --config /private/tmp/agentlinux-cloud-review/alice.json
```

Init changes only a new deployment. Default account concurrency stays one; `--max-per-owner 2` opts into two, within the existing two-global-session limit. A configured billing policy can impose a lower limit. Credentials expire in eight hours. Neither command enables a real provider or live billing.

The console can create bounded desktop sessions, list this identity/workspace's records, inspect runtime/expiry/ledger/limits, open each ready session's live viewer, and explicitly stop the selected guest. Session resources remain provider-controlled. The fake provider is always labeled **SIMULATED / NO LINUX VM**, including its PNG. Real provider acceptance, commercial permission, hosted identity, public TLS, distributed database and live payments remain outstanding. Builder is a pricing proposal, not a create option.

Opening a viewer starts a separate secured loopback web viewer and reuses guest capture/input coordinates and lease handling. Humans choose **Take control**, then release back to agents; no host pointer injection or host clipboard reading is used by guest control. Closing the console closes its viewers and releases human leases. It does **not** stop sessions: use Stop or let the bounded expiry/idle cleanup run. Stopping discards cloud guest files. Viewer slots are bounded to four per console process; stop a session or restart the console to free retained slots. No capture work runs without frame requests.

Security: literal 127.0.0.1 listeners only; exact Host and same-origin checks; no URL query credentials; CSP; no-store responses; bounded body, request concurrency and timeouts. Cloud config is read through the existing 0600 owner/symlink checks. Its token remains server-side. A fresh per-console random capability travels in a fragment, is removed from the address bar, and is sent as an Authorization header. It is intentionally memory-only: reopen the printed URL after reload/bookmark navigation. Sharing it grants this console's account actions until shutdown. Do not publish the private link or expose these development listeners through tunnels. Viewer credentials are independently generated. This is not public web authentication.

Create uses a stable per-intent idempotency key in memory and does not auto-retry mutations. An ambiguous transport error means refresh/reconcile before retrying; retrying the same tab/intent retains the key. Account inventory never refreshes idle lifetime. Browser polling pauses when hidden. Billing numbers come from the backend, separate from the owner-selected display price catalog. No raw card form or active paid purchase button is supplied.

Validation and remaining work are recorded in `docs/product-polish-verification.md`. Public deployment, real paid compute and live billing require separate authorization plus the existing launch gates.
