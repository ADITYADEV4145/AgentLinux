// SPDX-License-Identifier: Apache-2.0

package guest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
}

func New(machine string) (*Service, error) {
	if !protocol.ValidID(machine) {
		return nil, errors.New("invalid machine ID")
	}
	b, e := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if e != nil {
		return nil, e
	}
	return &Service{MachineID: machine, SessionID: protocol.ID(), BootID: strings.TrimSpace(string(b))}, nil
}
func (s *Service) info() *protocol.Info {
	u, _ := os.ReadFile("/proc/uptime")
	return &protocol.Info{GuestVersion: protocol.GuestVersion, Protocol: protocol.Version, MachineID: s.MachineID, SessionID: s.SessionID, BootID: s.BootID, OS: runtime.GOOS, Arch: runtime.GOARCH, Uptime: strings.TrimSpace(string(u)), Operations: []string{"health", "probe"}}
}
func (s *Service) Handle(c net.Conn) {
	defer c.Close()
	r := protocol.Reader(c)
	hello := false
	for count := 0; count < 64; count++ {
		_ = c.SetDeadline(time.Now().Add(s.timeout()))
		var q protocol.Request
		if e := protocol.Read(r, &q); e != nil {
			if !errors.Is(e, io.EOF) {
				_ = protocol.Write(c, protocol.Response{Version: protocol.Version, Error: &protocol.Failure{Code: "INVALID_MESSAGE", Message: "invalid or oversized request; connection closed"}})
			}
			return
		}
		p := protocol.Response{Version: protocol.Version, ID: q.ID}
		fail := func(code, msg string) { p.Error = &protocol.Failure{Code: code, Message: msg} }
		switch {
		case q.Version != protocol.Version:
			fail("VERSION_MISMATCH", "supported guest protocol is "+protocol.Version)
		case !protocol.ValidID(q.ID) || q.MachineID != s.MachineID:
			fail("IDENTITY_MISMATCH", "request identity does not match this guest")
		case q.Method == "hello":
			if q.SessionID != "" || hello {
				fail("INVALID_ARGUMENT", "hello is allowed once without a session")
			} else {
				hello = true
				p.Result = s.info()
			}
		case !hello:
			fail("HANDSHAKE_REQUIRED", "send hello before operations")
		case q.SessionID != s.SessionID:
			fail("STALE_SESSION", "guest service restarted; reconnect and handshake")
		case q.Method == "health":
			p.Result = s.info()
		case q.Method == "probe":
			if len(q.Nonce) < 1 || len(q.Nonce) > 128 {
				fail("INVALID_ARGUMENT", "probe nonce must be 1..128 bytes")
			} else {
				p.Result = s.info()
				hash := sha256.Sum256([]byte(q.Nonce))
				p.Result.NonceSHA256 = hex.EncodeToString(hash[:])
			}
		default:
			fail("UNSUPPORTED_OPERATION", "only hello, health and probe are supported")
		}
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
			go func() { defer func() { <-slots }(); s.Handle(c) }()
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
	return 15 * time.Second
}
