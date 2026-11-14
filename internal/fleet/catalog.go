// SPDX-License-Identifier: Apache-2.0
// Package fleet owns a private name catalog; disk paths are never inferred from names.
package fleet

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"syscall"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

type Entry struct {
	Name string `json:"name"`
	ID   string `json:"machineId"`
	Root string `json:"stateDirectory"`
}
type Budget struct {
	CPUs      int `json:"cpus"`
	MemoryGiB int `json:"memoryGiB"`
}
type reservation struct {
	ID        string `json:"machineId"`
	CPUs      int    `json:"cpus"`
	MemoryGiB int    `json:"memoryGiB"`
	PID       int    `json:"pendingPid"`
}
type catalog struct {
	Version      int           `json:"version"`
	Entries      []Entry       `json:"machines"`
	Budget       Budget        `json:"budget"`
	Reservations []reservation `json:"reservations"`
}
type Manager struct {
	Root          string
	DefaultRoot   string
	HostCPUs      int
	HostMemoryGiB int
	Observe       func(context.Context, Entry) (lifecycle.Observation, error)
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

func ValidName(s string) bool { return namePattern.MatchString(s) }
func CheckSocketPath(root string) error {
	if len(filepath.Join(root, "lima", "default", "ssh.sock")) >= 100 {
		return errors.New("machine namespace is too long for Lima Unix sockets; use a shorter owned path")
	}
	return nil
}
func (m *Manager) load() (catalog, error) {
	var c catalog
	e := storage.Read(m.Root, "catalog.json", &c)
	if os.IsNotExist(e) {
		return catalog{Version: 1}, nil
	}
	if e != nil {
		return c, e
	}
	if c.Version != 1 || len(c.Entries) > 64 || len(c.Reservations) > 64 {
		return c, errors.New("unsupported or oversized machine catalog")
	}
	names, ids, roots := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, v := range c.Entries {
		if !ValidName(v.Name) || !protocol.ValidID(v.ID) || !filepath.IsAbs(v.Root) || filepath.Clean(v.Root) != v.Root || names[v.Name] || ids[v.ID] || roots[v.Root] {
			return c, errors.New("invalid/duplicate catalog identity or path")
		}
		if err := CheckSocketPath(v.Root); err != nil {
			return c, err
		}
		names[v.Name] = true
		ids[v.ID] = true
		roots[v.Root] = true
	}
	for _, r := range c.Reservations {
		if !ids[r.ID] || r.CPUs < 1 || r.MemoryGiB < 1 || r.PID < 0 {
			return c, errors.New("invalid resource reservation")
		}
	}
	return c, nil
}
func (m *Manager) lock(ctx context.Context) (func(), error) {
	if _, err := storage.Root(m.Root); err != nil {
		return nil, err
	}
	return storage.LockWait(ctx, m.Root, 2*time.Second)
}
func (m *Manager) List(ctx context.Context) ([]Entry, error) {
	unlock, e := m.lock(ctx)
	if e != nil {
		return nil, e
	}
	defer unlock()
	c, e := m.load()
	if e != nil {
		return nil, e
	}
	sort.Slice(c.Entries, func(i, j int) bool { return c.Entries[i].Name < c.Entries[j].Name })
	return c.Entries, nil
}

// Resolve creates a catalog identity only when explicitly allowed. Existing custom
// namespaces must be registered, not moved or silently adopted by another name.
func (m *Manager) Resolve(ctx context.Context, name string, create bool) (Entry, error) {
	return m.resolve(ctx, name, "", create, false)
}
func (m *Manager) Register(ctx context.Context, name, root string) (Entry, error) {
	if _, e := storage.ExistingRoot(root); e != nil {
		return Entry{}, e
	}
	var s lifecycle.State
	if e := storage.Read(root, "state.json", &s); e != nil || !protocol.ValidID(s.MachineID) {
		return Entry{}, errors.New("registration requires an existing machine identity")
	}
	return m.resolve(ctx, name, root, true, false)
}
func (m *Manager) resolve(ctx context.Context, name, registered string, create bool, newOnly bool) (Entry, error) {
	if !ValidName(name) {
		return Entry{}, errors.New("machine name must be 1–32 lowercase letters/digits/hyphens, starting with a letter")
	}
	unlock, e := m.lock(ctx)
	if e != nil {
		return Entry{}, e
	}
	defer unlock()
	c, e := m.load()
	if e != nil {
		return Entry{}, e
	}
	for i, v := range c.Entries {
		if v.Name == name {
			if newOnly {
				return Entry{}, protocol.Fail("MACHINE_NAME_EXISTS", "Name already registered; use its Start action.")
			}
			if registered != "" && v.Root != registered {
				return Entry{}, errors.New("name already belongs to another namespace")
			}
			if registered != "" {
				var current lifecycle.State
				if e := storage.Read(registered, "state.json", &current); e != nil {
					return Entry{}, e
				}
				if current.MachineID != v.ID {
					for _, other := range c.Entries {
						if other.Name != name && other.ID == current.MachineID {
							return Entry{}, errors.New("replacement identity already registered")
						}
					}
					oldID := v.ID
					v.ID = current.MachineID
					c.Entries[i] = v
					kept := c.Reservations[:0]
					for _, r := range c.Reservations {
						if r.ID != oldID {
							kept = append(kept, r)
						}
					}
					c.Reservations = kept
					return v, storage.JSON(m.Root, "catalog.json", c)
				}
			}
			return v, nil
		}
	}
	if !create {
		return Entry{}, protocol.Fail("MACHINE_NOT_FOUND", "Unknown machine; use start --machine NAME or machines register.")
	}
	if len(c.Entries) >= 64 {
		return Entry{}, errors.New("catalog limit is 64 machines")
	}
	v := Entry{Name: name, ID: protocol.ID()}
	if registered != "" {
		v.Root = registered
	} else if name == "default" {
		v.Root = m.DefaultRoot
	} else {
		v.Root = filepath.Join(filepath.Dir(m.Root), "m", protocol.ID()[:8])
	}
	if newOnly {
		if _, e := os.Lstat(v.Root); e == nil {
			return Entry{}, errors.New("namespace already exists; explicitly register it instead")
		}
	}
	if e = CheckSocketPath(v.Root); e != nil {
		return Entry{}, e
	}
	var state lifecycle.State
	if e = storage.Read(v.Root, "state.json", &state); e == nil {
		if _, e = storage.ExistingRoot(v.Root); e != nil {
			return Entry{}, e
		}
		if !protocol.ValidID(state.MachineID) {
			return Entry{}, errors.New("existing namespace identity invalid")
		}
		v.ID = state.MachineID
	} else if !os.IsNotExist(e) {
		return Entry{}, e
	}
	for _, other := range c.Entries {
		if other.Root == v.Root || other.ID == v.ID {
			return Entry{}, errors.New("namespace or identity already registered")
		}
	}
	c.Entries = append(c.Entries, v)
	return v, storage.JSON(m.Root, "catalog.json", c)
}
func (m *Manager) limits(b Budget) (Budget, error) {
	if m.HostCPUs < 1 || m.HostMemoryGiB < 1 {
		return b, errors.New("cannot establish host resource limits")
	}
	if b.CPUs == 0 {
		b.CPUs = max(1, m.HostCPUs-2)
	}
	if b.MemoryGiB == 0 {
		b.MemoryGiB = m.HostMemoryGiB / 2
	}
	reserve := max(4, (m.HostMemoryGiB+2)/3)
	if b.CPUs < 1 || b.CPUs > m.HostCPUs || b.MemoryGiB < 1 || b.MemoryGiB > m.HostMemoryGiB-reserve {
		return b, errors.New("budget exceeds host limits or required host RAM reserve")
	}
	return b, nil
}
func (m *Manager) Configure(ctx context.Context, b Budget) (Budget, error) {
	b, e := m.limits(b)
	if e != nil {
		return b, e
	}
	unlock, e := m.lock(ctx)
	if e != nil {
		return b, e
	}
	defer unlock()
	c, e := m.load()
	if e != nil {
		return b, e
	}
	c.Budget = b
	return b, storage.JSON(m.Root, "catalog.json", c)
}
func (m *Manager) Limits(ctx context.Context) (Budget, error) {
	unlock, e := m.lock(ctx)
	if e != nil {
		return Budget{}, e
	}
	defer unlock()
	c, e := m.load()
	if e != nil {
		return Budget{}, e
	}
	return m.limits(c.Budget)
}

// Reserve serializes admission across processes, including startup before a VM
// exists. Crashed holders remain charged if runtime state cannot be established.
func (m *Manager) Reserve(ctx context.Context, selected Entry, spec config.Spec) (func(), error) {
	unlock, e := m.lock(ctx)
	if e != nil {
		return nil, e
	}
	defer unlock()
	c, e := m.load()
	if e != nil {
		return nil, e
	}
	limit, e := m.limits(c.Budget)
	if e != nil {
		return nil, e
	}
	if spec.CPUs < 1 || spec.MemoryGiB < 1 {
		return nil, errors.New("invalid reservation resources")
	}
	validSelection := false
	for _, entry := range c.Entries {
		if entry == selected {
			validSelection = true
		}
	}
	if !validSelection {
		return nil, errors.New("reservation selection does not match catalog")
	}
	cpu, ram := 0, 0
	for _, entry := range c.Entries {
		if entry.ID == selected.ID {
			continue
		}
		var held *reservation
		for i := range c.Reservations {
			if c.Reservations[i].ID == entry.ID {
				held = &c.Reservations[i]
				break
			}
		}
		pending := held != nil && held.PID > 0 && syscall.Kill(held.PID, 0) == nil
		if pending {
			cpu += held.CPUs
			ram += held.MemoryGiB
			continue
		}
		if _, err := os.Lstat(entry.Root); err == nil {
			if _, err = storage.ExistingRoot(entry.Root); err != nil {
				return nil, err
			}
		}
		var state lifecycle.State
		err := storage.Read(entry.Root, "state.json", &state)
		if os.IsNotExist(err) {
			if held == nil {
				continue
			}
			if m.Observe == nil {
				return nil, errors.New("runtime reconciliation unavailable")
			}
			o, e := m.Observe(ctx, entry)
			if e != nil || o.Exists {
				return nil, errors.New("orphaned startup requires explicit runtime inspection")
			}
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("cannot reconcile machine %s; resources fail closed", entry.Name)
		}
		if state.MachineID != entry.ID {
			return nil, errors.New("catalog identity changed; explicit re-registration required")
		}
		if m.Observe == nil {
			return nil, errors.New("runtime reconciliation unavailable")
		}
		observed, err := m.Observe(ctx, entry)
		if err != nil {
			return nil, fmt.Errorf("cannot inspect machine %s; resources fail closed", entry.Name)
		}
		if observed.Exists && observed.Status != "Stopped" {
			cpu += state.Spec.CPUs
			ram += state.Spec.MemoryGiB
		}
	}
	if cpu+spec.CPUs > limit.CPUs || ram+spec.MemoryGiB > limit.MemoryGiB {
		return nil, protocol.Fail("RESOURCE_BUDGET_EXCEEDED", fmt.Sprintf("Managed allocation would be %d/%d vCPU and %d/%d GiB RAM; stop a machine or explicitly configure the fleet budget.", cpu+spec.CPUs, limit.CPUs, ram+spec.MemoryGiB, limit.MemoryGiB))
	}
	found := false
	for i := range c.Reservations {
		if c.Reservations[i].ID == selected.ID {
			c.Reservations[i] = reservation{selected.ID, spec.CPUs, spec.MemoryGiB, os.Getpid()}
			found = true
		}
	}
	if !found {
		c.Reservations = append(c.Reservations, reservation{selected.ID, spec.CPUs, spec.MemoryGiB, os.Getpid()})
	}
	if e = storage.JSON(m.Root, "catalog.json", c); e != nil {
		return nil, e
	}
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		u, err := m.lock(ctx)
		if err != nil {
			return
		}
		defer u()
		latest, err := m.load()
		if err != nil {
			return
		}
		for i := range latest.Reservations {
			if latest.Reservations[i].ID == selected.ID && latest.Reservations[i].PID == os.Getpid() {
				latest.Reservations[i].PID = 0
			}
		}
		_ = storage.JSON(m.Root, "catalog.json", latest)
	}, nil
}

