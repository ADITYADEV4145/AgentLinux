#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Offline, user-owned bundle installer. Never downloads or modifies shell configuration."""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import stat
import sys
import tempfile

MARKER = 'agent-linux tools v1\n'
MAX_TOTAL = 64 << 20


def canonical(path):
    path = Path(path)
    if not path.is_absolute() or str(path) != os.path.normpath(str(path)):
        raise ValueError('use an absolute clean path')
    for part in (path, *path.parents):
        if part.is_symlink():
            raise ValueError('symlink path components are refused; use the resolved path')
    if path == Path.home() or len(path.parts) < 4:
        raise ValueError('refusing home/system/root prefix')
    return path


def regular(path, limit=MAX_TOTAL):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, 'rb') as f:
        s = os.fstat(f.fileno())
        if not stat.S_ISREG(s.st_mode) or s.st_nlink != 1 or s.st_size > limit:
            raise ValueError('expected bounded regular file without links: ' + path.name)
        data = f.read(limit + 1)
        if len(data) > limit:
            raise ValueError('file grew beyond installer limit')
        return data


def manifest(bundle):
    canonical(bundle)
    data = regular(bundle / 'manifest.json', 1 << 20)
    m = json.loads(data)
    if m['target'] != 'darwin-arm64' or not re.fullmatch(r'0\.5\.0-rc\.1', m['version']) or not re.fullmatch(r'[0-9a-f]{40}', m['commit']):
        raise ValueError('unsupported version/target/provenance')
    records = m['files']
    if not isinstance(records, dict) or not 5 <= len(records) <= 256:
        raise ValueError('invalid file inventory')
    total = 0
    for name, record in records.items():
        p = Path(name)
        if p.is_absolute() or '..' in p.parts or name != p.as_posix() or name == 'manifest.json':
            raise ValueError('unsafe manifest path')
        for parent in (bundle / name).parents:
            if parent == bundle:
                break
            if parent.is_symlink():
                raise ValueError('symlink in bundle')
        b = regular(bundle / name)
        total += len(b)
        if total > MAX_TOTAL or hashlib.sha256(b).hexdigest() != record['sha256'] or len(b) != record['size']:
            raise ValueError('bundle checksum/size mismatch')
    if not {'bin/agent-linux', 'bin/agent-linux-guest', 'LICENSE', 'install.py', 'README.md'} <= records.keys():
        raise ValueError('bundle missing required artifacts')
    actual = set()
    for base, dirs, files in os.walk(bundle, followlinks=False):
        for name in dirs + files:
            if (Path(base) / name).is_symlink():
                raise ValueError('symlink in bundle')
        actual.update((Path(base) / f).relative_to(bundle).as_posix() for f in files)
    if actual != set(records) | {'manifest.json'}:
        raise ValueError('unexpected/missing bundle contents')
    return m


def owned(prefix, create=False):
    prefix = canonical(prefix)
    if not prefix.exists():
        if not create:
            raise ValueError('installation does not exist')
        prefix.mkdir(mode=0o700, parents=True)
        (prefix / '.agent-linux-tools').write_text(MARKER)
        (prefix / '.agent-linux-tools').chmod(0o600)
    s = prefix.stat()
    if s.st_uid != os.getuid() or s.st_mode & 0o077:
        raise ValueError('prefix must be user-owned and private (0700)')
    if regular(prefix / '.agent-linux-tools', 128).decode() != MARKER:
        raise ValueError('invalid tools ownership marker; refusing to adopt directory')
    if set(p.name for p in prefix.iterdir()) - {'.agent-linux-tools', '.install-lock', 'releases', 'bin', 'current'}:
        raise ValueError('unexpected prefix contents; nothing removed')
    return prefix


