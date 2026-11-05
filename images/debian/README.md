# Guest provisioning (current recipe: Phase 3.5)

The selected image is Debian 13 genericcloud ARM64 build 20261001-2618. manifest.json records provenance and SHA-512; the same pin is compiled into internal/config/config.go. The checksum was cross-checked against Debian's HTTPS SHA512SUMS and Lima's pinned template. The downloader permits only HTTPS redirects to the documented Debian/ACC mirror hosts, verifies the full digest before use, and has no daily/latest fallback. This verifies integrity against a reviewed pin; **detached Debian image signatures are not automatically verified**. The Phase 0 aspiration to verify distributor signatures remains a release-hardening task; no independent signer-authenticity claim is made.

No image is built or redistributed by this repository. Debian/Linux/packages have aggregate licenses, including GPL source obligations when redistributed. Obtain packages/source and license notices from Debian before publishing images. See third_party/README.md.

## Reproduce

1. Build the host and static Linux ARM64 helper with `make build` (Go 1.26.1).
2. Install/check Lima 2.2.1 according to its upstream instructions. The project neither runs a curl-to-shell installer nor modifies global settings.
3. Run `bin/agent-linux doctor --lima /absolute/path/to/limactl`.
4. Run `bin/agent-linux up --local --network=nat --lima /absolute/path/to/limactl`. The initial default is 2 vCPUs, 4 GiB RAM, 24 GiB logical disk. This is an experiment configuration, not a supported minimum. A conservative new-machine check reserves logical disk capacity plus 6 GiB; restart requires 2 GiB free. CPU/RAM limits are also checked.

A developer can provide an absolute standalone QCOW2 cloud image with an explicitly trusted checksum:

```sh
bin/agent-linux up --local --network=nat \
  --image=/absolute/path/guest.qcow2 --image-sha512=<128-lowercase-hex-digits> \
  --guest-binary=/absolute/path/agent-linux-guest
```

The image must have ARM64 Linux, systemd, cloud-init, OpenSSH, sudo, useradd/usermod, gzip/base64, and a usable cloud-init root provisioning path. First boot installs the reviewed desktop package set from signed Debian apt repositories; it needs network access. Local image architecture is asserted by the developer until actual handshake confirms linux/arm64. The host checks QCOW2 header size/features before Lima conversion and rejects backing files, external-data/incompatible flags, encryption, and virtual sizes above the configured disk. This header check is not a complete filesystem/image safety audit.

## Installation and channel

Code in internal/lima/template.go renders the complete YAML into a private per-machine state directory, with no inherited templates. The static helper is gzip-compressed/base64-wrapped inside the root provisioning script; a 3 MiB template cap prevents Lima's observed 4 MiB input truncation. The script installs a root-owned executable, creates a separate `agent` account without sudo/group privileges, sets its password to an unusable value, and installs a restricted forced SSH command. It does not hardcode a password or private key.

A systemd service runs the helper as `agent`, starts at boot and restarts on exit/crash. It exposes only `/run/agent-linux/control.sock`, mode 0600 inside a mode-0700 RuntimeDirectory. The SSH forced command bridges stdio to that socket. The helper retains NoNewPrivileges/ProtectSystem, delegates command cgroups and allows guest home/tmp writes. Home/private-tmp/AF_UNIX-only restrictions from Phase 1 were changed because they prevented real desktop apps and browser networking. A separate unit manages Xvfb/Xfce and its fresh Xauthority/session generation. No desktop network listener is added; Xvfb is Unix-socket/Xauthority-only.

Per-machine control and guest host keys are generated locally. Root provisioning installs that guest host key; application SSH uses strict host-key checking with a private known_hosts and stable machine alias. Application SSH has no ambient SSH configuration, agent forwarding, X11 forwarding, PTY or reverse forwards. Lima's own trusted bootstrap management SSH still follows upstream defaults, which are weaker than our application pinning. Bootstrap admin access is not an agent tool.

Phase 2 retains Lima's loopback SSH TCP endpoint (`overVsock: false`). The guest port is reached through Lima's private user-mode network; the host forwarding socket must remain loopback-only. No non-loopback control endpoint is accepted. A raw vsock/SDK integration is deferred; do not claim this channel works without IP networking. NAT can reach host/LAN services and is not egress isolation. Host proxy propagation, mounts, host clipboard sharing, devices, containerd and unrelated port forwards are disabled.

The generated template and Lima cidata include a private **guest** host key. They remain in owner-only host state, never logs or Git. Retained provisioning data is necessary for restarts; deletion of an explicitly selected machine's state must include those copies. No host private credential is installed into the guest. Runtime files are not artifacts to share.

## Updates and interrupted runs

To update an image, select a dated Debian build, review the checksum/trust source and package licenses, update both manifest and compiled pin, then run isolated boot/probe/stop/restart tests. Record kernel/package changes and commit. Never replace a disk or switch to latest automatically.

`up` is create-or-resume. A file lock covers lifecycle mutations and status reconciliation. Interrupted pre-creation retains the machine UUID and retries configuration; an existing runtime instance is validated and reused. Missing previously created disks are an error, not a recreation request. Resource/image flags are creation-only and are rejected on an existing machine. `stop` preserves its disk. No deletion/force-stop CLI exists yet; for disposable development cleanup, use the explicit owned LIMA_HOME procedure in scripts/acceptance.py. Do not run global Lima prune/delete.

