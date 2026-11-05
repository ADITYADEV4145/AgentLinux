// SPDX-License-Identifier: Apache-2.0

package guest

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"net"
	"testing"
	"time"
)

func TestSessionAndProbe(t *testing.T) {
	s := &Service{MachineID: protocol.ID(), SessionID: protocol.ID(), BootID: "fixture-boot"}
	client, server := net.Pipe()
	defer client.Close()
	go s.Handle(server)
	_ = client.SetDeadline(time.Now().Add(time.Second))
	r := protocol.Reader(client)
	call := func(method, session, version string) protocol.Response {
		t.Helper()
		q := protocol.Request{Version: version, ID: protocol.ID(), MachineID: s.MachineID, Method: method, SessionID: session, Nonce: "test-nonce"}
		if e := protocol.Write(client, q); e != nil {
			t.Fatal(e)
		}
		var p protocol.Response
		if e := protocol.Read(r, &p); e != nil {
			t.Fatal(e)
		}
		return p
	}
	p := call("health", s.SessionID, protocol.Version)
	if p.Error == nil || p.Error.Code != "HANDSHAKE_REQUIRED" {
		t.Fatalf("missing handshake accepted: %#v", p)
	}
	p = call("hello", "", protocol.Version)
	if p.Result == nil || p.Result.SessionID != s.SessionID {
		t.Fatal("hello failed")
	}
	p = call("probe", s.SessionID, protocol.Version)
	hash := sha256.Sum256([]byte("test-nonce"))
	if p.Result == nil || p.Result.NonceSHA256 != hex.EncodeToString(hash[:]) {
		t.Fatal("probe not executed")
	}
	p = call("health", protocol.ID(), protocol.Version)
	if p.Error == nil || p.Error.Code != "STALE_SESSION" {
		t.Fatal("stale service session accepted")
	}
	p = call("host_exec", s.SessionID, protocol.Version)
	if p.Error == nil || p.Error.Code != "UNSUPPORTED_OPERATION" {
		t.Fatal("unknown/host execution accepted")
	}
	p = call("hello", "", "99.0")
	if p.Error == nil || p.Error.Code != "VERSION_MISMATCH" {
		t.Fatal("incompatible protocol accepted")
	}
}
func TestDisconnectAndMalformed(t *testing.T) {
	for _, bad := range []bool{false, true} {
		s := &Service{MachineID: protocol.ID(), SessionID: protocol.ID()}
		c, server := net.Pipe()
		done := make(chan struct{})
		go func() { s.Handle(server); close(done) }()
		if bad {
			_ = c.SetDeadline(time.Now().Add(time.Second))
			_, _ = c.Write([]byte("bad json\n"))
			var p protocol.Response
			if protocol.Read(protocol.Reader(c), &p) != nil || p.Error == nil {
				t.Fatal("malformed input not rejected")
			}
		}
		c.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("guest leaked disconnected handler")
		}
	}
}

func TestIdleTimeout(t *testing.T) {
	s := &Service{MachineID: protocol.ID(), SessionID: protocol.ID(), idleTimeout: 20 * time.Millisecond}
	c, server := net.Pipe()
	defer c.Close()
	done := make(chan struct{})
	go func() { s.Handle(server); close(done) }()
	_ = c.SetDeadline(time.Now().Add(time.Second))
	var p protocol.Response
	_ = protocol.Read(protocol.Reader(c), &p)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("idle guest connection did not terminate")
	}
}
