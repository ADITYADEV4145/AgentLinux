# Contributing

AgentLinux is an early local developer release candidate. Original code is Apache-2.0; contributions use that license. Discuss substantial interface/security/runtime changes before expanding scope. Keep cloud proposals separate from functioning local capabilities.

Use Go 1.26.1 and Python 3.10+; `make build test check` builds the host/static ARM64 guest helper, runs race-enabled Go and installer tests, vet and whitespace checks. Linux CI verifies logic/cross-compilation; it cannot boot Apple VZ. macOS hardware checks are opt-in and must name an isolated disposable state directory. Tests/mocks are not hardware acceptance.

Read docs/security.md, docs/architecture.md, docs/guest-protocol.md and docs/release.md. Add meaningful failure/boundary tests for behavior changes, preserve backward capability/error handling, keep stdout clean for MCP, and never silently fall back to host input/shell/clipboard or a fake cloud backend. Review generated/effective runtime configuration, not just marketing claims about isolation.

Do not commit credentials, private state/keys, VM images, tool downloads, screenshots of personal data or raw acceptance logs. Private evidence belongs in ignored artifacts. Record sanitized observed versions/results and skipped checks in docs/verification.md. Dependency changes need exact license notices; no guest-image redistribution without a component/source-obligation review. Commit using your own authorized identity; this repository does not add AI authors/co-authors.

A useful pull request explains the concrete problem, resulting behavior, validation and material limits. No release upload, tag, package registry publication, paid infrastructure or external deployment is implied by a build. See SECURITY.md for private reporting guidance.
