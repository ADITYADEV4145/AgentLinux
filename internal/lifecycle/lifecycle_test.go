// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"errors"
	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
	"os"
	"path/filepath"
	"testing"
)

type fake struct {
	o                                     Observation
	handshakeError, startError, stopError error
	created, started                      int
}

func (f *fake) Observe(context.Context) (Observation, error) { return f.o, nil }
func (f *fake) Create(context.Context, State) error {
	f.created++
	f.o = Observation{Exists: true, Status: "Stopped"}
	return nil
}
func (f *fake) Validate(context.Context) error { return nil }
func (f *fake) Start(context.Context) error {
	f.started++
	if f.startError != nil {
		return f.startError
	}
	f.o.Status = "Running"
	return nil
}
func (f *fake) Stop(context.Context) error {
	if f.stopError != nil {
		return f.stopError
	}
	f.o.Status = "Stopped"
	return nil
}
func (f *fake) Handshake(_ context.Context, s State) (*protocol.Info, error) {
	if f.handshakeError != nil {
		return nil, f.handshakeError
	}
	return &protocol.Info{MachineID: s.MachineID, SessionID: protocol.ID(), Protocol: protocol.Version}, nil
}
func setup(t *testing.T) (*Controller, *fake, config.Spec) {
	t.Helper()
	r, _ := filepath.EvalSymlinks(t.TempDir())
	_ = os.Chmod(r, 0700)
	r, e := storage.Root(r)
	if e != nil {
		t.Fatal(e)
	}
	binary := filepath.Join(r, "fixture")
	_ = os.WriteFile(binary, []byte("fixture"), 0600)
	spec := config.Default()
	spec.Network = "nat"
	spec.GuestBinary = binary
	f := &fake{}
	return &Controller{Root: r, Backend: f}, f, spec
}
func TestLifecycleReadinessAndRestart(t *testing.T) {
	c, f, spec := setup(t)
	f.handshakeError = errors.New("service not ready")
	s, e := c.Up(context.Background(), spec, true)
	if e == nil || s.State != "error" || s.Guest != nil {
		t.Fatal("reported process startup as ready")
	}
	id := s.MachineID
	f.handshakeError = nil
	s, e = c.Up(context.Background(), config.Spec{}, false)
	if e != nil || s.State != "ready" || s.MachineID != id || f.created != 1 {
		t.Fatalf("recovery recreated machine: %#v %v", s, e)
	}
	oldSession := s.Guest.SessionID
	s, e = c.Stop(context.Background())
	if e != nil || s.State != "stopped" {
		t.Fatal(e)
	}
	s, e = c.Up(context.Background(), config.Spec{}, false)
	if e != nil || s.Guest.SessionID == oldSession || s.MachineID != id {
		t.Fatal("restart identity/session failed")
	}
}
func TestInterruptedStartAndStaleState(t *testing.T) {
	c, f, spec := setup(t)
	f.startError = context.Canceled
	s, e := c.Up(context.Background(), spec, true)
	if !errors.Is(e, context.Canceled) || s.LastError != "INTERRUPTED" {
		t.Fatal("interruption not recorded")
	}
	f.startError = nil
	f.o.Status = "Running"
	s, e = c.Status(context.Background())
	if e != nil || s.State != "ready" {
		t.Fatal("stale state not reconciled")
	}
	f.o = Observation{}
	if _, e = c.Up(context.Background(), config.Spec{}, false); e == nil || f.created != 1 {
		t.Fatal("missing persistent disk recreated")
	}
}
func TestUnconfirmedStopAndOwnership(t *testing.T) {
	c, f, spec := setup(t)
	_, _ = c.Up(context.Background(), spec, true)
	f.stopError = context.DeadlineExceeded
	s, e := c.Stop(context.Background())
	if e == nil || s.State == "stopped" || s.Observed != "unknown" {
		t.Fatal("failed stop reported success")
	}
	c2, f2, _ := setup(t)
	f2.o = Observation{Exists: true, Status: "Running"}
	if _, e = c2.Up(context.Background(), spec, true); e == nil || f2.created != 0 {
		t.Fatal("unrecorded VM adopted")
	}
}

func TestInterruptedCreationRetainsFullDiskBudget(t *testing.T) {
	c, _, spec := setup(t)
	state := State{Schema: 1, MachineID: protocol.ID(), State: "error", Observed: "absent", Stage: "provision_configuration", Spec: spec}
	if e := storage.JSON(c.Root, "state.json", state); e != nil {
		t.Fatal(e)
	}
	checked := false
	c.Preflight = func(_ config.Spec, newDisk bool) error {
		checked = true
		if !newDisk {
			t.Fatal("interrupted pre-creation bypassed full disk reserve")
		}
		return errors.New("disk full")
	}
	if _, e := c.Up(context.Background(), config.Spec{}, false); e == nil || !checked {
		t.Fatal("expected preflight refusal")
	}
}
