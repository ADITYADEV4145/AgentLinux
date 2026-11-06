#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Opt-in local Chrome/CDP acceptance; never uses a personal browser profile.

Attach to an already running disposable fixture VM + viewer. --restart explicitly
stops/restarts only an agentlinux-viewer-acceptance-* test namespace.
"""
import argparse
import base64
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
import urllib.error
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('cli', 'lima', 'state-dir', 'viewer-output', 'evidence', 'chrome'):
        parser.add_argument('--' + name, type=Path, required=True)
    parser.add_argument('--restart', action='store_true')
    parser.add_argument('--interactive', action='store_true', help='verify real guest text/click/shortcut through the human web widgets')
    args = parser.parse_args()
    import websocket  # Test-only websocket-client, not a product dependency.
    os.umask(0o077)
    evidence = args.evidence.resolve()
    evidence.mkdir(parents=True, exist_ok=False)
    root = args.state_dir.resolve()
    if root.parent != Path('/private/tmp') or not root.name.startswith(('agentlinux-viewer-acceptance-', 'agentlinux-rc-acceptance-', 'agentlinux-launch-acceptance-')):
        raise RuntimeError('acceptance requires an explicitly selected disposable test namespace')
    identity = json.loads((root / 'state.json').read_text())['machineId']
    viewer_url = next(word for word in args.viewer_output.read_text().split() if word.startswith('http://127.0.0.1:'))
    origin, token = viewer_url.split('/#', 1)
    profile = Path(tempfile.mkdtemp(prefix='agentlinux-viewer-chrome-', dir='/private/tmp'))
    log = (evidence / 'chrome-stderr.txt').open('w')
    started = time.monotonic()
    chrome = subprocess.Popen([str(args.chrome), '--headless=new', '--no-first-run', '--no-default-browser-check',
                               '--user-data-dir=' + str(profile), '--remote-debugging-port=0',
                               '--remote-debugging-address=127.0.0.1', '--disable-background-networking', 'about:blank'],
                              stdout=subprocess.DEVNULL, stderr=log)
    ws = None
    try:
        deadline = time.monotonic() + 15
        while not (profile / 'DevToolsActivePort').exists():
            if time.monotonic() > deadline:
                raise RuntimeError('isolated Chrome did not start')
            time.sleep(.1)
        port = (profile / 'DevToolsActivePort').read_text().splitlines()[0]
        targets = json.load(urllib.request.urlopen('http://127.0.0.1:' + port + '/json'))
        target = next(t for t in targets if t['type'] == 'page')
        ws = websocket.create_connection(target['webSocketDebuggerUrl'], suppress_origin=True, timeout=15)
        seq = 0

        def cdp(method, params=None):
            nonlocal seq
            seq += 1
            ws.send(json.dumps({'id': seq, 'method': method, 'params': params or {}}))
            while True:
                value = json.loads(ws.recv())
                if value.get('id') == seq:
                    if value.get('error'):
                        raise RuntimeError(value['error'])
                    return value.get('result', {})

        def evaluate(expression):
            return cdp('Runtime.evaluate', {'expression': expression, 'returnByValue': True})['result'].get('value')

        def wait_for(predicate, timeout=20):
            end = time.monotonic() + timeout
            while time.monotonic() < end:
                if predicate():
                    return
                time.sleep(.2)
            raise RuntimeError('live viewer condition timed out')

        def capture(name):
            data = cdp('Page.captureScreenshot', {'format': 'png'})['data']
            (evidence / (name + '.png')).write_bytes(base64.b64decode(data))

        def api(path, auth=True, headers=None):
            h = {'Authorization': 'Bearer ' + token} if auth else {}
            h.update(headers or {})
            req = urllib.request.Request(origin + path, headers=h)
            try:
                with urllib.request.urlopen(req, timeout=15) as result:
                    return result.status, result.read()
            except urllib.error.HTTPError as exc:
                return exc.code, exc.read()

        def ctl(command, *argv):
            for _ in range(30):
                p = subprocess.run([str(args.cli.resolve()), command, '--state-dir', str(root),
                                    '--lima', str(args.lima.resolve()), '--json', *argv],
                                   capture_output=True, text=True, timeout=1250)
                value = json.loads(p.stdout)
                if value.get('ok'):
                    return value['result']
                if value.get('error', {}).get('code') != 'MACHINE_BUSY':
                    raise RuntimeError(value.get('error'))
                time.sleep(.2)
            raise RuntimeError('fixture machine remained busy')

        assert api('/api/frame', auth=False)[0] == 401
        assert api('/api/frame', headers={'Origin': 'https://evil.example'})[0] == 403
        assert api('/api/frame', headers={'Host': 'evil.example'})[0] == 403
        cdp('Page.enable')
        cdp('Page.navigate', {'url': viewer_url})
        wait_for(lambda: evaluate("document.getElementById('state')?.textContent.includes('LIVE')"))
        assert evaluate("document.getElementById('desktop').naturalWidth") == 1280
        assert evaluate("location.hash") == ''
        capture('web-live-before')
        # A real official MCP client changes the guest while the browser stays connected.
        env = os.environ.copy()
        repo = Path(__file__).resolve().parents[1]
        env.update({'GOCACHE': str(repo / '.cache/go-build'), 'GOMODCACHE': str(repo / '.cache/go-mod'),
                    'AGENTLINUX_ACCEPT_CLI': str(args.cli.resolve()), 'AGENTLINUX_ACCEPT_ROOT': str(root),
                    'AGENTLINUX_ACCEPT_LIMA': str(args.lima.resolve()), 'AGENTLINUX_ACCEPT_EVIDENCE': str(evidence)})
        p = subprocess.run(['go', 'test', '-v', './internal/mcpserver', '-run', '^TestRealStdioGuest$', '-count=1'],
                           cwd=repo, env=env, capture_output=True, text=True, timeout=240)
        (evidence / 'mcp-test.txt').write_text(p.stdout + p.stderr)
        assert p.returncode == 0, p.stdout + p.stderr
        time.sleep(6)
        capture('web-live-after-mcp')
        assert (evidence / 'web-live-before.png').read_bytes() != (evidence / 'web-live-after-mcp.png').read_bytes()
        if args.interactive:
            # Prepare a fresh known guest file before taking human ownership.
            ctl('exec','--','/usr/bin/touch','/home/agent/web-human.txt')
            ctl('open','editor')
            ctl('key','CTRL+O');ctl('type','/home/agent/web-human.txt');ctl('key','ENTER')
            time.sleep(1)
            ctl('key','ALT+F10')  # Maximize the actual guest editor.
            wait_for(lambda:evaluate("!document.getElementById('control').hidden"))
            evaluate("document.getElementById('control').click()")
            wait_for(lambda:evaluate("document.getElementById('control').textContent.includes('Give control')"))
            # Click the image center through Chrome, not the Mac pointer. A
            # maximized guest editor has its text area at the desktop center.
            center=evaluate("(()=>{const r=document.getElementById('desktop').getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()")
            cdp('Input.dispatchMouseEvent',{'type':'mousePressed','x':center['x'],'y':center['y'],'button':'left','clickCount':1})
            cdp('Input.dispatchMouseEvent',{'type':'mouseReleased','x':center['x'],'y':center['y'],'button':'left','clickCount':1})
            wait_for(lambda:evaluate('!inputBusy'))
            message='Web human café 日本語 👋'
            evaluate("document.getElementById('text').value="+json.dumps(message))
            evaluate("document.getElementById('send').click()")
            wait_for(lambda:evaluate("!inputBusy && document.getElementById('text').value===''"))
            evaluate("document.getElementById('key').value='CTRL+S';document.getElementById('press').click()")
            wait_for(lambda:evaluate('!inputBusy'))
            evaluate("document.getElementById('control').click()")
            wait_for(lambda:evaluate("document.getElementById('control').textContent==='Take control'"))
            def saved():
                value=ctl('exec','--','/usr/bin/cat','/home/agent/web-human.txt')
                return base64.b64decode(value.get('stdout') or '').decode()==message
            wait_for(saved)
            capture('web-human-verified')
        evaluate("document.getElementById('pause').click()")
        time.sleep(2)
        before = json.loads(api('/api/metrics')[1])['captureAttempts']
        time.sleep(3)
        after = json.loads(api('/api/metrics')[1])['captureAttempts']
        assert before == after, 'paused page still captures'
        evaluate("document.getElementById('pause').click()")
        if args.restart:
            assert ctl('stop')['state'] == 'stopped'
            wait_for(lambda: evaluate("document.getElementById('desktop').style.display === 'none'"))
            capture('web-disconnected')
            assert ctl('up', '--local')['state'] == 'ready'
            wait_for(lambda: evaluate("document.getElementById('state').textContent.includes('LIVE')"))
            capture('web-reconnected')
        assert ctl('status')['machineId'] == identity
        report = {'passed': True, 'browser': subprocess.check_output([str(args.chrome), '--version'], text=True).strip(),
                  'checks': ['live real PNG in isolated browser', 'no-token/cross-origin/Host rejection',
                             'fragment removed from URL', 'actual MCP changes appear live', 'pause stops capture'],
                  'humanInputTested':args.interactive,'restartTested': args.restart, 'durationSeconds': time.monotonic()-started,
                  'clientMetrics': evaluate('window.agentLinuxViewerMetrics'), 'metrics': json.loads(api('/api/metrics')[1])}
        (evidence / 'report.json').write_text(json.dumps(report, indent=2))
        print(json.dumps({k: report[k] for k in ('passed', 'browser', 'checks', 'restartTested')}, indent=2))
    finally:
        if ws:
            ws.close()
        chrome.terminate()
        try:
            chrome.wait(timeout=10)
        except subprocess.TimeoutExpired:
            chrome.kill()
            chrome.wait()
        log.close()
        shutil.rmtree(profile)


if __name__ == '__main__':
    main()
