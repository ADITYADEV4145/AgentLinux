#!/bin/sh
# Root-owned maintenance entrypoint, invoked only by explicit host CLI via guest admin.
set -eu
umask 077
export PATH=/usr/sbin:/usr/bin:/sbin:/bin HOME=/root LANG=C.UTF-8
export DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=l
[ "$(id -u)" = 0 ] || { echo 'guest administrator privileges required' >&2; exit 77; }
action=${1-}; [ "$#" -gt 0 ] && shift
case "$action" in
 install)
  [ "$#" -ge 1 ] && [ "$#" -le 32 ] || exit 64
  for package in "$@"; do
   case "$package" in ''|*[!a-z0-9+.-]*|[!a-z0-9]*) echo 'invalid package name' >&2; exit 64;; esac
   [ "${#package}" -le 100 ] || exit 64
  done;;
 refresh|upgrade) [ "$#" -eq 0 ] || exit 64;;
 *) echo 'choose install, refresh or upgrade' >&2; exit 64;;
esac
# No concurrent maintenance, including manually initiated invocations.
exec 9>/run/lock/agent-linux-software.lock
flock -n 9 || { echo 'package maintenance already running' >&2; exit 75; }
# Fail rather than installing from stale indexes after a network/metadata failure.
if ! apt-get -o APT::Update::Error-Mode=any -o Acquire::Retries=0 -o Acquire::http::Timeout=20 -o Acquire::https::Timeout=20 update; then
 echo 'AGENTLINUX_REPOSITORY_UNAVAILABLE: check guest networking, repository signatures and mirrors' >&2
 exit 69
fi
case "$action" in
 install) apt-get -y --no-install-recommends -o Dpkg::Options::=--force-confold install -- "$@";;
 upgrade) apt-get -y -o Dpkg::Options::=--force-confold upgrade;;
esac
install -d /var/lib/agent-linux
dpkg-query -W > /var/lib/agent-linux/packages.tsv.new
mv /var/lib/agent-linux/packages.tsv.new /var/lib/agent-linux/packages.tsv
date -u +%FT%TZ > /var/lib/agent-linux/last-maintenance
