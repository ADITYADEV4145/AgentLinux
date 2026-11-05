# Verification evidence

Phase 0 inspection: 2026-11-04. Phase 1 verification: 2026-11-05 (America/Chicago).

## Phase 1 results — 2026-11-05

**Actual local implementation and actual VM acceptance passed.** This section is current; the Phase 0 inspection below is historical. Desktop/MCP/viewer acceptance is still pending.

Host: Apple silicon arm64, macOS 26.5 build 25F71, 12 logical CPUs, 24 GiB RAM; hardware virtualization query returned 1. Free space was rechecked after cleanup and exceeded the conservative creation budget. Go 1.26.1; workspace-local Lima 2.2.1 from its official ARM64 release archive, checked against release SHA256SUMS. No global runtime install or privileged network helper was added.

Guest: Debian GNU/Linux 13 (trixie), genericcloud ARM64 build 20261001-2618, kernel `6.12.111+deb13-cloud-arm64`. Image SHA-512 matched the compiled pin and Debian's HTTPS manifest; no detached signature verification was performed. Service 0.1.0, protocol 1.0, 2 vCPUs/2 GiB RAM/16 GiB logical disk, plain VZ with NAT and no inherited host mounts. No desktop packages were installed.

### Automated checks actually run

- `make build`: host command and static Linux ARM64 helper built successfully.
- `make test`: `go test -race ./...` passed. Tests cover CLI parsing/errors; protocol framing, duplicate/unknown/malformed/oversized messages and versions; handshake/session association; fixed probe; arbitrary-operation rejection; disconnect and idle timeout; state transitions, interrupted startup and missing disk; unconfirmed shutdown; file locks/atomic state, symlinks/path traversal; resource budgets; effective config and QCOW2 safety; image mirror policy; provisioning-size regression; log redaction; subprocess-group deadline/output bounds.
- `make check`: `go vet ./...` and whitespace check passed. These are local checks, not a GitHub Actions run. Linux CI config was added but not executed remotely.
- Python acceptance script passed on a real Mac and compiled locally. The shell subprocess timeout test exercises a host-side test fixture only; **there is no guest shell-execution API** and no claim that guest arbitrary-command limits were implemented/tested.

### Real Mac observations

1. Downloaded/checksummed the selected Debian QCOW2, created a VZ disk with Lima, booted Linux and completed the actual strict-SSH hello and nonce SHA-256 probe. `ready` was emitted only after both checks. Service reported Linux/arm64, matching machine/session identity and protocol.
2. Inspected the test guest through trusted bootstrap administration: Debian/kernel as above, systemd service active, desktop/service account `agent` UID/GID 1001 with no supplementary groups. `findmnt` found no virtiofs, 9p or SSHFS mounts. Application channel's SSH key was restricted to the bridge.
3. Graceful `stop` returned only after runtime reported Stopped. Restart preserved the machine ID, changed Linux boot ID and service-session ID, and completed a fresh handshake/probe. A sentinel created inside the guest persisted across shutdown/restart. Initial sentinel read failed because the bootstrap admin lacked access to the agent-owned file; a privileged test-only read confirmed persistence. This was a test-permission issue, not a missing file.
4. Replaced the test known_hosts entry temporarily with a different public key: status failed with HOST_KEY_MISMATCH and did not report ready. Restored the correct entry, confirmed ready again, then stopped the VM.
5. Ran the checked-in `scripts/acceptance.py` against a fresh machine using the final helper and an existing verified image. It passed real up/status/stop/restart checks, stable machine/new boot/session checks, and removed its own test resources. Observed first readiness was **10.71 seconds with cached image** on this one run. This is not a cold-download benchmark or a performance guarantee.
6. Separately removed both earlier specifically identified stopped development instances and their generated host/control keys, image caches and private state after checking owner marker, machine IDs and runtime Stopped state. No global delete/prune was used. No VM created by this phase remains running. Workspace-local runtime/build caches remain ignored, with no guest credentials in tracked files.

### Failures that improved the implementation

The first network request followed Debian's official HTTPS mirror redirect, which the initial allowlist rejected. The allowlist now includes that documented mirror, while checksum pinning and HTTPS remain mandatory. An uncompressed embedded executable exceeded Lima's observed 4 MiB template input limit and was truncated; validation prevented boot. Compressed multiline provisioning with an explicit 3 MiB cap fixed it. Lima's ordinary JSON inventory hit `bufio.Scanner: token too long` on embedded config; a fixed selected-field format resolved it without returning config/key blobs. See ADR 0005. Failed attempts were not recorded as successful acceptance.

### Constraints and risks

