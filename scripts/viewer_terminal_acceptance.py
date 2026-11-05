#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Opt-in real Kitty/VM acceptance. Only controls a supplied isolated test socket.

Requires Kitty 0.50.0, an unlocked Mac, Pillow, Go, and optional ffmpeg. The VM
must be an explicitly selected agentlinux-viewer-acceptance-* fixture. No Mac
pointer, clipboard, global terminal settings or personal shell is controlled.
"""
import argparse
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import threading
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('cli', 'lima', 'state-dir', 'kitten', 'socket', 'evidence'):
        parser.add_argument('--' + name, type=Path, required=True)
    parser.add_argument('--ffmpeg', type=Path)
    parser.add_argument('--restart', action='store_true')
    args = parser.parse_args()
    os.umask(0o077)
    from PIL import Image, ImageChops, ImageStat  # Test-only, no product dependency.
    repo = Path(__file__).resolve().parents[1]
    evidence = args.evidence.resolve()
    evidence.mkdir(parents=True, exist_ok=False)
    root = args.state_dir.resolve()
    if root.parent != Path('/private/tmp') or not root.name.startswith('agentlinux-viewer-acceptance-'):
        raise RuntimeError('only an explicitly selected isolated test VM is accepted')
    identity = json.loads((root / 'state.json').read_text())['machineId']
    base = [str(args.kitten.resolve()), '@', '--to', 'unix:' + str(args.socket.resolve())]
    windows = []
    frames = []
    capture_errors = []
    stop_recording = threading.Event()
    record_thread = None
    started = time.monotonic()

    def rc(*argv):
        return subprocess.check_output(base + list(argv), text=True, stderr=subprocess.PIPE, timeout=15)

    def wait_for(test, timeout=20):
        end = time.monotonic() + timeout
        while time.monotonic() < end:
            if test():
                return
            time.sleep(.2)
        raise RuntimeError('terminal acceptance condition timed out')

    def text(wid):
        return rc('get-text', '--match', 'id:' + str(wid))

    def snapshot(wid, name):
        path = evidence / (name + '.png')
        rc('screenshot', '--match', 'id:' + str(wid), str(path))
        return path

    def launch(name):
        folder = evidence / name
        folder.mkdir(mode=0o700)
        command = [sys.executable, str(repo / 'scripts/viewer_terminal_probe.py'),
                   '--cli', str(args.cli.resolve()), '--state-dir', str(root),
                   '--lima', str(args.lima.resolve()), '--evidence', str(folder)]
        wid = int(rc('launch', '--type=os-window', '--os-window-title=Agent Linux viewer acceptance', *command).strip())
        windows.append(wid)
        wait_for(lambda: (folder / 'viewer-pid.txt').exists() and 'LIVE' in text(wid))
        return wid, folder

    def exit_probe(wid, folder, mode):
        if mode == 'sigterm':
            os.kill(int((folder / 'viewer-pid.txt').read_text()), signal.SIGTERM)
        else:
            rc('send-text', '--match', 'id:' + str(wid), 'q' if mode == 'q' else '\x03')
        wait_for(lambda: (folder / 'terminal-exit.json').exists())
        rc('send-text', '--match', 'id:' + str(wid), 'restoration-probe\n')
        wait_for(lambda: json.loads((folder / 'terminal-exit.json').read_text()).get('canonicalInputWorks'))
        result = json.loads((folder / 'terminal-exit.json').read_text())
        assert result['exitCode'] == 0 and result['terminalStateRestored'], result
        assert result['before'] == result['afterCanonicalInput'], result
        rc('close-window', '--match', 'id:' + str(wid))
        windows.remove(wid)
        return result

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
        raise RuntimeError('fixture remained busy')

    try:
        wid, folder = launch('normal-exit')
        # Wait for actual guest pixels, not merely a LIVE status or first GPU snapshot.
        def visible_frame(name='terminal-before'):
            candidate = snapshot(wid, name)
            picture = Image.open(candidate).convert('RGB')
            body = picture.crop((0, picture.height // 20, picture.width, picture.height * 19 // 20))
            return sum(ImageStat.Stat(body).mean) / 3 > 2
        wait_for(visible_frame)
        before = evidence / 'terminal-before.png'
        video_dir = evidence / 'recording-frames'
        video_dir.mkdir(mode=0o700)

        def record():
            while not stop_recording.is_set():
                path = video_dir / ('frame-%05d.png' % len(frames))
                try:
                    captured = time.monotonic()
                    rc('screenshot', '--match', 'id:' + str(wid), str(path))
                    frames.append({'file': str(path), 'seconds': captured - started})
                except Exception as exc:
                    capture_errors.append(type(exc).__name__)
                    return
                stop_recording.wait(.5)

        record_thread = threading.Thread(target=record)
        record_thread.start()
        env = os.environ.copy()
        env.update({'GOCACHE': str(repo / '.cache/go-build'), 'GOMODCACHE': str(repo / '.cache/go-mod'),
                    'AGENTLINUX_ACCEPT_CLI': str(args.cli.resolve()), 'AGENTLINUX_ACCEPT_ROOT': str(root),
                    'AGENTLINUX_ACCEPT_LIMA': str(args.lima.resolve()), 'AGENTLINUX_ACCEPT_EVIDENCE': str(evidence)})
        p = subprocess.run(['go', 'test', '-v', './internal/mcpserver', '-run', '^TestRealStdioGuest$', '-count=1'],
                           cwd=repo, env=env, capture_output=True, text=True, timeout=240)
        (evidence / 'mcp-test.txt').write_text(p.stdout + p.stderr)
        assert p.returncode == 0, p.stdout + p.stderr
        # Static-screen backoff can delay noticing the last change by five seconds.
        time.sleep(6)
        after = snapshot(wid, 'terminal-after-mcp')
        a, b = Image.open(before).convert('RGB'), Image.open(after).convert('RGB')
        assert a.size == b.size
        region = (int(a.width * .1), int(a.height * .1), int(a.width * .9), int(a.height * .9))
        difference = sum(ImageStat.Stat(ImageChops.difference(a.crop(region), b.crop(region))).mean) / 3
        assert difference > 10, 'guest image region did not visibly change; status text alone is insufficient'
        resizes = []
        for name, columns, rows in [('narrow', 80, 30), ('wide', 140, 25), ('tall', 70, 45), ('small', 18, 5), ('restored', 120, 40)]:
            rc('resize-os-window', '--match', 'id:' + str(wid), '--unit=cells', '--width=' + str(columns), '--height=' + str(rows))
            time.sleep(.8)
            info = json.loads(rc('ls'))
            w = next(w for o in info for t in o['tabs'] for w in t['windows'] if w['id'] == wid)
            assert w['columns'] > 0 and w['lines'] > 0
            if w['columns'] < 20 or w['lines'] < 6:
                wait_for(lambda: 'RESIZE' in text(wid) and 'q exits' in text(wid))
            if w['columns'] >= 20 and w['lines'] >= 6:
                wait_for(lambda: visible_frame('terminal-' + name))
            else:
                snapshot(wid, 'terminal-' + name)
            resizes.append({'name': name, 'requestedColumns': columns, 'requestedRows': rows,
                            'columns': w['columns'], 'rows': w['lines'],
                            'windowManagerClamped': (w['columns'], w['lines']) != (columns, rows)})
        if args.restart:
            assert ctl('stop')['state'] == 'stopped'
            wait_for(lambda: 'NOT_READY' in text(wid))
            snapshot(wid, 'terminal-disconnected')
            assert ctl('up', '--local')['state'] == 'ready'
            wait_for(lambda: 'LIVE' in text(wid))
            wait_for(lambda: visible_frame('terminal-reconnected'))
        stop_recording.set()
        record_thread.join(timeout=20)
        assert not record_thread.is_alive() and not capture_errors, capture_errors
        normal = exit_probe(wid, folder, 'q')
        state = ctl('status')
        assert state['state'] == 'ready' and state['machineId'] == identity
        exits = {'q': normal}
        for mode in ('ctrl-c', 'sigterm'):
            child, subfolder = launch(mode)
            exits[mode] = exit_probe(child, subfolder, mode)
        assert ctl('status')['state'] == 'ready'
        (evidence / 'recording-timestamps.json').write_text(json.dumps(frames, indent=2))
        if args.ffmpeg and frames:
            def quote(path):
                return "'" + path.replace("'", "'\\''") + "'"
            manifest = ['ffconcat version 1.0']
            for i, frame in enumerate(frames):
                duration = frames[i + 1]['seconds'] - frame['seconds'] if i + 1 < len(frames) else .5
                manifest += ['file ' + quote(frame['file']), 'duration %.6f' % duration]
            manifest += ['file ' + quote(frames[-1]['file'])]
            (evidence / 'demo.ffconcat').write_text('\n'.join(manifest) + '\n')
            subprocess.run([str(args.ffmpeg), '-hide_banner', '-loglevel', 'error', '-f', 'concat', '-safe', '0',
                            '-i', str(evidence / 'demo.ffconcat'), '-vf',
                            'scale=1280:800:force_original_aspect_ratio=decrease,pad=1280:800:(ow-iw)/2:(oh-ih)/2',
                            '-fps_mode', 'vfr', '-c:v', 'libx264', '-crf', '18', '-pix_fmt', 'yuv420p',
                            '-map_metadata', '-1', '-movflags', '+faststart', str(evidence / 'terminal-demo.mp4')], check=True, timeout=120)
        metrics = json.loads((folder / 'terminal-metrics.json').read_text())
        report = {'passed': True, 'terminal': subprocess.check_output([str(args.kitten), '--version'], text=True).strip(),
                  'imageRegionMeanDifference': difference, 'resizes': resizes, 'exits': exits,
                  'restartTested': args.restart, 'vmReadyAfterViewerExit': True, 'metrics': metrics,
                  'recordedFrames': len(frames), 'recordingIsSampledWindowCapture': True,
                  'recordingSeconds': frames[-1]['seconds'] - frames[0]['seconds'] if frames else 0,
                  'durationSeconds': time.monotonic() - started}
        (evidence / 'report.json').write_text(json.dumps(report, indent=2))
        print(json.dumps({k: report[k] for k in ('passed', 'terminal', 'resizes', 'restartTested', 'vmReadyAfterViewerExit', 'recordedFrames', 'recordingSeconds')}, indent=2))
    finally:
        stop_recording.set()
        if record_thread:
            record_thread.join(timeout=20)
        for wid in windows:
            try:
                rc('close-window', '--match', 'id:' + str(wid))
            except Exception:
                pass


if __name__ == '__main__':
    main()
