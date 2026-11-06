#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Free CLI/API acceptance using the SIMULATED provider; never calls Stripe/E2B."""
import argparse
import json
import os
from pathlib import Path
import signal
import socket
import subprocess
import tempfile
import time


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--cli', type=Path, required=True)
    args = p.parse_args()
    os.umask(0o077)
    cli = args.cli.resolve()
    repo = Path(__file__).resolve().parents[1]
    root = Path(tempfile.mkdtemp(prefix='agentlinux-billing-acceptance-', dir='/private/tmp' if Path('/private/tmp').exists() else None)).resolve()
    sock = socket.socket()
    sock.bind(('127.0.0.1', 0))
    port = sock.getsockname()[1]
    sock.close()
    def call(*a, expected=True):
        value = subprocess.run([str(cli), *a], capture_output=True, text=True, timeout=40)
        if expected and value.returncode:
            raise RuntimeError('CLI failed: ' + value.stderr)
        return value
    call('cloud', 'init', '--dev', '--state-dir', str(root), '--endpoint', f'http://127.0.0.1:{port}')
    call('billing', 'configure', '--state-dir', str(root), '--pricing', str(repo/'examples/billing.sandbox.json'), '--yes')
    log = (root/'server.log').open('w')
    proc = subprocess.Popen([str(cli), 'cloud', 'serve', '--dev', '--state-dir', str(root), '--listen', f'127.0.0.1:{port}'], stdout=log, stderr=log)
    try:
        deadline = time.monotonic() + 10
        while True:
            try:
                with socket.create_connection(('127.0.0.1', port), .1): break
            except OSError:
                if time.monotonic() > deadline: raise RuntimeError('server unavailable')
                time.sleep(.1)
        alice, bob, session = root/'alice.json', root/'bob.json', root/'session.json'
        call('cloud', 'up', '--config', str(alice), '--request-id', '0123456789abcdef0123456789abcdef', '--seconds', '60', '--output', str(session))
        current = json.loads(call('billing', 'status', '--json', '--config', str(alice)).stdout)
        assert current['mode'] == 'sandbox' and len(current['active']) == 1
        time.sleep(1.2)
        call('cloud', 'stop', '--config', str(session))
        deadline = time.monotonic()+10
        while json.loads(call('cloud', 'status', '--config', str(session)).stdout)['state'] != 'stopped':
            if time.monotonic() > deadline: raise RuntimeError('stop confirmation timed out')
            time.sleep(.1)
        final = json.loads(call('billing', 'status', '--json', '--config', str(alice)).stdout)
        assert len(final['history']) == 1 and final['history'][0]['quantitySeconds'] >= 1
        assert final['history'][0]['status'] == 'finalized' and final['finalizedNetMicros'] == 0
        other = json.loads(call('billing', 'status', '--json', '--config', str(bob)).stdout)
        assert not other['history'] and not other['active']
        assert call('billing', 'cancel', '--config', str(alice), expected=False).returncode != 0
        call('billing', 'cancel', '--config', str(alice), '--yes')
        assert call('cloud', 'up', '--config', str(alice), '--request-id', '1123456789abcdef0123456789abcdef', '--seconds', '60', '--output', str(root/'second.json'), expected=False).returncode != 0
        print(json.dumps({'passed':True,'provider':'SIMULATED','stripeCalls':0,'paidResources':0,'checks':['CLI configure/status/trial','authenticated ready-to-confirmed-stop ledger','workspace isolation','guarded cancellation and denied admission']}))
    finally:
        proc.send_signal(signal.SIGTERM)
        proc.wait(timeout=15)
        log.close()
        # This fresh fixture contains only generated development credentials and
        # fake provider state. Never recursively remove a caller-supplied path.
        assert root.name.startswith('agentlinux-billing-acceptance-')
        for item in root.iterdir():
            assert item.is_file() and not item.is_symlink()
            item.unlink()
        root.rmdir()


if __name__=='__main__': main()
