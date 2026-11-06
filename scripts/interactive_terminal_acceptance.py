#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Opt-in real Mac/Kitty test: single-command boot, human input, agent handoff.

Creates and deletes ONLY a fresh phase6 test VM. Requires an unlocked Mac,
reviewed Lima/Kitty, Go, and normal guest-image/package downloads. No cloud API,
Mac pointer, host clipboard, global configuration or personal VM is used.
"""
import argparse
import base64
import json
import hashlib
import re
import math
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time
from urllib.parse import quote


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('cli', 'lima', 'kitty', 'kitten', 'evidence'):
        parser.add_argument('--' + name, type=Path, required=True)
    parser.add_argument("--base-image", type=Path, help="optional cached public base; pinned SHA-512 verified before copying")
    args = parser.parse_args()
    os.umask(0o077)
    repo = Path(__file__).resolve().parents[1]
    cli, lima, kitty, kitten = (getattr(args, n).resolve() for n in ('cli', 'lima', 'kitty', 'kitten'))
    evidence = args.evidence.resolve()
    evidence.mkdir(mode=0o700, parents=True, exist_ok=False)
    root = Path(tempfile.mkdtemp(prefix='agentlinux-phase6-acceptance-', dir='/private/tmp'))
    socketdir = Path(tempfile.mkdtemp(prefix='agentlinux-phase6-kitty-', dir='/private/tmp'))
    sock = socketdir / 'rc.sock'
    (evidence / 'root.txt').write_text(str(root))
    if args.base_image:
        expected = re.search(r'const ImageSHA512 = "([a-f0-9]+)"', (repo/'internal/config/config.go').read_text()).group(1)
        with args.base_image.open('rb') as source:
            assert hashlib.file_digest(source, 'sha512').hexdigest() == expected
        (root/'.agent-linux-owner').write_text('agent-linux state v1\n')
        shutil.copyfile(args.base_image, root/'debian-arm64.qcow2')
    common = ['--state-dir', str(root), '--lima', str(lima), '--json']
    env = {'PATH': os.environ['PATH'], 'HOME': str(root), 'LIMA_HOME': str(root / 'lima'), 'TMPDIR': str(root / 'tmp'), 'LC_ALL': 'C'}
    log = (evidence / 'kitty.log').open('w')
    proc = subprocess.Popen([str(kitty), '--config', 'NONE', '--listen-on', 'unix:' + str(sock),
                             '-o', 'allow_remote_control=socket-only', '-o', 'shell_integration=disabled',
                             '-o', 'confirm_os_window_close=0', '-o', 'macos_quit_when_last_window_closed=yes',
                             'python3', '-c', 'import time; time.sleep(3600)'],
                            stdout=log, stderr=log, start_new_session=True)
    base = [str(kitten), '@', '--to', 'unix:' + str(sock)]
    wid = None
    report = {'passed': False, 'simulated': False, 'singleCommand': 'agent-linux start', 'checks': []}

    def rc(*a, data=None):
        return subprocess.check_output(base + list(a), input=data, stderr=subprocess.PIPE, timeout=15).decode()

    def wait(test, timeout=25):
        end = time.monotonic() + timeout
        while time.monotonic() < end:
            if proc.poll() is not None:
                raise RuntimeError("isolated Kitty exited; test cannot continue")
            try:
                if test():
                    return
            except subprocess.CalledProcessError:
                pass  # A read-only guest oracle may not exist yet; never replay an action.
            time.sleep(.2)
        raise RuntimeError('acceptance condition timed out')

    def text():
        return rc('get-text', '--match', 'id:' + str(wid))

    def send(s):
        rc('send-text', '--match', 'id:' + str(wid), '--stdin', data=s.encode())
        time.sleep(.3)

    def ctl(action, *a, error=False):
        for attempt in range(20):
            x = subprocess.run([str(cli), action, *common, *a], capture_output=True, text=True, timeout=1300)
            value = json.loads(x.stdout)
            if value.get('ok'):
                return value['result']
            if value.get('error', {}).get('code') == 'MACHINE_BUSY' and action in ('status', 'screenshot', 'stop'):
                time.sleep(.2)
                continue
            if error:
                return value
            raise RuntimeError(value.get('error'))
        raise RuntimeError('read-only status remained busy')

    def admin(*a):
        return subprocess.check_output([str(lima), 'shell', '--workdir=/', 'default', 'sudo', '-n', *a], env=env, stderr=subprocess.PIPE, timeout=20).decode()

    def active_title():
        return admin('-u', 'agent', 'env', 'DISPLAY=:1', 'XAUTHORITY=/run/agent-linux-desktop/Xauthority',
                     'DBUS_SESSION_BUS_ADDRESS=unix:path=/run/agent-linux-desktop/bus', 'xdotool', 'getactivewindow', 'getwindowname')

    def release():
        send('\x1bOQ')
        wait(lambda: 'AGENT CAN CONTROL' in text())

    def acquire():
        send('\x1bOQ')
        wait(lambda: 'YOU CONTROL' in text())

    try:
        wait(lambda: sock.exists())
        folder = evidence / 'terminal'
        folder.mkdir()
        started = time.monotonic()
        wid = int(rc('launch', '--type=os-window', '--os-window-title=Agent Linux interactive acceptance',
                     sys.executable, str(repo / 'scripts/viewer_terminal_probe.py'), '--start',
                     '--cli', str(cli), '--lima', str(lima), '--state-dir', str(root), '--evidence', str(folder)).strip())
        print('Waiting for real single-command VM boot and input ownership', flush=True)
        wait(lambda: 'YOU CONTROL' in text(), timeout=1250)
        report['bootAndViewerSeconds'] = time.monotonic() - started
        state = ctl('status')
        assert state['guest']['guestVersion'] == '0.6.0' and state['guest']['desktop']['ready'], state
        report['guest'] = state['guest']
        rc('screenshot', '--match', 'id:' + str(wid), str(evidence / 'single-command.png'))
        report['checks'].append('one start command booted real VM, desktop handshake, Kitty image, human lease')
        denied = ctl('key', 'ENTER', error=True)
        assert denied['error']['code'] == 'INPUT_OWNED', denied
        report['checks'].append('separate agent/CLI mutation denied while human owns input')
        release()
        ctl('exec', '--', '/usr/bin/touch', '/home/agent/human-editor.txt')
        ctl('open', 'editor')
        acquire()
        send('\x0f')  # Ctrl+O into Linux, not the Mac.
        send('\x1b[200~/home/agent/human-editor.txt\x1b[201~')
        send('\r')
        wait(lambda: 'human-editor.txt' in active_title())
        message = 'Human q typing works\ncafé — 日本語 👋\n'
        send('\x1b[200~' + message + '\x1b[201~')
        send('\x13')
        wait(lambda: admin('cat', '/home/agent/human-editor.txt') == message)
        report['checks'].append('real PTY keyboard/Unicode paste and exact saved editor bytes; q is ordinary text')
        ctl('screenshot', '--output', str(evidence / 'human-editor.png'))
        release()
        ctl('open', 'terminal')
        acquire()
        command = "printf '%s' 'terminal café 日本語' > /home/agent/human-terminal.txt"
        send('\x1b[200~' + command + '\x1b[201~')
        send('\r')
        wait(lambda: admin('cat', '/home/agent/human-terminal.txt') == 'terminal café 日本語')
        send('\x03')  # guest Ctrl+C must not close the interactive viewer.
        assert 'YOU CONTROL' in text()
        report['checks'].append('automatic terminal paste; guest shell command; Ctrl+C stays in Linux')
        release()
        ctl('open', 'chromium')
        page = '<meta charset="utf-8"><title>Human fixture</title><style>body{font:24px sans-serif;background:#dbeafe}input,button{position:absolute;top:100px;height:50px;font:24px sans-serif}input{left:100px;width:300px}button{left:450px;width:150px}</style><h1>Real Linux human input</h1><form onsubmit="event.preventDefault();document.title=\'Human verified \'+document.querySelector(\'input\').value"><input><button>Verify</button></form>'
        ctl('key', 'CTRL+L')
        ctl('type', 'data:text/html;charset=utf-8,' + quote(page))
        ctl('key', 'ENTER')
        ctl('key', 'F11')
        wait(lambda: 'Human fixture' in active_title())
        acquire()
        size = json.loads((folder / 'tty-size.json').read_text())
        cols, rows, px, py = size['columns'], size['rows'], size['pixelWidth'], size['pixelHeight']
        cw, ch = px / cols, py / rows
        drawcols = math.floor(min(px, (rows - 2) * ch * 1280 / 800) / cw)
        drawrows = math.ceil(drawcols * cw * 800 / 1280 / ch)
        column, row = 1 + (cols - drawcols) // 2, 2 + (rows - 2 - drawrows) // 2
        col = math.floor(column - 1 + 150 / 1280 * drawcols) + 1
        line = math.floor(row - 1 + 120 / 800 * (drawcols * cw * 800 / 1280) / ch) + 1
        send(f'\x1b[<0;{col};{line}M\x1b[<0;{col};{line}m')
        send('\x1b[200~human café 日本語\x1b[201~')
        send('\t\r')
        wait(lambda: 'Human verified human café 日本語' in active_title())
        ctl('screenshot', '--output', str(evidence / 'human-browser.png'))
        report['checks'].append('SGR terminal mouse mapped through actual image placement; Unicode browser form verified')
        release()
        # Deliberately keep Chromium fullscreen: handoff must recover keyboard focus.
        sdk_env = os.environ.copy()
        sdk_env.update(GOCACHE=str(repo / '.cache/go-build'), GOMODCACHE=str(repo / '.cache/go-mod'),
                       AGENTLINUX_ACCEPT_FOCUS_DEBUG="1", AGENTLINUX_ACCEPT_CLI=str(cli), AGENTLINUX_ACCEPT_ROOT=str(root),
                       AGENTLINUX_ACCEPT_LIMA=str(lima), AGENTLINUX_ACCEPT_EVIDENCE=str(evidence))
        test = subprocess.run(['go', 'test', '-v', './internal/mcpserver', '-run', '^TestRealStdioGuest$', '-count=1'],
                              cwd=repo, env=sdk_env, capture_output=True, text=True, timeout=200)
        (evidence / 'mcp.txt').write_text(test.stdout + test.stderr)
        assert test.returncode == 0, test.stdout + test.stderr
        report['checks'].append('real official-SDK MCP operates same guest after human handoff while viewer remains open')
        rc('resize-os-window', '--match', 'id:' + str(wid), '--unit=cells', '--width=80', '--height=30')
        time.sleep(2)
        rc('screenshot', '--match', 'id:' + str(wid), str(evidence / 'resized.png'))
        send('\x1d')  # Ctrl+] exits only the viewer.
        wait(lambda: (folder / 'terminal-exit.json').exists())
        send('restoration-probe\n')
        wait(lambda: json.loads((folder / 'terminal-exit.json').read_text()).get('canonicalInputWorks'))
        restored = json.loads((folder / 'terminal-exit.json').read_text())
        assert restored['exitCode'] == 0 and restored['terminalStateRestored'] and restored['inputModesRestored'], restored
        assert ctl('status')['state'] == 'ready'
        assert not (root / 'input-lease.json').exists()
        report['terminal'] = restored
        report['checks'].append('resize, Ctrl+] exit, terminal restoration, lease release, VM still ready')
        report['passed'] = True
    except BaseException as exc:
        report['failureType'] = type(exc).__name__
        if wid is not None:
            try:
                rc('screenshot', '--match', 'id:' + str(wid), str(evidence / 'failure-terminal.png'))
                (evidence / 'failure-text.txt').write_text(text())
            except Exception:
                pass
        raise
    finally:
        (evidence / 'report.json').write_text(json.dumps(report, indent=2))
        if wid is not None:
            try:
                rc('close-window', '--match', 'id:' + str(wid))
            except Exception:
                pass
        proc.terminate()
        try:
            proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait()
        log.close()
        if sock.exists():
            sock.unlink()
        socketdir.rmdir()
        # Deliberate disposable-only cleanup; no default namespace is consulted.
        if (root / 'state.json').exists():
            saved = json.loads((root / 'state.json').read_text())
            stopped = ctl('stop', error=True)
            if isinstance(stopped, dict) and stopped.get('error'):
                assert 'recorded VM missing' in stopped['error']['message'], stopped
            # A killed downloader cannot run its defer; remove only this owned
            # fixture's known regular partial-download files before guarded delete.
            for partial in root.glob('.download-*'):
                assert partial.is_file() and not partial.is_symlink()
                partial.unlink()
            ctl('delete', '--machine-id', saved['machineId'], '--yes')
            assert set(x.name for x in root.iterdir()) == {'.agent-linux-owner', 'lock'}
            report['cleanup'] = 'only owned disposable VM removed; marker/lock retained'
            (evidence / 'report.json').write_text(json.dumps(report, indent=2))
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
