// SPDX-License-Identifier: Apache-2.0
package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

type deletingBackend struct {
	*fake
	deleteError error
	deleted     int
}

func (b *deletingBackend) Delete(context.Context) error {
	b.deleted++
	if b.deleteError != nil {
		return b.deleteError
	}
	b.o = Observation{}
	return nil
}
func removalFixture(t *testing.T) (*Controller, *deletingBackend, State) {
	c, f, spec := setup(t)
	bin := filepath.Join(t.TempDir(), "helper")
	os.WriteFile(bin, []byte("fixture"), 0600)
	spec.GuestBinary = bin
	os.Remove(filepath.Join(c.Root, "fixture"))
	b := &deletingBackend{fake: f}
	c.Backend = b
	s, e := c.Up(context.Background(), spec, true)
	if e != nil {
		t.Fatal(e)
	}
	return c, b, s
}
func TestDeleteGuardsAndNoSymlinkTraversal(t *testing.T) {
	c, b, s := removalFixture(t)
	if _, e := c.Remove(context.Background(), protocol.ID(), false, ""); e == nil || b.deleted != 0 {
		t.Fatal("wrong identity removed data")
	}
	if _, e := c.Remove(context.Background(), s.MachineID, false, ""); e == nil || b.deleted != 0 {
		t.Fatal("deleted running machine")
	}
	c.Stop(context.Background())
	unrelated := filepath.Join(t.TempDir(), "preserve")
	os.WriteFile(unrelated, []byte("safe"), 0600)
	os.WriteFile(filepath.Join(c.Root, "unrelated"), []byte("safe"), 0600)
	if _, e := c.Remove(context.Background(), s.MachineID, false, ""); e == nil || b.deleted != 0 {
		t.Fatal("unknown state entry removed")
	}
	os.Remove(filepath.Join(c.Root, "unrelated"))
	os.Symlink(filepath.Dir(unrelated), filepath.Join(c.Root, "tmp"))
	if _, e := c.PlanRemoval(s.MachineID, "delete"); e != nil {
		t.Fatal(e)
	}
	out, e := c.Remove(context.Background(), s.MachineID, false, "")
	if e != nil || out.State != "not_created" {
		t.Fatal(e, out)
	}
	if _, e = os.Stat(unrelated); e != nil {
		t.Fatal("followed nested symlink")
	}
	if _, e = storage.ExistingRoot(c.Root); e != nil {
		t.Fatal("lock/marker removed", e)
	}
	if _, e = c.Remove(context.Background(), s.MachineID, false, ""); e == nil {
		t.Fatal("replayed old identity")
	}
}
func TestResetIdentityAndInterruptedDelete(t *testing.T) {
	c, b, s := removalFixture(t)
	c.Stop(context.Background())
	b.deleteError = errors.New("interrupted runtime deletion")
	if _, e := c.Remove(context.Background(), s.MachineID, true, s.Spec.GuestBinary); e == nil {
		t.Fatal("deletion error hidden")
	}
	if _, e := c.Up(context.Background(), config.Spec{}, false); e == nil {
		t.Fatal("started after interrupted delete")
	}
	status, e := c.Status(context.Background())
	if e != nil || status.State != "deleting" {
		t.Fatal("status erased interrupted deletion", status, e)
	}
	if _, e = c.Up(context.Background(), config.Spec{}, false); e == nil {
		t.Fatal("status made interrupted delete resumable")
	}
	b.deleteError = nil
	next, e := c.Remove(context.Background(), s.MachineID, true, s.Spec.GuestBinary)
	if e != nil || next.State != "ready" || next.MachineID == s.MachineID || b.created != 2 {
		t.Fatal(next, e)
	}
}
func TestExistingRootDoesNotAdoptOrCreate(t *testing.T) {
	p := filepath.Join(t.TempDir(), "absent")
	if _, e := storage.ExistingRoot(p); e == nil {
		t.Fatal("adopted missing root")
	}
	if _, e := os.Stat(p); !os.IsNotExist(e) {
		t.Fatal("created root")
	}
}

func TestResetRefusesInputsInsideDeletedStateThroughAlias(t *testing.T) {
	c, b, s := removalFixture(t)
	c.Stop(context.Background())
	helper := filepath.Join(c.Root, "control")
	os.WriteFile(helper, []byte("fixture"), 0600)
	alias := filepath.Join(t.TempDir(), "alias")
	os.Symlink(c.Root, alias)
	for _, input := range []string{helper, filepath.Join(alias, "control")} {
		if _, e := c.Remove(context.Background(), s.MachineID, true, input); e == nil || b.deleted != 0 {
			t.Fatal("reset removed its own input", input, e)
		}
	}
}
