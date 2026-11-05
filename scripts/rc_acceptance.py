#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Real-Mac acceptance of an already installed CLI and explicitly selected disposable VM.
Never creates a VM. --reset and --delete explicitly destroy only the named fixture.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time


def main():
    os.umask(0o077)
    p = argparse.ArgumentParser(description=__doc__)
    for name in ('cli', 'lima', 'state-dir', 'machine-id', 'evidence'):
        p.add_argument('--' + name, required=True)
    p.add_argument('--chrome')
    p.add_argument('--reset', action='store_true')
    p.add_argument('--delete', action='store_true')
    a = p.parse_args()
    repo = Path(__file__).resolve().parents[1]
    root = Path(a.state_dir)
    if root != root.resolve() or root.parent != Path('/private/tmp') or not root.name.startswith('agentlinux-rc-acceptance-'):
        p.error('requires explicit canonical disposable agentlinux-rc-acceptance-* state')
    if json.loads((root / 'state.json').read_text())['machineId'] != a.machine_id:
        p.error('fixture identity mismatch')
    evidence = Path(a.evidence).resolve()
    evidence.mkdir(parents=True, exist_ok=False)
    timings = {}
    outcomes = {}

    def run(name, command, *argv, success=True):
        started = time.monotonic()
        result = subprocess.run([a.cli, command, '--state-dir', str(root), '--lima', a.lima, '--json', *argv],
                                capture_output=True, text=True, timeout=1600)
        timings[name] = time.monotonic() - started
        (evidence / (name + '.json')).write_text(result.stdout)
        (evidence / (name + '-stderr.txt')).write_text(result.stderr)
        if success:
            assert result.returncode == 0, name + ': ' + result.stdout + result.stderr[-1000:]
            value = json.loads(result.stdout)
            assert value['ok'], value
            return value['result']
        assert result.returncode != 0, 'unsafe operation unexpectedly passed: ' + name
        return result.returncode

    def guest(name, code):
        return run(name, 'exec', '--', '/usr/bin/python3', '-c', code)

    state = run('initial-status', 'status')
    assert state['machineId'] == a.machine_id and state['state'] == 'ready'
    home = evidence / 'host-files'
    home.mkdir(mode=0o700)
    source = home / 'project.txt'
    payload = ('UTF-8 café 日本語 👋\n' * 16000).encode()
    source.write_bytes(payload)
    sentinel = home / 'unrelated.txt'
    sentinel.write_text('do not touch')
    uploaded = run('copy-in', 'copy-in', '--source', str(source))
    destination = home / 'results.txt'
    run('copy-out', 'copy-out', '--source', uploaded['path'], '--output', str(destination))
    assert destination.read_bytes() == payload
    run('overwrite-refused', 'copy-out', '--source', uploaded['path'], '--output', str(destination), success=False)
    link = home / 'link'
    link.symlink_to(source)
    run('host-symlink-refused', 'copy-in', '--source', str(link), success=False)
    run('traversal-refused', 'copy-out', '--source', '/home/agent/../etc/passwd', '--output', str(home / 'unsafe'), success=False)
    guest('guest-symlink', "import os;os.symlink('/etc/passwd','/home/agent/unsafe-link')")
    run('guest-symlink-refused', 'copy-out', '--source', '/home/agent/unsafe-link', '--output', str(home / 'unsafe'), success=False)
    guest('persist-marker', "from pathlib import Path;Path('/home/agent/rc-marker').write_text('persist')")
    run('screenshot', 'screenshot', '--output', str(evidence / 'guest.png'))
    guest('isolation-audit', "import json,os,subprocess;print(json.dumps({'uid':os.getuid(),'groups':os.getgroups(),'mounts':open('/proc/mounts').read(),'sudo':subprocess.run(['sudo','-n','true'],capture_output=True).returncode,'environmentKeys':sorted(os.environ)}))")
    env = os.environ.copy()
    env.update({'GOCACHE': str(repo / '.cache/go-build'), 'GOMODCACHE': str(repo / '.cache/go-mod'),
                'AGENTLINUX_ACCEPT_CLI': a.cli, 'AGENTLINUX_ACCEPT_ROOT': str(root),
                'AGENTLINUX_ACCEPT_LIMA': a.lima, 'AGENTLINUX_ACCEPT_EVIDENCE': str(evidence)})
    cp = subprocess.run(['go', 'test', '-v', './internal/mcpserver', '-run', '^TestRealStdioGuest$', '-count=1'],
                        cwd=repo, env=env, capture_output=True, text=True, timeout=240)
    (evidence / 'mcp-test.txt').write_text(cp.stdout + cp.stderr)
    assert cp.returncode == 0, cp.stdout + cp.stderr
    outcomes['mcp'] = True
    if a.chrome:
        with (evidence / 'viewer-output.txt').open('w') as out, (evidence / 'viewer-stderr.txt').open('w') as err:
            viewer = subprocess.Popen([a.cli, 'view', '--web', '--state-dir', str(root), '--lima', a.lima,
                                       '--metrics', str(evidence / 'viewer-metrics.json')], stdout=out, stderr=err)
            try:
                end = time.monotonic() + 20
                while 'http://127.0.0.1:' not in (evidence / 'viewer-output.txt').read_text():
                    if viewer.poll() is not None or time.monotonic() > end:
                        raise RuntimeError('viewer failed to become available')
                    time.sleep(.1)
                cmd = ['python3', str(repo / 'scripts/viewer_web_acceptance.py'), '--cli', a.cli, '--lima', a.lima,
                       '--state-dir', str(root), '--viewer-output', str(evidence / 'viewer-output.txt'),
                       '--evidence', str(evidence / 'web'), '--chrome', a.chrome]
                cp = subprocess.run(cmd, cwd=repo, env=env, capture_output=True, text=True, timeout=240)
                (evidence / 'web-test.txt').write_text(cp.stdout + cp.stderr)
                assert cp.returncode == 0, cp.stdout + cp.stderr
                outcomes['liveWebViewer'] = True
            finally:
                viewer.terminate()
                viewer.wait(timeout=15)
    assert run('post-viewer-status', 'status')['state'] == 'ready'
    run('delete-preview', 'delete', '--machine-id', a.machine_id, success=False)
    run('wrong-id-refused', 'delete', '--machine-id', '0' * 32, '--yes', success=False)
    run('running-delete-refused', 'delete', '--machine-id', a.machine_id, '--yes', success=False)
    run('stop', 'stop')
    run('restart', 'up', '--local')
    probe = guest('persistence', "from pathlib import Path;print(Path('/home/agent/rc-marker').read_text())")
    assert base64.b64decode(probe['stdout']).strip() == b'persist'
    run('stop-before-reset', 'stop')
    identity = a.machine_id
    if a.reset:
        # No secret/input/state/log data are included in the public summary.
        state = run('reset', 'reset', '--machine-id', identity, '--yes')
        assert state['state'] == 'ready' and state['machineId'] != identity
        identity = state['machineId']
        probe = guest('clean-state', "from pathlib import Path;assert not Path('/home/agent/rc-marker').exists();print('clean')")
        assert base64.b64decode(probe['stdout']).strip() == b'clean'
        outcomes['reset'] = True
        run('stop-after-reset', 'stop')
    if a.delete:
        state = run('delete', 'delete', '--machine-id', identity, '--yes')
        assert state['state'] == 'not_created'
        assert set(p.name for p in root.iterdir()) == {'.agent-linux-owner', 'lock'}
        outcomes['delete'] = True
    assert source.read_bytes() == payload and sentinel.read_text() == 'do not touch'
    outcomes['unrelatedFilesUnchanged'] = True
    outcomes['fileRoundTripSHA256'] = hashlib.sha256(payload).hexdigest()
    report = {'passed': True, 'timingsSeconds': timings, 'outcomes': outcomes,
              'cleanMac': False, 'notes': 'Existing developer Mac; real installed bundle and new disposable guest.'}
    (evidence / 'report.json').write_text(json.dumps(report, indent=2))
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
