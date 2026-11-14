// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"context"
	"os"
	"sync"

	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/fleet"
	"github.com/ADITYADEV4145/AgentLinux/internal/host"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/lima"
	"github.com/ADITYADEV4145/AgentLinux/internal/machine"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

type localDashboard struct {
	manager *fleet.Manager
	runtime string
	mu      sync.Mutex
	planned map[string]config.Spec
}

func (d *localDashboard) entry(ctx context.Context, id string) (fleet.Entry, error) {
	entries, e := d.manager.List(ctx)
	if e != nil {
		return fleet.Entry{}, e
	}
	for _, entry := range entries {
		if entry.ID == id {
			return entry, nil
		}
	}
	return fleet.Entry{}, protocol.Fail("MACHINE_IDENTITY_CHANGED", "Machine is no longer in the selected local catalog.")
}
func (d *localDashboard) controller(ctx context.Context, id string) (*lifecycle.Controller, error) {
	v, e := d.entry(ctx, id)
	if e != nil {
		return nil, e
	}
	root, e := storage.Root(v.Root)
	if e != nil {
		return nil, e
	}
	exe, e := lima.Find(d.runtime)
	if e != nil {
		return nil, e
	}
	return &lifecycle.Controller{Root: root, ExpectedID: v.ID, Backend: &lima.Backend{Root: root, Executable: exe}, Preflight: func(s config.Spec, newDisk bool) error { return host.Inspect(root, exe).Validate(s, newDisk) }, Reserve: func(ctx context.Context, s config.Spec) (func(), error) { return d.manager.Reserve(ctx, v, s) }}, nil
}
func (d *localDashboard) List(ctx context.Context) (any, error) {
	entries, e := d.manager.List(ctx)
	if e != nil {
		return nil, e
	}
	limits, e := d.manager.Limits(ctx)
	if e != nil {
		return nil, e
	}
	rows := []map[string]any{}
	for _, v := range entries {
		row := map[string]any{"name": v.Name, "id": v.ID, "state": "not_created", "cpus": 0, "memoryGiB": 0, "allocated": false}
		var state lifecycle.State
		err := storage.Read(v.Root, "state.json", &state)
		if err == nil {
			if state.MachineID != v.ID {
				row["state"] = "identity_changed"
			} else {
				c, e := d.controller(ctx, v.ID)
				if e != nil {
					row["state"] = "unavailable"
				} else {
					s, e := c.Status(ctx)
					if s.MachineID == "" {
						s = state
					}
					row["state"] = s.State
					if e != nil && (s.Observed == "unknown" || s.MachineID == "") {
						row["state"] = "unavailable"
					}
					row["cpus"] = s.Spec.CPUs
					row["memoryGiB"] = s.Spec.MemoryGiB
					row["allocated"] = s.Observed == "Running" || s.Observed == "Starting" || s.Observed == "Stopping"
				}
			}
		} else if !os.IsNotExist(err) {
			row["state"] = "invalid_state"
		}
		rows = append(rows, row)
	}
	return map[string]any{"machines": rows, "budget": limits, "scope": "Managed guest allocations only; excludes unmanaged VMs and other host processes."}, nil
}
func (d *localDashboard) Create(ctx context.Context, name string, cpu, ram, disk int) (string, error) {
	if !fleet.ValidName(name) {
		return "", protocol.Fail("INVALID_ARGUMENT", "Use a lowercase machine name starting with a letter (maximum 32 characters).")
	}
	entries, e := d.manager.List(ctx)
	if e != nil {
		return "", e
	}
	for _, v := range entries {
		if v.Name == name {
			return "", protocol.Fail("MACHINE_NAME_EXISTS", "That name is already registered; start it from the inventory.")
		}
	}
	spec := config.Default()
	spec.Network = "nat"
	spec.CPUs = cpu
	spec.MemoryGiB = ram
	spec.DiskGiB = disk
	spec.GuestBinary = defaultGuestBinary()
	if e = spec.Validate(); e != nil {
		return "", protocol.Fail("INVALID_ARGUMENT", e.Error())
	}
	if e = host.Inspect(d.manager.Root, d.runtime).Validate(spec, true); e != nil {
		return "", protocol.Fail("PREREQUISITES_MISSING", e.Error())
	}
	entry, e := d.manager.Create(ctx, name)
	if e != nil {
		return "", e
	}
	d.mu.Lock()
	d.planned[entry.ID] = spec
	d.mu.Unlock()
	return entry.ID, nil
}
func (d *localDashboard) Start(ctx context.Context, id string) error {
	c, e := d.controller(ctx, id)
	if e != nil {
		return e
	}
	spec := config.Default()
	spec.Network = "nat"
	spec.GuestBinary = defaultGuestBinary()
	d.mu.Lock()
	if planned, ok := d.planned[id]; ok {
		spec = planned
	}
	d.mu.Unlock()
	var state lifecycle.State
	e = storage.Read(c.Root, "state.json", &state)
	creation := os.IsNotExist(e)
	if e != nil && !creation {
		return e
	}
	_, e = c.Up(ctx, spec, creation)
	return e
}
func (d *localDashboard) Stop(ctx context.Context, id string) error {
	c, e := d.controller(ctx, id)
	if e != nil {
		return e
	}
	_, e = c.Stop(ctx)
	return e
}
func (d *localDashboard) Computer(ctx context.Context, id string) (machine.Machine, error) {
	c, e := d.controller(ctx, id)
	if e != nil {
		return nil, e
	}
	s, e := c.Status(ctx)
	if e != nil {
		return nil, e
	}
	if s.State != "ready" {
		return nil, protocol.Fail("NOT_READY", "Start the selected machine and wait for the actual guest handshake.")
	}
	c.ControlOwner = protocol.ID()
	return machine.Local{Controller: c}, nil
}
