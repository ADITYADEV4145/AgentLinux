#!/bin/sh
# Root-installed launcher; every process below runs as the unprivileged guest agent.
set -eu
umask 077
export DISPLAY=:1 XAUTHORITY=/run/agent-linux-desktop/Xauthority
export XDG_RUNTIME_DIR=/run/agent-linux-desktop
export DBUS_SESSION_BUS_ADDRESS=unix:path=/run/agent-linux-desktop/bus
xauth -f "$XAUTHORITY" add "$DISPLAY" MIT-MAGIC-COOKIE-1 "$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')"
Xvfb "$DISPLAY" -screen 0 1280x800x24 -nolisten tcp -auth "$XAUTHORITY" &
xpid=$!
trap 'kill "$xpid" 2>/dev/null || true' EXIT INT TERM
i=0
until xdpyinfo >/dev/null 2>&1; do i=$((i+1)); [ "$i" -lt 100 ] || exit 1; sleep 0.1; done
dbus-daemon --session --fork --address="$DBUS_SESSION_BUS_ADDRESS"
xfconf-query -c xfwm4 -p /general/use_compositing -n -t bool -s false
xfce4-session &
session=$!
i=0
until xprop -root _NET_SUPPORTING_WM_CHECK 2>/dev/null | grep -q 'window id # 0x'; do i=$((i+1)); [ "$i" -lt 100 ] || exit 1; sleep 0.1; done
xprop -root -f _AGENT_LINUX_GENERATION 8s -set _AGENT_LINUX_GENERATION "$(cat /proc/sys/kernel/random/uuid)"
wait "$session"
