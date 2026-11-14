#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Measure an explicitly selected local test VM; no cloud, credentials or host UI.
Creates no VM. --actions is restricted to our disposable acceptance namespaces.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import signal
import statistics
import subprocess
import time
import urllib.request


def measure(args):
    os.umask(0o077)
    if not args.state_dir.is_absolute() or args.state_dir.is_symlink():
        raise ValueError('explicit absolute nonsymlink VM namespace required')
    if args.actions and (args.state_dir.parent != Path('/private/tmp') or not args.state_dir.name.startswith('agentlinux-hardening-')):
        raise ValueError('actions require a disposable /private/tmp/agentlinux-hardening-* namespace')
    if args.evidence.exists(): raise ValueError('evidence directory must be new')
    args.evidence.mkdir(parents=True, mode=0o700)
    common = ['--state-dir', str(args.state_dir), '--lima', str(args.lima)]
    def call(command, *tail):
        start = time.monotonic()
        p = subprocess.run([str(args.cli), command, *common, *map(str, tail)], capture_output=True, timeout=90)
        if p.returncode: raise RuntimeError(command+' failed; inspect private VM logs; output deliberately omitted')
        return p, time.monotonic() - start
    report = {'kind':'real-local-VM', 'system':platform.system(), 'architecture':platform.machine(), 'screenshots':[], 'actionToChangedFrameSeconds':[], 'limits':'Changed PNG after action is timing evidence, not semantic task verification. CPU/RSS viewer-only, excludes SSH/Lima/guest/terminal.'}
    state, _ = call('status','--json')
    status = json.loads(state.stdout)['result']
    if status['state'] != 'ready': raise ValueError('actual guest must be ready before measurement')
    report['backendVersion'] = status.get('backendVersion')
    report['guestVersion'] = (status.get('guest') or {}).get('guestVersion')
    report['machineSpec'] = {k:status['spec'][k] for k in ('cpus','memoryGiB','diskGiB')}
    for n in range(10):
        _, seconds = call('screenshot','--output',args.evidence/('capture-%02d.png'%n))
        report['screenshots'].append(seconds)
    report['screenshotMedianSeconds'] = statistics.median(report['screenshots'])
    report['screenshotP95Seconds'] = sorted(report['screenshots'])[-1]
    log = args.evidence/'viewer.stdout'
    err = args.evidence/'viewer.stderr'
    metrics = args.evidence/'viewer-metrics.json'
    command = [str(args.cli),'view',*common,'--web','--metrics',str(metrics)]
    if args.capture_transport: command += ['--capture-transport',args.capture_transport]
    with log.open('wb') as out,err.open('wb') as errors:
        viewer = subprocess.Popen(command,stdout=out,stderr=errors)
        try:
            url = None
            deadline = time.monotonic()+15
            while time.monotonic()<deadline:
                for word in log.read_text().split():
                    if word.startswith('http://127.0.0.1:') and '#' in word: url=word
                if url: break
                if viewer.poll() is not None: raise RuntimeError('viewer exited before listener readiness')
                time.sleep(.05)
            if not url: raise RuntimeError('viewer listener timeout')
            endpoint,token=url.split('#')
            def frame():
                req=urllib.request.Request(endpoint+'api/frame',headers={'Authorization':'Bearer '+token})
                with urllib.request.urlopen(req,timeout=12) as response:
                    data=response.read((8<<20)+1)
                    if len(data)>8<<20: raise RuntimeError('oversized frame')
                    return hashlib.sha256(data).hexdigest()
            previous=frame()
            if args.actions:
                call('open','editor')
                for n in range(3):
                    previous=frame();start=time.monotonic()
                    call('type','Agent Linux benchmark %d — café 日本語\n'%n)
                    while time.monotonic()-start<15:
                        changed=frame()
                        if changed!=previous: break
                        time.sleep(.05)
                    else: raise RuntimeError('no changed guest frame after input')
                    report['actionToChangedFrameSeconds'].append(time.monotonic()-start)
            for _ in range(5): frame();time.sleep(.25)
        finally:
            viewer.send_signal(signal.SIGINT)
            try: viewer.wait(timeout=15)
            except subprocess.TimeoutExpired:
                viewer.kill();viewer.wait();raise RuntimeError('viewer cleanup exceeded deadline')
    if not metrics.exists(): raise RuntimeError('viewer did not write cleanup metrics')
    report['viewer'] = json.loads(metrics.read_text())
    (args.evidence/'report.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({k:v for k,v in report.items() if k not in ('viewer','screenshots')},indent=2))


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--cli',type=Path,required=True)
    p.add_argument('--lima',type=Path,required=True)
    p.add_argument('--state-dir',type=Path,required=True)
    p.add_argument('--evidence',type=Path,required=True)
    p.add_argument('--actions',action='store_true')
    p.add_argument('--capture-transport',choices=['persistent','oneshot'])
    args=p.parse_args()
    try: measure(args)
    except (ValueError,RuntimeError,subprocess.TimeoutExpired) as e:p.exit(1,str(e)+'\n')


if __name__=='__main__':main()
