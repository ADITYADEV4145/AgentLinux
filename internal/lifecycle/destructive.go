// SPDX-License-Identifier: Apache-2.0
package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

type RemovalPlan struct {
	Action         string   `json:"action"`
	MachineID      string   `json:"machineId"`
	StateDirectory string   `json:"stateDirectory"`
	Removes        []string `json:"removes"`
	Preserves      []string `json:"preserves"`
	Entries        []string `json:"ownedEntries"`
}

var ownedEntries = map[string]bool{"state.json": true, "machine.yaml": true, "config.sha256": true, "debian-arm64.qcow2": true, "guest_host": true, "guest_host.pub": true, "control": true, "control.pub": true, "known_hosts": true, "lima": true, "tmp": true, "events.jsonl": true, "input-lease.json": true, "viewer-change.json": true, "backend.jsonl": true, ".agent-linux-owner": true, "lock": true}

func (c *Controller) removalPlan(id, action string) (State, RemovalPlan, error) {
	s, e := c.load()
	if e != nil {
		return s, RemovalPlan{}, e
	}
	p := RemovalPlan{Action: action, MachineID: s.MachineID, StateDirectory: c.Root, Removes: []string{"selected VM disk and all guest data", "private Lima namespace, generated keys, configuration and cached base image"}, Preserves: []string{"application ownership marker and lock", "all files outside this state directory"}}
	if !protocol.ValidID(id) || s.MachineID != id {
		return s, p, errors.New("machine ID does not match selected state; inspect status and pass --machine-id exactly")
	}
	entries, e := os.ReadDir(c.Root)
	if e != nil {
		return s, p, e
	}
	for _, entry := range entries {
		if entry.Name() != "lock" && entry.Name() != ".agent-linux-owner" {
			p.Entries = append(p.Entries, entry.Name())
		}
		if !ownedEntries[entry.Name()] {
			return s, p, errors.New("unexpected entry in state directory; inspect it before destructive action (nothing removed)")
		}
	}
	return s, p, nil
}
func (c *Controller) PlanRemoval(id, action string) (RemovalPlan, error) {
	if _, e := storage.ExistingRoot(c.Root); e != nil {
		return RemovalPlan{}, e
	}
	unlock, e := storage.Lock(c.Root)
	if e != nil {
		return RemovalPlan{}, e
	}
	defer unlock()
	_, p, e := c.removalPlan(id, action)
	return p, e
}

// Remove requires exact identity confirmation under the same lock as deletion.
// Keep the lock inode/marker to avoid unsafe concurrent recreation of a namespace.
func (c *Controller) Remove(ctx context.Context, id string, reset bool, guestBinary string) (State, error) {
	if _, e := storage.ExistingRoot(c.Root); e != nil {
		return State{}, e
	}
	unlock, e := storage.Lock(c.Root)
	if e != nil {
		return State{}, e
	}
	defer unlock()
	s, _, e := c.removalPlan(id, "delete")
	if e != nil {
		return s, e
	}
	if reset {
		s.Spec.GuestBinary = guestBinary
		if e = s.Spec.Validate(); e != nil {
			return s, e
		}
		for _, p := range []string{s.Spec.Image, guestBinary} {
			if filepath.IsAbs(p) {
				resolved, e := filepath.EvalSymlinks(p)
				if e != nil {
					return s, e
				}
				if pathWithin(c.Root, resolved) {
					return s, errors.New("reset inputs must be outside deleted state directory")
				}
			}
		}

		if c.Preflight != nil {
			if e = c.Preflight(s.Spec, true); e != nil {
				return s, e
			}
		}
	}
	o, e := c.Backend.Observe(ctx)
	if e != nil {
		return s, e
	}
	if o.Exists && o.Status != "Stopped" {
		return s, errors.New("stop the selected VM first; deletion/reset never forces a running machine off")
	}
	if o.Exists {
		if e = c.Backend.Validate(ctx); e != nil {
			return s, e
		}
	}
	b, ok := c.Backend.(interface{ Delete(context.Context) error })
	if !ok {
		return s, protocol.Fail("UNSUPPORTED_OPERATION", "backend has no guarded deletion")
	}
	s.State = "deleting"
	s.Stage = "explicit_delete"
	s.Guest = nil
	if e = c.save(&s); e != nil {
		return s, e
	}
	if o.Exists {
		if e = b.Delete(ctx); e != nil {
			return s, e
		}
	}
	o, e = c.Backend.Observe(ctx)
	if e != nil {
		return s, e
	}
	if o.Exists {
		return s, errors.New("runtime deletion not confirmed; private state retained")
	}
	r, e := os.OpenRoot(c.Root)
	if e != nil {
		return s, e
	}
	defer r.Close()
	// Only known names, rooted deletion, no symlink traversal. Keep state until the end for interruption recovery.
	for name := range ownedEntries {
		if name == "state.json" || name == "lock" || name == ".agent-linux-owner" {
			continue
		}
		if e = r.RemoveAll(name); e != nil {
			return s, e
		}
	}
	if e = r.Remove("state.json"); e != nil {
		return s, e
	}
	if reset {
		return c.upLocked(ctx, s.Spec, true)
	}
	return State{Schema: 1, State: "not_created", Observed: "absent"}, nil
}
func pathWithin(root, p string) bool {
	rel, e := filepath.Rel(root, p)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
