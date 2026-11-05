#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Opt-in Mac acceptance: real X11 UI, guest commands and restart/cancellation checks."""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import tempfile
import time
from urllib.parse import quote


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--cli', type=Path, required=True)
    parser.add_argument('--lima', type=Path, required=True)
    parser.add_argument('--image', type=Path, help='optional copy of the checksum-pinned image')
    parser.add_argument('--evidence', type=Path, required=True, help='new directory, not overwritten')
    parser.add_argument('--observe-host', type=Path, help='optional compiled read-only observe_host.swift fixture')
    args = parser.parse_args()
    cli, lima = args.cli.resolve(), args.lima.resolve()
    evidence = args.evidence.resolve()
    evidence.mkdir(parents=True, exist_ok=False)
    root = Path(tempfile.mkdtemp(prefix='agentlinux-desktop-acceptance-', dir='/private/tmp'))
    common = ['--state-dir', str(root), '--lima', str(lima), '--json']
    image_manifest = json.loads((Path(__file__).resolve().parents[1] / 'images/debian/manifest.json').read_text())
    identity = None
    checks = []
    active_command = None

    def call(command, *argv, allow_error=False):
        p = subprocess.run([str(cli), command] + common + list(argv), capture_output=True, text=True, timeout=1250)
        result = json.loads(p.stdout)
        if not result.get('ok') and not allow_error:
            raise RuntimeError(result.get('error'))
        return result.get('result') if result.get('ok') else result

    def command(*argv, timeout='20s', output_limit=65536):
        return call('exec', '--command-timeout=' + timeout, '--output-limit=' + str(output_limit), '--', *argv)

    def stdout(result):
        return base64.b64decode(result.get('stdout') or '').decode('utf-8')

    def snap(name):
        frame = call('screenshot', '--output=' + str(evidence / (name + '.png')))
        assert frame['width'] == 1280 and frame['height'] == 800 and frame['scale'] == 1
        return frame

    def observe_host():
        if not args.observe_host:
            return None
        return json.loads(subprocess.check_output([str(args.observe_host.resolve())], text=True))

    try:
        up = ['--local', '--network=nat', '--timeout=20m']
        if args.image:
            up += ['--image=' + str(args.image.resolve()), '--image-sha512=' + image_manifest['sha512']]
        before = call('up', *up)
        identity = before['machineId']
        assert before['guest']['desktop']['ready']
        assert before['guest']['capabilities']['arbitraryGuestExec']
        checks.append('actual X11 desktop readiness')
        host_before = observe_host()
        snap('desktop')
        assert command('/usr/bin/touch', '/home/agent/acceptance.txt')['exitCode'] == 0
        call('open', 'editor')
        call('key', 'CTRL+O')
        call('type', '/home/agent/acceptance.txt')
        call('key', 'ENTER')
        call('click', '300', '250')
        call('click', '300', '250')  # Regression: repeated same-coordinate move must not hang.
        call('type', 'ASCII: hello from Agent Linux\n')
        call('type', 'Unicode: café — 日本語 👋\n')
        call('key', 'CTRL+S')
        expected = 'ASCII: hello from Agent Linux\nUnicode: café — 日本語 👋\n'
        assert stdout(command('/bin/cat', '/home/agent/acceptance.txt')) == expected
        snap('editor')
        checks.append('real editor ASCII/Unicode input and exact saved file')
        call('open', 'browser')
        html = '<meta charset="utf-8"><title>Agent Linux acceptance</title><body style="height:4000px;background:linear-gradient(#e0f2fe,#7c3aed)"><h1>Linux guest browser</h1><p>ASCII and Unicode: café — 日本語 👋</p>'
        call('key', 'CTRL+L')
        call('type', 'data:text/html;charset=utf-8,' + quote(html))
        call('key', 'ENTER')
        call('click', '--clicks=2', '220', '220')
        call('click', '--button=right', '800', '500')
        call('key', 'ESC')
        call('drag', '700', '50', '720', '70')
        call('scroll', '800', '600', '1', '2')
        call('move', '850', '600')
        assert stdout(command('/usr/bin/xdotool', 'getmouselocation')).startswith('x:850 y:600 ')
        snap('browser')
        assert hashlib.sha256((evidence / 'desktop.png').read_bytes()).digest() != hashlib.sha256((evidence / 'browser.png').read_bytes()).digest()
        checks.append('visible Chromium, changed PNG, double/right-click, drag, scroll, exact guest pointer')
        host_after = observe_host()
        if host_before:
            observation = {'pointerUnchanged': host_before['pointerX'] == host_after['pointerX'] and host_before['pointerY'] == host_after['pointerY'],
                           'clipboardCounterUnchanged': host_before['clipboardChangeCount'] == host_after['clipboardChangeCount']}
            if not all(observation.values()):
                raise RuntimeError('host observation changed; exclude external user activity before interpreting this test')
            checks.append('host pointer and clipboard change counter unchanged; no clipboard contents read')
        else:
            observation = {'notRun': 'optional host observation fixture not supplied'}
        r = command('/bin/sh', '-c', 'printf stdout; printf stderr >&2; exit 7')
        assert r['exitCode'] == 7 and stdout(r) == 'stdout' and base64.b64decode(r['stderr']) == b'stderr'
        r = command('/bin/sh', '-c', 'setsid /bin/sleep 60 >/dev/null 2>&1 & echo $!; wait', timeout='300ms')
        assert r['timedOut'] and r['cleanupConfirmed']
        child = int(stdout(r).strip())
        assert command('/bin/sh', '-c', f'test ! -e /proc/{child}/stat || test "$(cut -d" " -f3 /proc/{child}/stat)" = Z')['exitCode'] == 0
        r = command('/bin/sh', '-c', 'while :; do printf flood; done', output_limit=64)
        assert r['truncated'] and r['cleanupConfirmed'] and len(base64.b64decode(r['stdout'] or '')) <= 64
        p = subprocess.Popen([str(cli), 'exec'] + common + ['--timeout=75s', '--command-timeout=60s', '--', '/bin/sh', '-c', 'echo $$ > /home/agent/cancel.pid; sleep 60'], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        active_command = p
        time.sleep(2)
        p.send_signal(signal.SIGINT)
        canceled, _ = p.communicate(timeout=15)
        active_command = None
        assert not json.loads(canceled)['ok']
        assert command('/bin/sh', '-c', 'p=$(cat /home/agent/cancel.pid); test ! -e /proc/$p/stat || test "$(cut -d" " -f3 /proc/$p/stat)" = Z')['exitCode'] == 0
        checks.append('guest stdout/stderr/exit, timeout incl detached child, output flood, disconnect cancellation')
        frame = snap('before-display-restart')
        command('/usr/bin/pkill', '-KILL', '-x', 'Xvfb')
        after = call('status', '--timeout=60s')
        assert after['guest']['desktop']['generation'] != frame['generation']
        rejected = call('click', '--frame-id=' + frame['id'], '20', '20', allow_error=True)
        assert rejected['error']['code'] == 'STALE_FRAME'
        checks.append('display crash recovery and stale frame rejection')
        old_session = after['guest']['sessionId']
        call('exec', '--', '/usr/bin/pkill', '-KILL', '-f', '^/usr/local/bin/agent-linux-guest --machine-id=', allow_error=True)
        recovered = call('status', '--timeout=60s')
        assert recovered['guest']['sessionId'] != old_session
        checks.append('guest helper crash recovery and new service session')
        package_versions = stdout(command('/usr/bin/dpkg-query', '-W', 'xvfb', 'xfce4-session', 'xdotool', 'scrot', 'xclip', 'mousepad', 'chromium'))
        assert call('stop')['state'] == 'stopped'
        restarted = call('up', '--local', '--timeout=90s')
        assert restarted['machineId'] == identity
        assert restarted['guest']['bootId'] != before['guest']['bootId']
        assert restarted['guest']['desktop']['ready']
        assert stdout(command('/bin/cat', '/home/agent/acceptance.txt')) == expected
        assert call('stop')['state'] == 'stopped'
        checks.append('VM stop/restart, new boot/display/service, persisted guest file')
        report = {'passed': True, 'hostObservation': observation, 'guestVersion': restarted['guest']['guestVersion'],
                  'protocol': restarted['guest']['protocol'], 'backendVersion': restarted['backendVersion'],
                  'capabilities': restarted['guest']['capabilities'], 'packageVersions': package_versions, 'checks': checks}
        (evidence / 'report.json').write_text(json.dumps(report, indent=2, ensure_ascii=False))
        print(json.dumps(report, indent=2, ensure_ascii=False), flush=True)
    finally:
        if active_command is not None and active_command.poll() is None:
            active_command.send_signal(signal.SIGINT)
            active_command.communicate(timeout=15)
        if (root / 'state.json').exists():
            state = json.loads((root / 'state.json').read_text())
            if identity and identity != state.get('machineId'):
                raise RuntimeError('cleanup identity mismatch; preserving test resources')
            if (root / 'lima/default').exists():
                stopped = call('stop')
                if stopped['state'] != 'stopped':
                    raise RuntimeError('cleanup shutdown unconfirmed: ' + str(root))
                env = {'PATH': os.environ['PATH'], 'HOME': str(root), 'LIMA_HOME': str(root / 'lima'), 'TMPDIR': str(root / 'tmp'), 'LC_ALL': 'C'}
                subprocess.run([str(lima), 'delete', '--force', 'default'], env=env, check=True, capture_output=True, timeout=60)
        if root.is_symlink() or root.parent != Path('/private/tmp') or not root.name.startswith('agentlinux-desktop-acceptance-'):
            raise RuntimeError('unsafe cleanup path')
        shutil.rmtree(root)
        print('Removed only owned desktop acceptance VM/state; evidence retained.', flush=True)


if __name__ == '__main__':
    main()
