#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Build a deterministic unsigned local RC bundle. No upload, signing or downloads of binaries."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess
import tarfile


def copy_reviewed(repo, name, bundle):
    relative = Path(name)
    if relative.is_absolute() or '..' in relative.parts:
        raise ValueError('unsafe package source path')
    source = repo / relative
    if source.is_symlink() or not source.resolve().is_relative_to(repo):
        raise ValueError('package inputs must stay inside the reviewed repository')
    fd = os.open(source, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, 'rb') as f:
        info = os.fstat(f.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_size > (8 << 20):
            raise ValueError('expected bounded regular source file without links')
        data = f.read((8 << 20) + 1)
        if len(data) > (8 << 20):
            raise ValueError('package input grew beyond limit')
    target = bundle / relative
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(data)


def main():
    os.umask(0o077)
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--output', type=Path, required=True, help='new output directory')
    p.add_argument('--allow-dirty', action='store_true', help='mark a private development bundle; not a release build')
    a = p.parse_args()
    repo = Path(__file__).resolve().parents[1]
    dirty = bool(subprocess.check_output(['git', 'status', '--porcelain'], cwd=repo))
    if dirty and not a.allow_dirty:
        p.error('commit reviewed source first, or explicitly use --allow-dirty for private development')
    version = '0.5.0-rc.1'
    commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=repo, text=True).strip()
    if a.output.exists():
        p.error('output exists; use a new directory')
    a.output.mkdir(parents=True, mode=0o700)
    bundle = a.output.resolve() / ('agent-linux-' + version + '-darwin-arm64')
    bundle.mkdir(mode=0o700)
    (bundle / 'bin').mkdir()
    env = os.environ.copy()
    env.update({'CGO_ENABLED': '0', 'GOARCH': 'arm64', 'GOFLAGS': '', 'GOWORK': 'off', 'GOTOOLCHAIN': 'go1.26.9', 'GOCACHE': str(repo / '.cache/go-build'), 'GOMODCACHE': str(repo / '.cache/go-mod')})
    go_version = subprocess.check_output(['go', 'version'], cwd=repo, env=env, text=True).strip()
    if not go_version.startswith('go version go1.26.9 '):
        p.error('release candidate packaging requires the selected Go 1.26.9 toolchain')
    for target, name, command in [('darwin', 'agent-linux', './cmd/agent-linux'), ('linux', 'agent-linux-guest', './cmd/agent-linux-guest')]:
        env['GOOS'] = target
        flags = '-s -w -buildid= -X github.com/ADITYADEV4145/AgentLinux/internal/buildinfo.Commit=' + commit
        subprocess.run(['go', 'build', '-trimpath', '-buildvcs=false', '-ldflags=' + flags, '-o', str(bundle / 'bin' / name), command], cwd=repo, env=env, check=True)
    for name in ['LICENSE', 'README.md', 'CONTRIBUTING.md', 'SECURITY.md', 'go.mod', 'go.sum']:
        copy_reviewed(repo, name, bundle)
    # Never sweep ignored local notes, keys, downloads or editor files into a bundle.
    paths = subprocess.check_output(['git', 'ls-files', '-z', '--cached', '--others', '--exclude-standard', '--', 'docs', 'third_party', 'examples', 'images', 'internal/mcpserver/instructions.txt', 'scripts/cost_model.py', 'scripts/billing_acceptance.py'], cwd=repo, text=True).split('\0')
    for name in sorted(set(paths) - {''}):
        copy_reviewed(repo, name, bundle)
    copy_reviewed(repo, 'scripts/install.py', bundle)
    (bundle / 'scripts/install.py').rename(bundle / 'install.py')
    if not any((bundle / 'scripts').iterdir()):
        (bundle / 'scripts').rmdir()
    inventory = []
    for name in ['agent-linux', 'agent-linux-guest']:
        inventory.append(subprocess.check_output(['go', 'version', '-m', str(bundle / 'bin' / name)], cwd=repo, env=env, text=True).replace(str(bundle / 'bin' / name), name))
    (bundle / 'DEPENDENCIES.txt').write_text('\n'.join(inventory))
    records = {}
    for path in sorted(bundle.rglob('*')):
        if path.is_file():
            b = path.read_bytes()
            records[path.relative_to(bundle).as_posix()] = {'size': len(b), 'sha256': hashlib.sha256(b).hexdigest()}
            path.chmod(0o755 if path.parent.name == 'bin' or path.name == 'install.py' else 0o644)
    m = {'version': version, 'commit': commit, 'dirty': dirty, 'target': 'darwin-arm64', 'signing': 'no Developer ID; Go ad-hoc signature possible; not notarized',
         'go': go_version, 'files': records}
    (bundle / 'manifest.json').write_text(json.dumps(m, sort_keys=True, indent=2) + '\n')
    archive = a.output.resolve() / (bundle.name + '.tar.gz')
    with archive.open('wb') as raw, gzip.GzipFile(fileobj=raw, mode='wb', filename='', mtime=0) as compressed, tarfile.open(fileobj=compressed, mode='w') as tar:
        for path in [bundle, *sorted(bundle.rglob('*'))]:
            info = tar.gettarinfo(str(path), arcname=str(path.relative_to(bundle.parent)))
            info.uid = info.gid = 0
            info.uname = info.gname = ''
            info.mtime = 0
            info.mode = 0o755 if path.is_dir() or path.parent.name == 'bin' or path.name == 'install.py' else 0o644
            if info.isfile():
                with path.open('rb') as f:
                    tar.addfile(info, f)
            else:
                tar.addfile(info)
    checksum = hashlib.sha256(archive.read_bytes()).hexdigest()
    (a.output / 'SHA256SUMS').write_text(checksum + '  ' + archive.name + '\n')
    print(json.dumps({'bundle': str(bundle), 'archive': str(archive), 'sha256': checksum, 'dirty': dirty}))


if __name__ == '__main__':
    main()
