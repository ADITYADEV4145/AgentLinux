#!/usr/bin/env python3
"""Opt-in real-Mac acceptance. Creates and cleans only its own temporary machine."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--cli', required=True, type=Path)
    parser.add_argument('--lima', required=True, type=Path)
    parser.add_argument('--image', type=Path, help='optional existing copy of the pinned QCOW2 image')
    args = parser.parse_args()
    cli, lima = args.cli.resolve(), args.lima.resolve()
    manifest = json.loads((Path(__file__).resolve().parents[1] / 'images/debian/manifest.json').read_text())
    root = Path(tempfile.mkdtemp(prefix='agentlinux-acceptance-', dir='/private/tmp'))
    common = ['--state-dir', str(root), '--lima', str(lima), '--json']
    machine_id = None
    stopped = False

    def call(command):
        result = subprocess.run([str(cli)] + command + common, capture_output=True, text=True, timeout=1250)
        try:
            data = json.loads(result.stdout)
        except ValueError as exc:
            raise RuntimeError('CLI did not produce structured output') from exc
        if result.returncode:
            raise RuntimeError(data.get('error', {}).get('message', 'CLI failed'))
        return data['result']

    try:
        created = ['up', '--local', '--network=nat', '--timeout=20m']
        if args.image:
            created += ['--image', str(args.image.resolve()), '--image-sha512', manifest['sha512']]
        started = time.monotonic()
        before = call(created)
        machine_id = before['machineId']
        assert before['state'] == 'ready' and before['guest']['nonceSha256']
        ready_seconds = round(time.monotonic() - started, 2)
        assert call(['status'])['state'] == 'ready'
        assert call(['stop'])['state'] == 'stopped'
        stopped = True
        after = call(['up', '--local', '--timeout=90s'])
        stopped = False
        assert after['machineId'] == machine_id
        assert after['guest']['bootId'] != before['guest']['bootId']
        assert after['guest']['sessionId'] != before['guest']['sessionId']
        assert call(['stop'])['state'] == 'stopped'
        stopped = True
        print(json.dumps({'passed': True, 'readySeconds': ready_seconds,
                          'guestVersion': after['guest']['guestVersion'],
                          'protocol': after['guest']['protocol'],
                          'guestOS': after['guest']['os'], 'guestArch': after['guest']['arch'],
                          'backendVersion': after['backendVersion'],
                          'checks': ['real guest handshake and nonce probe', 'status', 'graceful stop',
                                     'restart with stable machine ID and new boot/session IDs']}, indent=2), flush=True)
    finally:
        state = root / 'state.json'
        if state.exists() and not state.is_symlink():
            record = json.loads(state.read_text())
            machine_id = machine_id or record.get('machineId')
            if record.get('machineId') != machine_id:
                raise RuntimeError('cleanup identity mismatch; preserving resources at ' + str(root))
            if not stopped:
                try:
                    stopped = call(['stop'])['state'] == 'stopped'
                except Exception:
                    pass
            if stopped and (root / 'lima/default').is_dir():
                env = {'PATH': os.environ['PATH'], 'HOME': str(root),
                       'LIMA_HOME': str(root / 'lima'), 'TMPDIR': str(root / 'tmp'), 'LC_ALL': 'C'}
                deleted = subprocess.run([str(lima), 'delete', '--force', 'default'], env=env,
                                         capture_output=True, timeout=60)
                if deleted.returncode:
                    raise RuntimeError('test VM deletion unconfirmed; inspect ' + str(root))
            elif (root / 'lima/default').exists():
                raise RuntimeError('test VM may still run; preserved at ' + str(root))
        if not root.name.startswith('agentlinux-acceptance-') or root.parent != Path('/private/tmp'):
            raise RuntimeError('unsafe cleanup path')
        shutil.rmtree(root)
        print('Owned temporary test resources removed.', flush=True)


if __name__ == '__main__':
    main()
