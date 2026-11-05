// SPDX-License-Identifier: Apache-2.0
package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Opt-in real hardware experiment. Use ONLY a disposable guest: this creates
// files/windows and types commands into its terminal. Never selects a default VM.
func TestRealWorkstation(t *testing.T) {
	if os.Getenv("AGENTLINUX_WORKSTATION") != "disposable" {
		t.Skip("explicit disposable guest required")
	}
	cli, root, lima, dir := os.Getenv("AGENTLINUX_ACCEPT_CLI"), os.Getenv("AGENTLINUX_ACCEPT_ROOT"), os.Getenv("AGENTLINUX_ACCEPT_LIMA"), os.Getenv("AGENTLINUX_ACCEPT_EVIDENCE")
	if cli == "" || lima == "" || dir == "" || filepath.Dir(root) != "/private/tmp" || !strings.HasPrefix(filepath.Base(root), "agentlinux-phase55-") {
		t.Fatal("explicit phase55 fixture paths required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "workstation-comparison", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: exec.Command(cli, "mcp", "--state-dir", root, "--lima", lima)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	var latencies []float64
	completed := []string{}
	passed := false
	started := time.Now()
	defer func() {
		b, _ := json.MarshalIndent(map[string]any{"passed": passed && !t.Failed(), "completed": completed, "seconds": time.Since(started).Seconds(), "screenshotSeconds": latencies, "client": "official Go SDK stdio, deterministic; not an AI model"}, "", "  ")
		_ = os.WriteFile(filepath.Join(dir, "workstation-report.json"), b, 0600)
	}()
	invoke := func(name string, args any) any {
		t.Helper()
		_, v := call(t, session, "computer_"+name, args)
		if !v.OK {
			t.Fatalf("%s: %s %s", name, v.Code, v.Message)
		}
		return v.Data
	}
	seq := 0
	snap := func(label string) string {
		t.Helper()
		before := time.Now()
		r, v := call(t, session, "computer_screenshot", nil)
		latencies = append(latencies, time.Since(before).Seconds())
		if !v.OK {
			t.Fatal(v)
		}
		frame := v.Data.(map[string]any)
		if frame["width"] != float64(1280) || frame["height"] != float64(800) || frame["scale"] != float64(1) {
			t.Fatal(frame)
		}
		seq++
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%02d-%s.png", seq, label)), r.Content[1].(*mcp.ImageContent).Data, 0600); err != nil {
			t.Fatal(err)
		}
		return frame["id"].(string)
	}
	input := func(kind string, args map[string]any) { t.Helper(); args["frameId"] = snap(kind); invoke(kind, args) }
	key := func(k string) { t.Helper(); input("key", map[string]any{"key": k}) }
	text := func(s string) { t.Helper(); input("type", map[string]any{"text": s}) }
	command := func(argv ...string) string {
		t.Helper()
		d := invoke("run_command", map[string]any{"argv": argv}).(map[string]any)
		encoded, _ := d["stdout"].(string)
		b, e := base64.StdEncoding.DecodeString(encoded)
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	open := func(app string) { t.Helper(); invoke("open", map[string]any{"app": app}); snap(app) }
	if invoke("status", nil).(map[string]any)["state"] != "ready" {
		t.Fatal("not ready")
	}
	snap("desktop")
	completed = append(completed, "ready, dimensions, real PNG")
	open("launcher")
	key("ESC")
	completed = append(completed, "launcher")
	suffix := fmt.Sprintf("workstation-%d", time.Now().UnixNano())
	open("files")
	key("CTRL+SHIFT+N")
	text(suffix)
	key("ENTER")
	command("/usr/bin/test", "-d", "/home/agent/"+suffix)
	completed = append(completed, "file-manager creates folder")
	open("terminal")
	input("type", map[string]any{"text": "printf 'terminal verified\\n' > /home/agent/" + suffix + ".terminal", "pasteMode": "terminal"})
	key("ENTER")
	time.Sleep(200 * time.Millisecond)
	if command("/bin/cat", "/home/agent/"+suffix+".terminal") != "terminal verified\n" {
		t.Fatal("terminal keyboard command failed")
	}
	completed = append(completed, "terminal keyboard command")
	file := "/home/agent/" + suffix + ".txt"
	command("/usr/bin/touch", file)
	open("editor")
	key("CTRL+O")
	text(file)
	key("ENTER")
	// Wait for the file-open transition instead of launching the editor again:
	// a single-instance editor can interpret a second launch as a new blank tab.
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(command("/usr/bin/xdotool", "getactivewindow", "getwindowname"), suffix) {
		if time.Now().After(deadline) {
			t.Fatal("editor did not focus the requested file")
		}
		time.Sleep(100 * time.Millisecond)
	}
	sample := "Agent Linux workstation\ncafé — 日本語 👋\n"
	text(sample)
	key("CTRL+A")
	key("CTRL+C")
	key("CTRL+END")
	key("CTRL+V")
	key("CTRL+S")
	if command("/bin/cat", file) != sample+sample {
		t.Fatal("Unicode/guest clipboard mismatch")
	}
	snap("saved-unicode")
	completed = append(completed, "editor exact Unicode, guest clipboard, save")
	page := `<meta charset="utf-8"><title>Ready</title><style>body{background:#eef2ff;font:24px sans-serif;height:4000px}input,button{position:absolute;top:100px;height:50px;font:24px sans-serif}input{left:100px;width:300px}button{left:450px;width:150px}#drag{position:absolute;left:100px;top:250px;width:300px;height:100px;background:#93c5fd}</style><h1>Agent Linux controlled fixture</h1><form onsubmit="event.preventDefault();document.title='Submitted '+document.querySelector('input').value"><input autofocus><button>Verify</button></form><div id=drag onmousedown="this.dataset.down='yes'" onmouseup="if(this.dataset.down==='yes')document.title='Dragged'">Drag here</div><script>onscroll=()=>{if(scrollY>0)document.title='Scrolled'}</script>`
	for _, browser := range []string{"chromium", "firefox"} {
		open(browser)
		key("CTRL+L")
		text("data:text/html;charset=utf-8," + url.PathEscape(page))
		key("ENTER")
		time.Sleep(time.Second)
		key("F11")
		time.Sleep(400 * time.Millisecond)
		snap(browser + "-fixture")
		input("click", map[string]any{"x": 150, "y": 120, "button": "left", "clicks": 1})
		text("café 日本語")
		key("TAB")
		key("ENTER")
		if !strings.Contains(command("/usr/bin/xdotool", "getactivewindow", "getwindowname"), "Submitted café 日本語") {
			t.Fatal("browser form not verified: ", browser)
		}
		snap(browser + "-submitted")
		input("drag", map[string]any{"x": 150, "y": 280, "toX": 300, "toY": 300, "button": "left"})
		if !strings.Contains(command("/usr/bin/xdotool", "getactivewindow", "getwindowname"), "Dragged") {
			t.Fatal("drag not observed")
		}
		input("scroll", map[string]any{"x": 800, "y": 600, "dx": 0, "dy": 3})
		time.Sleep(200 * time.Millisecond)
		if !strings.Contains(command("/usr/bin/xdotool", "getactivewindow", "getwindowname"), "Scrolled") {
			t.Fatal("scroll not observed")
		}
		input("move", map[string]any{"x": 850, "y": 600})
		if !strings.HasPrefix(command("/usr/bin/xdotool", "getmouselocation"), "x:850 y:600 ") {
			t.Fatal("coordinate mismatch")
		}
		input("click", map[string]any{"x": 700, "y": 400, "button": "right", "clicks": 1})
		key("ESC")
		input("click", map[string]any{"x": 700, "y": 400, "button": "left", "clicks": 2})
		key("F11")
		completed = append(completed, browser+" Unicode form, semantic drag/scroll, coordinates, right/double-click dispatched")
	}
	open("editor")
	if !strings.Contains(command("/usr/bin/xdotool", "getactivewindow", "getwindowname"), suffix) {
		t.Fatal("editor focus wrong")
	}
	open("files")
	if !strings.Contains(strings.ToLower(command("/bin/sh", "-c", `xprop -id "$(xdotool getactivewindow)" WM_CLASS`)), "thunar") && !strings.Contains(strings.ToLower(command("/bin/sh", "-c", `xprop -id "$(xdotool getactivewindow)" WM_CLASS`)), "dolphin") {
		t.Fatal("file manager focus wrong")
	}
	completed = append(completed, "application focus switching")
	key("ALT+TAB")
	snap("final")
	inventory := command("/bin/sh", "-c", "cat /proc/meminfo | head -5; df -B1 /; ps -eo comm,rss,pcpu --sort=-rss | head -15")
	if e := os.WriteFile(filepath.Join(dir, "inventory.txt"), []byte(inventory), 0600); e != nil {
		t.Fatal(e)
	}
	passed = true
}
