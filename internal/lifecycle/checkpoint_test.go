// SPDX-License-Identifier: Apache-2.0
package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type checkpointBackend struct{ *fake }

func (b *checkpointBackend) CheckpointCompatibility(context.Context, State) (string, error) {
	return strings.Repeat("a", 64), nil
}
func TestRecoveryBlocksBootAndRetainsRollbackUntilHandshake(t *testing.T) {
	c, b, spec := setup(t)
	c.Backend = &checkpointBackend{b}
	ctx := context.Background()
	state, e := c.Up(ctx, spec, true)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Checkpoint(ctx, "create", "safe", "", ""); e == nil {
		t.Fatal("running VM checkpointed")
	}
	if _, e = c.Stop(ctx); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"lima/default/disk", "lima/default/vz-efi", "machine.yaml", "config.sha256", "lima/default/lima.yaml"} {
		path := filepath.Join(c.Root, p)
		if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, []byte("original "+p), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = c.Checkpoint(ctx, "create", "safe", "", ""); e != nil {
		t.Fatal(e)
	}
	manager, e := c.checkpointManager(ctx, state)
	if e != nil {
		t.Fatal(e)
	}
	manager.Fault = func(phase string) error {
		if phase == "installed:lima/default/disk" {
			return errors.New("power loss")
		}
		return nil
	}
	if _, e = manager.Restore(ctx, "safe"); e == nil {
		t.Fatal("fault not injected")
	}
	journalPath := filepath.Join(c.Root, "recovery/journal.json")
	before, e := os.ReadFile(filepath.Join(c.Root, "state.json"))
	if e != nil {
		t.Fatal(e)
	}
	started := b.started
	if _, e = c.Up(ctx, spec, false); e == nil || b.started != started {
		t.Fatal("incomplete restore booted")
	}
	if _, e = c.Status(ctx); e == nil {
		t.Fatal("incomplete restore hidden")
	}
	after, _ := os.ReadFile(filepath.Join(c.Root, "state.json"))
	if string(before) != string(after) {
		t.Fatal("status changed interrupted restore state")
	}
	if _, e = c.Checkpoint(ctx, "recover", "", "resume", state.MachineID); e != nil {
		t.Fatal(e)
	}
	b.handshakeError = errors.New("guest unavailable")
	if _, e = c.Up(ctx, spec, false); e == nil {
		t.Fatal("failed guest reported ready")
	}
	if _, e = os.Stat(journalPath); e != nil {
		t.Fatal("rollback removed before handshake")
	}
	b.handshakeError = nil
	if _, e = c.Up(ctx, spec, false); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(journalPath); !os.IsNotExist(e) {
		t.Fatal("verified restore not confirmed")
	}
}
