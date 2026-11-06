# Local release candidate build, install and removal

Version: **0.5.0-rc.1**. Local artifacts only: no GitHub release/tag, package registry, public download, paid infrastructure or deployment is created by these scripts. This is an initial source/developer RC, not a production/security certification.

## Obtain the external runtime deliberately

Use [Lima's official v2.2.1 release](https://github.com/lima-vm/lima/releases/tag/v2.2.1), select `lima-2.2.1-Darwin-arm64.tar.gz` and follow its checksum/provenance guidance. Review the upstream publisher/tag trust rather than treating a same-channel checksum as identity proof. Extract the complete verified archive into a separate user-owned runtime directory, retaining its `bin`, `share` and `libexec` layout/notices. Do not copy only limactl; VZ/helper artifacts are required. Run `/absolute/runtime/bin/limactl --version` (must be 2.2.1), then pass that path with --lima. There is no need to start a global Lima machine: AgentLinux creates its own private namespace. No upstream runtime is installed/downloaded by our installer.

## Build from reviewed source

Prerequisites: Git, **Go 1.26.9**, Python **3.10+** (standard library only), macOS developer command-line tools for local development. First builds fetch pinned Go modules through Go's checksum verification; later builds can use a complete module cache. Runtime: Apple silicon Mac, separately installed **Lima 2.2.1**, OpenSSH/ssh-keygen, at least 30 GiB free for the default 24-GiB logical disk plus conversion/headroom, and enough RAM for the default 4-GiB guest (at most half host RAM). Bare clean macOS may not include Python/Go/Lima; install those prerequisites deliberately first. No Homebrew bootstrap, sudo or shell-profile modification is hidden in our installer.

```sh
git clone https://github.com/ADITYADEV4145/AgentLinux.git
cd AgentLinux
# Inspect/select the intended source commit before building.
make build test check
python3 scripts/package.py --output dist/rc-build
# Dirty worktrees are refused; --allow-dirty explicitly marks a private development bundle.
```

The package contains a macOS ARM64 CLI, static Linux ARM64 guest helper, README/docs, original Apache-2.0 LICENSE, pinned Go module metadata, third-party notices, installer, per-file SHA-256 manifest, and dependency inventory from both binaries (`DEPENDENCIES.txt`). Lima, Kitty, Go compiler, browser binaries, Debian QCOW2 and virtual disks are **not bundled**. Debian and guest applications are obtained/provisioned at first VM creation; their redistribution obligations are not sidestepped by pretending an image is included.

`-trimpath`, no build ID, disabled implicit VCS metadata, explicit commit metadata, normalized archive ordering/modes/uid/gid/mtime and gzip mtime make builds reproducible for the same source, Go version and build inputs. There are no wall-clock timestamps or absolute local paths in the manifest. Two builds should produce identical binaries and archive SHA-256; see verification.md for actual comparison. This is not a claim that apt-provisioned VM disk bytes reproduce bit-for-bit. Native Go builds on different host OS/toolchain versions are not guaranteed identical. `version --json` reports version/commit/guest/protocol/runtime; a dirty development manifest discloses that its source differs from the embedded base commit.

The SHA256SUMS file checks corruption of the archive, and manifest verification checks installed content. **A checksum obtained from the same compromised location does not authenticate the publisher.** There is no binary-fetching installer or authenticated remote binary release in this phase. The current credible path is to inspect/pin repository source and build locally. A future binary release must add a reviewed authenticity process (for example independently verifiable source/tag identity plus release attestations/signatures and pinned verification policy) before describing automatic binary downloads as trusted. Checksums alone do not establish that process.

## Install and upgrade without sudo

```sh
# Choose a new private tools prefix. Keep it separate from machine state.
python3 scripts/install.py \
  --bundle "$PWD/dist/rc-build/agent-linux-0.5.0-rc.1-darwin-arm64" \
  --prefix "$HOME/.local/share/agent-linux-tools"
"$HOME/.local/share/agent-linux-tools/bin/agent-linux" version --json
```

Use the full printed CLI path, or intentionally add that directory to PATH yourself. The installer does not modify PATH/shell configuration or system files. It validates every regular-file hash/size/path, rejects links/unexpected bundle entries, stages a complete version, verifies copied bytes, then atomically switches `current`. Existing versions remain for deliberate recovery. Failed verification/copy/activation leaves the previous `current` intact; an inactive complete version can remain after activation failure. Identical version+commit cannot be overwritten with different bytes. A private lock prevents concurrent installer changes. Tools uninstall does not remove the lock inode/ownership marker, so simultaneous callers cannot create competing lock files. A killed installer can leave a `.install-*` staging directory or `current.<random>` link; the prior `current` remains the activation authority. If recovery flags an unexpected entry, inspect and remove only that selected abandoned staging entry, then retry from the verified bundle. Do not recursively erase the prefix/parent or replace its ownership marker.

Upgrade by reviewing/building a new source commit and repeating install with the **same prefix**. Stop active tools/VM first for a predictable operator workflow. Existing VM disks/templates are not silently upgraded: `up` resumes the existing helper/recipe; version checks accept older supported guests and unsupported operations fail. Export wanted data, then use explicit reset or a new state directory to install the newer guest helper/recipe. Old version directories can be retained until you have verified the new tool; automatic garbage collection/rollback selection is deferred.

## Signing and Gatekeeper

No Developer ID signing or notarization credentials are used/configured by this workflow. Go's Darwin ARM64 toolchain may apply an **ad-hoc signature**, which is not an Apple Developer ID identity or notarization. The package is therefore publisher-unsigned and not notarized. No successful notarization/Gatekeeper-distribution claim is made. Local source-built tools worked on the tested Mac; quarantined downloaded artifacts and a fresh Mac's policy may behave differently. Do not disable Gatekeeper globally or run blanket quarantine removal commands. A future signed distribution needs the owner's signing identity, reviewed entitlements, notarization submission and ticket verification. See [Apple Developer ID](https://developer.apple.com/developer-id/) and [Apple notarization](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution).

## Persistence, reset, delete and uninstall

`stop` preserves the guest disk and identity. `up --local` resumes it without reinstalling apt packages. Reset reprovisions the pinned base/recipe, rotates machine identity/keys and waits for the guest handshake; package repositories can change, so “clean” means newly provisioned, not a byte-identical historic VM. Reset is not a snapshot/instant restore and is not promised to be fast.

```sh
agent-linux status --json
agent-linux stop
# Review the exact selected directory/data removal plan (no --yes = no deletion):
agent-linux delete --machine-id MACHINE_ID
agent-linux delete --machine-id MACHINE_ID --yes
# Or, instead of delete, after stopping the selected existing VM:
agent-linux reset --machine-id MACHINE_ID --yes
```

Use the printed machine ID, never a guessed/example ID, and the same explicit `--state-dir`/`--lima` if nondefault. Both commands require exact identity and a stopped VM. They refuse missing/unowned/unsafe paths, symlink ancestors, unexpected top-level state entries, another active controller, and a changed ID. The plan appears before any confirmed removal. Reset deletes all guest files/settings and cached image, then downloads/verifies/reprovisions anew. Review any custom image/helper before reset; reset inputs must be outside the deleted state directory. No forced termination of a running VM, adoption of unrelated disks, snapshot semantics or secure-erasure guarantee.

If interrupted, inspect `status` and backend logs. `deleting` is a real recoverable stage: use the original selected ID to finish explicit delete/reset; `up` refuses to resume during deletion. State is retained until runtime deletion is confirmed and known state entries are cleaned. A deletion leaves only the app marker/lock, not credentials or VM data. They are intentionally retained to preserve locking; do not recursively delete arbitrary parent directories. A killed download can leave a bounded `.download-*` file; inspect that private file and remove only that selected partial before retrying a destructive action if it is flagged as unexpected.

```sh
# Export anything wanted, stop the VM, then optionally delete it explicitly first.
python3 /path/to/reviewed/bundle/install.py \
  --prefix "$HOME/.local/share/agent-linux-tools" --uninstall
# Review plan, then repeat with --yes:
python3 /path/to/reviewed/bundle/install.py \
  --prefix "$HOME/.local/share/agent-linux-tools" --uninstall --yes
```

Uninstall removes only validated tool launchers/version directories. It leaves VM data, external Lima/Kitty, shell settings and installer marker/lock intact. It refuses unexpected files rather than deleting them. Removing a running executable does not stop its already-running process or VM. Stop explicitly first. No automatic host-project deletion or system-wide uninstall occurs.

Phase 8 build uses Go 1.26.9. `make` and packaging select that exact patch through Go toolchain support; if absent, Go can fetch it through its authenticated toolchain mechanism into the configured module cache. No system compiler or shell setting is changed. Public supporting `examples`, image recipe/provenance metadata, MCP instructions and cost-model/simulated-billing helper scripts now accompany the local bundle so the bundled billing/demo references are usable. The bundle is not a full Git checkout; build/development test scripts require the reviewed source repository. No OS disk or external runtime/compiler is included.
