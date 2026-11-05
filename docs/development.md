# Development and package maintenance

The default recipe adds Git, OpenSSH client, curl/CA certificates, Bash/completion, common shell utilities, Python 3 with venv, Nano, less and man pages. Mousepad is the graphical editor; xfce4-terminal runs the guest shell. Thunar and Xfce settings remain part of the small desktop. No host project directory, SSH agent, keychain, clipboard or credential is mounted/forwarded.

Node.js/npm and compiler headers belong to optional profiles because not every desktop needs their dependency trees. The exact installed versions come from the signed Debian 13 repositories at provisioning/maintenance time and are recorded in `/var/lib/agent-linux/packages.tsv`. Debian's packaged Node release/security policy is distinct from upstream Node LTS; select a separately reviewed toolchain inside the guest if a project needs a newer upstream version. These optional profiles are recipe selections, not claims of testing every toolchain/project.

```sh
bin/agent-linux packages profiles --json
bin/agent-linux packages install --profile web-dev  # Debian nodejs + npm
bin/agent-linux packages install --profile build    # build-essential, pkg-config, python3-dev
bin/agent-linux packages install jq                 # ordinary Debian package name
bin/agent-linux packages refresh
bin/agent-linux packages upgrade
bin/agent-linux exec -- /usr/bin/git --version
bin/agent-linux exec -- /usr/bin/python3 -m venv /home/agent/Projects/example/.venv
bin/agent-linux open terminal
```

Create the example project directory first. `exec` argv runs only in Linux; shell syntax requires explicit `/bin/bash -lc`. Guest command scopes clean up background descendants, so long-lived interactive development servers should be launched from the guest terminal, not assumed to survive an `exec` result. Servers remain subject to desktop/VM shutdown. NAT outbound access is available; no automatic host publication of development server ports is implemented.

## Privileges and failure behavior

The desktop, forced SSH bridge, guest service and MCP command tool run as non-sudo `agent`, with a locked password. MCP cannot elevate through the package API. The host `packages`/`browsers install` CLI explicitly uses Lima's existing bootstrap **guest** administrator over its controlled namespace. It runs only a fixed root-owned guest maintenance helper inside a bounded systemd unit. It never invokes a host package manager. This trusts Lima's management channel, which has weaker initial host-key enrollment than our pinned application channel; no new listener, shared reusable credential, sudo permission for `agent`, or privileged network service is added.

Package names are validated on both host and guest: no options, paths, URLs, versions, release selectors or injected shell fragments. Only configured guest apt sources are used. The helper refreshes indexes with strict failure-on-error and bounded network waits, keeps existing modified config files, then installs/upgrades. It does not add repositories, disable signatures, perform a distribution upgrade or reset user files. Maintainer scripts run as guest root, as with any normal apt install; this is intentional guest administrator access, not host administrator access. Existing guest admin/root users can independently alter repositories and the guest OS.

Per-machine locks prevent simultaneous lifecycle/MCP/maintenance operations in our CLI. A fixed systemd unit plus guest flock prevents overlapping maintenance jobs. The unit has a nine-minute maximum runtime and 15-second stop grace; host CLI default deadline is ten minutes. A disconnect/host cancellation can leave the guest job running until completion or that bound. Never claim automatic rollback/cancellation success: `PACKAGE_OUTCOME_UNKNOWN` directs the operator to `systemctl status agent-linux-software` in the guest and private backend diagnostics before retrying. Killing dpkg mid-operation can require explicit guest-admin repair (`dpkg --configure -a`); no automatic destructive repair is attempted.

Missing packages return `PACKAGE_UNAVAILABLE`. Repository refresh errors return `REPOSITORY_UNAVAILABLE` (check networking, signatures, mirrors and disk), not a false successful installation from stale indexes. A missing legacy helper or other failed job returns an actionable uncertain-operation error. Default logs do not contain host secrets; apt diagnostics are bounded/sanitized in private state. New package maintainer scripts are untrusted guest output. No unattended update promise is made.

See [browser choices](browsers.md), [image/persistence model](../images/debian/README.md) and [real verification](verification.md). Official package references: [Debian apt-get manual](https://manpages.debian.org/trixie/apt/apt-get.8.en.html), [Debian packaged Node.js](https://packages.debian.org/trixie/nodejs).
