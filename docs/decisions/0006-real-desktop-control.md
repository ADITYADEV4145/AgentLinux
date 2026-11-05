# ADR 0006: Guest desktop and execution control

Accepted for Phase 2, 2026-11-05.

Provision Xfce/Xvfb with Xauthority, one 1280×800 X11 display, Mousepad/terminal and Chromium. A separate systemd desktop unit generates a fresh display identity only after X/window-manager readiness. The existing helper reports this readiness and uses scrot/xdotool/xclip locally in Linux. No Mac input/screen/clipboard API is part of the product. Wayland is deferred; X11 tools are not interchangeable with it.

Expand the internal protocol to 2.0 with separate request/response bounds and real PNG/result validation. Require recent frame references for input. Keep CLI convenience auto-capture explicit; stable coordinates do not imply stable application layout. Refuse old protocol guests and preserve their disks rather than pretend they were upgraded. No implicit migration.

Enable explicitly requested arbitrary guest execution as the non-sudo agent user. Replace Phase 1's private-tmp/hidden-home/AF_UNIX-only restrictions where they prevented X11, guest file access and browser networking; retain the host/guest boundary and guest system protections. Use delegated command cgroups with atomic clone3 placement, kill/cleanup on completion, timeout, disconnect and output overflow. Unsupported cleanup fails closed. This is not a containment claim against malicious guest code manipulating its own runtime.

Actual implementation evidence corrected two assumptions: xdotool --sync hung when asked to move to an already-current coordinate, so motion now verifies pointer position without that flag; xclip's one-request lifetime was insufficient for clipboard format/data negotiation, so text keeps a bounded guest-only owner alive and acknowledges dispatch, with actual insertion verified separately. Service restart closes owned GUI apps; acceptance must reopen the editor after helper replacement.

The fresh-VM acceptance task verified desktop PNGs, exact ASCII/Unicode saved text, Chromium, mouse actions, cgroup timeout/output/disconnect cleanup, desktop/helper crash recovery, stale-frame rejection and VM restart/persistence. A read-only test fixture observed unchanged host pointer and clipboard change count without reading clipboard contents. See verification.md. No cloud, MCP, live viewer, billing or public deployment is added.
