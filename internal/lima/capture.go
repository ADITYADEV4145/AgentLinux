// SPDX-License-Identifier: Apache-2.0
package lima

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/process"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

// CaptureTransport reuses one authenticated, observation-only SSH channel within
// a viewer. Mutations always retain their existing no-replay path.
type CaptureTransport struct {
	ctx    context.Context
	cancel context.CancelFunc
	gate   chan struct{}
	mu     sync.Mutex
	active io.ReadWriteCloser
	link   *captureLink // serialized by gate, not touched by Close
	stats  CaptureStats
	dial   func(context.Context, lifecycle.State, lifecycle.Observation) (io.ReadWriteCloser, error)
}
type CaptureStats struct {
	Opens    uint64 `json:"connectionsOpened"`
	Reused   uint64 `json:"connectionsReused"`
	Failures uint64 `json:"failures"`
}
type captureLink struct {
	conn    io.ReadWriteCloser
	reader  *bufio.Reader
	guest   *protocol.Info
	key     string
	used    int
	touched time.Time
}

func NewCaptureTransport(parent context.Context, b *Backend) *CaptureTransport {
	ctx, cancel := context.WithCancel(parent)
	c := &CaptureTransport{ctx: ctx, cancel: cancel, gate: make(chan struct{}, 1)}
	c.dial = func(ctx context.Context, s lifecycle.State, o lifecycle.Observation) (io.ReadWriteCloser, error) {
		cmd := process.Prepare(ctx, "ssh", b.sshArgs(s, o)...)
		cmd.Env = b.env()
		in, e := cmd.StdinPipe()
		if e != nil {
			return nil, e
		}
		out, e := cmd.StdoutPipe()
		if e != nil {
			in.Close()
			return nil, e
		}
		cmd.Stderr = &process.Buffer{Limit: 4096}
		if e = cmd.Start(); e != nil {
			in.Close()
			out.Close()
			return nil, e
		}
		return &captureProcess{Cmd: cmd, in: in, out: out}, nil
	}
	context.AfterFunc(ctx, c.closeActive)
	return c
}
func (c *CaptureTransport) Close() { c.cancel(); c.closeActive() }
func (c *CaptureTransport) closeActive() {
	c.mu.Lock()
	active := c.active
	c.active = nil
	c.mu.Unlock()
	if active != nil {
		_ = active.Close()
	}
}
func (c *CaptureTransport) Stats() CaptureStats { c.mu.Lock(); defer c.mu.Unlock(); return c.stats }
func (c *CaptureTransport) drop() {
	if c.link != nil {
		_ = c.link.conn.Close()
		c.link = nil
	}
	c.closeActive()
}
func (c *CaptureTransport) Call(ctx context.Context, s lifecycle.State, o lifecycle.Observation, q protocol.Request) (result *protocol.Info, err error) {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	if q.Method != "screenshot" {
		return nil, protocol.Fail("UNSUPPORTED_OPERATION", "persistent capture channel accepts screenshots only")
	}
	select {
	case c.gate <- struct{}{}:
		defer func() { <-c.gate }()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
	if c.ctx.Err() != nil {
		return nil, c.ctx.Err()
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if !protocol.ValidID(s.MachineID) || o.Port < 1 || o.Port > 65535 {
		return nil, protocol.Fail("DISCONNECTED", "invalid capture endpoint identity")
	}
	key := s.MachineID + ":" + strconv.Itoa(o.Port)
	if c.link != nil && (c.link.key != key || c.link.used >= 48 || time.Since(c.link.touched) > 45*time.Second) {
		c.drop()
	}
	defer func() {
		if err != nil {
			c.mu.Lock()
			c.stats.Failures++
			c.mu.Unlock()
			c.drop()
		}
	}()
	if c.link == nil {
		conn, e := c.dial(c.ctx, s, o)
		if e != nil {
			return nil, protocol.Fail("DISCONNECTED", "cannot open pinned guest capture channel")
		}
		c.mu.Lock()
		c.active = conn
		c.stats.Opens++
		c.mu.Unlock()
		c.link = &captureLink{conn: conn, reader: protocol.Reader(conn), key: key}
	} else {
		c.mu.Lock()
		c.stats.Reused++
		c.mu.Unlock()
	}
	link := c.link
	// A canceled operation destroys the entire read stream. It can never leave a
	// late response to be mistaken for the next request's result.
	stop := context.AfterFunc(ctx, func() { _ = link.conn.Close() })
	defer stop()
	if c.ctx.Err() != nil {
		_ = link.conn.Close()
		return nil, c.ctx.Err()
	}
	call := func(request protocol.Request) (*protocol.Info, error) {
		if e := protocol.WriteRequest(link.conn, request); e != nil {
			return nil, protocol.Fail("DISCONNECTED", "capture request write failed; reconnect on next observation")
		}
		var response protocol.Response
		if e := protocol.Read(link.reader, &response); e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, protocol.Fail("DISCONNECTED", "guest capture disconnected; reconnect on next observation or use view --capture-transport oneshot for diagnostics")
		}
		if e := checkResponse(request, response); e != nil {
			return nil, e
		}
		return response.Result, nil
	}
	if link.guest == nil {
		hello := protocol.Request{Version: protocol.Version, ID: protocol.ID(), MachineID: s.MachineID, Method: "hello"}
		g, e := call(hello)
		if e != nil {
			return nil, e
		}
		if !protocol.CompatibleGuestVersion(g.GuestVersion) || !protocol.ValidID(g.SessionID) || g.BootID == "" || g.OS != "linux" || g.Arch != "arm64" {
			return nil, protocol.Fail("VERSION_MISMATCH", "capture guest identity/version is incompatible")
		}
		probe := protocol.Request{Version: protocol.Version, ID: protocol.ID(), MachineID: s.MachineID, SessionID: g.SessionID, Method: "probe", Nonce: protocol.ID()}
		observation, e := call(probe)
		if e != nil {
			return nil, e
		}
		if observation.SessionID != g.SessionID || observation.BootID != g.BootID || observation.NonceSHA256 != sha256Probe(probe.Nonce) {
			return nil, protocol.Fail("STALE_SESSION", "capture session or nonce probe mismatch")
		}
		link.guest = g
	}
	if q.SessionID != "" && q.SessionID != link.guest.SessionID {
		return nil, protocol.Fail("STALE_SESSION", "capture request belongs to an earlier guest session")
	}
	q.Version = protocol.Version
	q.ID = protocol.ID()
	q.MachineID = s.MachineID
	q.SessionID = link.guest.SessionID
	result, err = call(q)
	if err != nil {
		return nil, err
	}
	if result.SessionID != q.SessionID || result.BootID != link.guest.BootID || result.Frame == nil || result.Frame.SessionID != q.SessionID {
		return nil, protocol.Fail("STALE_FRAME", "capture response session mismatch or missing frame")
	}
	if err = result.Frame.Validate(); err != nil {
		return nil, err
	}
	link.used++
	link.touched = time.Now()
	return result, nil
}

type captureProcess struct {
	*exec.Cmd
	in   io.WriteCloser
	out  io.ReadCloser
	once sync.Once
}

func (p *captureProcess) Read(b []byte) (int, error)  { return p.out.Read(b) }
func (p *captureProcess) Write(b []byte) (int, error) { return p.in.Write(b) }
func (p *captureProcess) Close() error {
	p.once.Do(func() {
		_ = p.in.Close()
		_ = p.out.Close()
		if p.Process != nil {
			_ = p.Cancel()
		}
		_ = p.Wait()
	})
	return nil
}