def lock(prefix):
    p = prefix / '.install-lock'
    fd = os.open(p, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    s = os.fstat(fd)
    if not stat.S_ISREG(s.st_mode) or s.st_nlink != 1 or s.st_uid != os.getuid() or s.st_mode & 0o077:
        os.close(fd)
        raise ValueError('unsafe installer lock')
    fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
    return fd


def validate_layout(prefix):
    for name in ('bin', 'releases'):
        p = prefix / name
        if p.exists() and (p.is_symlink() or not p.is_dir()):
            raise ValueError('unsafe install subdirectory')
    current = prefix / 'current'
    if current.is_symlink():
        target = os.readlink(current)
        if not re.fullmatch(r'releases/0\.5\.0-rc\.1-[0-9a-f]{12}(?:-dev)?', target):
            raise ValueError('unexpected current symlink')
    elif current.exists():
        raise ValueError('current must be installer-owned symlink')
    if (prefix / 'bin').exists():
        for p in (prefix / 'bin').iterdir():
            if p.name not in ('agent-linux', 'agent-linux-guest') or not p.is_symlink() or os.readlink(p) != '../current/bin/' + p.name:
                raise ValueError('unexpected launcher; refusing overwrite/removal')


def install(bundle, prefix):
    m = manifest(bundle)  # Verify everything before mutating installation.
    prefix = owned(prefix, create=True)
    fd = lock(prefix)
    stage = None
    try:
        validate_layout(prefix)
        (prefix / 'releases').mkdir(mode=0o700, exist_ok=True)
        (prefix / 'bin').mkdir(mode=0o700, exist_ok=True)
        release = m['version'] + '-' + m['commit'][:12] + ('-dev' if m['dirty'] else '')
        dest = prefix / 'releases' / release
        if dest.exists() or dest.is_symlink():
            if dest.is_symlink() or manifest(dest) != m:
                raise ValueError('release identity already exists with different content')
        else:
            stage = Path(tempfile.mkdtemp(prefix='.install-', dir=prefix / 'releases'))
            for name, record in m['files'].items():
                out = stage / name
                out.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
                out.write_bytes(regular(bundle / name))
                out.chmod(0o755 if name.startswith('bin/') or name == 'install.py' else 0o644)
            (stage / 'manifest.json').write_bytes(regular(bundle / 'manifest.json'))
            manifest(stage)  # Detect changed source/copy failures before activating.
            os.rename(stage, dest)
            stage = None
        for name in ('agent-linux', 'agent-linux-guest'):
            launcher = prefix / 'bin' / name
            if not launcher.is_symlink():
                launcher.symlink_to('../current/bin/' + name)
        # The sole activation point: old version remains current through failed verification/copy.
        new = prefix / ('current.' + secrets.token_hex(8))
        try:
            new.symlink_to('releases/' + release)
            os.replace(new, prefix / 'current')
        finally:
            new.unlink(missing_ok=True)
        return prefix / 'bin' / 'agent-linux'
    finally:
        if stage:
            shutil.rmtree(stage)
        os.close(fd)


def uninstall(prefix, yes):
    prefix = owned(prefix)
    fd = lock(prefix)
    try:
        validate_layout(prefix)
        releases = prefix / 'releases'
        if releases.exists():
            for p in releases.iterdir():
                if p.is_symlink() or not p.is_dir() or not re.fullmatch(r'0\.5\.0-rc\.1-[0-9a-f]{12}(?:-dev)?', p.name):
                    raise ValueError('unexpected release entry; inspect before uninstall')
                manifest(p)
        print('Remove tool launchers and releases under: ' + str(prefix), file=sys.stderr)
        print('VM state, guest data, Lima and shell settings remain unchanged.', file=sys.stderr)
        if not yes:
            raise ValueError('nothing removed; pass --yes explicitly to uninstall these tools')
        (prefix / 'current').unlink(missing_ok=True)
        if (prefix / 'bin').exists():
            for p in (prefix / 'bin').iterdir():
                p.unlink()
            (prefix / 'bin').rmdir()
        if releases.exists():
            shutil.rmtree(releases)  # Validated user-owned install tree; never VM state or arbitrary prefix.
    finally:
        os.close(fd)
    # Leave marker/lock so concurrent callers never replace the active lock inode.


def main():
    os.umask(0o077)
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--bundle', type=Path)
    p.add_argument('--prefix', type=Path, required=True)
    p.add_argument('--uninstall', action='store_true')
    p.add_argument('--yes', action='store_true')
    args = p.parse_args()
    if os.getuid() == 0:
        p.error('run as your own user; sudo/root installation is refused')
    try:
        if args.uninstall:
            if args.bundle:
                p.error('--bundle is not used with --uninstall')
            uninstall(args.prefix, args.yes)
        else:
            if args.bundle is None or args.yes:
                p.error('installation requires --bundle; --yes is only for uninstall')
            print(install(args.bundle, args.prefix))
    except (OSError, ValueError, KeyError, TypeError) as exc:
        print('INSTALL_FAILED: ' + str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
