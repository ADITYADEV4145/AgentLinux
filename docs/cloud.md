# Phase 6: development cloud backend

Status (2026-11-05): implemented development control plane and HTTP machine client; default provider is **SIMULATED**, with no Linux VM. Optional E2B SDK adapter is implemented but **not provider-tested**. No account, paid machine, public listener, deployment, billing product or release was created. Phase 7 now adds an opt-in sandbox metering/payment foundation; see [billing](billing.md). Live billing remains disabled.

## Recommendation and prior art

Use E2B Desktop only for a private, explicitly authorized prototype, subject to account permissions. Keep the provider interface so a VM-per-session provider can replace it. **Hosted commercial use is blocked pending provider approval:** [E2B terms](https://e2b.dev/terms) restrict resale/providing access and competing services. An MIT SDK does not grant permission to resell the managed service. Obtain written clarification before offering this product to customers. This is an identified dependency, not legal clearance.

Official sources inspected on 2026-11-05; prices and account limits can change:

| Approach | Verified capabilities and costs | Costs/risks we would own |
| --- | --- | --- |
| E2B Desktop | [Maintained Python implementation](https://github.com/e2b-dev/E2B/tree/main/packages/desktop-python), SDK 2.7.2, Xvfb/Xfce, screenshots and input. [Security documentation](https://docs.e2b.dev/security) describes Firecracker microVM isolation; this is a provider claim, not independently audited here. [Pricing](https://e2b.dev/pricing) lists Hobby 20 concurrent / 1-hour sessions and Pro $150/month plus usage, 100 concurrent / 24-hour sessions; published 2-vCPU CPU component $0.000028/sec. These are not all-in desktop costs or our retail prices. | Commercial authorization, region/account quotas, exact template OS/architecture/apps, dependency review, memory/storage/egress terms, actual latency and abuse controls still require validation. Provider image is not our local ARM64 Debian workstation. |
| Own EC2 VM per session | [Lifecycle](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-lifecycle.html) and [on-demand pricing](https://aws.amazon.com/ec2/pricing/on-demand/) define running compute billing, generally per second with 60-second minimum for applicable Linux instances. Instance type/region, EBS, addresses and egress are additional dimensions. Stopped compute does not mean all resources are free. | Custom desktop image, guest service, authenticated channel, scheduler, patched hosts/images, quotas, launch latency, EBS/ENI/orphan cleanup and incident response. More control and closer workstation parity; substantially more operations. No AWS implementation or resources exist in this phase. |
| Daytona alternative | [Computer-use documentation](https://www.daytona.io/docs/en/computer-use/) describes Xvfb/Xfce/VNC/noVNC automation. [Pricing](https://www.daytona.io/pricing) publishes resource rates and trial terms. | Independent isolation, service terms, location, quotas, cleanup and workload validation are pending. Not integrated or tested. |

Do not extrapolate measured local performance to any provider. A future single-region deployment would place a TLS identity gateway, API/worker and durable transactional database in one selected region, create one labeled sandbox per session, and proxy authenticated frames through that gateway. Production identity/OIDC, TLS ingress, distributed locking, HA database, provider region verification and egress measurements are **not implemented**. Development JSON state and one process are intentional prototype limits.

## Run the free development fixture

Build with `make build`. Use a fresh private directory outside a repository; credentials must never be committed. The following fixed request ID is a public example identifier, not a credential. `cloud init` creates random expiring credentials, not a password or public admin account.

```sh
mkdir -m 700 /private/tmp/agent-linux-cloud-demo
agent-linux cloud init --dev --state-dir /private/tmp/agent-linux-cloud-demo
agent-linux cloud serve --dev --state-dir /private/tmp/agent-linux-cloud-demo
# A second terminal, same Mac:
agent-linux cloud up --config /private/tmp/agent-linux-cloud-demo/alice.json \
  --request-id 0123456789abcdef0123456789abcdef --seconds 300 \
  --output /private/tmp/agent-linux-cloud-demo/session.json
agent-linux cloud status --config /private/tmp/agent-linux-cloud-demo/session.json
agent-linux mcp --cloud-config /private/tmp/agent-linux-cloud-demo/session.json
agent-linux view --interactive --cloud-config /private/tmp/agent-linux-cloud-demo/session.json
agent-linux cloud stop --config /private/tmp/agent-linux-cloud-demo/session.json
```

The viewer prominently labels the fixture **SIMULATED**. It exercises the real API/auth/client/viewer wiring, not cloud Linux. Add `--view` to `cloud up` to create, wait and open the interactive viewer in one command. `start --cloud-config FILE` opens an existing session and never silently allocates paid compute. Alice and Bob have separate workspaces. `cloud revoke --config FILE` invalidates that identity and queues termination of its machines. Interrupting `cloud serve` preserves cleanup intents; restart with the same namespace/provider to reconcile. Fake resources are in-memory and disappear across server restarts; this is deliberately unlike provider persistence.

Only literal `127.0.0.1` can be bound. Public deployment is refused; this does not support remote customer access. Client supports HTTPS for future deployments, disallows redirects/proxy inheritance/URL credentials, and permits HTTP only with explicit development config on literal loopback. Client credentials are private 0600 files in private, owned directories with symlink checks; never put them into MCP arguments as raw tokens. `mcp-config --cloud-config FILE --client codex` prints a path-based snippet without modifying client settings.

## Contract, ownership and lifecycle

`machine.Machine` is shared by local and remote implementations: status/capabilities, screenshot, input, exec and app open. Capabilities determine available MCP tools. E2B currently supports screenshot and qualified input only; **exec, app-open, files and persistence are unsupported** and never silently routed to the Mac or a local VM. A user controlling a guest terminal can still run arbitrary guest commands. GUI access is powerful even when the dedicated exec tool is absent.

The control plane authenticates every request, binds session to identity/workspace, and returns not-found for another user's ID on every session route, including unsupported routes. Version `cloud-prototype-1` must match exactly. Session IDs and creation idempotency keys are opaque 32-hex identifiers. Stable keys deduplicate creation; conflicting parameters fail. State progresses `creating → starting → ready → stopping → stopped`; readiness requires provider desktop/component checks, decoded frame and generation. `ready` is not inferred from an allocated process. Failure diagnostics remain attached to pending/cleanup state rather than pretending termination succeeded.

Persist intent before provider creation. Label resources by deployment/session, never blindly repeat an ambiguous creation, recover labeled resources after a lost reply, remove duplicates and unknown resources only within this deployment, and confirm absence before final stopped state. Incomplete/truncated inventory fails reconciliation. Active requests use per-session exclusion. Revocation is checked before and after calls; a mutation already dispatched cannot be undone, but its result is withheld after revocation. Deletion waits for an in-flight provider action. The provider API may still be unreachable; reservations and stopping state remain until reconciliation confirms absence.

Default limits: 2 sessions globally, 1 per identity, 900-second maximum, 300-second idle expiry, cumulative 1,800-second reserved-runtime budget. Configuration is operator-owned JSON and validated within prototype maxima (4 sessions, 900-second lifetime, 14,400-second budget). Reserve full requested lifetime before allocation; terminal observations conservatively account for failed/provisioning time. Status exposes expiry, idle deadline, estimated runtime and limits. Screenshots/status do not keep a session alive; actions do. These are resource guards, **not a billing ledger or guaranteed spending cap**. Provider billing lag, control-plane outages and external resources can create charges beyond these estimates. No live prices, payment integration or customer trial are activated by Phase 6. Phase 7 adds separately configured sandbox-only metering and payment operations.

## Viewing and human control

No public VNC endpoint or provider URL is returned. Authenticated bounded PNG requests flow through the API into the existing viewer hub. One latest frame, bounded cached frame references, 8-MiB PNG / 16-MiB response limits, request deadlines, per-user/global concurrency and rate limits bound work. Capture starts only when requested by a viewer/agent; the worker's readiness check also captures a frame. Polling is intentionally simple; no video, codec or latency claims. Desktop generation and frame identity prevent old screenshot actions after restart. Reconnect repeats observations, never uncertain mutations.

Human input uses a 15-second renewable lease; F2 gives control to agents or takes it back. Stop, expiry and generation changes revoke ownership. Local leases use the existing cross-process lock and private lease file; cloud leases are server memory and disappear on restart. Agents receive `INPUT_OWNED` while a human owns input. This coordinates cooperating frontends; it does not isolate guest X11 applications or defend against another malicious same-UID host process. Text uses the guest clipboard only. No host pointer/clipboard or filesystem mount is needed.

## Optional E2B adapter: gated and unverified

Do not run this as part of ordinary local setup. No SDK is installed automatically. The operator must separately authorize potential cost, review provider terms and template, install Python 3.11+ with **e2b-desktop==2.7.2**, and provide server-only `E2B_API_KEY`. The template must be explicitly selected and reviewed for 2–4 CPUs, 4–8 GiB RAM and root disk no larger than 40 GiB; there is no implicit generic desktop template. Resource violations queue termination after probe; they cannot prevent the initial provider allocation cost. SDK source [pyproject](https://github.com/e2b-dev/E2B/blob/main/packages/desktop-python/pyproject.toml) declares MIT and dependencies; complete transitive locking/license review is pending. No credential is included in this repository.

Initialize a fresh namespace with `--provider e2b-desktop-2.7.2`, then serve with the same provider and explicit `--allow-provider-create --python /absolute/reviewed/venv/bin/python --template REVIEWED_TEMPLATE`. Without opt-in, no E2B subprocess/API call runs. Keys are passed only to the short-lived SDK worker environment; errors never echo SDK stderr or payloads. The worker invokes fixed guest commands, not host shell input. SDK create uses session idempotency key, labels, 1280×800 resolution, bounded remaining TTL, kill-on-timeout and no auto-resume. It checks running before connect, refuses oversized resource profiles, and confirms deletion through provider absence.

Network configuration requests no unauthenticated public ingress and denies private, loopback and link-local destinations using [E2B network options](https://docs.e2b.dev/sandbox/network). Internet/browser access remains enabled. These settings are **not tested enforcement guarantees**; verify DNS rebinding, metadata IPv4/IPv6, host/LAN-equivalent destinations and ingress before untrusted users. For an EC2 alternative, omit instance IAM roles and disable the metadata endpoint; [IMDSv2](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-service.html) alone does not prevent guest code obtaining metadata. Never expose provider credentials or host mounts to a guest.

## Provider acceptance still required

1. Obtain written commercial permission as applicable; select/record region, quotas, template digest, OS/architecture, exact apps, prices and egress policy.
2. Lock/review SDK dependencies, test actual network/metadata restrictions and inaccessible raw desktop endpoints.
3. On separately authorized credentials, create one short-lived labeled session, verify screenshot→Unicode/key/mouse→changed screenshot through CLI/MCP/viewer, and record provider versions and measured startup/stream resources.
4. Exercise cross-user access, token revocation, provider outage, lost create reply, desktop restart, idle/TTL expiration, API/server restart, confirmed kill, and inventory orphan recovery.
5. Confirm no owned resources remain in the provider console; reconcile provider charges. Stop is terminal and discards guest data. There is no snapshot, cloud home, local/cloud disk migration or recovery promise.

Audit records contain session identity/state/timestamps/outcome, with bounded local rotation; no screenshots, command arguments, input text or tokens are logged. Back up control-plane state securely: losing the deployment ID/labels and credentials impairs automatic cleanup. Provider TTL is a requested independent backstop, not a verified guarantee. Retained metadata is bounded to 1,000 sessions; no account-deletion or production retention service is implemented.
