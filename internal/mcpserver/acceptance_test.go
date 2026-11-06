// SPDX-License-Identifier: Apache-2.0
package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestRealStdioGuest is opt-in hardware acceptance, never a simulated VM.
// Start a disposable VM first. See docs/mcp.md for exact prerequisites.
func TestRealStdioGuest(t *testing.T) {
	cli := os.Getenv("AGENTLINUX_ACCEPT_CLI")
	if cli == "" {
		t.Skip("real Mac/VM acceptance not requested")
	}
	root, lima, dir := os.Getenv("AGENTLINUX_ACCEPT_ROOT"), os.Getenv("AGENTLINUX_ACCEPT_LIMA"), os.Getenv("AGENTLINUX_ACCEPT_EVIDENCE")
	if root == "" || lima == "" || dir == "" {
		t.Fatal("all AGENTLINUX_ACCEPT_* paths required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "agent-linux-real-acceptance", Version: "1"}, nil)
	session, e := client.Connect(ctx, &mcp.CommandTransport{Command: exec.Command(cli, "mcp", "--state-dir", root, "--lima", lima)}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer session.Close()
	_, v := call(t, session, "computer_status", nil)
	// A viewer may hold the cross-process lock during a capture. Status is a
	// read: retry only explicit busy responses, never an uncertain mutation.
	for attempt := 0; !v.OK && v.Code == "MACHINE_BUSY" && attempt < 20; attempt++ {
		time.Sleep(100 * time.Millisecond)
		_, v = call(t, session, "computer_status", nil)
	}
	if !v.OK {
		t.Fatal(v)
	}
	state := v.Data.(map[string]any)
	if state["state"] != "ready" {
		t.Fatal(v)
	}
	snap := func(name string) string {
		r, v := call(t, session, "computer_screenshot", nil)
		if !v.OK {
			t.Fatal(v)
		}
		im := r.Content[1].(*mcp.ImageContent)
		if e := os.WriteFile(filepath.Join(dir, name+".png"), im.Data, 0600); e != nil {
			t.Fatal(e)
		}
		return v.Data.(map[string]any)["id"].(string)
	}
	snap("mcp-before")
	// New guests advertise the additional launch targets; legacy v2 guests remain usable.
	if guest, ok := state["guest"].(map[string]any); ok {
		if caps, ok := guest["capabilities"].(map[string]any); ok {
			if apps, ok := caps["apps"].([]any); ok {
				for _, app := range apps {
					if app == "files" {
						_, opened := call(t, session, "computer_open", map[string]any{"app": "files"})
						if !opened.OK {
							t.Fatal(opened)
						}
						snap("mcp-files")
					}
				}
			}
		}
	}
	_, v = call(t, session, "computer_open", map[string]any{"app": "browser"})
	if !v.OK {
		t.Fatal(v)
	}
	if os.Getenv("AGENTLINUX_ACCEPT_FOCUS_DEBUG") != "" {
		for _, argv := range [][]string{{"/usr/bin/xdotool", "getactivewindow", "getwindowname"}, {"/usr/bin/xdotool", "getwindowfocus", "getwindowname"}, {"/usr/bin/xprop", "-root", "_NET_ACTIVE_WINDOW", "_NET_CLIENT_LIST_STACKING"}} {
			_, diagnostic := call(t, session, "computer_run_command", map[string]any{"argv": argv})
			t.Logf("focus diagnostic %v: %+v", argv, diagnostic)
		}
	}
	frame := snap("mcp-browser")
	// A human may hand over a fullscreen browser. Observe its guest window
	// state and reveal browser chrome before navigating; do not assume Ctrl+L
	// works in every fullscreen mode.
	_, windowState := call(t, session, "computer_run_command", map[string]any{"argv": []string{"/bin/sh", "-c", "xprop -id $(xdotool getactivewindow) _NET_WM_STATE"}})
	if !windowState.OK {
		t.Fatal(windowState)
	}
	if data, ok := windowState.Data.(map[string]any); ok {
		encoded, _ := data["stdout"].(string)
		states, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("observed guest browser state: %s", states)
		if strings.Contains(string(states), "_NET_WM_STATE_FULLSCREEN") {
			_, result := call(t, session, "computer_key", map[string]any{"frameId": frame, "key": "F11"})
			if !result.OK {
				t.Fatal(result)
			}
			time.Sleep(300 * time.Millisecond)
			frame = snap("mcp-browser-unfullscreen")
		}
	}
	_, v = call(t, session, "computer_key", map[string]any{"frameId": frame, "key": "CTRL+L"})
	if !v.OK {
		t.Fatal(v)
	}
	page := "data:text/html;charset=utf-8," + url.PathEscape(`<meta charset="utf-8"><title>MCP Linux verified</title><h1>MCP controls this Linux guest</h1><p>café — 日本語 👋</p>`)
	frame = snap("mcp-address")
	_, v = call(t, session, "computer_type", map[string]any{"frameId": frame, "text": page})
	if !v.OK {
		t.Fatal(v)
	}
	frame = snap("mcp-typed")
	_, v = call(t, session, "computer_key", map[string]any{"frameId": frame, "key": "ENTER"})
	if !v.OK {
		t.Fatal(v)
	}
	time.Sleep(time.Second)
	snap("mcp-after")
	_, v = call(t, session, "computer_run_command", map[string]any{"argv": []string{"/usr/bin/xdotool", "search", "--onlyvisible", "--name", "MCP Linux verified"}})
	if !v.OK {
		t.Fatal(v)
	}
	execution := v.Data.(map[string]any)
	decoded, e := base64.StdEncoding.DecodeString(execution["stdout"].(string))
	if e != nil || strings.TrimSpace(string(decoded)) == "" {
		t.Fatal("browser title not observed", v, e)
	}
	for _, q := range []struct {
		argv    []string
		timeout int
		code    string
	}{
		{[]string{"/bin/sh", "-c", "echo bounded-error >&2; exit 7"}, 20000, "COMMAND_FAILED"},
		{[]string{"/bin/sleep", "30"}, 100, "COMMAND_TIMEOUT"},
	} {
		r, v := call(t, session, "computer_run_command", map[string]any{"argv": q.argv, "timeoutMs": q.timeout})
		if !r.IsError || v.Code != q.code || v.Data == nil {
			t.Fatal(v)
		}
	}
	report := map[string]any{"client": "official Go SDK v1.8.0 CommandTransport (real stdio)", "mcpProtocol": session.InitializeResult().ProtocolVersion, "machineId": state["machineId"], "guest": state["guest"], "checks": []string{"initialize/list tools", "real status", "PNG images and metadata", "guest browser open/key/Unicode type", "changed screenshot", "visible guest browser title verified via guest command", "nonzero command result and timeout cleanup"}}
	b, _ := json.MarshalIndent(report, "", "  ")
	if e = os.WriteFile(filepath.Join(dir, "mcp-report.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
}
