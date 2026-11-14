// SPDX-License-Identifier: Apache-2.0
package lima

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

func captureFixture(t *testing.T, behavior string) (*CaptureTransport, lifecycle.State, lifecycle.Observation, *atomic.Int32) {
	t.Helper()
	c := NewCaptureTransport(context.Background(), nil)
	t.Cleanup(c.Close)
	state := lifecycle.State{MachineID: protocol.ID()}
	observed := lifecycle.Observation{Port: 2200}
	var opened atomic.Int32
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 1280, 800))); err != nil {
		t.Fatal(err)
	}
	c.dial = func(_ context.Context, _ lifecycle.State, _ lifecycle.Observation) (io.ReadWriteCloser, error) {
		left, right := net.Pipe()
		attempt := opened.Add(1)
		go func() {
			defer right.Close()
			reader := protocol.Reader(right)
			session := protocol.ID()
			for {
				var q protocol.Request
				if protocol.Read(reader, &q) != nil {
					return
				}
				info := &protocol.Info{Protocol: protocol.Version, GuestVersion: protocol.GuestVersion, MachineID: state.MachineID, SessionID: session, BootID: "boot", OS: "linux", Arch: "arm64"}
				switch q.Method {
				case "probe":
					info.NonceSHA256 = sha256Probe(q.Nonce)
					if behavior == "nonce" {
						info.NonceSHA256 = "wrong"
					}
				case "screenshot":
					if behavior == "disconnect" && attempt == 1 {
						return
					}
					if behavior == "stall" && attempt == 1 {
						_, _ = io.Copy(io.Discard, right)
						return
					}
					info.Frame = &protocol.Frame{ID: protocol.ID(), SessionID: session, Generation: "desktop", Sequence: 1, CapturedAt: time.Now(), Width: 1280, Height: 800, Scale: 1, Format: "png", PNG: imageBytes.Bytes()}
					if behavior == "boot" {
						info.BootID = "other-boot"
					}
				}
				if protocol.Write(right, protocol.Response{Version: protocol.Version, ID: q.ID, Result: info}) != nil {
					return
				}
			}
		}()
		return left, nil
	}
	return c, state, observed, &opened
}
func TestCaptureReusesAndRecyclesBoundedChannel(t *testing.T) {
	c, s, o, opened := captureFixture(t, "")
	for n := 0; n < 50; n++ {
		if _, err := c.Call(context.Background(), s, o, protocol.Request{Method: "screenshot"}); err != nil {
			t.Fatal(n, err)
		}
	}
	if opened.Load() != 2 || c.Stats().Reused != 48 {
		t.Fatal(opened.Load(), c.Stats())
	}
}
func TestCaptureDoesNotTransportMutations(t *testing.T) {
	c, s, o, opened := captureFixture(t, "")
	if _, err := c.Call(context.Background(), s, o, protocol.Request{Method: "input"}); err == nil || opened.Load() != 0 {
		t.Fatal("mutation reached capture channel")
	}
}
func TestCaptureDisconnectAndCancellationReconnectOnlyNextObservation(t *testing.T) {
	for _, behavior := range []string{"disconnect", "stall"} {
		t.Run(behavior, func(t *testing.T) {
			c, s, o, opened := captureFixture(t, behavior)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			if _, err := c.Call(ctx, s, o, protocol.Request{Method: "screenshot"}); err == nil {
				t.Fatal("failure hidden")
			}
			if opened.Load() != 1 {
				t.Fatal("operation replayed")
			}
			if _, err := c.Call(context.Background(), s, o, protocol.Request{Method: "screenshot"}); err != nil {
				t.Fatal(err)
			}
			if opened.Load() != 2 {
				t.Fatal("next observation did not reconnect")
			}
		})
	}
}
func TestCaptureRejectsNonceBootAndSessionMismatch(t *testing.T) {
	for _, behavior := range []string{"nonce", "boot"} {
		c, s, o, _ := captureFixture(t, behavior)
		if _, err := c.Call(context.Background(), s, o, protocol.Request{Method: "screenshot"}); err == nil {
			t.Fatal("identity mismatch accepted", behavior)
		}
	}
	c, s, o, _ := captureFixture(t, "")
	if _, err := c.Call(context.Background(), s, o, protocol.Request{Method: "screenshot", SessionID: protocol.ID()}); err == nil {
		t.Fatal("old session accepted")
	}
}
