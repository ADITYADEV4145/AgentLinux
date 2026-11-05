#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Run inside an isolated Kitty acceptance window; compare exact TTY state."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import termios


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ('cli', 'state-dir', 'lima', 'evidence'):
        p.add_argument('--' + name, type=Path, required=True)
    args = p.parse_args()
    os.umask(0o077)
    evidence = args.evidence.resolve()
    before_attrs = termios.tcgetattr(0)
    before = subprocess.check_output(['stty', '-g'], text=True).strip()
    viewer = subprocess.Popen([str(args.cli.resolve()), 'view', '--state-dir', str(args.state_dir.resolve()),
                               '--lima', str(args.lima.resolve()), '--metrics', str(evidence / 'terminal-metrics.json')])
    (evidence / 'viewer-pid.txt').write_text(str(viewer.pid))
    code = viewer.wait()
    after = subprocess.check_output(['stty', '-g'], text=True).strip()
    after_attrs = termios.tcgetattr(0)
    # Darwin sets the transient PENDIN flag on raw -> canonical; it clears on input.
    # Check all persistent termios values, retaining the exact strings for audit.
    pending = getattr(termios, 'PENDIN', 0)
    before_attrs[3] &= ~pending
    after_attrs[3] &= ~pending
    report = {'exitCode': code, 'terminalStateRestored': before_attrs == after_attrs,
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
