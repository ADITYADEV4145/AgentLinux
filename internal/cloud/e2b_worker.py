# SPDX-License-Identifier: Apache-2.0
"""Server-only optional SDK adapter. No CLI login, key logging, or public VNC.

Not independently provider-tested. The exact SDK/template/network prerequisites
and pending command-cleanup capability are documented in docs/cloud.md.
"""
import base64
import io
import json
import os
import re
import shlex
import sys
import time
from datetime import datetime
from importlib.metadata import version


class ContractError(Exception):
    pass


def require(ok, code="INVALID_MESSAGE"):
    if not ok:
        raise ContractError(code)


def resource(info):
    return {"id": info.sandbox_id,
            "session": (info.metadata or {}).get("agent_linux_session", ""),
            "deployment": (info.metadata or {}).get("agent_linux_deployment", ""),
            "state": str(info.state)}


def run(q):
    require(os.environ.get("AGENT_LINUX_ENABLE_E2B") == "operator-enabled")
    require(version("e2b-desktop") == "2.7.2")
    from e2b_desktop import Sandbox
    from e2b import Sandbox as CoreSandbox
    from e2b.sandbox.sandbox_api import SandboxQuery, SandboxState
    from e2b.exceptions import NotFoundException
    from PIL import Image
    method = q["method"]
    if method == "list":
        # Explicit all billable/resumable states; never silently truncate pages.
        paginator = CoreSandbox.list(query=SandboxQuery(state=[SandboxState.RUNNING, SandboxState.PAUSED], metadata={"agent_linux_deployment":q["deployment"]}))
        values = []
        pages = 0
        while paginator.has_next:
            pages += 1
            require(pages <= 20, "RESOURCE_LIMIT")
            for item in paginator.next_items():
                r = resource(item)
                if r["deployment"] == q["deployment"]:
                    values.append(r)
                    require(len(values) <= 1000, "RESOURCE_LIMIT")
        return {"resources": values}
    if method == "delete":
        r = q["resource"]
        try:
            info = CoreSandbox.get_info(r["id"], request_timeout=10)
            require(resource(info)["deployment"] == r["deployment"])
            require(resource(info)["session"] == r["session"])
            CoreSandbox.kill(r["id"], request_timeout=10)
            try:
                CoreSandbox.get_info(r["id"], request_timeout=10)
            except NotFoundException:
                return {}
            raise ContractError("PROVIDER_UNAVAILABLE")
        except NotFoundException:
            return {}
    s = q["session"]
    if method == "create":
        remaining = int(datetime.fromisoformat(s["expires"].replace("Z", "+00:00")).timestamp() - time.time())
        require(1 <= remaining <= 900)
        box = Sandbox.create(template=q["template"], resolution=(1280, 800), dpi=96,
                             timeout=remaining, idempotency_key=s["id"],
                             metadata={"agent_linux_deployment": q["deployment"], "agent_linux_session": s["id"]},
                             lifecycle={"on_timeout": "kill", "auto_resume": False},
                             network={"allow_public_traffic": False,
                                      "deny_out": ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16", "127.0.0.0/8", "::1/128", "fc00::/7", "fe80::/10"]},
                             request_timeout=15)
        return {"resource": {"id": box.sandbox_id, "session": s["id"], "deployment": q["deployment"], "state": "running"}}
    require(re.fullmatch(r"[A-Za-z0-9_-]{1,100}", s["providerId"]) is not None)
    info = CoreSandbox.get_info(s["providerId"], request_timeout=10)
    require((info.metadata or {}).get("agent_linux_session") == s["id"])
    # Do not resume a paused session or extend its lifetime on reconnect.
    require(str(getattr(info.state, "value", info.state)) == "running", "DESKTOP_UNAVAILABLE")
    require(2 <= info.cpu_count <= 4 and 4096 <= info.memory_mb <= 8192, "RESOURCE_LIMIT")
    box = Sandbox.connect(s["providerId"], timeout=1, request_timeout=10)
    def command(cmd):
        return box.commands.run(cmd, envs={"DISPLAY": ":0"}, timeout=12).stdout
    generation = command("pgrep -x xfce4-panel >/dev/null && pgrep -x xfdesktop >/dev/null && xprop -root _NET_SUPPORTING_WM_CHECK && xprop -root _AGENT_LINUX_GENERATION")
    require("window id # 0x" in generation, "DESKTOP_UNAVAILABLE")
    match = re.search(r'_AGENT_LINUX_GENERATION.*= "([0-9a-f-]{36})"', generation)
    if match is None:
        require(method == "probe", "STALE_FRAME")
        import uuid
        generation = str(uuid.uuid4())
        command("xprop -root -f _AGENT_LINUX_GENERATION 8s -set _AGENT_LINUX_GENERATION " + shlex.quote(generation))
    else:
        generation = match.group(1)
    if method == "probe":
        # Real decoded frame + WM/components, not just provider 'running'.
        data = bytes(box.screenshot())
        require(0 < len(data) <= 8 << 20, "RESOURCE_LIMIT")
        with Image.open(io.BytesIO(data)) as image:
            require(image.size == (1280, 800), "DESKTOP_UNAVAILABLE")
            image.verify()
        disk = int(command("df -B1 --output=size / | tail -1").strip())
        require(disk <= 40 * 1024**3, "RESOURCE_LIMIT")
        command("command -v xdotool && command -v python3")
        command("xdotool mouseup 1 mouseup 2 mouseup 3 keyup ctrl alt shift super")
        inputs = ["move", "click", "drag", "scroll", "key"]
        if command("command -v xclip || true").strip():
            inputs.append("text")
        return {"probe": {"ready": True, "generation": generation,
                          "capabilities": {"displays": 1, "format": "png", "origin": "top-left", "scale": 1,
                                           "input": inputs, "pasteModes": ["standard", "terminal"],
                                           "maxPngBytes": 8 << 20, "maxTextBytes": 65536,
                                           "maxOutputBytes": 0, "maxExecMs": 0, "arbitraryGuestExec": False}}}
    op = q["operation"]
    require(generation == s["generation"], "STALE_FRAME")
    if op["method"] == "screenshot":
        data = bytes(box.screenshot())
        require(0 < len(data) <= 8 << 20, "RESOURCE_LIMIT")
        after = command("xprop -root _AGENT_LINUX_GENERATION")
        require(generation in after, "STALE_FRAME")
        return {"output": {"png": base64.b64encode(data).decode(), "generation": generation}}
    require(op["method"] == "input", "UNSUPPORTED_OPERATION")
    action = op["input"]
    kind = action["kind"]
    require(kind in ("move", "click", "drag", "scroll", "text", "key"), "UNSUPPORTED_OPERATION")
    # Fixed guest-only runner. Every dynamic argument is a JSON/base64 value;
    # shell interpolation never receives raw input text or key strings.
    payload = base64.b64encode(json.dumps(action).encode()).decode()
    guest = '''import base64,json,os,re,subprocess,sys,time
q=json.loads(base64.b64decode(sys.argv[1]));kind=q['kind'];clip=None
def x(*a):subprocess.run(['xdotool',*map(str,a)],check=True,timeout=5,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
def pos(a,b):
 if not (0<=a<1280 and 0<=b<800):raise ValueError('coordinates')
 x('mousemove',a,b)
button={'left':1,'middle':2,'right':3,'':1}[q.get('button','')]
try:
 if kind in ('move','click','scroll','drag'):pos(q['x'],q['y'])
 if kind=='click':x('click','--clearmodifiers','--repeat',q.get('clicks') or 1,'--delay',100,button)
 if kind=='drag':
  x('mousedown',button);pos(q['toX'],q['toY']);x('mouseup',button)
 if kind=='scroll':
  for value,neg,plus in [(q.get('dx',0),6,7),(q.get('dy',0),4,5)]:
   if abs(value)>100:raise ValueError('scroll')
   if value:x('click','--repeat',abs(value),neg if value<0 else plus)
 if kind=='key':
  parts=q['key'].upper().split('+');mods={'CTRL':'ctrl','ALT':'alt','SHIFT':'shift','SUPER':'super'}
  keys={'ENTER':'Return','RETURN':'Return','TAB':'Tab','ESC':'Escape','ESCAPE':'Escape','BACKSPACE':'BackSpace','DELETE':'Delete','HOME':'Home','END':'End','PAGEUP':'Prior','PAGEDOWN':'Next','LEFT':'Left','RIGHT':'Right','UP':'Up','DOWN':'Down','SPACE':'space'}
  last=parts[-1];key=keys.get(last)
  if key is None:
   if not re.fullmatch(r'[A-Z0-9]|F(?:[1-9]|1[0-2])',last):raise ValueError('key')
   key=last if len(last)>1 else last.lower()
  x('key','--clearmodifiers','+'.join([mods[p] for p in parts[:-1]]+[key]))
 if kind=='text':
  text=q['text'].encode('utf-8')
  if len(text)>65536 or 0 in text:raise ValueError('text')
  clip=subprocess.Popen(['xclip','-selection','clipboard','-in','-quiet'],stdin=subprocess.PIPE,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
  clip.stdin.write(text);clip.stdin.close();time.sleep(.15)
  x('key','--clearmodifiers','ctrl+shift+v' if q.get('pasteMode')=='terminal' else 'ctrl+v');time.sleep(.15)
finally:
 try:x('mouseup',1,'mouseup',2,'mouseup',3,'keyup','ctrl','alt','shift','super')
 finally:
  if clip is not None:
   clip.kill();clip.wait()
'''
    command("python3 -c " + shlex.quote(guest) + " " + shlex.quote(payload))
    return {"output": {"generation": generation, "input": {"dispatched": True, "frameId": action["frameId"], "generation": generation}}}


def main():
    try:
        raw = sys.stdin.buffer.read(262145)
        require(len(raw) <= 262144)
        q = json.loads(raw)
        result = run(q)
        print(json.dumps({"ok": True, **result}))
    except ContractError as exc:
        print(json.dumps({"ok": False, "code": str(exc)}))
    except Exception:
        # SDK errors may include credentials, URLs, commands or screen content.
        print(json.dumps({"ok": False, "code": "PROVIDER_UNAVAILABLE"}))


if __name__ == "__main__":
    main()
