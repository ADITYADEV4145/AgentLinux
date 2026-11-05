#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Opt-in Phase 3.5 real Mac acceptance; creates and deletes only its own VM."""
import argparse
import base64
import json
import os
from pathlib import Path
import shutil
import statistics
import subprocess
import tempfile
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('cli', 'lima', 'evidence'):
        parser.add_argument('--' + name, type=Path, required=True)
    parser.add_argument('--observe-host', type=Path)
    parser.add_argument('--resume-state', type=Path, help='debug only: resume a previously created isolated test namespace')
    parser.add_argument('--keep-on-failure', action='store_true', help='debug only: preserve this isolated VM after failure; clean it up explicitly')
    args = parser.parse_args()
    os.umask(0o077)
    repo = Path(__file__).resolve().parents[1]
    cli, lima, evidence = args.cli.resolve(), args.lima.resolve(), args.evidence.resolve()
    evidence.mkdir(parents=True, exist_ok=False)
    root = args.resume_state.resolve() if args.resume_state else Path(tempfile.mkdtemp(prefix='agentlinux-full-desktop-', dir='/private/tmp'))
    if root.parent != Path('/private/tmp') or not root.name.startswith('agentlinux-full-desktop-') or root.is_symlink():
        raise RuntimeError('resume is limited to explicitly selected isolated test namespaces')
    (evidence / 'root.txt').write_text(str(root))
    common = ['--state-dir', str(root), '--lima', str(lima), '--json']
    identity = None
    passed = False
    test_file = '/home/agent/' + evidence.name + '-work.txt'
    checks = []
    timings = {}
    env = {'PATH': os.environ['PATH'], 'HOME': str(root), 'LIMA_HOME': str(root / 'lima'),
           'TMPDIR': str(root / 'tmp'), 'LC_ALL': 'C'}

    def call(command, *argv, error=False, action=None):
        command_args = [str(cli), command] + ([action] if action else []) + common + list(argv)
        p = subprocess.run(command_args, capture_output=True, text=True, timeout=1250)
        value = json.loads(p.stdout)
        if not value.get('ok') and not error:
            raise RuntimeError(value.get('error'))
        return value['result'] if value.get('ok') else value

    def command(*argv):
        return call('exec', '--', *argv)

    def output(result):
        assert result['exitCode'] == 0 and result['cleanupConfirmed'], result
        return base64.b64decode(result.get('stdout') or '').decode()

    def admin(*argv):
        return subprocess.run([str(lima), 'shell', '--workdir=/', 'default', 'sudo', '-n', *argv],
                              env=env, capture_output=True, text=True, check=True, timeout=60)

    def snap(name):
        return call('screenshot', '--output=' + str(evidence / (name + '.png')))

    def observe():
        return json.loads(subprocess.check_output([str(args.observe_host.resolve())], text=True)) if args.observe_host else None

    try:
        if args.resume_state:
            admin('/bin/systemctl', 'restart', 'agent-linux-desktop', 'agent-linux')
        started = time.monotonic()
        state = call('up', '--local') if args.resume_state else call('up', '--local', '--network=nat')
        timings['resumeReadySeconds' if args.resume_state else 'coldUpSeconds'] = round(time.monotonic() - started, 3)
        identity = state['machineId']
        assert state['guest']['desktop']['ready']
        (evidence / 'up.json').write_text(json.dumps(state, indent=2))
        checks.append('real boot/handshake, panel/desktop/window-manager readiness')
        host_before = observe()
        for app in ('launcher', 'files', 'settings', 'terminal'):
            print('Opening ' + app, flush=True)
            call('open', app)
            snap(app)
            if app == 'files' and not args.resume_state:
                call('key', 'CTRL+SHIFT+N')
                call('type', 'Phase35Folder')
                call('key', 'ENTER')
                output(command('/usr/bin/test', '-d', '/home/agent/Phase35Folder'))
        call('type', '--paste-mode=terminal', "printf 'terminal works\\n' > /home/agent/terminal-check.txt")
        call('key', 'ENTER')
        assert output(command('/bin/cat', '/home/agent/terminal-check.txt')) == 'terminal works\n'
        checks.append('actual launcher, Thunar, settings, terminal command via guest keyboard')
        if args.resume_state:
            command('/usr/bin/pkill', '-x', 'mousepad')  # Only the isolated acceptance editor.
        output(command('/usr/bin/touch', test_file))
        call('open', 'editor')
        call('key', 'CTRL+O')
        call('type', test_file)
        call('key', 'ENTER')
        call('open', 'editor')  # Window placement varies when several desktop apps are open.
        snap('editor-before-type')
        assert Path(test_file).name in output(command('/usr/bin/xdotool', 'getactivewindow', 'getwindowname'))
        text = 'Agent Linux desktop\ncafé — 日本語 👋\n'
        call('type', text)
        call('key', 'CTRL+A')
        call('key', 'CTRL+C')
        call('key', 'CTRL+END')
        call('key', 'CTRL+V')
        call('key', 'CTRL+S')
        assert output(command('/bin/cat', test_file)) == text + text
        snap('editor-clipboard')
        checks.append('exact saved ASCII/Unicode plus guest-owned copy/paste')
        for browser in ('chromium', 'firefox'):
            print('Testing ' + browser, flush=True)
            call('open', browser)
            call('key', 'CTRL+L')
            call('type', 'https://example.com')
            call('key', 'ENTER')
            deadline = time.monotonic() + 30
            while True:
                visible = output(command('/bin/sh', '-c', 'w=$(xdotool getactivewindow); xdotool getwindowname "$w"; xprop -id "$w" WM_CLASS'))
                if 'Example Domain' in visible and browser in visible.lower():
                    break
                if time.monotonic() > deadline:
                    raise RuntimeError('target browser did not show the public page: ' + visible)
                time.sleep(0.5)
            snap(browser)
        browsers = call('browsers', action='list')
        assert all(b['installed'] for b in browsers['browsers'][:2])
        (evidence / 'browsers.json').write_text(json.dumps(browsers, indent=2))
        checks.append('Chromium and Firefox ESR open public page, actual package versions reported')
        print('Testing package maintenance', flush=True)
        call('packages', 'jq', action='install')
        assert output(command('/usr/bin/jq', '--version')).startswith('jq-')
        missing = call('packages', 'agentlinux-acceptance-nonexistent-package', action='install', error=True)
        assert missing['error']['code'] == 'PACKAGE_UNAVAILABLE', missing
        checks.append('explicit guest-admin small package install and missing-package error')
        # Block only APT in this disposable guest; keep SSH/MCP networking operational.
        proxy = '/etc/apt/apt.conf.d/99agentlinux-acceptance-proxy'
        try:
            admin('/bin/sh', '-c', 'printf \'%s\\n\' \'Acquire::http::Proxy "http://127.0.0.1:9";\' \'Acquire::https::Proxy "http://127.0.0.1:9";\' > ' + proxy)
            unavailable = call('packages', action='refresh', error=True)
            assert unavailable['error']['code'] == 'REPOSITORY_UNAVAILABLE', unavailable
        finally:
            admin('/bin/rm', '-f', proxy)
        checks.append('APT network failure is visible, no stale-index false success')
        # Real official MCP client, new guest image, actual stdio screenshots and actions.
        sdk_env = os.environ.copy()
        sdk_env.update({'GOCACHE': str(repo / '.cache/go-build'), 'GOMODCACHE': str(repo / '.cache/go-mod'),
                        'AGENTLINUX_ACCEPT_CLI': str(cli), 'AGENTLINUX_ACCEPT_ROOT': str(root),
                        'AGENTLINUX_ACCEPT_LIMA': str(lima), 'AGENTLINUX_ACCEPT_EVIDENCE': str(evidence)})
        sdk = subprocess.run(['go', 'test', '-v', './internal/mcpserver', '-run', '^TestRealStdioGuest$', '-count=1'],
                             cwd=repo, env=sdk_env, capture_output=True, text=True, timeout=240)
        (evidence / 'mcp-test.txt').write_text(sdk.stdout + sdk.stderr)
        assert sdk.returncode == 0, sdk.stdout + sdk.stderr
        checks.append('existing official MCP stdio client sees and operates the new real guest')
        host_after = observe()
        observation = {'before': host_before, 'after': host_after}
        if host_before:
            observation['unchanged'] = host_before == host_after
            assert observation['unchanged'], observation
            checks.append('Mac pointer coordinates and clipboard counter unchanged (read-only fixture)')
        else:
            observation['status'] = 'not measured; no host observation fixture supplied'
        inventory = output(command('/usr/bin/dpkg-query', '-W'))
        (evidence / 'packages.tsv').write_text(inventory)
        baseline = output(command('/bin/sh', '-c', 'cat /etc/os-release; cat /etc/debian_version; uname -r; dpkg --print-architecture; cat /var/lib/agent-linux/recipe; cat /proc/meminfo | head -5; df -B1 /; du -sx -B1 /usr /home/agent'))
        (evidence / 'guest-system.txt').write_text(baseline)
        output(command('/usr/bin/git', '--version'))
        output(command('/usr/bin/python3', '-c', 'import venv, ssl; print("Python ready")'))
        output(command('/usr/bin/curl', '--fail', '--silent', '--show-error', 'https://example.com'))
        checks.append('default Git/Python/venv/TLS/curl work; SSH package inventory recorded')
        samples = []
        for i in range(5):
            start = time.monotonic()
            snap('timing-' + str(i))
            samples.append(round(time.monotonic() - start, 4))
        timings['screenshotSeconds'] = samples
        timings['screenshotMedianSeconds'] = statistics.median(samples)
        sizes = {}
        for name, path in {'downloadedBase': root / 'debian-arm64.qcow2', 'vmDisk': root / 'lima/default/disk'}.items():
            st = path.stat()
            sizes[name] = {'logicalBytes': st.st_size, 'allocatedBytes': st.st_blocks * 512}
        assert call('stop')['state'] == 'stopped'
        start = time.monotonic()
        restarted = call('up', '--local')
        timings['warmUpSeconds'] = round(time.monotonic() - start, 3)
        assert restarted['machineId'] == identity and restarted['guest']['bootId'] != state['guest']['bootId']
        assert restarted['guest']['desktop']['ready']
        assert output(command('/bin/cat', test_file)) == text + text
        assert output(command('/usr/bin/jq', '--version')).startswith('jq-')
        assert output(command('/bin/cat', '/home/agent/terminal-check.txt')) == 'terminal works\n'
        call('open', 'files')
        call('open', 'firefox')
        snap('after-restart')
        checks.append('real VM stop/up preserves files, installed jq and browser launch; new boot/session')
        report = {'passed': True, 'resumedDebugNamespace': bool(args.resume_state), 'checks': checks, 'guest': restarted['guest'], 'backendVersion': restarted['backendVersion'],
                  'timings': timings, 'imageSizes': sizes, 'hostObservation': observation}
        passed = True
        (evidence / 'report.json').write_text(json.dumps(report, indent=2, ensure_ascii=False))
        print(json.dumps({k: report[k] for k in ('passed', 'checks', 'timings', 'imageSizes', 'hostObservation')}, indent=2), flush=True)
    finally:
        if args.keep_on_failure and not passed:
            print('Debug VM preserved at ' + str(root) + '; stop and remove only this namespace after diagnosis.', flush=True)
        else:
            if (root / 'state.json').exists():
                actual = json.loads((root / 'state.json').read_text())
                if identity and actual['machineId'] != identity:
                    raise RuntimeError('cleanup identity mismatch; preserving VM')
                if (root / 'lima/default').exists():
                    if call('stop')['state'] != 'stopped':
                        raise RuntimeError('shutdown unconfirmed; preserving VM')
                    subprocess.run([str(lima), 'delete', '--force', 'default'], env=env, check=True, capture_output=True, timeout=60)
            if root.parent != Path('/private/tmp') or not root.name.startswith('agentlinux-full-desktop-') or root.is_symlink():
                raise RuntimeError('unsafe cleanup path')
            shutil.rmtree(root)
            print('Removed only owned Phase 3.5 VM/state; private evidence retained.', flush=True)



if __name__ == '__main__':
    main()
