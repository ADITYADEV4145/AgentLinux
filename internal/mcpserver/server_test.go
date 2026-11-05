// SPDX-License-Identifier: Apache-2.0
package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"sync"
	"testing"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const frameID = "0123456789abcdef0123456789abcdef"

type fake struct {
	mu      sync.Mutex
	inputs  []protocol.Input
	fail    error
	wait    chan struct{}
	entered chan struct{}
}

func (f *fake) Status(ctx context.Context) (lifecycle.State, error) {
	if f.entered != nil {
		select {
		case f.entered <- struct{}{}:
		default:
		}
	}
	if f.wait != nil {
		select {
		case <-f.wait:
		case <-ctx.Done():
			return lifecycle.State{}, ctx.Err()
		}
	}
	return lifecycle.State{State: "ready"}, f.fail
}
func (f *fake) Screenshot(context.Context) (*protocol.Frame, error) {
	if f.fail != nil {
		return nil, f.fail
	}
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1280, 800)))
	return &protocol.Frame{ID: frameID, SessionID: frameID, Generation: "generation", Sequence: 1, CapturedAt: time.Now(), Width: 1280, Height: 800, Scale: 1, Format: "png", PNG: b.Bytes()}, nil
}
func (f *fake) Input(_ context.Context, i protocol.Input) (*protocol.InputResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inputs = append(f.inputs, i)
	return &protocol.InputResult{Dispatched: true, FrameID: i.FrameID}, f.fail
}
func (f *fake) Exec(context.Context, protocol.ExecRequest) (*protocol.Execution, error) {
	return &protocol.Execution{ExitCode: 0, CleanupConfirmed: true, Stdout: []byte("hello")}, f.fail
}
func (f *fake) Open(context.Context, string) error { return f.fail }
func connect(t *testing.T, f *fake, o Options) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	a, b := mcp.NewInMemoryTransports()
	s := New(f, o)
	ss, e := s.Connect(ctx, a, nil)
	if e != nil {
		t.Fatal(e)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "official-sdk-test", Version: "1"}, nil)
	cs, e := client.Connect(ctx, b, nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { cs.Close(); ss.Close(); cancel() })
	return cs
}
func call(t *testing.T, c *mcp.ClientSession, name string, args any) (*mcp.CallToolResult, Result) {
	t.Helper()
	r, e := c.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if e != nil {
		t.Fatal(e)
	}
	var v Result
	b, e := json.Marshal(r.StructuredContent)
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	return r, v
}
func TestOfficialClientToolsAndImage(t *testing.T) {
	f := &fake{}
	c := connect(t, f, Options{})
	list, e := c.ListTools(context.Background(), nil)
	if e != nil || len(list.Tools) != 10 {
		t.Fatalf("tools: %v %v", list, e)
	}
	_, v := call(t, c, "computer_status", map[string]any{})
	if !v.OK {
		t.Fatal(v)
	}
	r, v := call(t, c, "computer_screenshot", nil)
	if !v.OK || len(r.Content) != 2 {
		t.Fatal(r)
	}
	im, ok := r.Content[1].(*mcp.ImageContent)
	if !ok || im.MIMEType != "image/png" {
		t.Fatal(r)
	}
	cfg, e := png.DecodeConfig(bytes.NewReader(im.Data))
	if e != nil || cfg.Width != 1280 || cfg.Height != 800 {
		t.Fatal(cfg, e)
	}
	data := v.Data.(map[string]any)
	if data["id"] != frameID || data["png"] != nil || data["scale"] != float64(1) {
		t.Fatal(data)
	}
	_, v = call(t, c, "computer_type", map[string]any{"frameId": frameID, "text": "café — 日本語 👋"})
	if !v.OK {
		t.Fatal(v)
	}
	if len(f.inputs) != 1 || f.inputs[0].Text != "café — 日本語 👋" || f.inputs[0].Kind != "text" {
		t.Fatal(f.inputs)
	}
	_, v = call(t, c, "computer_run_command", map[string]any{"argv": []string{"/bin/echo", "hello"}})
	if !v.OK {
		t.Fatal(v)
	}
}
func TestSchemaAndBackendErrors(t *testing.T) {
	f := &fake{}
	c := connect(t, f, Options{})
	for _, q := range []struct {
		name string
		args any
	}{
		{"computer_click", map[string]any{"x": 2, "y": 2}},
		{"computer_click", map[string]any{"frameId": frameID, "x": 1280, "y": 0}},
		{"computer_click", map[string]any{"frameId": frameID, "x": 2, "y": 2, "button": "host"}},
		{"computer_key", map[string]any{"frameId": frameID, "key": "CTRL+CTRL+A"}},
		{"computer_type", map[string]any{"frameId": frameID, "text": "secret", "unknown": true}},
		{"computer_run_command", map[string]any{"argv": []string{"/bin/true"}, "timeoutMs": 60001}},
		{"computer_run_command", map[string]any{"argv": []string{"/bin/true"}, "cwd": "relative"}},
	} {
		r, v := call(t, c, q.name, q.args)
		if !r.IsError || v.Code != "INVALID_ARGUMENT" {
			t.Fatalf("%s: %+v", q.name, v)
		}
	}
	if len(f.inputs) != 0 {
		t.Fatal("invalid requests reached guest")
	}
	for _, code := range []string{"DESKTOP_UNAVAILABLE", "DISCONNECTED", "UNSUPPORTED_OPERATION", "STALE_FRAME", "OUTCOME_UNKNOWN"} {
		f.fail = protocol.Fail(code, "helpful context")
		r, v := call(t, c, "computer_screenshot", nil)
		if !r.IsError || v.Code != code || v.Message != "helpful context" {
			t.Fatal(v)
		}
	}
}
func TestConcurrentAndDeadline(t *testing.T) {
	f := &fake{wait: make(chan struct{}), entered: make(chan struct{}, 1)}
	c := connect(t, f, Options{Timeout: 150 * time.Millisecond})
	done := make(chan *mcp.CallToolResult, 1)
	go func() {
		r, _ := c.CallTool(context.Background(), &mcp.CallToolParams{Name: "computer_status"})
		done <- r
	}()
	<-f.entered
	r, v := call(t, c, "computer_screenshot", nil)
	if !r.IsError || v.Code != "MACHINE_BUSY" {
		t.Fatal(v)
	}
	select {
	case r = <-done:
		if r == nil || !r.IsError {
			t.Fatal(r)
		}
		b, _ := json.Marshal(r.StructuredContent)
		if !bytes.Contains(b, []byte(`"TIMEOUT"`)) {
			t.Fatal(string(b))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("deadline not enforced")
	}
	close(f.wait)
	_, v = call(t, c, "computer_status", nil)
	if !v.OK {
		t.Fatal("gate not released", v)
	}
}

type badFrame struct {
	*fake
	frame *protocol.Frame
}

func (f badFrame) Screenshot(context.Context) (*protocol.Frame, error) { return f.frame, nil }

type badCommand struct {
	*fake
	execution *protocol.Execution
}

func (f badCommand) Exec(context.Context, protocol.ExecRequest) (*protocol.Execution, error) {
	return f.execution, nil
}

func TestFrameLimitsAndCommandFailure(t *testing.T) {
	valid, _ := (&fake{}).Screenshot(context.Background())
	valid.PNG = make([]byte, protocol.MaxPNG+1)
	for _, frame := range []*protocol.Frame{nil, {}, valid} {
		s := New(badFrame{&fake{}, frame}, Options{})
		ctx := context.Background()
		a, b := mcp.NewInMemoryTransports()
		ss, e := s.Connect(ctx, a, nil)
		if e != nil {
			t.Fatal(e)
		}
		c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
		cs, e := c.Connect(ctx, b, nil)
		if e != nil {
			t.Fatal(e)
		}
		r, v := call(t, cs, "computer_screenshot", nil)
		if !r.IsError || v.OK || len(r.Content) != 1 {
			t.Fatal(v)
		}
		cs.Close()
		ss.Close()
	}
	s := New(badCommand{&fake{}, &protocol.Execution{ExitCode: 7, Stderr: []byte("bounded failure"), CleanupConfirmed: true}}, Options{})
	a, b := mcp.NewInMemoryTransports()
	ss, e := s.Connect(context.Background(), a, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer ss.Close()
	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, e := c.Connect(context.Background(), b, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer cs.Close()
	r, v := call(t, cs, "computer_run_command", map[string]any{"argv": []string{"/bin/false"}})
	if !r.IsError || v.Code != "COMMAND_FAILED" || v.Data.(map[string]any)["exitCode"] != float64(7) {
		t.Fatal(v)
	}
}
func TestCancellationAndPrivateDebug(t *testing.T) {
	var log bytes.Buffer
	f := &fake{wait: make(chan struct{}), entered: make(chan struct{}, 1)}
	c := connect(t, f, Options{Debug: &log})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _, _ = c.CallTool(ctx, &mcp.CallToolParams{Name: "computer_status"}) }()
	<-f.entered
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("client cancellation blocked")
	}
	// A successful call proves the server released its gate after receiving cancellation.
	close(f.wait)
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, v := call(t, c, "computer_status", nil)
		if v.OK {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cancellation left gate locked")
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, _ = call(t, c, "computer_type", map[string]any{"frameId": frameID, "text": "PRIVATE_SCREEN_PASSWORD"})
	if bytes.Contains(log.Bytes(), []byte("PRIVATE_SCREEN_PASSWORD")) || bytes.Contains(log.Bytes(), []byte(frameID)) {
		t.Fatal("debug leaked input")
	}
	if !bytes.Contains(log.Bytes(), []byte("tool=computer_type code=OK")) {
		t.Fatal(log.String())
	}
}

func TestSupportedProtocolRevisions(t *testing.T) {
	for _, version := range []string{"2025-03-26", "2025-06-18", "2025-11-25", "2026-07-28"} {
		t.Run(version, func(t *testing.T) {
			a, b := mcp.NewInMemoryTransports()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			ss, e := New(&fake{}, Options{}).Connect(ctx, a, nil)
			if e != nil {
				t.Fatal(e)
			}
			defer ss.Close()
			client := mcp.NewClient(&mcp.Implementation{Name: "compatibility-test", Version: "1"}, nil)
			cs, e := client.Connect(ctx, b, &mcp.ClientSessionOptions{ProtocolVersion: version})
			if e != nil {
				t.Fatal(e)
			}
			defer cs.Close()
			if cs.InitializeResult().ProtocolVersion != version {
				t.Fatal(cs.InitializeResult())
			}
			_, v := call(t, cs, "computer_status", nil)
			if !v.OK {
				t.Fatal(v)
			}
		})
	}
}
