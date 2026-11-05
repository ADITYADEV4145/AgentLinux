# Plasma/X11 research adapter

This is an opt-in comparison build, **not a supported second production image**.
It copies the reviewed source to a new directory, substitutes Debian's Plasma
6.3.6/KWin-X11/Dolphin/Konsole/KWrite/System Settings/KRunner packages and fixed
application targets, and builds a separate CLI and guest helper. It never edits
production source, global configuration or any existing VM. It needs the same
Go toolchain, reviewed Lima 2.2.1, ARM64 Mac, network and image trust as the core.
Review the generated source before booting. Package versions can change with APT.

```sh
python3 experiments/plasma/build.py --output "$PWD/.tools/plasma-research"
# Choose a NEW private disposable namespace; never select your normal VM.
PLASMA_STATE=$(mktemp -d /private/tmp/agentlinux-phase55-plasma-XXXXXXXX)
.tools/plasma-research/agent-linux up --local --network=nat \
  --state-dir "$PLASMA_STATE" --lima "$PWD/.tools/lima/bin/limactl" \
  --cpus 4 --memory 8 --disk 40
.tools/plasma-research/agent-linux screenshot --state-dir "$PLASMA_STATE" \
  --lima "$PWD/.tools/lima/bin/limactl" --output /tmp/plasma-research.png
```

The generated CLI finds the sibling experimental guest binary. Its sealed Lima
recipe must contain the Plasma launcher/helper: manually replacing files in a
running Xfce guest is insufficient, because Lima reapplies the stored provisioner
on restart. Readiness requires KWin, plasmashell, the managed X display and a fresh
generation. No Xfce session is started. Qt Quick uses software rendering and KWin
composition is disabled. This does not exercise native Wayland, GPU rendering,
sound, normal login/display-manager or software-store flows.

Dolphin/KIO needs to create sockets under the **guest-owned** desktop runtime
directory. The generated guest helper unit additionally allows writes there;
ProtectSystem and all host isolation rules remain. It is not a host filesystem
share. No host listener, clipboard or pointer input is introduced.

Use the generated CLI with the same opt-in `TestRealWorkstation` environment
shown in docs/workstation.md. Always inspect genuine screenshots and check the
actual session's processes after every restart. The first manual experiment
reverted to Xfce; its restart timings were rejected from the Plasma comparison.

Stop this exact fixture, inspect its machine ID/removal plan, and delete it
explicitly with that ID when finished. Never use global Lima delete/prune or
reset a normal machine. Keep private images/logs/keys outside Git.

This adapter intentionally depends on the reviewed source layout; review/retest
it after changes. It is not a stable profile API or a promise to maintain two
production desktops. Current upstream Plasma 6.8 has retired X11 sessions; the
Debian 6.3 result is not evidence for modern Wayland capture/input. See
[workstation decision and actual results](../../docs/workstation.md).