The code and recipe are repeatable, but resulting disk bytes, SSH keys, boot IDs and timestamps intentionally differ per machine. This is not a bit-reproducible image build. Phase 2 installs Xvfb, Xfce components, Mousepad, Chromium/sandbox, scrot, xdotool, xclip, X11/auth/D-Bus tools and fonts. The exact installed package list is saved in /var/lib/agent-linux/packages.tsv. The desktop install marker avoids repeating apt on every boot; security updates require explicit package/image maintenance, not an assumption that restarting updates packages. MCP is implemented; Phase 4 adds a verified live local web fallback and Kitty 0.50.0 rendering with real visual/MCP/resize/cleanup acceptance on macOS 26.5.


Phase 2 uses protocol 2.0/service 0.2.0 and does not automatically upgrade a Phase 1 disk. Use a fresh --state-dir; no existing machine is reset or overwritten. The package install is repeatable from the recipe but not byte-reproducible: the Debian security repository can change. Record package versions during acceptance and review image/recipe updates deliberately. The X11/Wayland and guest-exec trust limitations are in docs/guest-protocol.md.

## Phase 3.5: practical desktop, existing machines and updates

New machines retain the dated Debian 13 ARM64 base above and apply recipe `desktop-3.5.0` from `internal/software/catalog.go` through the sealed provisioning template. The recipe adds Firefox ESR, App Finder/menu support, guest home folders, desktop icons/utilities, GVFS/thumbnail/accessibility services and the small development baseline. Existing Thunar/settings/terminal/editor/Chromium packages are reused. Helper 0.3.5 still uses guest protocol 2.0; the new host also accepts legacy 0.2.0 helpers. Old helper capabilities and available apps do not become new merely by replacing the host CLI.

Distinguish three layers:

| Layer/action | Actual behavior |
| --- | --- |
| Dated base download | Reviewed Debian genericcloud build/digest; only used to create a new machine |
| New-machine recipe | Installs from signed Debian 13 repositories at provisioning time; exact package versions recorded, not a frozen apt snapshot |
| Existing persistent machine | Has its own disk/packages/data; `up` reuses its sealed template and does not rewrite it with today's recipe |
| `packages refresh` | Explicitly refreshes guest apt indexes; installs no upgrades |
| `packages install NAME` | Refreshes indexes and installs named supported packages/dependencies into the selected existing guest |
| `packages upgrade` | Refreshes indexes and applies ordinary apt-get upgrade, preserving modified config files; no dist-upgrade/removal policy or unattended-update claim |
| `stop` then `up` | Preserves disk, user files, installed packages and browser profile directories; new boot/session/display identities; running processes and unsaved memory are lost |
| Desktop/helper restart | Files persist; app processes tied to a restarted service may close; clipboard selections can disappear; old frame IDs expire |
| Reset/ephemeral | No reset command, snapshot or ephemeral mode implemented; no rollback guarantee |
| Deletion | Not exposed as a product CLI yet; explicit developer removal of an owned VM/state permanently loses its disk/data/keys |

The root-owned `/var/lib/agent-linux/recipe` and package inventory identify provisioning/maintenance state. Kernel and live point release can be read with guest `uname -r` and `/etc/os-release`; do not confuse those with base image build date. `/var/lib/agent-linux/last-maintenance` records successful explicit maintenance. Some kernel/security updates need a stop/up reboot; browser updates need reopening. No restart silently refreshes apt or promises lifelong security updates.

The `agent` desktop/MCP user still has no sudo. The explicit host `packages` CLI uses Lima's existing guest administrator to run a root-owned helper in a bounded guest systemd job. It installs nothing on the Mac and exposes no privileged network listener. See [privilege/failure model](../../docs/development.md). Network/signature errors fail refresh rather than silently reusing stale indexes. Current product mode requires NAT; if connectivity is blocked, maintenance is unavailable until restored. An implemented `--network=off` mode is still future work; do not imply the control channel has been validated offline.

### Preserving and extending a Phase 2/3 machine

Old sealed machines are preserved and continue using their installed helper/recipe; they are not automatically migrated. Their old helper lacks new fixed-app targets and the new maintenance executable. Prefer creating a separate Phase 3.5 namespace and explicitly moving desired documents/projects through user-managed guest networking. Never delete the old state directory to “upgrade.” There is no automatic guest-to-guest disk/application migration.

For a user who intentionally wants to administer an existing guest, Lima's management route remains available. This grants guest root and uses the trusted bootstrap channel; review the selected namespace and do not run a global delete/prune. No new host mount, SSH agent or environment forwarding is needed:

```sh
# STATE must be the selected existing private machine namespace; LIMA is absolute limactl.
# No --preserve-env, --sync or host directory share.
LIMA_HOME="$STATE/lima" "$LIMA" shell --workdir=/ default sudo -n apt-get update
LIMA_HOME="$STATE/lima" "$LIMA" shell --workdir=/ default sudo -n apt-get install firefox-esr git curl python3-venv
```

These commands change packages in that guest only; they do **not** install a new AgentLinux helper or new host CLI launch capabilities. Launch added programs from the guest terminal/menu and verify them, or use the fresh-namespace path for the complete new recipe. Package and helper migrations require an explicit reviewed future migration mechanism. No reusable password/key or unattended remote admin listener is introduced.

Distribution remains separate from local development: no VM image is published/bundled here. Both browser choices require their full component notices and applicable corresponding source on image redistribution; GVFS/tumbler/Xfce and new development packages add package-specific licenses. See third_party/README.md and `/usr/share/doc/PACKAGE/copyright` inside the guest before distributing a bundle.
