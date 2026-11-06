// SPDX-License-Identifier: Apache-2.0
package lifecycle

import (
	"context"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type leaseBackend struct {
	*fake
	generation, session string
	calls               int
}

func (b *leaseBackend) Call(_ context.Context, s State, q protocol.Request) (*protocol.Info, error) {
	b.calls++
	return &protocol.Info{SessionID: b.session, Frame: &protocol.Frame{Generation: b.generation}}, nil
}
func TestLocalLeaseCrossProcessMutationsExpiryRestartAndRelease(t *testing.T) {
	c, f, spec := setup(t)
	ctx := context.Background()
	s, e := c.Up(ctx, spec, true)
	if e != nil {
		t.Fatal(e)
	}
	b := &leaseBackend{fake: f, generation: "generation", session: protocol.ID()}
	c.Backend = b
	c.ControlOwner = protocol.ID()
	if e = c.ControlLease(ctx, "acquire", b.generation); e != nil {
		t.Fatal(e)
	}
	agent := &Controller{Root: c.Root, Backend: b}
	for _, method := range []string{"input", "exec", "open", "files"} {
		before := b.calls
		_, e = agent.GuestCall(ctx, protocol.Request{Method: method})
		if e == nil || !strings.Contains(e.Error(), "INPUT_OWNED") || b.calls != before {
			t.Fatal("mutation bypass", method, e)
		}
	}
	if _, e = agent.GuestCall(ctx, protocol.Request{Method: "screenshot"}); e != nil {
		t.Fatal("observation blocked", e)
	}
	if _, e = c.GuestCall(ctx, protocol.Request{Method: "input"}); e != nil {
		t.Fatal(e)
	}
	b.generation = "restarted"
	if e = c.ControlLease(ctx, "renew", b.generation); e == nil {
		t.Fatal("renew survived restart")
	}
	if _, e = os.Stat(filepath.Join(c.Root, "input-lease.json")); !os.IsNotExist(e) {
		t.Fatal("lease not revoked", e)
	}
	if e = c.ControlLease(ctx, "acquire", b.generation); e != nil {
		t.Fatal(e)
	}
	var l inputLease
	if e = storage.Read(c.Root, "input-lease.json", &l); e != nil {
		t.Fatal(e)
	}
	l.Expires = time.Now().Add(-time.Second)
	_ = storage.JSON(c.Root, "input-lease.json", l)
	if _, e = agent.GuestCall(ctx, protocol.Request{Method: "open"}); e != nil {
		t.Fatal("expired lease retained", e)
	}
	if _, e = c.GuestCall(ctx, protocol.Request{Method: "input"}); e == nil {
		t.Fatal("old owner continued after expiry")
	}
	if e = c.ControlLease(ctx, "acquire", b.generation); e != nil {
		t.Fatal(e)
	}
	if e = c.ControlLease(ctx, "release", ""); e != nil {
		t.Fatal(e)
	}
	if e = c.ControlLease(ctx, "acquire", b.generation); e != nil {
		t.Fatal(e)
	}
	if _, e = c.Stop(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(c.Root, "input-lease.json")); !os.IsNotExist(e) {
		t.Fatal("stop retained ownership", e)
	}
	_ = s
}

func TestPrivateAdvisoryViewerRevisionAcrossControllers(t *testing.T) {
	c, f, spec := setup(t)
	ctx := context.Background()
	if _, e := c.Up(ctx, spec, true); e != nil {
		t.Fatal(e)
	}
	c.Backend = &leaseBackend{fake: f, generation: "generation", session: protocol.ID()}
	other := &Controller{Root: c.Root}
	if other.ObservationRevision() != "" {
		t.Fatal("invented revision")
	}
	if _, e := c.GuestCall(ctx, protocol.Request{Method: "screenshot"}); e != nil {
		t.Fatal(e)
	}
	if other.ObservationRevision() != "" {
		t.Fatal("observation is not mutation")
	}
	if _, e := c.GuestCall(ctx, protocol.Request{Method: "open"}); e != nil {
		t.Fatal(e)
	}
	previous := other.ObservationRevision()
	if !protocol.ValidID(previous) {
		t.Fatal("no private cross-process wake signal")
	}
	if _, e := c.GuestCall(ctx, protocol.Request{Method: "input"}); e != nil {
		t.Fatal(e)
	}
	if other.ObservationRevision() == previous {
		t.Fatal("revision not changed")
	}
	if e := storage.Write(c.Root, "viewer-change.json", []byte(`"malformed"`)); e != nil {
		t.Fatal(e)
	}
	if other.ObservationRevision() != "" {
		t.Fatal("malformed revision accepted")
	}
}
