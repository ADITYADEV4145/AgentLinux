# GNOME 50 guest feasibility probe (experimental)

This directory contains a **guest-only research probe**, not an AgentLinux backend or supported desktop profile. The selected product direction and observed limitations are in [desktop-quality.md](../../docs/desktop-quality.md). The existing CLI still provisions Debian/Xfce; do not substitute an Ubuntu image under its Debian package recipe.

The probe uses Mutter's explicitly private D-Bus APIs to test PipeWire capture, physical key/modifier dispatch, Unicode guest-clipboard paste and guest application windows. It does not integrate MCP, manage a VM, enable RDP/VNC, change host settings or access the Mac desktop/clipboard. Version-specific success is not a stable API guarantee. Pointer method acknowledgements are not comprehensive semantic drag/drop testing, and software-store readiness is not asserted by this script.

## Exact evaluated environment

- Apple silicon macOS 26.5, Lima 2.2.1/VZ; separate private disposable namespace, plain mode, no mounts, additional disks, port-forward rules or agent/X11 forwarding, no inherited templates/proxy credentials.
- Ubuntu 26.04.1 ARM64, cloud image `https://cloud-images.ubuntu.com/releases/resolute/release-20260927/ubuntu-26.04-server-cloudimg-arm64.img`, SHA-256 `63a93bd5a8d76e33b15ceb5daa3657bd79be804748051ab178e643b0f5da22e7`. Checked against the official dated checksum list and pinned Lima template; detached image signature verification was not performed. No image is bundled or redistributed.
- 4 vCPUs, 8 GiB RAM, 40 GiB disk. Full `ubuntu-desktop` with recommended packages; also `gnome-remote-desktop pipewire-bin gstreamer1.0-pipewire gstreamer1.0-tools python3-gi gir1.2-gstreamer-1.0 mesa-utils`. App Center (`snap install snap-store --channel=2/stable`) and `gnome-software` were explicitly added. Normal APT/Snap installations succeeded; GUI store flow remains incompletely validated.
- Non-sudo evaluation user `desktop-user`, password disabled, `loginctl enable-linger desktop-user`, normal XDG user directories. Private session bus `/run/user/UID/bus`, fixed virtual monitor 1280×800, software rendering and no /dev/dri device.

Full provisioning exceeded Lima's boot-script wait. The initial start returned DEGRADED, not a ready desktop. We waited for package completion and a clean `dpkg --audit` before the following steps. Do not overwrite the current/default VM, disable host protections or run global Lima prune/delete. Obtain and verify a separate ARM64 image/runtime first.

## Reproduce inside a disposable guest

These are **Linux guest administrator steps**, not macOS commands. An evaluator must intentionally create the non-sudo account and root opt-in marker after the full package install succeeds:

```sh
# Inside the dedicated Linux guest only, as its administrator:
sudo useradd --create-home --shell /bin/bash desktop-user  # only if not already present
sudo usermod --password '*' desktop-user
sudo loginctl enable-linger desktop-user
sudo install -d /var/lib/agent-linux-evaluation
# Only after apt/snap have finished and dpkg --audit is clean:
printf 'completed\n' | sudo tee /var/lib/agent-linux-evaluation/installed >/dev/null
```

Launch the compositor as that user with HOME=/home/desktop-user, XDG_RUNTIME_DIR=/run/user/UID, DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/UID/bus, XDG_SESSION_TYPE=wayland, XDG_CURRENT_DESKTOP=ubuntu:GNOME and GNOME_SHELL_SESSION_MODE=ubuntu. The tested compositor command was `gnome-shell --headless --virtual-monitor=1280x800` with LIBGL_ALWAYS_SOFTWARE=1 in an explicitly created transient guest systemd unit. Run `xdg-user-dirs-update` for the user and import WAYLAND_DISPLAY=wayland-0 and the desktop variables into its D-Bus/systemd activation environment before launching applications. Normal full-session launcher/service/activation behavior needs additional validation; this standalone compositor is not a finished managed GNOME session.

Then, **as the non-root guest user**, with that session environment:

```sh
python3 /path/to/probe.py --output /home/desktop-user/new-evaluation-output
```

Guards require Linux, a detected VM, the root-owned opt-in marker, GNOME Shell 50.x, and a new output directory under the guest user's home. `--help` works without GI/guest setup; running the probe on macOS is refused before any desktop API is opened. Dependencies are the guest's distro PyGObject/GStreamer packages, not host GUI-automation libraries.

The script creates its own test documents and captures actual guest PNGs; no archive extraction or host input is used. It leaves the opened guest applications available for inspection and closes its remote-desktop session on exit. Capture/request waits and PNG dimensions/size are bounded. The report must pass physical-key and exact Unicode-content checks; inspect the real images as well, especially browser/software-manager content. Text Editor's final newline is explicitly included in the expected bytes. The probe's `productionIntegration` is always false.

After an actual VM stop/start, the test document persisted. The transient standalone compositor did not auto-start; a production profile must solve managed GNOME session readiness/recovery. Use only the selected private namespace/name to stop/delete your own disposable VM after saving evidence; do not remove unrelated/default state or mistake a running OS for a ready desktop.

Original probe code is Apache-2.0. GNOME/Mutter/PipeWire/GStreamer/distro components retain their own licenses and are external guest dependencies; this is not clearance to redistribute a desktop image.