// Rebind is used only after an explicitly confirmed reset/deletion. Already-open
// clients retain their old ID and cannot follow the replacement.
func (m *Manager) Rebind(ctx context.Context, old Entry, id string) error {
	if !protocol.ValidID(id) {
		return errors.New("invalid replacement identity")
	}
	unlock, e := m.lock(ctx)
	if e != nil {
		return e
	}
	defer unlock()
	c, e := m.load()
	if e != nil {
		return e
	}
	found := false
	for i := range c.Entries {
		if c.Entries[i] == old {
			c.Entries[i].ID = id
			found = true
		}
	}
	if !found {
		return errors.New("catalog changed during replacement")
	}
	kept := c.Reservations[:0]
	for _, r := range c.Reservations {
		if r.ID != old.ID {
			kept = append(kept, r)
		}
	}
	c.Reservations = kept
	return storage.JSON(m.Root, "catalog.json", c)
}

func (m *Manager) Create(ctx context.Context, name string) (Entry, error) {
	return m.resolve(ctx, name, "", true, true)
}

// ReadEntries uses an atomic catalog snapshot and does not acquire a writer lock
// or create state. Lifecycle identity checks and admission revalidate mutations.
func (m *Manager) ReadEntries() ([]Entry, error) {
	if _, e := storage.ExistingRoot(m.Root); e != nil {
		return nil, e
	}
	c, e := m.load()
	return c.Entries, e
}
func (m *Manager) Lookup(name string) (Entry, error) {
	if !ValidName(name) {
		return Entry{}, errors.New("invalid machine name")
	}
	entries, e := m.ReadEntries()
	if e != nil {
		return Entry{}, e
	}
	for _, v := range entries {
		if v.Name == name {
			return v, nil
		}
	}
	return Entry{}, protocol.Fail("MACHINE_NOT_FOUND", "Unknown machine; use start --machine NAME or machines register.")
}
