// SPDX-License-Identifier: Apache-2.0
package fleet

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

func fixture(t *testing.T) *Manager {
	t.Helper()
	root, e := os.MkdirTemp("/private/tmp", "af-test-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	return &Manager{Root: filepath.Join(root, "catalog"), DefaultRoot: filepath.Join(root, "default"), HostCPUs: 12, HostMemoryGiB: 24, Observe: func(context.Context, Entry) (lifecycle.Observation, error) { return lifecycle.Observation{}, nil }}
}
func TestCatalogNamesPathsIdentityAndRegistration(t *testing.T) {
	m := fixture(t)
	ctx := context.Background()
	for _, name := range []string{"../work", "Work", "", "work/name"} {
		if _, e := m.Resolve(ctx, name, true); e == nil {
			t.Fatal("unsafe name", name)
		}
	}
	a, e := m.Resolve(ctx, "work", true)
	if e != nil {
		t.Fatal(e)
	}
	b, e := m.Resolve(ctx, "work", false)
	if e != nil || a != b {
		t.Fatal("identity not stable", e)
	}
	if filepath.Base(a.Root) == "work" || len(filepath.Base(a.Root)) != 8 {
		t.Fatal("directory derived from user name")
	}
	if _, e = m.Resolve(ctx, "other", false); e == nil {
		t.Fatal("read created unknown machine")
	}
	root, e := storage.Root(a.Root)
	if e != nil {
		t.Fatal(e)
	}
	if e = storage.JSON(root, "state.json", lifecycle.State{Schema: 1, MachineID: a.ID, State: "stopped"}); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Register(ctx, "alias", root); e == nil {
		t.Fatal("same namespace accepted twice")
	}
	if CheckSocketPath("/"+string(make([]byte, 110))) == nil {
		t.Fatal("oversized socket path")
	}
	if e = os.Chmod(filepath.Join(m.Root, "catalog.json"), 0644); e != nil {
		t.Fatal(e)
	}
	if _, e = m.List(ctx); e == nil {
		t.Fatal("public catalog accepted")
	}
}
func TestConcurrentStartupReservationsAndHostReserve(t *testing.T) {
	m := fixture(t)
	ctx := context.Background()
	a, _ := m.Resolve(ctx, "work", true)
	b, _ := m.Resolve(ctx, "research", true)
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, 2)
	releases := make(chan func(), 2)
	for _, entry := range []Entry{a, b} {
		wg.Add(1)
		go func(v Entry) {
			defer wg.Done()
			<-start
			release, e := m.Reserve(ctx, v, config.Spec{CPUs: 6, MemoryGiB: 8})
			if e == nil {
				releases <- release
			}
			results <- e
		}(entry)
	}
	close(start)
	wg.Wait()
	close(results)
	success, denied := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else {
			denied++
		}
	}
	if success != 1 || denied != 1 {
		t.Fatal("oversubscribed pending starts", success, denied)
	}
	(<-releases)()
	if _, e := m.Configure(ctx, Budget{CPUs: 12, MemoryGiB: 17}); e == nil {
		t.Fatal("host RAM reserve bypass")
	}
	if _, e := m.Configure(ctx, Budget{CPUs: 12, MemoryGiB: 16}); e != nil {
		t.Fatal(e)
	}
	// A completed/crashed startup with no VM releases its reservation on reconciliation.
	if release, e := m.Reserve(ctx, b, config.Spec{CPUs: 2, MemoryGiB: 4}); e != nil {
		t.Fatal(e)
	} else {
		release()
	}
}
func TestRunningMachinesCountedAndUnknownRuntimeFailsClosed(t *testing.T) {
	m := fixture(t)
	ctx := context.Background()
	a, _ := m.Resolve(ctx, "work", true)
	b, _ := m.Resolve(ctx, "research", true)
	root, e := storage.Root(a.Root)
	if e != nil {
		t.Fatal(e)
	}
	if e = storage.JSON(root, "state.json", lifecycle.State{Schema: 1, MachineID: a.ID, State: "ready", Spec: config.Spec{CPUs: 8, MemoryGiB: 10}}); e != nil {
		t.Fatal(e)
	}
	m.Observe = func(context.Context, Entry) (lifecycle.Observation, error) {
		return lifecycle.Observation{Exists: true, Status: "Running"}, nil
	}
	if _, e = m.Reserve(ctx, b, config.Spec{CPUs: 4, MemoryGiB: 4}); e == nil {
		t.Fatal("running allocation ignored")
	}
	m.Observe = func(context.Context, Entry) (lifecycle.Observation, error) {
		return lifecycle.Observation{}, errors.New("runtime unavailable")
	}
	if _, e = m.Reserve(ctx, b, config.Spec{CPUs: 1, MemoryGiB: 1}); e == nil {
		t.Fatal("unknown runtime released allocation")
	}
}
