# ADR 0011: Ubuntu LTS / GNOME as the modern desktop target

Date: 2026-11-05. Status: recommended target with a real Mac feasibility spike; production default switch is gated.

Select Ubuntu 26.04.1 LTS ARM64 with Ubuntu GNOME 50 and a full application set, initially 4 CPUs/8 GiB/40 GiB. It offers a current cohesive desktop and longer maintenance window while reusing APT/systemd knowledge. Fedora Workstation 45/GNOME 51 is a strong current alternative, but adds shorter release qualification cycles and DNF/RPM/SELinux integration. Neither is a drop-in replacement for the existing X11 service.

An independent Mac VZ VM ran the full Ubuntu desktop package set, captured actual Wayland PNGs, displayed Files/Ptyxis/Settings/Firefox and saved physical-key plus Unicode guest-clipboard input. Image-format validation passed against the existing frame contract. Pointer gestures, GUI-store flow and managed-session/restart behavior have incomplete coverage. The research adapter uses private Mutter APIs; production should prefer maintained GNOME Remote Desktop or explicit standard portal/libei grants, with authenticated guest-local boundaries.

Retain Debian/Xfce/X11 as the working RC backend until full GNOME screenshot/input, lifecycle/session readiness, normal software installation/updates, actual MCP client operation and live Kitty/web viewing pass on the same guest. No image/default/config migration is authorized by a recommendation, and no existing machine is changed. Do not select an older/stripped desktop merely to save resources or treat Xwayland as generic Wayland automation.

See [desktop quality comparison and acceptance gate](../desktop-quality.md), [verification](../verification.md) and the guarded experimental guest probe in experiments/gnome. No public desktop image/release or paid infrastructure is created.
