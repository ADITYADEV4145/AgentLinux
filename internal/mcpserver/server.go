// SPDX-License-Identifier: Apache-2.0
// Package mcpserver adapts the common machine interface to the official MCP SDK.
package mcpserver

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/machine"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed instructions.txt
var Instructions string

const Version = "0.3.0"

type Options struct {
	Debug   io.Writer     // Only operation name, outcome code and duration; never arguments/results.
	Timeout time.Duration // Shorter deadlines are useful in tests; maximum is 70 seconds.
}
type Result struct {
	OK      bool   `json:"ok"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}
type arguments struct {
	protocol.Input
	protocol.ExecRequest
	App string `json:"app"`
}

// New exposes the implemented local contract. Backend capabilities/readiness are
// queried live by computer_status; unavailable features fail without fallback.
func New(computer machine.Machine, opts Options) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "agent-linux", Version: Version}, &mcp.ServerOptions{Instructions: Instructions, Capabilities: &mcp.ServerCapabilities{}})
	var logMu sync.Mutex
	gate := make(chan struct{}, 1) // No queued mutations, automatic retries or blind replay.
	limit := opts.Timeout
	if limit <= 0 || limit > 70*time.Second {
		limit = 70 * time.Second
	}
	add := func(name, description string, properties map[string]*jsonschema.Schema, required []string) {
		schema := &jsonschema.Schema{Type: "object", Properties: properties, Required: required, AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}}}
		resolved, err := schema.Resolve(nil)
		if err != nil {
			panic(err)
		} // Developer-owned static schemas only.
		s.AddTool(&mcp.Tool{Name: name, Description: description, InputSchema: schema,
			OutputSchema: &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{"ok": {Type: "boolean"}, "code": {Type: "string"}, "message": {Type: "string"}, "data": {}}, Required: []string{"ok", "code"}},
			Annotations:  &mcp.ToolAnnotations{ReadOnlyHint: name == "computer_status" || name == "computer_screenshot"}}, func(parent context.Context, req *mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
			started := time.Now()
			code := "INTERNAL_ERROR"
			defer func() {
				if opts.Debug != nil {
					logMu.Lock()
					defer logMu.Unlock()
					fmt.Fprintf(opts.Debug, "tool=%s code=%s duration_ms=%d\n", name, code, time.Since(started).Milliseconds())
				}
			}()
			finish := func(data any, failure error, image []byte) *mcp.CallToolResult {
				output := Result{OK: failure == nil, Code: "OK", Data: data}
				if failure != nil {
					output.Data = nil
					output.Code, output.Message = failureCode(failure)
					output.OK = false
				}
				code = output.Code
				b, _ := json.Marshal(output)
				contents := []mcp.Content{&mcp.TextContent{Text: string(b)}}
				if len(image) > 0 && failure == nil {
					contents = append(contents, &mcp.ImageContent{Data: image, MIMEType: "image/png"})
				}
				return &mcp.CallToolResult{IsError: failure != nil, StructuredContent: output, Content: contents}
			}
			invalid := func() *mcp.CallToolResult {
				return finish(nil, protocol.Fail("INVALID_ARGUMENT", "arguments do not match the tool schema; inspect tools/list"), nil)
			}
			if len(req.Params.Arguments) > protocol.MaxRequest {
				return invalid(), nil
			}
			raw := req.Params.Arguments
			if len(raw) == 0 {
				raw = json.RawMessage(`{}`)
			}
			var object map[string]any
			if json.Unmarshal(raw, &object) != nil || object == nil || resolved.Validate(object) != nil {
				return invalid(), nil
			}
			var a arguments
			if json.Unmarshal(raw, &a) != nil {
				return invalid(), nil
			}
			select {
			case gate <- struct{}{}:
				defer func() { <-gate }()
			default:
				return finish(nil, protocol.Fail("MACHINE_BUSY", "another MCP operation is in progress; observe before retrying"), nil), nil
			}
			ctx, cancel := context.WithTimeout(parent, limit)
			defer cancel()
			switch name {
			case "computer_status":
				v, e := computer.Status(ctx)
				return finish(v, e, nil), nil
			case "computer_screenshot":
				f, e := computer.Screenshot(ctx)
				if e != nil {
					return finish(nil, e, nil), nil
				}
				if f == nil {
					return finish(nil, protocol.Fail("INVALID_FRAME", "backend returned no screenshot"), nil), nil
				}
				if e = f.Validate(); e != nil {
					return finish(nil, e, nil), nil
				}
				metadata := *f
				metadata.PNG = nil
				return finish(metadata, nil, f.PNG), nil
			case "computer_run_command":
				if a.TimeoutMS == 0 {
					a.TimeoutMS = 20000
				}
				if a.OutputLimit == 0 {
					a.OutputLimit = 65536
				}
				if a.Cwd == "" {
					a.Cwd = "/home/agent"
				}
				if e := a.ExecRequest.Validate(); e != nil {
					return finish(nil, e, nil), nil
				}
				v, e := computer.Exec(ctx, a.ExecRequest)
				if e == nil && v == nil {
					e = protocol.Fail("INVALID_MESSAGE", "backend returned no command result")
				}
				if e == nil && (v.TimedOut || v.Canceled || !v.CleanupConfirmed || v.ExitCode != 0 || v.Error != "") {
					reason := "COMMAND_FAILED"
					if v.TimedOut {
						reason = "COMMAND_TIMEOUT"
					}
					if v.Canceled {
						reason = "CANCELED"
					}
					if !v.CleanupConfirmed {
						reason = "CLEANUP_UNCONFIRMED"
					}
					// Keep bounded guest stdout/stderr and exit status available even on command failure.
					r := finish(nil, protocol.Fail(reason, "guest command did not complete successfully; inspect data"), nil)
					output := Result{OK: false, Code: reason, Message: "guest command did not complete successfully; inspect data", Data: v}
					b, _ := json.Marshal(output)
					r.StructuredContent = output
					r.Content = []mcp.Content{&mcp.TextContent{Text: string(b)}}
					return r, nil
				}
				return finish(v, e, nil), nil
			case "computer_open":
				e := computer.Open(ctx, a.App)
				return finish(map[string]string{"app": a.App, "note": "verify the visible result with a screenshot"}, e, nil), nil
			default:
				kind := strings.TrimPrefix(name, "computer_")
				if kind == "type" {
					kind = "text"
				}
				a.Kind = kind
				if e := a.Input.Validate(); e != nil {
					return finish(nil, e, nil), nil
				}
				v, e := computer.Input(ctx, a.Input)
				return finish(v, e, nil), nil
			}
		})
	}
	str := func(description string) *jsonschema.Schema {
		return &jsonschema.Schema{Type: "string", Description: description}
	}
	integer := func(min, max float64) *jsonschema.Schema {
		return &jsonschema.Schema{Type: "integer", Minimum: &min, Maximum: &max}
	}
	enum := func(values ...string) *jsonschema.Schema {
		a := []any{}
		for _, v := range values {
			a = append(a, v)
		}
		return &jsonschema.Schema{Type: "string", Enum: a}
	}
	add("computer_status", "Check real local VM status, guest handshake, desktop readiness and capabilities. Does not start the VM.", nil, nil)
	add("computer_screenshot", "Capture a fresh guest PNG and metadata. Use original image pixels (top-left origin), never preview-scaled coordinates. Frame IDs expire after 10 seconds or a session restart.", nil, nil)
	for _, kind := range []string{"move", "click", "drag", "scroll", "type", "key"} {
		p := map[string]*jsonschema.Schema{"frameId": str("ID of a screenshot you have observed within 10 seconds")}
		required := []string{"frameId"}
		if kind == "move" || kind == "click" || kind == "drag" || kind == "scroll" {
			p["x"] = integer(0, 1279)
			p["y"] = integer(0, 799)
			required = append(required, "x", "y")
		}
		if kind == "click" || kind == "drag" {
			p["button"] = enum("left", "middle", "right")
		}
		if kind == "click" {
			p["clicks"] = integer(1, 2)
		}
		if kind == "drag" {
			p["toX"] = integer(0, 1279)
			p["toY"] = integer(0, 799)
			required = append(required, "toX", "toY")
		}
		if kind == "scroll" {
			p["dx"] = integer(-100, 100)
			p["dy"] = integer(-100, 100)
			required = append(required, "dx", "dy")
		}
		if kind == "type" {
			p["text"] = str("UTF-8 text, at most 64 KiB; guest clipboard paste, verify insertion")
			p["pasteMode"] = enum("standard", "terminal")
			required = append(required, "text")
		}
		if kind == "key" {
			p["key"] = str("ENTER, TAB, ESC, arrows, F1..F12, A..Z, 0..9; unique CTRL/ALT/SHIFT/SUPER modifiers joined with +; use type for Unicode")
			required = append(required, "key")
		}
		add("computer_"+kind, "Send "+kind+" only to the Linux guest. Requires an observed, current frameId. Verify the result with a fresh screenshot; dispatch is not proof of application success.", p, required)
	}
	add("computer_open", "Open a supported Linux guest app; verify with screenshot.", map[string]*jsonschema.Schema{"app": enum("browser", "editor", "terminal")}, []string{"app"})
	add("computer_run_command", "Run arbitrary argv ONLY inside Linux, never on the Mac. No implicit shell; explicitly use /bin/sh -c for shell syntax. stdout/stderr are base64 bytes. May modify all agent-owned guest data. Deadline/output limits kill command descendants; this is not an adversarial sandbox.", map[string]*jsonschema.Schema{
		"argv": {Type: "array", Items: str("guest argv element"), MinItems: intPtr(1), MaxItems: intPtr(128)}, "cwd": str("absolute guest working directory, default /home/agent"), "env": {Type: "object", AdditionalProperties: str("guest environment value")}, "timeoutMs": integer(1, 60000), "outputLimit": integer(1, 262144),
	}, []string{"argv"})
	return s
}
func intPtr(n int) *int { return &n }
func failureCode(err error) (string, string) {
	var f *protocol.Failure
	if errors.As(err, &f) {
		return f.Code, f.Message
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "TIMEOUT", "operation deadline exceeded; an action may have completed, observe before retrying"
	}
	if errors.Is(err, context.Canceled) {
		return "CANCELED", "request canceled; observe before retrying"
	}
	if strings.Contains(err.Error(), "MACHINE_BUSY") {
		return "MACHINE_BUSY", "machine is controlled by another operation"
	}
	// Unknown host errors may contain credentials, paths or commands. Do not echo.
	return "BACKEND_ERROR", "local backend failed; inspect private lifecycle diagnostics"
}
