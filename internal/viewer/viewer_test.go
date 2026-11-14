// SPDX-License-Identifier: Apache-2.0
package viewer

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/machine"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

const identity = "0123456789abcdef0123456789abcdef"

type computer struct {
	machine.Machine
	mu      sync.Mutex
	calls   int
	err     error
	frame   *protocol.Frame
	wait    chan struct{}
	entered chan struct{}
}

func (c *computer) Screenshot(ctx context.Context) (*protocol.Frame, error) {
	c.mu.Lock()
	c.calls++
	e, f := c.err, c.frame
	c.mu.Unlock()
	if c.entered != nil {
		select {
		case c.entered <- struct{}{}:
		default:
		}
	}
	if c.wait != nil {
		select {
		case <-c.wait:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return f, e
}
func validFrame(t *testing.T) *protocol.Frame {
	t.Helper()
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1280, 800)))
	return &protocol.Frame{ID: identity, SessionID: identity, Generation: "generation", Sequence: 1, CapturedAt: time.Now(), Width: 1280, Height: 800, Scale: 1, Format: "png", PNG: b.Bytes()}
}
func expire(h *Hub) { h.mu.Lock(); h.next = time.Time{}; h.mu.Unlock() }
func TestCapabilityAndPlacement(t *testing.T) {
	if !Candidate(true, "xterm-kitty", "", "", "", "") || Candidate(false, "xterm-kitty", "", "", "", "") || Candidate(true, "xterm-256color", "", "", "", "") || Candidate(true, "xterm-kitty", "", "", "tmux", "") {
		t.Fatal("capability detection")
	}
	if Supported([]byte("OK")) || !Supported([]byte("\x1b_Gi=31;OK\x1b\\")) {
		t.Fatal("query not confirmed")
	}
	for _, s := range []Size{{120, 40, 1200, 800}, {80, 30, 1440, 1080}, {200, 20, 2000, 400}, {30, 80, 300, 1600}} {
		p := Fit(s)
		if p.Columns < 1 || p.Columns > s.Columns || p.Rows > s.Rows-2 || p.Row < 2 || p.Row+p.Rows > s.Rows {
			t.Fatal(s, p)
		}
	}
	if Fit(Size{80, 24, 0, 0}).Columns != 0 || Fit(Size{10, 4, 100, 100}).Columns != 0 {
		t.Fatal("invented pixel size")
	}
}
func TestPNGChunksAndExitEncoding(t *testing.T) {
	data := bytes.Repeat([]byte("0123456789\x1b]52;clipboard"), 1000)
	var output bytes.Buffer
	if e := Render(context.Background(), func(_ context.Context, b []byte) error { _, e := output.Write(b); return e }, Placement{Column: 2, Row: 3, Columns: 80, Rows: 30}, data); e != nil {
		t.Fatal(e)
	}
	var encoded string
	for _, part := range strings.Split(output.String(), "\x1b_G")[1:] {
		header, payload, ok := strings.Cut(part, ";")
		if !ok {
			continue
		}
		payload, _, _ = strings.Cut(payload, "\x1b\\")
		if strings.Contains(header, "a=d") {
			continue
		}
		if len(payload) > 4096 {
			t.Fatal("oversized Kitty chunk")
		}
		encoded += payload
	}
	decoded, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil || !bytes.Equal(data, decoded) {
		t.Fatal("PNG was not sent as bounded direct data", e)
	}
	if strings.Contains(output.String(), "]52;") {
		t.Fatal("guest escape interpreted")
	}
	var keys ExitKeys
	if keys.Feed([]byte("\x1b_Gi=31;query reply\x1b\\")) || keys.Feed([]byte("\x1b[1;2q")) {
		t.Fatal("terminal reply mistaken for exit")
	}
	if !keys.Feed([]byte("q")) {
		t.Fatal("exit key ignored")
	}
}
func TestHubDedupInvalidAndBoundedCapture(t *testing.T) {
	c := &computer{frame: validFrame(t)}
	h := NewHub(c, time.Second)
	s, e := h.Next(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = h.Next(context.Background()); e != nil || c.calls != 1 {
		t.Fatal("cache ignored")
	}
	for i := 0; i < 4; i++ {
		expire(h)
		if _, e = h.Next(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
	m := h.Metrics()
	if m.Changes != 1 || m.Frames != 5 || s.Digest == "" {
		t.Fatal(m)
	}
	c.frame = &protocol.Frame{}
	expire(h)
	if _, e = h.Next(context.Background()); e == nil {
		t.Fatal("invalid frame displayed")
	}
	c.frame = validFrame(t)
	c.frame.PNG = make([]byte, protocol.MaxPNG+1)
	expire(h)
	if _, e = h.Next(context.Background()); Code(e) != "FRAME_TOO_LARGE" {
		t.Fatal(e)
	}
	c = &computer{frame: validFrame(t), wait: make(chan struct{}), entered: make(chan struct{}, 1)}
	h = NewHub(c, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _, _ = h.Next(ctx) }()
	<-c.entered
	if _, e = h.Next(context.Background()); Code(e) != "MACHINE_BUSY" {
		t.Fatal("capture queue grew", e)
	}
	cancel()
	wait, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if e = h.WaitIdle(wait); e != nil {
		t.Fatal("capture cleanup did not drain", e)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("capture cancellation not propagated")
	}
}
func TestWebAuthorizationAndConditionalFrames(t *testing.T) {
	c := &computer{frame: validFrame(t)}
	h := NewHub(c, time.Second)
	w := &Web{host: "127.0.0.1:47777", token: "fixture-capability-not-a-real-token", id: identity, hub: h, gate: make(chan struct{}, 4)}
	request := func(path, host, origin, auth, etag string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "http://"+host+path, nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Authorization", auth)
		req.Header.Set("If-None-Match", etag)
		out := httptest.NewRecorder()
		w.handle(out, req)
		return out
	}
	if r := request("/api/frame", w.host, "", "", ""); r.Code != 401 || c.calls != 0 {
		t.Fatal("unauthorized capture")
	}
	if r := request("/api/frame", "evil.test", "", "Bearer "+w.token, ""); r.Code != 403 {
		t.Fatal("DNS rebinding allowed")
	}
	if r := request("/api/frame", w.host, "https://evil.test", "Bearer "+w.token, ""); r.Code != 403 {
		t.Fatal("cross-origin request allowed")
	}
	r := request("/api/frame", w.host, "http://"+w.host, "Bearer "+w.token, "")
	if r.Code != 200 || r.Header().Get("Content-Type") != "image/png" || !bytes.Equal(r.Body.Bytes(), c.frame.PNG) {
		t.Fatal("real PNG missing")
	}
	cached := request("/api/frame", w.host, "", "Bearer "+w.token, r.Header().Get("ETag"))
	if cached.Code != 304 || cached.Body.Len() != 0 || c.calls != 1 {
		t.Fatal("unchanged image retransmitted")
	}
	if !strings.Contains(r.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") || r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("missing web isolation headers")
	}
	c.err = protocol.Fail("DISCONNECTED", "private details")
	expire(h)
	r = request("/api/frame", w.host, "", "Bearer "+w.token, "")
	if r.Code != 503 || strings.Contains(r.Body.String(), "private details") {
		t.Fatal("disconnect hidden or details leaked")
	}
	c.err = nil
	expire(h)
	r = request("/api/frame", w.host, "", "Bearer "+w.token, "")
	if r.Code != 200 {
		t.Fatal("no reconnect")
	}
}

type terminalDouble struct {
	mu        sync.Mutex
	data      bytes.Buffer
	closed    bool
	sizeCalls int
	fail      bool
}

func (t *terminalDouble) Write(_ context.Context, b []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.fail {
		return fmt.Errorf("terminal unplugged")
	}
	_, e := t.data.Write(b)
	return e
}
func (t *terminalDouble) Read(ctx context.Context) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (t *terminalDouble) Size() (Size, error) {
	t.sizeCalls++
	if t.sizeCalls > 2 {
		return Size{80, 30, 800, 600}, nil
	}
	return Size{120, 40, 1200, 800}, nil
}
func (t *terminalDouble) Close() error { t.closed = true; return nil }
func TestTerminalDisconnectRecoveryResizeCleanup(t *testing.T) {
	c := &computer{frame: validFrame(t), err: protocol.Fail("DISCONNECTED", "do not print private message")}
	h := NewHub(c, 250*time.Millisecond)
	terminal := &terminalDouble{}
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	go func() { time.Sleep(100 * time.Millisecond); c.mu.Lock(); c.err = nil; c.mu.Unlock() }()
	if e := RunTerminal(ctx, h, terminal, identity); e != nil {
		t.Fatal(e)
	}
	if !terminal.closed || !strings.Contains(terminal.data.String(), "DISCONNECTED") || !strings.Contains(terminal.data.String(), "a=p") || strings.Contains(terminal.data.String(), "private message") {
		t.Fatal("recovery/cleanup failed")
	}
	terminal = &terminalDouble{fail: true}
	if e := RunTerminal(context.Background(), NewHub(c, time.Second), terminal, identity); e == nil || !terminal.closed {
		t.Fatal("write failure did not restore terminal")
	}
}

func TestNarrowStatusRemainsActionable(t *testing.T) {
	line := statusLines(Size{Columns: 18, Rows: 5}, identity, "RESIZE: terminal too small", "0.1s")
	if !strings.Contains(line, "RESIZE") || !strings.Contains(line, "q exits") {
		t.Fatal("tiny terminal hid resize or exit guidance", line)
	}
	line = statusLines(Size{Columns: 30, Rows: 8}, identity, "NOT_READY", "--")
	if !strings.Contains(line, "NOT_READY") {
		t.Fatal("narrow terminal hid disconnected state", line)
	}
	line = statusLines(Size{Columns: 1, Rows: 1}, identity, "LIVE", "0s")
	if line != "\x1b[1;1H\x1b[2K" || statusLines(Size{}, identity, "LIVE", "0s") != "" {
		t.Fatal("degenerate terminal could wrap or address an invalid row", line)
	}
}

type revisionComputer struct {
	*computer
	revision string
}

func (c *revisionComputer) ObservationRevision() string { return c.revision }
func TestCrossProcessRevisionWakesStaticViewer(t *testing.T) {
	c := &revisionComputer{computer: &computer{frame: validFrame(t)}, revision: "before"}
	h := NewHub(c, time.Second)
	if _, e := h.Next(context.Background()); e != nil {
		t.Fatal(e)
	}
	h.mu.Lock()
	h.next = time.Now().Add(5 * time.Second)
	h.unchanged = 30
	h.mu.Unlock()
	if _, e := h.Next(context.Background()); e != nil {
		t.Fatal(e)
	}
	if c.calls != 1 {
		t.Fatal("unchanged revision bypassed backpressure")
	}
	c.revision = "after"
	if _, e := h.Next(context.Background()); e != nil {
		t.Fatal(e)
	}
	if c.calls != 2 || h.unchanged > 1 {
		t.Fatal("separate process mutation did not wake capture")
	}
}
