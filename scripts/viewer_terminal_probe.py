#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Run inside an isolated Kitty acceptance window; compare exact TTY state."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import termios
import fcntl
import struct
import re
import select
import time
import tty


def input_modes():
    saved=termios.tcgetattr(0)
    values={}
    try:
        tty.setraw(0)
        os.write(1,b"\x1b[?1000$p\x1b[?1006$p\x1b[?2004$p")
        data=b""; deadline=time.monotonic()+1
        while len(values)<3 and len(data)<1024 and time.monotonic()<deadline:
            ready,_,_=select.select([0],[],[],max(0,deadline-time.monotonic()))
            if not ready:break
            data+=os.read(0,128)
            for mode,state in re.findall(rb"\x1b\[\?(1000|1006|2004);([12])\$y",data):values[mode.decode()]=int(state)
        if len(values)!=3:raise RuntimeError("terminal did not answer input-mode queries")
        return values
    finally:termios.tcsetattr(0,termios.TCSANOW,saved)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ('cli', 'state-dir', 'lima', 'evidence'):
        p.add_argument('--' + name, type=Path, required=True)
    p.add_argument("--start", action="store_true", help="exercise single-command boot plus interactive view")
    p.add_argument("--interactive", action="store_true")
    args = p.parse_args()
    os.umask(0o077)
    evidence = args.evidence.resolve()
    before_attrs = termios.tcgetattr(0)
    before = subprocess.check_output(['stty', '-g'], text=True).strip()
    (evidence/'tty-size.json').write_text(json.dumps(dict(zip(('rows','columns','pixelWidth','pixelHeight'), struct.unpack('HHHH',fcntl.ioctl(0,termios.TIOCGWINSZ,b'\0'*8))))))
    before_modes=input_modes() if args.start or args.interactive else None
    command = 'start' if args.start else 'view'
    flags = ['--interactive'] if args.interactive and not args.start else []
    viewer = subprocess.Popen([str(args.cli.resolve()), command, *flags, '--state-dir', str(args.state_dir.resolve()),
                               '--lima', str(args.lima.resolve()), '--metrics', str(evidence / 'terminal-metrics.json')])
    (evidence / 'viewer-pid.txt').write_text(str(viewer.pid))
    code = viewer.wait()
    after_modes=input_modes() if before_modes is not None else None
    after = subprocess.check_output(['stty', '-g'], text=True).strip()
    after_attrs = termios.tcgetattr(0)
    # Darwin sets the transient PENDIN flag on raw -> canonical; it clears on input.
    # Check all persistent termios values, retaining the exact strings for audit.
    pending = getattr(termios, 'PENDIN', 0)
    before_attrs[3] &= ~pending
    after_attrs[3] &= ~pending
    report = {'inputModesBefore':before_modes,'inputModesAfter':after_modes,'inputModesRestored':before_modes==after_modes,'exitCode': code, 'terminalStateRestored': before_attrs == after_attrs,
              'exactImmediateStateMatch': before == after, 'ignoredKernelTransient': 'PENDIN',
              'before': before, 'after': after}
    (evidence / 'terminal-exit.json').write_text(json.dumps(report, indent=2))
    print('VIEWER_EXITED. Type restoration-probe then Enter: ', end='', flush=True)
    report['canonicalInputWorks'] = input() == 'restoration-probe'
    report['afterCanonicalInput'] = subprocess.check_output(['stty', '-g'], text=True).strip()
    (evidence / 'terminal-exit.json').write_text(json.dumps(report, indent=2))
    print('TERMINAL RESTORED:', report['terminalStateRestored'] and report['canonicalInputWorks'], flush=True)
    input('Press Enter to close this isolated test window. ')


if __name__ == '__main__':
    main()
