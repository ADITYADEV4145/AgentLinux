#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Opt-in installed-product Mac review. Owns one disposable VM and Kitty window.

Requires reviewed bundle/Lima/Kitty, authenticated Codex for --agent, and test-only
Pillow. No cloud, payment APIs, Mac pointer/clipboard or global client settings.
"""
import argparse
import base64
import json
import os
from pathlib import Path
import shutil
import signal
import statistics
import subprocess
import sys
import tempfile
import threading
import time


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ('bundle','lima','kitty','kitten','evidence'):
        p.add_argument('--'+name,type=Path,required=True)
    p.add_argument('--image',type=Path,help='reviewed pinned public image cache; report explicitly labels cached-image provisioning')
    p.add_argument('--chrome',type=Path,help='optional isolated Chrome web/human acceptance')
    p.add_argument('--agent',action='store_true',help='run actual authenticated Codex browser-form demo; may use existing model allowance')
    p.add_argument('--codex',default='codex')
    args=p.parse_args();os.umask(0o077)
    from PIL import Image,ImageChops,ImageStat
    repo=Path(__file__).resolve().parents[1]
    evidence=args.evidence.resolve();evidence.mkdir(parents=True,exist_ok=False,mode=0o700)
    root=Path(tempfile.mkdtemp(prefix='agentlinux-launch-acceptance-',dir='/private/tmp'))
    toolroot=Path(tempfile.mkdtemp(prefix='agentlinux-launch-tools-',dir='/private/tmp'));prefix=toolroot/'tools'
    socketdir=Path(tempfile.mkdtemp(prefix='agentlinux-launch-rc-',dir='/private/tmp'));sock=socketdir/'rc.sock'
    sentinel=toolroot/'unrelated.txt';sentinel.write_text('preserve this test sentinel')
    (evidence/'fixture-root.txt').write_text(str(root))
    bundle=args.bundle.resolve();lima=args.lima.resolve();cli=prefix/'bin/agent-linux'
    durations={};report={'passed':False,'cleanMac':False,'cloudTested':False,'imageCached':bool(args.image),'runs':{'freshSetup':1,'warmStartup':3,'screenshot':5,'reset':1,'viewerChange':1},'timingsSeconds':durations}
    kitty=None;wid=None;recorder=None;stop_recording=threading.Event();frames=[];capture_errors=[]
    def measure(name,argv,timeout=1500,expect=True):
        start=time.monotonic();r=subprocess.run([str(x) for x in argv],capture_output=True,text=True,timeout=timeout)
        durations[name]=time.monotonic()-start
        (evidence/(name+'.stdout')).write_text(r.stdout);(evidence/(name+'.stderr')).write_text(r.stderr)
        if expect and r.returncode:raise RuntimeError(name+' failed; inspect private evidence')
        return r
    def ctl(name,action,*a,expect=True):
        r=measure(name,[cli,action,'--state-dir',root,'--lima',lima,'--json',*a],expect=expect)
        if r.returncode and not expect:return json.loads(r.stdout)
        return json.loads(r.stdout)['result']
    def guest(name,code):
        v=ctl(name,'exec','--','/usr/bin/python3','-c',code)
        return base64.b64decode(v.get('stdout') or '').decode()
    base=[str(args.kitten.resolve()),'@','--to','unix:'+str(sock)]
    def rc(*a,data=None):return subprocess.check_output(base+list(a),input=data,stderr=subprocess.PIPE,timeout=15).decode()
    def wait(test,timeout=30):
        end=time.monotonic()+timeout
        while time.monotonic()<end:
            if kitty is not None and kitty.poll() is not None:raise RuntimeError('owned Kitty exited')
            if test():return
            time.sleep(.2)
        raise RuntimeError('acceptance condition timed out')
    def text():return rc('get-text','--match','id:'+str(wid))
    def snapshot(name):
        path=evidence/(name+'.png');rc('screenshot','--match','id:'+str(wid),str(path));return path
    try:
        measure('install',[sys.executable,repo/'scripts/install.py','--bundle',bundle,'--prefix',prefix])
        report['version']=json.loads(measure('version',[cli,'version','--json']).stdout)
        report['doctor']=json.loads(measure('doctor',[cli,'doctor','--state-dir',root,'--lima',lima,'--json']).stdout)['result']
        report['host']={'macOS':subprocess.check_output(['sw_vers','-productVersion'],text=True).strip(),'build':subprocess.check_output(['sw_vers','-buildVersion'],text=True).strip(),'chip':subprocess.check_output(['sysctl','-n','machdep.cpu.brand_string'],text=True).strip(),'memoryBytes':int(subprocess.check_output(['sysctl','-n','hw.memsize'],text=True))}
        up=['--local','--network=nat','--cpus','2','--memory','4','--disk','24','--timeout','20m']
        if args.image:
            manifest=json.loads((repo/'images/debian/manifest.json').read_text())
            up+=['--image',str(args.image.resolve()),'--image-sha512',manifest['sha512']]
        state=ctl('fresh-setup','up',*up);assert state['state']=='ready' and state['guest']['desktop']['ready']
        report['guest']=state['guest'];report['configuration']=state['spec']
        for n in range(5):ctl('screenshot-'+str(n),'screenshot','--output',str(evidence/('guest-'+str(n)+'.png')))
        payload=(repo/'examples/demo/project-budget.html').read_text()
        guest('install-demo-page',"from pathlib import Path;Path('/home/agent/launch-demo.html').write_text("+repr(payload)+")")
        guest('persistence-marker',"from pathlib import Path;Path('/home/agent/launch-marker').write_text('persist')")
        report['guestResources']=json.loads(guest('guest-resources',"import json,os;print(json.dumps({'meminfo':open('/proc/meminfo').read(),'load':os.getloadavg()}))"))
        pidfile=root/'lima/default/ha.pid'
        if pidfile.exists():
            pid=int(pidfile.read_text().strip());command=subprocess.check_output(['ps','-p',str(pid),'-o','command='],text=True)
            if str(root) not in command:raise RuntimeError('supervisor ownership mismatch')
            report['limaSupervisorSample']=subprocess.check_output(['ps','-p',str(pid),'-o','pid=,%cpu=,rss='],text=True).strip()
            report['supervisorNote']='one point sample; percent CPU and KiB RSS; excludes separately hosted VM processes, not total product usage'
        log=(evidence/'kitty.log').open('w')
        kitty=subprocess.Popen([str(args.kitty.resolve()),'--config','NONE','--listen-on','unix:'+str(sock),'-o','allow_remote_control=socket-only','-o','shell_integration=disabled','-o','confirm_os_window_close=0','-o','macos_quit_when_last_window_closed=yes',sys.executable,'-c','import time;time.sleep(3600)'],stdout=log,stderr=log,start_new_session=True)
        wait(lambda:sock.exists())
        folder=evidence/'terminal';folder.mkdir(mode=0o700)
        wid=int(rc('launch','--type=os-window','--os-window-title=Agent Linux launch demo',sys.executable,str(repo/'scripts/viewer_terminal_probe.py'),'--cli',str(cli),'--lima',str(lima),'--state-dir',str(root),'--evidence',str(folder)).strip())
        wait(lambda:'LIVE' in text());snapshot('terminal-before')
        t0=time.monotonic()
        def record():
            directory=evidence/'recording-frames';directory.mkdir()
            while not stop_recording.is_set() and len(frames)<900:
                path=directory/('frame-%05d.png'%len(frames))
                try:
                    at=time.monotonic();rc('screenshot','--match','id:'+str(wid),str(path));frames.append({'file':str(path),'seconds':at-t0})
                except Exception as exc:capture_errors.append(type(exc).__name__);return
                stop_recording.wait(.5)
        recorder=threading.Thread(target=record);recorder.start()
        if args.agent:
            measure('agent-demo',[sys.executable,repo/'scripts/mcp_agent_acceptance.py','--cli',cli,'--lima',lima,'--state-dir',root,'--evidence',evidence/'agent','--codex',args.codex,'--task','budget'],timeout=500)
            checked=ctl('verify-agent-result','exec','--','/usr/bin/xdotool','search','--onlyvisible','--name','Budget verified 300.00')
            assert checked['exitCode']==0
            report['actualAgentPassed']=True
        else:
            # Explicit SDK acceptance is a real MCP client, not an AI model demo.
            env=os.environ.copy();env.update(GOCACHE=str(repo/'.cache/go-build'),GOMODCACHE=str(repo/'.cache/go-mod'),AGENTLINUX_ACCEPT_CLI=str(cli),AGENTLINUX_ACCEPT_ROOT=str(root),AGENTLINUX_ACCEPT_LIMA=str(lima),AGENTLINUX_ACCEPT_EVIDENCE=str(evidence))
            r=subprocess.run(['go','test','-v','./internal/mcpserver','-run','^TestRealStdioGuest$','-count=1'],cwd=repo,env=env,capture_output=True,text=True,timeout=240)
            (evidence/'mcp.txt').write_text(r.stdout+r.stderr);assert r.returncode==0
            report['actualAgentPassed']=False;report['officialSDKPassed']=True
        report['postTaskGuestResources']=json.loads(guest('post-task-resources',"import json,os;print(json.dumps({'meminfo':open('/proc/meminfo').read(),'load':os.getloadavg()}))"))
        report['stateAllocatedKiB']=int(subprocess.check_output(['du','-sk',str(root)],text=True).split()[0])
        time.sleep(6);ctl('agent-guest-screenshot','screenshot','--output',str(evidence/'agent-result.png'));before=snapshot('terminal-agent-result')
        picture=Image.open(before).convert('RGB');region=(picture.width//10,picture.height//10,picture.width*9//10,picture.height*9//10)
        started=time.monotonic();ctl('viewer-change-action','open','files')
        def changed():
            latest=snapshot('terminal-latency-sample')
            image=Image.open(latest).convert('RGB')
            return sum(ImageStat.Stat(ImageChops.difference(picture.crop(region),image.crop(region))).mean)/3>8
        wait(changed,20);durations['viewer-action-to-visible']=time.monotonic()-started
        report['viewerLatencyMethod']='guest files-app open request through changed central terminal image; 200ms polling plus window capture; includes app startup; n=1'
        rc('resize-os-window','--match','id:'+str(wid),'--unit=cells','--width=80','--height=30');time.sleep(1);snapshot('terminal-resized')
        stop_recording.set();recorder.join(timeout=20);assert not recorder.is_alive() and not capture_errors
        (evidence/'recording-timestamps.json').write_text(json.dumps(frames,indent=2))
        rc('send-text','--match','id:'+str(wid),'q')
        wait(lambda:(folder/'terminal-exit.json').exists())
        rc('send-text','--match','id:'+str(wid),'restoration-probe\n')
        wait(lambda:json.loads((folder/'terminal-exit.json').read_text()).get('canonicalInputWorks'))
        report['terminal']=json.loads((folder/'terminal-exit.json').read_text());assert report['terminal']['terminalStateRestored']
        report['viewerMetrics']=json.loads((folder/'terminal-metrics.json').read_text())
        assert ctl('after-viewer-status','status')['state']=='ready'
        if args.chrome:
            viewer_output=evidence/'web-output.txt'
            with viewer_output.open('w') as out,(evidence/'web-stderr.txt').open('w') as err:
                viewer=subprocess.Popen([str(cli),'view','--web','--interactive','--state-dir',str(root),'--lima',str(lima)],stdout=out,stderr=err)
                try:
                    wait(lambda:'http://127.0.0.1:' in viewer_output.read_text())
                    measure('web-acceptance',[sys.executable,repo/'scripts/viewer_web_acceptance.py','--cli',cli,'--lima',lima,'--state-dir',root,'--viewer-output',viewer_output,'--evidence',evidence/'web','--chrome',args.chrome,'--interactive'],timeout=250)
                    report['webAcceptance']=json.loads((evidence/'web/report.json').read_text())
                finally:
                    viewer.send_signal(signal.SIGTERM);viewer.wait(timeout=20)
        for n in range(3):
            ctl('warm-stop-'+str(n),'stop');old=state['guest']['bootId'];state=ctl('warm-start-'+str(n),'up','--local','--timeout','90s')
            assert state['state']=='ready' and state['guest']['bootId']!=old
            assert guest('persistence-'+str(n),"from pathlib import Path;print(Path('/home/agent/launch-marker').read_text())").strip()=='persist'
        ctl('stop-before-reset','stop');old=state['machineId'];state=ctl('reset','reset','--machine-id',old,'--yes','--timeout','20m')
        assert state['state']=='ready' and state['machineId']!=old
        assert guest('reset-clean',"from pathlib import Path;print(Path('/home/agent/launch-marker').exists())").strip()=='False'
        ctl('stop-after-reset','stop');ctl('delete','delete','--machine-id',state['machineId'],'--yes')
        assert {x.name for x in root.iterdir()}=={'.agent-linux-owner','lock'}
        measure('uninstall',[sys.executable,bundle/'install.py','--prefix',prefix,'--uninstall','--yes'])
        assert sentinel.read_text()=='preserve this test sentinel' and not cli.exists()
        report['unrelatedSentinelUnchanged']=True;report['passed']=True
    except BaseException as exc:
        report['failureType']=type(exc).__name__
        raise
    finally:
        stop_recording.set()
        if recorder:recorder.join(timeout=20)
        if kitty is not None:
            if wid is not None:
                try:rc('close-window','--match','id:'+str(wid))
                except Exception:pass
            kitty.terminate()
            try:kitty.wait(timeout=10)
            except subprocess.TimeoutExpired:kitty.kill();kitty.wait()
            log.close()
        if sock.exists():sock.unlink()
        socketdir.rmdir()
        if (root/'state.json').exists():
            saved=json.loads((root/'state.json').read_text())
            result=ctl('cleanup-stop','stop',expect=False)
            if isinstance(result,dict) and result.get('error'):
                assert 'recorded VM missing' in result['error']['message'],result
            for partial in root.glob('.download-*'):
                assert partial.is_file() and not partial.is_symlink();partial.unlink()
            ctl('cleanup-delete','delete','--machine-id',saved['machineId'],'--yes')
        report['cleanup']='owned VM removed; marker/lock retained; existing workstation not controlled'
        (evidence/'report.json').write_text(json.dumps(report,indent=2))
    print(json.dumps({'passed':report['passed'],'actualAgentPassed':report.get('actualAgentPassed'),'timingsSeconds':durations,'cleanup':report['cleanup']},indent=2))

if __name__=='__main__':main()
