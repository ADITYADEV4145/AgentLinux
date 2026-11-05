#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Runs as guest agent, on the managed guest display, before the first session.
# One-time defaults: subsequent user changes and persistent sessions are retained.
set -eu
umask 077
marker="$HOME/.config/agent-linux/workstation-5.5.0"
[ ! -f "$marker" ] || exit 0
mkdir -p "$HOME/.config/agent-linux" "$HOME/.config/xfce4/xfconf/xfce-perchannel-xml"
set_value() { xfconf-query -c "$1" -p "$2" -n -t "$3" -s "$4"; }
set_value xsettings /Net/ThemeName string Greybird
set_value xsettings /Net/IconThemeName string Adwaita
set_value xsettings /Gtk/FontName string 'Noto Sans 10'
set_value xsettings /Gtk/MonospaceFontName string 'Noto Sans Mono 11'
set_value xsettings /Xft/Antialias int 1
set_value xsettings /Xft/Hinting int 1
set_value xfwm4 /general/theme string Greybird
set_value xfwm4 /general/title_font string 'Noto Sans Bold 10'
set_value xfwm4 /general/click_to_focus bool true
set_value xfwm4 /general/focus_new bool true
set_value xfwm4 /general/use_compositing bool false
# Xvfb's sole output is named screen. Keep the guest resolution at 1280x800.
set_value xfce4-desktop /backdrop/screen0/monitorscreen/workspace0/last-image string /usr/local/share/backgrounds/agent-linux.svg
set_value xfce4-desktop /backdrop/screen0/monitorscreen/workspace0/image-style int 5
set_value xfce4-desktop /desktop-icons/file-icons/show-filesystem bool false
set_value xfce4-desktop /desktop-icons/file-icons/show-home bool true
set_value xfce4-desktop /desktop-icons/file-icons/show-trash bool true
# Use only maintained built-in panel plugins, with explicit labels and task buttons.
# Configure through xfconf (rather than editing a live daemon's XML files).
xfconf-query -c xfce4-panel -p /configver -n -t int -s 2
xfconf-query -c xfce4-panel -p /panels -n -a -t int -s 1
set_value xfce4-panel /panels/panel-1/position string 'p=10;x=640;y=800'
set_value xfce4-panel /panels/panel-1/position-locked bool true
set_value xfce4-panel /panels/panel-1/size int 40
set_value xfce4-panel /panels/panel-1/length uint 100
set_value xfce4-panel /panels/panel-1/length-adjust bool false
xfconf-query -c xfce4-panel -p /panels/panel-1/plugin-ids -n -a -t int -s 1 -t int -s 2 -t int -s 3 -t int -s 4 -t int -s 5 -t int -s 6
set_value xfce4-panel /plugins/plugin-1 string applicationsmenu
set_value xfce4-panel /plugins/plugin-1/button-title string Applications
set_value xfce4-panel /plugins/plugin-1/show-button-title bool true
set_value xfce4-panel /plugins/plugin-2 string tasklist
set_value xfce4-panel /plugins/plugin-2/flat-buttons bool false
set_value xfce4-panel /plugins/plugin-3 string separator
set_value xfce4-panel /plugins/plugin-3/expand bool true
set_value xfce4-panel /plugins/plugin-3/style uint 0
set_value xfce4-panel /plugins/plugin-4 string systray
set_value xfce4-panel /plugins/plugin-5 string clock
set_value xfce4-panel /plugins/plugin-5/digital-format string '%a %d %b  %H:%M'
set_value xfce4-panel /plugins/plugin-6 string showdesktop
# No screen-lock/suspend daemon is installed by this virtual-display recipe.
# The host can still lock/suspend independently; no host setting is changed.
touch "$marker"