One Mac/runtime/image combination was exercised. macOS 14 is a development target, not a tested minimum; Intel, other host versions and cloud were not tested. NAT host/LAN reachability remains a limitation, and no host/LAN network scan was performed. Vsock/offline networking, distributor signature validation, installed release/notarization, image redistribution compliance, physical-device access, desktop capture/input, Unicode UI behavior, MCP/client compatibility and viewers remain untested/unimplemented. A VM is a risk-reduction boundary, not a malicious-code security guarantee. The guest bootstrap admin and Lima runtime remain trusted, including upstream management SSH behavior. No screen recording/accessibility grants were requested, and no host desktop input API exists in this phase.

### Reproduce safely

```sh
make build test check
bin/agent-linux doctor --lima /absolute/path/to/limactl --json
python3 scripts/acceptance.py --cli ./bin/agent-linux --lima /absolute/path/to/limactl
```

The acceptance script creates a dedicated temporary root, uses explicit NAT, stops/restarts a real VM, and deletes only its own stopped machine. It does not use a personal default machine. It can reuse an existing copy of the pinned QCOW2 with `--image /absolute/path/image.qcow2`. On unconfirmed shutdown/deletion it preserves the state and reports its location. More extensive protocol failure tests use explicit in-process/backend doubles and do not constitute hardware evidence.

### Phase 1 exit verdict

Local creation/start/actual readiness/shutdown: met. Guest service/protocol: met. Relevant automated checks: passed. Real hardware validation: completed with the limits above. Paid resources, public deployments, package publication and billing: none. Staged review passed across 41 text files: no generated/private paths, binaries, actual private-key blocks, common credential-token patterns, private home paths/email or AI attribution trailers. These checks and content review are not a universal secret-detection guarantee. Phase-end commit follows that review; the final response gives the commit hash and clean/dirty status. Nothing was pushed unless separately reported. Next action: guest desktop/capture/input slice, not cloud or UI polish.

## Historical Phase 0 evidence

## What was actually done

The initially empty workspace was connected to the supplied GitHub repository before this phase. At Phase 0 start, `git status --short --branch` reported main with no commits and no tracked/untracked project files. No AGENTS.md or CLAUDE.md existed in the inspected workspace/ancestor locations; supplied user instructions remain authoritative.

Read-only local observations:

| Check | Observed result | What it does not prove |
| --- | --- | --- |
| `uname -m` | arm64 | Exact chip generation or virtualization availability |
| `sw_vers` | macOS 26.5, build 25F71 | Compatibility with any uninstalled VM runtime |
| `xcode-select -p` | Xcode application developer directory selected | Signed VZ app builds/runs successfully |
| `xcrun swift --version` (initial inspection) | Swift 6.2.1 | A Swift implementation was compiled |
| `go version` (initial inspection) | go1.26.1 darwin/arm64 | Go module/dependency build success |
| `git --version` (initial inspection) | 2.55.0 | Remote branches contain code |
| `df -h .` (repeated in Phase 0) | About 13 GiB available; data volume 97% used | Sufficient space for guest/image/conversion/provisioning |
| Tool lookup | Homebrew, Node/npm, Python, Go, Rust available | Any dependency has been installed for this project |
| Runtime lookup | Lima, Tart, QEMU, Docker absent from PATH in initial inspection; Lima still absent in Phase 0 | Apps cannot exist outside PATH |
| `command -v kitty` | No PATH entry | Kitty app is not installed elsewhere or terminal graphics are unsupported |
| CPU/memory sysctl attempt (initial inspection) | Sandbox denied query | Host capacity is known |

Research used TinyFish search and multi-URL fetch on official project docs, source/license files and release pages. [research.md](research.md) records links, retrieved facts and gaps. No provider account/API credential was accessed. No code, images, binaries, or installers were downloaded into this repository. TinyFish tools' batch_create was unavailable; no browser automation jobs were created.

## Phase 0 checks

Documentation validation checks local relative links, required deliverables, fenced-code balance, trailing whitespace and accidental sensitive-file patterns. Secret review checks staged content for private keys, common credential token formats, private email, user home paths and attribution trailers. These are limited checks plus content review, not a security certification or a comprehensive secret-scanning guarantee.

Results and final commit state are recorded below after execution. No application code exists, so there are no product builds, unit/contract tests, MCP integration tests or VM acceptance results in Phase 0. No mocks/fakes were run either. No paid resources, public deployment, package publication or billing activation occurred.

## First real Mac end-to-end experiment (planned, not run)

Prerequisites: Phase 1 implementation; validated runtime/image license and checksum; available memory and enough disk for the measured provisioning budget; read-only `doctor` pass. Install only the deliberately selected Lima release, without privileged networking helpers. Record Mac chip/CPU/RAM, macOS, Go/Lima versions, exact guest image URL/digest, packages, logical/physical disk sizes and network choice. Do not assume today's 13 GiB free suffices.

