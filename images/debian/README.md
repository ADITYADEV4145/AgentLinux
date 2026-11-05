# Guest provisioning (Phase 2)

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

The code and recipe are repeatable, but resulting disk bytes, SSH keys, boot IDs and timestamps intentionally differ per machine. This is not a bit-reproducible image build. Phase 2 installs Xvfb, Xfce components, Mousepad, Chromium/sandbox, scrot, xdotool, xclip, X11/auth/D-Bus tools and fonts. The exact installed package list is saved in /var/lib/agent-linux/packages.tsv. The desktop install marker avoids repeating apt on every boot; security updates require an explicit image/provisioning revision, not an assumption that restarting updates packages. MCP and live viewers remain unimplemented.


Phase 2 uses protocol 2.0/service 0.2.0 and does not automatically upgrade a Phase 1 disk. Use a fresh --state-dir; no existing machine is reset or overwritten. The package install is repeatable from the recipe but not byte-reproducible: the Debian security repository can change. Record package versions during acceptance and review image/recipe updates deliberately. The X11/Wayland and guest-exec trust limitations are in docs/guest-protocol.md.
