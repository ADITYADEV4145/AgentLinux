// SPDX-License-Identifier: Apache-2.0

package guest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/ADITYADEV4145/AgentLinux/internal/desktop"
	"github.com/ADITYADEV4145/AgentLinux/internal/filetransfer"
	"github.com/ADITYADEV4145/AgentLinux/internal/guestexec"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

const Socket = "/run/agent-linux/control.sock"

type Service struct {
	MachineID, SessionID, BootID string
	idleTimeout                  time.Duration
	Desktop                      *desktop.Desktop
	Files                        *filetransfer.Guest
}

func New(machine string) (*Service, error) {
	if !protocol.ValidID(machine) {
		return nil, errors.New("invalid machine ID")
	}
	b, e := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if e != nil {
		return nil, e
	}
	return &Service{MachineID: machine, SessionID: protocol.ID(), BootID: strings.TrimSpace(string(b)), Desktop: desktop.New(), Files: &filetransfer.Guest{Home: "/home/agent"}}, nil
}
func (s *Service) info() *protocol.Info {
	u, _ := os.ReadFile("/proc/uptime")
	return &protocol.Info{GuestVersion: protocol.GuestVersion, Protocol: protocol.Version, MachineID: s.MachineID, SessionID: s.SessionID, BootID: s.BootID, OS: runtime.GOOS, Arch: runtime.GOARCH, Uptime: strings.TrimSpace(string(u)), Operations: []string{"health", "probe", "screenshot", "input", "exec", "open", "files"}, Capabilities: protocol.SupportedCapabilities()}
}
func (s *Service) Handle(c net.Conn) {
	defer c.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type event struct {
		q   protocol.Request
		err error
	}
	events := make(chan event, 1)
	go func() {
		defer close(events)
		r := protocol.RequestReader(c)
		for count := 0; count < 64; count++ {
			_ = c.SetReadDeadline(time.Now().Add(s.timeout()))
			var q protocol.Request
			e := protocol.Read(r, &q)
			if e != nil {
				cancel()
				if !errors.Is(e, io.EOF) {
					select {
					case events <- event{err: e}:
					case <-time.After(time.Second):
					}
				}
				return
			}
			select {
			case events <- event{q: q}:
			case <-ctx.Done():
				return
			}
		}
	}()
	hello := false
	for ev := range events {
		if ev.err != nil {
			_ = c.SetWriteDeadline(time.Now().Add(time.Second))
			_ = protocol.Write(c, protocol.Response{Version: protocol.Version, Error: &protocol.Failure{Code: "INVALID_MESSAGE", Message: "invalid or oversized request; connection closed"}})
			return
		}
		q := ev.q
		p := protocol.Response{Version: protocol.Version, ID: q.ID}
		fail := func(code, msg string) { p.Error = &protocol.Failure{Code: code, Message: msg} }
		switch {
		case q.Version != protocol.Version:
			fail("VERSION_MISMATCH", "supported guest protocol is "+protocol.Version)
		case !protocol.ValidID(q.ID) || q.MachineID != s.MachineID:
			fail("IDENTITY_MISMATCH", "request identity does not match this guest")
		case q.Method == "hello":
			if q.SessionID != "" || hello {
				fail("INVALID_ARGUMENT", "hello allowed once without session")
			} else {
				hello = true
				p.Result = s.info()
			}
		case !hello:
			fail("HANDSHAKE_REQUIRED", "send hello before operations")
		case q.SessionID != s.SessionID:
			fail("STALE_SESSION", "guest service restarted; reconnect and observe")
		case q.Method == "health":
			p.Result = s.info()
			if s.Desktop != nil {
				p.Result.Desktop = s.Desktop.Status(ctx)
			}
		case q.Method == "probe":
			if len(q.Nonce) < 1 || len(q.Nonce) > 128 {
				fail("INVALID_ARGUMENT", "probe nonce must be 1..128 bytes")
			} else {
				p.Result = s.info()
				hash := sha256.Sum256([]byte(q.Nonce))
				p.Result.NonceSHA256 = hex.EncodeToString(hash[:])
				if s.Desktop != nil {
					p.Result.Desktop = s.Desktop.Status(ctx)
				}
			}
		default:
			p.Result = s.info()
			var e error
			switch q.Method {
			case "files":
				if s.Files == nil || q.File == nil {
					e = protocol.Fail("UNSUPPORTED_OPERATION", "file service unavailable; create a new guest for file transfer")
				} else {
					p.Result.File, e = s.Files.Handle(*q.File)
				}
			case "exec":
				if q.Exec == nil {
					e = protocol.Fail("INVALID_ARGUMENT", "exec parameters required")
				} else {
					p.Result.Execution, e = guestexec.Run(ctx, *q.Exec)
				}
			case "screenshot":
				if s.Desktop == nil {
					e = protocol.Fail("DESKTOP_UNAVAILABLE", "desktop adapter unavailable")
				} else {
					p.Result.Frame, e = s.Desktop.Screenshot(ctx, s.SessionID)
				}
			case "input":
				if s.Desktop == nil || q.Input == nil {
					e = protocol.Fail("INVALID_ARGUMENT", "input parameters and desktop required")
				} else {
					p.Result.InputResult, e = s.Desktop.Input(ctx, *q.Input)
				}
			case "open":
				if s.Desktop == nil {
					e = protocol.Fail("DESKTOP_UNAVAILABLE", "desktop unavailable")
				} else {
					e = s.Desktop.Open(ctx, q.App)
				}
			default:
				e = protocol.Fail("UNSUPPORTED_OPERATION", "unknown guest operation")
			}
			if e != nil {
				p.Result = nil
				var typed *protocol.Failure
				if errors.As(e, &typed) {
					p.Error = typed
				} else {
					fail("GUEST_OPERATION_FAILED", "guest operation failed; inspect service diagnostics")
				}
			}
		}
		_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if protocol.Write(c, p) != nil {
			return
		}
		if p.Error != nil && (p.Error.Code == "VERSION_MISMATCH" || p.Error.Code == "IDENTITY_MISMATCH") {
			return
		}
	}
}
func (s *Service) Serve(ctx context.Context, path string) error {
	// RuntimeDirectory is owned by systemd; never unlink an existing socket (another service may own it).
	l, e := net.Listen("unix", path)
	if e != nil {
		return e
	}
	defer l.Close()
	if e = os.Chmod(path, 0600); e != nil {
		return e
	}
	go func() { <-ctx.Done(); l.Close() }()
	slots := make(chan struct{}, 4)
	for {
		c, e := l.Accept()
		if e != nil {
			if ctx.Err() != nil {
				return nil
			}
			return e
		}
		select {
		case slots <- struct{}{}:
			go func() {
				defer func() { <-slots }()
				stop := context.AfterFunc(ctx, func() { c.Close() })
				defer stop()
				s.Handle(c)
			}()
		default:
			c.Close()
		}
	}
}
func Bridge(ctx context.Context, in io.Reader, out io.Writer, path string) error {
	c, e := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", path)
	if e != nil {
		return e
	}
	defer c.Close()
	done := make(chan error, 2)
	go func() {
		_, e := io.Copy(c, in)
		if u, ok := c.(*net.UnixConn); ok {
			u.CloseWrite()
		}
		done <- e
	}()
	go func() { _, e := io.Copy(out, c); done <- e }()
	select {
	case e = <-done:
		return e
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) timeout() time.Duration {
	if s.idleTimeout > 0 {
		return s.idleTimeout
	}
	return 75 * time.Second
}
