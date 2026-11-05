# ADR 0004: Permissive core, provider-backed cloud later

Status: recommendation; license adoption and provider selection are deferred.

Recommend Apache-2.0 for original CLI, broker, guest helper, schemas, image recipes, and viewers. Its patent grant and notice requirements suit a reusable infrastructure core. MIT is simpler but lacks the same explicit patent grant. Neither would require publishing a future hosted control plane.

Prefer evaluating a managed sandbox provider before operating a VM fleet. E2B and Daytona expose relevant desktop APIs, but their image/architecture, authorization, persistence, licensing, region, and cleanup semantics must pass our contract tests before selection. Do not equate public SDK source with an open-source hosted service.

Tart is not the default: current source uses FSL-1.1-ALv2 with a competing-use restriction; older official documentation describes Fair Source. No Tart code is incorporated. Upstream licenses must be rechecked at the exact revision used. Guest distro packages keep their individual licenses; Apache-2.0 on our code does not relicense the image.

Cost: potential provider limitations and lock-in; keep provider IDs private and feature differences explicit. Billing, public deployment, paid resources, and publishing require separate authorization.

Evidence: [dependency review](../architecture.md#dependencies-and-licensing), [sources R14–R18](../research.md).
