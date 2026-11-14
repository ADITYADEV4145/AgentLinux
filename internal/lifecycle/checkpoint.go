// SPDX-License-Identifier: Apache-2.0
package lifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ADITYADEV4145/AgentLinux/internal/checkpoint"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

func (c *Controller) checkpointManager(ctx context.Context, s State) (checkpoint.Manager, error) {
	b, ok := c.Backend.(interface {
		CheckpointCompatibility(context.Context, State) (string, error)
	})
	if !ok {
		return checkpoint.Manager{}, protocol.Fail("UNSUPPORTED_OPERATION", "backend has no verified stopped-disk checkpoint layout")
	}
	compatibility, e := b.CheckpointCompatibility(ctx, s)
	return checkpoint.Manager{Root: c.Root, MachineID: s.MachineID, Compatibility: compatibility, Progress: c.Progress}, e
}
func (c *Controller) pendingRecovery(ctx context.Context, s State) (*checkpoint.Journal, error) {
	if _, e := os.Lstat(filepath.Join(c.Root, checkpoint.Recovery)); os.IsNotExist(e) {
		if _, e = os.Lstat(filepath.Join(c.Root, "recovery-confirmed.json")); os.IsNotExist(e) {
			return nil, nil
		}
	}
	m, e := c.checkpointManager(ctx, s)
	if e != nil {
		return nil, e
	}
	return m.Pending()
}
func (c *Controller) allowBoot(ctx context.Context, s State) error {
	pending, e := c.pendingRecovery(ctx, s)
	if e != nil {
		return e
	}
	if pending != nil && pending.Phase != "awaiting_ready" && pending.Phase != "confirmed_cleanup" {
		return protocol.Fail("RECOVERY_REQUIRED", "Disk/configuration recovery is incomplete. Stop and use checkpoint recover --mode resume|rollback explicitly before booting.")
	}
	return nil
}
func (c *Controller) confirmRecovery(ctx context.Context, s State) error {
	pending, e := c.pendingRecovery(ctx, s)
	if e != nil || pending == nil {
		return e
	}
	if pending.Phase != "awaiting_ready" && pending.Phase != "confirmed_cleanup" {
		return errors.New("recovery is incomplete")
	}
	if pending.Kind == "helper-upgrade" {
		b, ok := c.Backend.(interface {
			Call(context.Context, State, protocol.Request) (*protocol.Info, error)
		})
		if !ok {
			return errors.New("cannot verify installed helper")
		}
		result, e := b.Call(ctx, s, protocol.Request{Method: "exec", SessionID: s.Guest.SessionID, Exec: &protocol.ExecRequest{Argv: []string{"/usr/bin/sha256sum", "/usr/local/bin/agent-linux-guest"}, Cwd: "/home/agent", TimeoutMS: 5000, OutputLimit: 1024}})
		if e != nil {
			return e
		}
		if result.Execution == nil || result.Execution.ExitCode != 0 || !result.Execution.CleanupConfirmed || !strings.HasPrefix(string(result.Execution.Stdout), pending.HelperSHA256+" ") {
			return protocol.Fail("HELPER_UPGRADE_UNVERIFIED", "Installed helper digest did not match. Rollback data is retained; stop and recover explicitly.")
		}
	}
	m, e := c.checkpointManager(ctx, s)
	if e != nil {
		return e
	}
	return m.Confirm()
}
func (c *Controller) checkpointStopped(ctx context.Context, s State) error {
	if s.State == "not_created" || s.State == "deleting" {
		return errors.New("checkpoint requires an existing selected machine")
	}
	o, e := c.Backend.Observe(ctx)
	if e != nil {
		return e
	}
	if !o.Exists || o.Status != "Stopped" {
		return protocol.Fail("MACHINE_NOT_STOPPED", "Stop the selected VM explicitly first. Checkpoints never stop a running VM.")
	}
	return nil
}
func (c *Controller) Checkpoints(ctx context.Context) ([]checkpoint.Manifest, *checkpoint.Journal, error) {
	unlock, e := storage.LockWait(ctx, c.Root, 2e9)
	if e != nil {
		return nil, nil, e
	}
	defer unlock()
	s, e := c.load()
	if e != nil {
		return nil, nil, e
	}
	m, e := c.checkpointManager(ctx, s)
	if e != nil {
		return nil, nil, e
	}
	list, e := m.List()
	if e != nil {
		return nil, nil, e
	}
	pending, e := m.Pending()
	return list, pending, e
}
func (c *Controller) Checkpoint(ctx context.Context, action, name, mode, id string) (any, error) {
	unlock, e := storage.Lock(c.Root)
	if e != nil {
		return nil, e
	}
	defer unlock()
	s, e := c.load()
	if e != nil {
		return nil, e
	}
	if action != "create" && (!protocol.ValidID(id) || s.MachineID != id) {
		return nil, errors.New("exact --machine-id required for checkpoint mutation")
	}
	if action != "delete" {
		if e = c.checkpointStopped(ctx, s); e != nil {
			return nil, e
		}
	}
	m, e := c.checkpointManager(ctx, s)
	if e != nil {
		return nil, e
	}
	switch action {
	case "create":
		if e = c.Backend.Validate(ctx); e != nil {
			return nil, e
		}
		return m.Create(ctx, name)
	case "delete":
		return map[string]string{"deleted": name}, m.Delete(name)
	case "restore":
		if e = c.Backend.Validate(ctx); e != nil {
			return nil, e
		}
		j, e := m.Restore(ctx, name)
		if e != nil {
			return nil, e
		}
		e = c.invalidateStoppedRecovery()
		return j, e
	case "recover":
		if mode != "resume" && mode != "rollback" {
			return nil, errors.New("recover mode must be resume or rollback")
		}
		if e = m.Recover(ctx, mode == "rollback"); e != nil {
			return nil, e
		}
		e = c.invalidateStoppedRecovery()
		return map[string]string{"recovery": mode, "next": "up --local explicitly; rollback retained until readiness"}, e
	default:
		return nil, errors.New("unknown checkpoint action")
	}
}
func (c *Controller) invalidateStoppedRecovery() error {
	if e := storage.SafeFile(c.Root, "input-lease.json"); e != nil {
		return e
	}
	if e := os.Remove(filepath.Join(c.Root, "input-lease.json")); e != nil && !os.IsNotExist(e) {
		return e
	}
	if e := storage.JSON(c.Root, "viewer-change.json", protocol.ID()); e != nil {
		return e
	}
	s, e := c.load()
	if e != nil {
		return e
	}
	s.State = "stopped"
	s.Observed = "Stopped"
	s.Guest = nil
	s.Stage = "explicit_recovery_pending_readiness"
	s.LastError = ""
	return c.save(&s)
}
func (c *Controller) RestartDesktop(ctx context.Context, id string) (State, error) {
	unlock, e := storage.Lock(c.Root)
	if e != nil {
		return State{}, e
	}
	defer unlock()
	s, e := c.load()
	if e != nil {
		return s, e
	}
	if s.MachineID != id || !protocol.ValidID(id) {
		return s, errors.New("exact --machine-id required")
	}
	if e = c.allowBoot(ctx, s); e != nil {
		return s, e
	}
	o, e := c.Backend.Observe(ctx)
	if e != nil {
		return s, e
	}
	if o.Status != "Running" {
		return s, errors.New("desktop restart requires a running VM")
	}
	if e = c.Backend.Validate(ctx); e != nil {
		return s, e
	}
	b, ok := c.Backend.(interface{ RestartDesktop(context.Context) error })
	if !ok {
		return s, errors.New("desktop restart unsupported")
	}
	oldGeneration := ""
	if s.Guest != nil && s.Guest.Desktop != nil {
		oldGeneration = s.Guest.Desktop.Generation
	}
	s.State = "starting"
	s.Guest = nil
	s.Stage = "explicit_desktop_restart"
	if e = c.save(&s); e != nil {
		return s, e
	}
	if e = b.RestartDesktop(ctx); e != nil {
		return c.fail(&s, e)
	}
	g, e := c.Backend.Handshake(ctx, s)
	if e != nil {
		return c.fail(&s, e)
	}
	if g.Desktop == nil || !g.Desktop.Ready || g.Desktop.Generation == oldGeneration {
		return c.fail(&s, errors.New("fresh desktop generation not verified"))
	}
	s.State = "ready"
	s.Stage = "desktop_ready"
	s.Guest = g
	s.Observed = "Running"
	if e = storage.JSON(c.Root, "viewer-change.json", protocol.ID()); e != nil {
		return s, e
	}
	return s, c.save(&s)
}
func (c *Controller) UpgradeHelper(ctx context.Context, id, binaryPath string) (State, error) {
	unlock, e := storage.Lock(c.Root)
	if e != nil {
		return State{}, e
	}
	defer unlock()
	s, e := c.load()
	if e != nil {
		return s, e
	}
	if s.MachineID != id || !protocol.ValidID(id) {
		return s, errors.New("exact machine identity required")
	}
	if e = c.checkpointStopped(ctx, s); e != nil {
		return s, e
	}
	info, e := os.Lstat(binaryPath)
	if e != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 32<<20 {
		return s, errors.New("helper must be a bounded regular file")
	}
	binary, e := os.ReadFile(binaryPath)
	if e != nil {
		return s, e
	}
	digest := sha256.Sum256(binary)
	m, e := c.checkpointManager(ctx, s)
	if e != nil {
		return s, e
	}
	if e = c.Backend.Validate(ctx); e != nil {
		return s, e
	}
	name := "before-helper-" + protocol.ID()[:12]
	if _, e = m.Create(ctx, name); e != nil {
		return s, e
	}
	// Restore the just-created checkpoint into a durable recovery transaction. The
	// rollback disk stays untouched while next boot applies the helper-only recipe.
	if _, e = m.Restore(ctx, name); e != nil {
		return s, e
	}
	if e = m.RequireHelper(hex.EncodeToString(digest[:])); e != nil {
		return s, e
	}
	b, ok := c.Backend.(interface {
		UpgradeHelper(context.Context, string) error
	})
	if !ok {
		return s, errors.New("helper upgrade unsupported; rollback is retained")
	}
	if e = b.UpgradeHelper(ctx, binaryPath); e != nil {
		return s, fmt.Errorf("helper metadata incomplete; explicit rollback required: %w", e)
	}
	s.Spec.GuestBinary = binaryPath
	s.State = "stopped"
	s.Observed = "Stopped"
	s.Guest = nil
	s.Stage = "helper_upgrade_pending_readiness"
	if e = c.save(&s); e != nil {
		return s, e
	}
	if e = m.UpgradeInstalled(); e != nil {
		return s, e
	}
	return c.upLocked(ctx, s.Spec, false)
}
