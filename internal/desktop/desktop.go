// SPDX-License-Identifier: Apache-2.0
// Package desktop is Linux guest-only computer control. It never targets the host.
package desktop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/process"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

const Display = ":1"
const Authority = "/run/agent-linux-desktop/Xauthority"

func Env() []string {
	return []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/home/agent", "USER=agent", "LOGNAME=agent", "LANG=C.UTF-8", "DISPLAY=" + Display, "XAUTHORITY=" + Authority, "XDG_RUNTIME_DIR=/run/agent-linux-desktop", "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/agent-linux-desktop/bus"}
}

type Runner func(context.Context, string, ...string) (process.Result, error)
type observation struct {
	generation string
	when       time.Time
}
type Desktop struct {
	mu       sync.Mutex
	sequence uint64
	frames   map[string]observation
	run      Runner
	apps     map[string]*exec.Cmd
	capture  func(context.Context) ([]byte, error)
}

func New() *Desktop {
	return &Desktop{frames: map[string]observation{}, apps: map[string]*exec.Cmd{}, run: func(ctx context.Context, name string, args ...string) (process.Result, error) {
		return process.Run(ctx, Env(), name, args...)
	}}
}
func (d *Desktop) Status(ctx context.Context) *protocol.DesktopStatus {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.status(ctx)
}
func (d *Desktop) status(ctx context.Context) *protocol.DesktopStatus {
	result := &protocol.DesktopStatus{Display: Display, Server: "X11/Xvfb", Width: protocol.Width, Height: protocol.Height}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	r, e := d.run(ctx, "/usr/bin/xdotool", "getdisplaygeometry")
	if e != nil || strings.TrimSpace(string(r.Out)) != "1280 800" {
		result.Reason = "managed X11 display is unavailable or has unsupported dimensions"
		return result
	}
	r, e = d.run(ctx, "/usr/bin/xprop", "-root", "_AGENT_LINUX_GENERATION")
	if e != nil {
		result.Reason = "desktop generation unavailable"
		return result
	}
	parts := strings.SplitN(strings.TrimSpace(string(r.Out)), " = ", 2)
	if len(parts) != 2 {
		result.Reason = "desktop session is not ready"
		return result
	}
	gen, e := strconv.Unquote(parts[1])
	if e != nil || len(gen) != 36 {
		result.Reason = "invalid managed desktop generation"
		return result
	}
	r, e = d.run(ctx, "/usr/bin/xprop", "-root", "_NET_SUPPORTING_WM_CHECK")
	if e != nil || !strings.Contains(string(r.Out), "window id # 0x") {
		result.Reason = "X11 exists but window manager is unavailable"
		return result
	}
	for _, name := range []string{"xfce4-panel", "xfdesktop"} {
		if _, e = d.run(ctx, "/usr/bin/pgrep", "-u", "agent", "-x", name); e != nil {
			result.Reason = "desktop component unavailable: " + name
			return result
		}
	}
	result.Ready = true
	result.Generation = gen
	return result
}
func (d *Desktop) Screenshot(ctx context.Context, session string) (*protocol.Frame, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	before := d.status(ctx)
	if !before.Ready {
		return nil, protocol.Fail("DESKTOP_UNAVAILABLE", before.Reason)
	}
	var data []byte
	var e error
	if d.capture != nil {
		data, e = d.capture(ctx)
	} else {
		data, e = capturePNG(ctx)
	}
	if e != nil {
		return nil, e
	}
	after := d.status(ctx)
	if !after.Ready || after.Generation != before.Generation {
		return nil, protocol.Fail("STALE_FRAME", "display restarted during capture")
	}
	d.sequence++
	frame := &protocol.Frame{ID: protocol.ID(), SessionID: session, Generation: after.Generation, Sequence: d.sequence, CapturedAt: time.Now().UTC(), Width: protocol.Width, Height: protocol.Height, Scale: 1, Format: "png", PNG: data}
	if e := frame.Validate(); e != nil {
		return nil, e
	}
	for id, entry := range d.frames {
		if time.Since(entry.when) > 10*time.Second || entry.generation != after.Generation {
			delete(d.frames, id)
		}
	}
	if len(d.frames) >= 16 {
		var oldest string
		var when time.Time
		for id, entry := range d.frames {
			if oldest == "" || entry.when.Before(when) {
				oldest = id
				when = entry.when
			}
		}
		delete(d.frames, oldest)
	}
	d.frames[frame.ID] = observation{after.Generation, time.Now()}
	return frame, nil
}
func (d *Desktop) Input(ctx context.Context, i protocol.Input) (*protocol.InputResult, error) {
	if e := i.Validate(); e != nil {
		return nil, e
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	status := d.status(ctx)
	if !status.Ready {
		return nil, protocol.Fail("DESKTOP_UNAVAILABLE", status.Reason)
	}
	entry, ok := d.frames[i.FrameID]
	if !ok || entry.generation != status.Generation || time.Since(entry.when) > 10*time.Second {
		return nil, protocol.Fail("STALE_FRAME", "capture a fresh frame after a display/session change or 10 seconds")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	defer d.release()
	xdo := func(args ...string) error { _, e := d.run(ctx, "/usr/bin/xdotool", args...); return e }
	num := strconv.Itoa
	move := func(x, y int) error {
		if e := xdo("mousemove", num(x), num(y)); e != nil {
			return e
		}
		for attempt := 0; attempt < 5; attempt++ {
			r, e := d.run(ctx, "/usr/bin/xdotool", "getmouselocation")
			if e != nil {
				return e
			}
			fields := strings.Fields(string(r.Out))
			if len(fields) >= 2 && fields[0] == "x:"+num(x) && fields[1] == "y:"+num(y) {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Millisecond):
			}
		}
		return fmt.Errorf("guest pointer did not reach requested coordinates")
	}
	button := map[string]string{"": "1", "left": "1", "middle": "2", "right": "3"}[i.Button]
	var e error
	switch i.Kind {
	case "move":
		e = move(i.X, i.Y)
	case "click":
		if e = move(i.X, i.Y); e == nil {
			n := i.Clicks
			if n == 0 {
				n = 1
			}
			e = xdo("click", "--clearmodifiers", "--repeat", num(n), "--delay", "100", button)
		}
	case "drag":
		if e = move(i.X, i.Y); e == nil {
			if e = xdo("mousedown", button); e == nil {
				for step := 1; step <= 10; step++ {
					if e = move(i.X+(i.ToX-i.X)*step/10, i.Y+(i.ToY-i.Y)*step/10); e != nil {
						break
					}
					select {
					case <-ctx.Done():
						e = ctx.Err()
					case <-time.After(15 * time.Millisecond):
					}
				}
				if e == nil {
					e = xdo("mouseup", button)
				}
			}
		}
	case "scroll":
		if e = move(i.X, i.Y); e == nil {
			for _, v := range []struct {
				delta              int
				negative, positive string
			}{{i.DY, "4", "5"}, {i.DX, "6", "7"}} {
				n := v.delta
				b := v.positive
				if n < 0 {
					n = -n
					b = v.negative
				}
				if n > 0 {
					if e = xdo("click", "--clearmodifiers", "--repeat", num(n), "--delay", "10", b); e != nil {
						break
					}
				}
			}
		}
	case "key":
		key, _ := protocol.TranslateKey(i.Key)
		e = xdo("key", "--clearmodifiers", key)
	case "text":
		mode := i.PasteMode
		if mode == "auto" {
			mode = "standard"
			w, err := d.run(ctx, "/usr/bin/xdotool", "getactivewindow")
			if err != nil {
				return nil, protocol.Fail("INPUT_FAILED", "cannot determine focused guest application")
			}
			id := strings.TrimSpace(string(w.Out))
			if _, err = strconv.ParseUint(id, 10, 64); err != nil {
				return nil, protocol.Fail("INPUT_FAILED", "invalid focused guest window")
			}
			app, err := d.run(ctx, "/usr/bin/xprop", "-id", id, "WM_CLASS")
			if err != nil {
				return nil, protocol.Fail("INPUT_FAILED", "cannot inspect focused guest application")
			}
			class := strings.ToLower(string(app.Out))
			for _, name := range []string{"terminal", "konsole", "ptyxis", "xterm", "alacritty", "kitty"} {
				if strings.Contains(class, name) {
					mode = "terminal"
					break
				}
			}
		}
		e = d.paste(ctx, i.Text, mode)
	}
	if e != nil {
		return nil, protocol.Fail("INPUT_FAILED", "guest input failed or timed out; state may have changed, observe before retrying")
	}
	after := d.status(ctx)
	if !after.Ready || after.Generation != status.Generation {
		return nil, protocol.Fail("OUTCOME_UNKNOWN", "desktop changed during input; capture before retrying")
	}
	return &protocol.InputResult{Dispatched: true, FrameID: i.FrameID, Generation: status.Generation, Note: "input dispatched; verify application outcome with a fresh screenshot"}, nil
}
func (d *Desktop) release() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _ = d.run(ctx, "/usr/bin/xdotool", "keyup", "ctrl", "alt", "shift", "super", "mouseup", "1", "mouseup", "2", "mouseup", "3")
}
func (d *Desktop) paste(ctx context.Context, text, mode string) error {
	if text == "" {
		return nil
	}
	clipCtx, clipCancel := context.WithCancel(ctx)
	defer clipCancel()
	clip := process.Prepare(clipCtx, "/usr/bin/xclip", "-selection", "clipboard", "-in", "-quiet", "-loops", "0")
	clip.Env = Env()
	clip.Stdin = strings.NewReader(text)
	clip.Stdout = &process.Buffer{Limit: 4096}
	clip.Stderr = &process.Buffer{Limit: 4096}
	if e := clip.Start(); e != nil {
		return e
	}

	done := make(chan struct{})
	var waitErr error
	go func() { waitErr = clip.Wait(); close(done) }()
	defer func() { clipCancel(); <-done }()
	// Let the guest selection owner initialize. Keep it alive across format/data queries;
	// dispatch is acknowledged, application text insertion must still be verified.
	select {
	case <-done:
		return fmt.Errorf("guest clipboard owner ended before paste: %v", waitErr)
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}
	chord := "ctrl+v"
	if mode == "terminal" {
		chord = "ctrl+shift+v"
	}
	if _, e := d.run(ctx, "/usr/bin/xdotool", "key", "--clearmodifiers", chord); e != nil {
		return e
	}
	select {
	case <-done:
		return fmt.Errorf("guest clipboard ownership lost during paste: %v", waitErr)
	case <-ctx.Done():
		clipCancel()
		<-done
		return ctx.Err()
	case <-time.After(500 * time.Millisecond):
		clipCancel()
		<-done
		return nil
	}

}
func (d *Desktop) Open(ctx context.Context, app string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.status(ctx)
	if !s.Ready {
		return protocol.Fail("DESKTOP_UNAVAILABLE", s.Reason)
	}
	argv, class, e := application(app)
	if e != nil {
		return e
	}
	// Focus a visible existing window, including one opened through the menu.
	existing, e := d.run(ctx, "/usr/bin/xdotool", "search", "--onlyvisible", "--class", class)
	if e == nil && len(strings.Fields(string(existing.Out))) > 0 {
		return d.activate(ctx, strings.Fields(string(existing.Out))[0])
	}
	if _, e = os.Stat(argv[0]); e != nil {
		return protocol.Fail("APP_UNAVAILABLE", "guest application is not installed; inspect browsers list or install the documented Debian package")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = Env()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	dev, e := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if e != nil {
		return e
	}
	defer dev.Close()
	cmd.Stdin = dev
	cmd.Stdout = dev
	cmd.Stderr = dev
	if e = cmd.Start(); e != nil {
		return e
	}
	d.apps[app] = cmd
	go func() { _ = cmd.Wait() }()
	check, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	window, e := d.run(check, "/usr/bin/xdotool", "search", "--sync", "--onlyvisible", "--class", class)
	if e != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		delete(d.apps, app)
		return protocol.Fail("APP_UNAVAILABLE", "application window did not become visible")
	}
	fields := strings.Fields(string(window.Out))
	if len(fields) == 0 {
		return protocol.Fail("APP_UNAVAILABLE", "application returned no visible window")
	}
	return d.activate(check, fields[0])
}
func (d *Desktop) activate(ctx context.Context, id string) error {
	if _, e := strconv.ParseUint(id, 10, 64); e != nil {
		return protocol.Fail("APP_UNAVAILABLE", "invalid guest window identity")
	}
	if _, e := d.run(ctx, "/usr/bin/xdotool", "windowactivate", "--sync", id); e != nil {
		return protocol.Fail("APP_UNAVAILABLE", "guest application window could not be focused")
	}
	// EWMH activation alone can leave keyboard focus on a different guest app,
	// especially after switching away from a fullscreen browser. Focus the actual
	// selected guest window and verify it before acknowledging app activation.
	if _, e := d.run(ctx, "/usr/bin/xdotool", "windowraise", id); e != nil {
		return protocol.Fail("APP_UNAVAILABLE", "guest window could not be raised")
	}
	if _, e := d.run(ctx, "/usr/bin/xdotool", "windowfocus", "--sync", id); e != nil {
		return protocol.Fail("APP_UNAVAILABLE", "guest keyboard focus unavailable")
	}
	focused, e := d.run(ctx, "/usr/bin/xdotool", "getwindowfocus")
	if e != nil || strings.TrimSpace(string(focused.Out)) != id {
		return protocol.Fail("APP_UNAVAILABLE", "guest window activation did not establish keyboard focus")
	}
	return nil
}
func application(app string) ([]string, string, error) {
	switch app {
	case "editor":
		return []string{"/usr/bin/mousepad"}, "Mousepad", nil
	case "terminal":
		return []string{"/usr/bin/xfce4-terminal", "--disable-server"}, "Xfce4-terminal", nil
	case "browser", "chromium":
		return []string{"/usr/bin/chromium", "--no-first-run", "--disable-dev-shm-usage", "about:blank"}, "Chromium", nil
	case "firefox":
		return []string{"/usr/bin/firefox-esr", "--new-window", "about:blank"}, "firefox-esr", nil
	case "files":
		return []string{"/usr/bin/thunar", "/home/agent"}, "Thunar", nil
	case "settings":
		return []string{"/usr/bin/xfce4-settings-manager"}, "Xfce4-settings-manager", nil
	case "launcher":
		return []string{"/usr/bin/xfce4-appfinder", "--disable-server"}, "Xfce4-appfinder", nil
	default:
		return nil, "", protocol.Fail("INVALID_ARGUMENT", "unknown guest application; inspect status capabilities")
	}
}

func capturePNG(ctx context.Context) ([]byte, error) {
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := process.Prepare(c, "/usr/bin/scrot", "--silent", "--format", "png", "--file", "-")
	cmd.Env = Env()
	out := &process.Buffer{Limit: protocol.MaxPNG}
	errout := &process.Buffer{Limit: 4096}
	cmd.Stdout = out
	cmd.Stderr = errout
	if e := cmd.Run(); e != nil {
		return nil, protocol.Fail("CAPTURE_FAILED", "guest screenshot failed; inspect managed desktop")
	}
	if out.Truncated {
		return nil, protocol.Fail("FRAME_TOO_LARGE", "guest PNG exceeds 8 MiB")
	}
	return out.Bytes(), nil
}
