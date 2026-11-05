#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Experimental guest-only GNOME 50 probe, NOT the AgentLinux backend.
Uses Mutter's explicitly private/version-specific API only to establish feasibility.
"""
import argparse,json,os,subprocess,time,sys,stat,struct
from pathlib import Path

p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--output',required=True,help='new private directory under the evaluation guest user home')
a=p.parse_args()
if not __debug__:
 p.error('run this verification probe without Python -O; assertions must stay active')
os.umask(0o077)
if sys.platform!='linux' or os.getuid()==0:
 p.error('run only as the unprivileged user inside the marked disposable Linux VM')
marker=Path('/var/lib/agent-linux-evaluation/installed')
try:
 st=marker.lstat()
 assert stat.S_ISREG(st.st_mode) and st.st_uid==0 and not st.st_mode & 0o022
 assert marker.read_text().strip()=='completed'
 assert subprocess.run(['systemd-detect-virt','--vm'],capture_output=True).returncode==0
 assert subprocess.check_output(['gnome-shell','--version'],text=True).startswith('GNOME Shell 50.')
except (OSError,AssertionError):
 p.error('requires the root-marked GNOME 50 evaluation VM; never run on a host desktop')
out=Path(a.output)
if not out.is_absolute() or not out.resolve().is_relative_to(Path.home().resolve()):
 p.error('output must be a new directory under the evaluation user home')
out.mkdir(mode=0o700,parents=True,exist_ok=False)
import gi
gi.require_version('Gst','1.0')
from gi.repository import Gio,GLib,Gst
Gst.init(None);bus=Gio.bus_get_sync(Gio.BusType.SESSION,None)
RD='org.gnome.Mutter.RemoteDesktop';SC='org.gnome.Mutter.ScreenCast'
def call(name,path,interface,method,signature=None,values=()):
 return bus.call_sync(name,path,interface,method,GLib.Variant(signature,values) if signature else None,None,Gio.DBusCallFlags.NONE,10000,None)
rp=call(RD,'/org/gnome/Mutter/RemoteDesktop',RD,'CreateSession').unpack()[0];RI=RD+'.Session'
sid=call(RD,rp,'org.freedesktop.DBus.Properties','Get','(ss)',(RI,'SessionId')).unpack()[0]
sp=call(SC,'/org/gnome/Mutter/ScreenCast',SC,'CreateSession','(a{sv})',({'remote-desktop-session-id':GLib.Variant('s',sid)},)).unpack()[0]
stream=call(SC,sp,SC+'.Session','RecordMonitor','(sa{sv})',('',{'cursor-mode':GLib.Variant('u',1)})).unpack()[0]
node=[];loop=GLib.MainLoop()
def added(*args):node.append(args[-2].unpack()[0]);loop.quit()
bus.signal_subscribe(SC,SC+'.Stream','PipeWireStreamAdded',stream,None,Gio.DBusSignalFlags.NONE,added,None)
call(RD,rp,RI,'Start')
GLib.timeout_add_seconds(15,lambda:(loop.quit(),False)[1]);loop.run();assert node,'no PipeWire stream'

def pump(seconds):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  while GLib.MainContext.default().pending():GLib.MainContext.default().iteration(False)
  time.sleep(.01)

def capture(name):
 pipeline=Gst.Pipeline.new(None)
 src=Gst.ElementFactory.make('pipewiresrc');src.set_property('path',str(node[0]));src.set_property('num-buffers',1)
 convert=Gst.ElementFactory.make('videoconvert');encoder=Gst.ElementFactory.make('pngenc');sink=Gst.ElementFactory.make('filesink');sink.set_property('location',str(out/(name+'.png')))
 for element in [src,convert,encoder,sink]:assert element;pipeline.add(element)
 assert src.link(convert) and convert.link(encoder) and encoder.link(sink)
 started=time.monotonic();pipeline.set_state(Gst.State.PLAYING)
 msg=pipeline.get_bus().timed_pop_filtered(20*Gst.SECOND,Gst.MessageType.ERROR|Gst.MessageType.EOS)
 pipeline.set_state(Gst.State.NULL)
 if msg is None:raise RuntimeError('capture timeout')
 if msg.type==Gst.MessageType.ERROR:raise RuntimeError(str(msg.parse_error()))
 image=(out/(name+'.png')).read_bytes()
 assert len(image)<=8<<20 and image[:8]==b'\x89PNG\r\n\x1a\n' and struct.unpack('>II',image[16:24])==(1280,800), 'invalid capture geometry/PNG'
 return time.monotonic()-started

def key(code,pressed):call(RD,rp,RI,'NotifyKeyboardKeycode','(ub)',(code,pressed))
def combo(*keys):
 for k in keys:key(k,True)
 for k in reversed(keys):key(k,False)
 pump(.3)
def text(value):
 for c in value:
  n=ord(c);sym=n if n<256 else 0x01000000|n
  if c=='\n':sym=0xff0d
  for pressed in (True,False):call(RD,rp,RI,'NotifyKeyboardKeysym','(ub)',(sym,pressed))
 pump(.5)
clipboard='Agent Linux GNOME test: café — 日本語 👋'
clipboard_requests=[]
def transfer(*args):
 mime,serial=args[-2].unpack()
 try:
  result,fds=bus.call_with_unix_fd_list_sync(RD,rp,RI,'SelectionWrite',GLib.Variant('(u)',(serial,)),GLib.VariantType.new('(h)'),Gio.DBusCallFlags.NONE,5000,None,None)
  fd=fds.get(result.unpack()[0]);os.write(fd,clipboard.encode());os.close(fd)
  call(RD,rp,RI,'SelectionWriteDone','(ub)',(serial,True));clipboard_requests.append(mime)
 except Exception as e:print('clipboard transfer error',type(e).__name__,flush=True)
bus.signal_subscribe(RD,RI,'SelectionTransfer',rp,None,Gio.DBusSignalFlags.NONE,transfer,None)
call(RD,rp,RI,'EnableClipboard','(a{sv})',({},))
call(RD,rp,RI,'SetSelection','(a{sv})',({'mime-types':GLib.Variant('as',['text/plain;charset=utf-8','text/plain'])},))
report={'experimental':True,'privateMutterAPI':True,'captureSeconds':{}}
try:
 pump(2);report['captureSeconds']['desktop']=capture('desktop')
 target=out/'typed.txt';target.write_text('')
 subprocess.Popen(['gnome-text-editor','--new-window',str(target)],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 pump(4)
 # Physical key codes plus a modifier, not unsupported arbitrary Unicode keysyms.
 combo(29,30);combo(14);combo(30);combo(42,30);combo(29,31);pump(1)
 report['physicalKeysSavedExactly']=target.read_text()=='aA\n'
 combo(29,30);combo(14);pump(.5)
 call(RD,rp,RI,'SetSelection','(a{sv})',({'mime-types':GLib.Variant('as',['text/plain;charset=utf-8','text/plain'])},))
 combo(29,47);pump(3);combo(29,31);pump(1)
 report['unicodeSavedExactly']=target.read_text()==clipboard+'\n'
 report['editorFinalNewline']=True
 report['savedText']=target.read_text()
 report['clipboardTransfers']=clipboard_requests
 report['captureSeconds']['editor']=capture('editor')
 # Exercise real pointer movement/clicks, scroll, and held-button drag, all guest-only.
 call(RD,rp,RI,'NotifyPointerMotionAbsolute','(sdd)',(stream,640.,400.))
 for pressed in (True,False):call(RD,rp,RI,'NotifyPointerButton','(ib)',(272,pressed))
 call(RD,rp,RI,'NotifyPointerAxisDiscrete','(ui)',(0,1))
 call(RD,rp,RI,'NotifyPointerButton','(ib)',(272,True))
 call(RD,rp,RI,'NotifyPointerMotionAbsolute','(sdd)',(stream,680.,410.))
 call(RD,rp,RI,'NotifyPointerButton','(ib)',(272,False))
 report['inputMethodsAcknowledged']=True
 combo(125);pump(1);report['captureSeconds']['launcher']=capture('launcher');combo(1)
 subprocess.Popen(['nautilus','--new-window',str(Path.home())],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 pump(2);report['captureSeconds']['files']=capture('files')
 subprocess.Popen(['firefox','--new-window','about:blank'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 pump(3)
 clipboard='data:text/html;charset=utf-8,'+__import__('urllib.parse',fromlist=['quote']).quote('<title>Ubuntu GNOME evaluation</title><h1>Agent Linux: real Ubuntu GNOME guest</h1><p>Experimental Wayland capture and input.</p><p>Production MCP migration remains pending.</p>')
 call(RD,rp,RI,'SetSelection','(a{sv})',({'mime-types':GLib.Variant('as',['text/plain;charset=utf-8','text/plain'])},))
 combo(29,38);combo(29,47);pump(.5);combo(28);pump(5)
 report['captureSeconds']['browser']=capture('browser')
 for name,argv in [('terminal',['ptyxis','--new-window']),('settings',['gnome-control-center','system']),('app-center',['snap','run','snap-store']),('gnome-software',['gnome-software'])]:
  subprocess.Popen(argv,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
  pump(4)
  report['captureSeconds'][name]=capture(name)
 report['productionIntegration']=False
 report['coreInputChecksPassed']=bool(report['physicalKeysSavedExactly'] and report['unicodeSavedExactly'])
 report['mouseGestureSemanticsVerified']=False
 report['softwareStoreFlowVerified']=False
 (out/'report.json').write_text(json.dumps(report,indent=2));print(json.dumps(report))
 assert report['coreInputChecksPassed'], 'guest input content checks failed; inspect report and actual images'
finally:
 call(RD,rp,RI,'Stop')