1. Create an isolated test root with `mktemp -d`; put per-machine runtime/state there. No user home/project mounts. Record every resource UUID/path created in a cleanup manifest. Use a small deterministic HTML form bundled with the test in the guest; no internet task or credentials needed after provisioning.
2. Render and inspect the effective Lima config: plain VZ ARM64, empty mounts, no proxy values/host certificates, no SSH agent/X11 forwarding, no static service forwarding, no inherited user metadata beyond a generic guest identity. Verify the checksum/signature chain, then boot/provision and measure stage durations and disk growth. Package repository access during provisioning is explicit.
3. Verify guest host key enrollment and strict application SSH pinning. Inspect guest mounts/environment, Xauthority, service UID, absence of sudo for desktop user and host-home/SSH-agent exposure. Test a deliberate host-key mismatch: application connection must fail. Only trusted test harness/admin inspection performs these checks; no agent host shell tool is added.
4. Start Xvfb/Xfce and guest Chromium with browser sandbox intact. Wait for separate display/helper readiness. Capture a PNG; decode with dimension/byte limits and inspect desktop content. Record capture metadata/latency and evidence outside the repository. Empty/error PNGs do not pass readiness.
5. Through CLI's application path, screenshot, click a deterministic field and enter the mixed Unicode test string from product.md. Submit via key chord, recapture and verify exact DOM/text with an in-guest test oracle. The oracle is test-only and cannot substitute for visual input or claim the screenshot proves the text value. Confirm changed pixels in the expected region and unchanged out-of-bounds behavior. Manually verify host pointer/clipboard before/after without logging their contents.
6. Send an invalid coordinate, unknown action, malformed/oversized reply fixture and expired session/frame reference. Expect typed errors; no host command or filesystem effect. Kill/restart the guest helper/display, interrupt a write acknowledgement and disconnect SSH; reject old generations, release inputs where possible, report unknown outcome and require observation instead of duplicate actions.
7. Write a sentinel **inside the guest** using the reviewed test fixture. Stop gracefully, confirm runtime stopped, start again, and verify sentinel persistence/new session identity. An old session input must fail. Inject stop timeout: report still-running/unknown rather than success, and require explicit force-stop.
8. Measure host/guest CPU/RSS, actual disk allocation, cold provision/warm ready time and screenshot/input latencies. Probe guest access only to owned host/LAN test listeners and document observed network behavior. Do not scan the user's network. Test offline mode separately only if runtime supports it; otherwise record unsupported.
9. Delete only the test machine's manifest-owned resources after checking UUID/paths and avoiding symlink traversal. Confirm no owned VM/process/tunnel remains. Retain redacted evidence deliberately outside source control, or delete only evidence this test created. Never run global Lima prune/delete commands. If cleanup fails, report paths/IDs and CLEANUP_PENDING; do not mark the experiment passed.

First acceptance: complete steps 1–9 three consecutive times on a recorded Mac configuration. Early bring-up may run the smaller boot → frame → action → frame → stop loop first; it does not satisfy the full gate. Failure artifacts and manual observations must accompany results.

## Later real tests (not run)

- Phase 2: real MCP client discovery/schema/negotiation, screenshot image + metadata, action errors, human ownership, persistence and broker crash reconciliation. Test doubles cover backend failures only and are labeled separately.
- Phase 3: real Kitty graphics/keyboard/mouse with exact version, Retina/resize mapping, bounded refresh/backpressure and terminal restoration; browser token/Origin/CSRF/expiry and takeover checks. Do not report 2 fps scheduling as 2 fps achieved.
- Phase 4: clean-Mac install/uninstall/recovery and measured compatibility/resource matrix; license/SBOM/distribution review and image updates. No hardware support matrix is established yet.
- Phase 5: provider contract, authentication/workspace isolation, stream authorization, usage reconciliation, hard TTL/idle cleanup and retained-storage deletion, only with authorization for paid resources.

## Exit-criteria assessment

- Concrete local first-slice implementation path: met at design level (Lima plain/VZ, Debian ARM64 X11, SSH helper, bounded PNG/control). Hardware/strict key provisioning compatibility remains an explicit Phase 1 gate, not an architectural choice left unspecified.
- Local/cloud boundary: specified; cloud is not implemented and no feature parity is claimed.
- Risks/assumptions/validation: documented, including network reachability, disk pressure, X11 trust, Unicode paste, protocol/image bounds and licenses.
- No paid resources/public deployment: met.
- Phase commit: required after documentation checks; no implementation work is included.

## Phase 0 final check record

Documentation check passed for all 10 Markdown files: required deliverables exist, relative links resolve, code fences balance, no trailing whitespace or private home-path/email patterns were found. Staged checks also passed across 11 allowed text files: git diff --cached --check, credential/private-key/token patterns, private paths/email patterns and attribution-trailer checks. These are bounded pattern checks, not proof that every possible secret format is detectable. No product tests were run. Commit attribution uses the repository owner’s GitHub handle and noreply address rather than a private email, with no AI attribution trailers. The completion report gives the commit hash and post-commit git status; this file cannot embed its own final commit hash without changing that hash.
