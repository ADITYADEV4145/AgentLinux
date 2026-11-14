// SPDX-License-Identifier: Apache-2.0
package checkpoint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

func fixture(t *testing.T) Manager {
	t.Helper()
	base, e := filepath.EvalSymlinks("/tmp")
	if e != nil {
		t.Fatal(e)
	}
	root, e := os.MkdirTemp(base, "acp-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if _, e = storage.Root(root); e != nil {
		t.Fatal(e)
	}
	for _, path := range paths {
		p := filepath.Join(root, path)
		if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			t.Fatal(e)
		}
		data := []byte("original " + path)
		if path == paths[0] {
			data = make([]byte, 4<<20)
			copy(data, "original disk")
			data[len(data)-1] = 42
		}
		if e = os.WriteFile(p, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	return Manager{Root: root, MachineID: protocol.ID(), Compatibility: strings.Repeat("a", 64)}
}
func mutate(t *testing.T, m Manager) {
	t.Helper()
	for _, path := range paths {
		if e := os.WriteFile(filepath.Join(m.Root, path), []byte("changed "+path), 0600); e != nil {
			t.Fatal(e)
		}
	}
}
func read(t *testing.T, m Manager, path string) string {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(m.Root, path))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func TestRestoreRollbackAndConfirmation(t *testing.T) {
	m := fixture(t)
	ctx := context.Background()
	original := read(t, m, paths[0])
	manifest, e := m.Create(ctx, "before-update")
	if e != nil {
		t.Fatal(e)
	}
	if len(manifest.Files) != len(paths) || !manifest.Complete {
		t.Fatal("incomplete checkpoint")
	}
	mutate(t, m)
	if _, e = m.Restore(ctx, "before-update"); e != nil {
		t.Fatal(e)
	}
	if read(t, m, paths[0]) != original {
		t.Fatal("wrong disk restored")
	}
	j, e := m.Pending()
	if e != nil || j.Phase != "awaiting_ready" {
		t.Fatal(j, e)
	}
	if _, e = m.Create(ctx, "another"); e == nil {
		t.Fatal("checkpoint created during pending recovery")
	}
	if e = m.Recover(ctx, true); e != nil {
		t.Fatal(e)
	}
	if read(t, m, paths[0]) != "changed "+paths[0] {
		t.Fatal("rollback did not preserve pre-restore disk")
	}
	if e = m.Confirm(); e != nil {
		t.Fatal(e)
	}
	if j, e = m.Pending(); e != nil || j != nil {
		t.Fatal("journal retained after verified confirmation", e)
	}
	if e = m.Delete("before-update"); e != nil {
		t.Fatal(e)
	}
	list, e := m.List()
	if e != nil || len(list) != 0 {
		t.Fatal("empty list", e)
	}
}
func TestInterruptedInstallResumesOrRollsBack(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		t.Run(map[bool]string{false: "resume", true: "rollback"}[rollback], func(t *testing.T) {
			m := fixture(t)
			ctx := context.Background()
			original := read(t, m, paths[0])
			if _, e := m.Create(ctx, "safe"); e != nil {
				t.Fatal(e)
			}
			mutate(t, m)
			m.Fault = func(phase string) error {
				if phase == "installed:"+paths[0] {
					return errors.New("power loss")
				}
				return nil
			}
			if _, e := m.Restore(ctx, "safe"); e == nil {
				t.Fatal("fault not injected")
			}
			j, e := m.Pending()
			if e != nil || j.Phase != "installing" {
				t.Fatal(j, e)
			}
			m.Fault = nil
			if e = m.Recover(ctx, rollback); e != nil {
				t.Fatal(e)
			}
			want := original
			if rollback {
				want = "changed " + paths[0]
			}
			if read(t, m, paths[0]) != want {
				t.Fatal("recovery wrong disk")
			}
		})
	}
}
func TestStagingCancellationIntegrityAndOrigin(t *testing.T) {
	m := fixture(t)
	ctx := context.Background()
	if _, e := m.Create(ctx, "safe"); e != nil {
		t.Fatal(e)
	}
	other := m
	other.MachineID = protocol.ID()
	if _, e := other.List(); e == nil {
		t.Fatal("cross-machine checkpoint accepted")
	}
	m.Fault = func(phase string) error {
		if phase == "staging" {
			return errors.New("interrupted staging")
		}
		return nil
	}
	before := read(t, m, paths[0])
	if _, e := m.Restore(ctx, "safe"); e == nil {
		t.Fatal("fault not injected")
	}
	m.Fault = nil
	if e := m.Recover(ctx, true); e != nil {
		t.Fatal(e)
	}
	if read(t, m, paths[0]) != before {
		t.Fatal("staging altered original")
	}
	p := filepath.Join(m.Root, "checkpoints/safe/files", paths[0])
	if e := os.WriteFile(p, []byte("corrupted"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := m.Restore(ctx, "safe"); e == nil {
		t.Fatal("corruption ignored")
	}
	if j, e := m.Pending(); e != nil || j != nil {
		t.Fatal("corrupt checkpoint began restore")
	}
}
func TestLimitsUnsafePathsHardlinksAndIncompleteCleanup(t *testing.T) {
	m := fixture(t)
	ctx := context.Background()
	for _, name := range []string{"../other", "", "/tmp", "two/parts"} {
		if _, e := m.Create(ctx, name); e == nil {
			t.Fatal("unsafe name", name)
		}
	}
	disk := filepath.Join(m.Root, paths[0])
	if e := os.Link(disk, filepath.Join(m.Root, "alias")); e != nil {
		t.Fatal(e)
	}
	if _, e := m.Create(ctx, "hardlink"); e == nil {
		t.Fatal("hardlink accepted")
	}
	if e := os.Remove(filepath.Join(m.Root, "alias")); e != nil {
		t.Fatal(e)
	}
	if e := m.Delete("hardlink"); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"one", "two", "three"} {
		if _, e := m.Create(ctx, name); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := m.Create(ctx, "four"); e == nil {
		t.Fatal("limit ignored")
	}
	p := filepath.Join(m.Root, "checkpoints/one/files", paths[1])
	if e := os.Remove(p); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(filepath.Join(m.Root, paths[1]), p); e != nil {
		t.Fatal(e)
	}
	if e := m.Delete("one"); e == nil {
		t.Fatal("symlink deletion accepted")
	}
}
func TestCanceledCreationIsListedAndExplicitlyDeleted(t *testing.T) {
	m := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := m.Create(ctx, "canceled"); e == nil {
		t.Fatal("cancellation ignored")
	}
	list, e := m.List()
	if e != nil || len(list) != 1 || list[0].Status != "incomplete" {
		t.Fatal(list, e)
	}
	if e = m.Delete("canceled"); e != nil {
		t.Fatal(e)
	}
}

func TestSparseFallbackAndInsufficientSpace(t *testing.T) {
	originalClone, originalSpace := cloneFile, availableSpace
	defer func() { cloneFile = originalClone; availableSpace = originalSpace }()
	cloneFile = func(*os.Root, *os.File, string) (bool, error) { return false, nil }
	availableSpace = func(*os.File) (uint64, error) { return 0, nil }
	m := fixture(t)
	ctx := context.Background()
	if _, e := m.Create(ctx, "full"); e == nil || !strings.Contains(e.Error(), "insufficient free space") {
		t.Fatal("space limit not enforced", e)
	}
	if e := m.Delete("full"); e != nil {
		t.Fatal(e)
	}
	availableSpace = func(*os.File) (uint64, error) { return 256 << 20, nil }
	saved, e := m.Create(ctx, "sparse")
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range saved.Files {
		if f.Copy != "sparse-copy" {
			t.Fatal("fallback not exercised")
		}
	}
	data, e := os.ReadFile(filepath.Join(m.Root, "checkpoints/sparse/files", paths[0]))
	if e != nil {
		t.Fatal(e)
	}
	if len(data) != 4<<20 || data[len(data)-1] != 42 {
		t.Fatal("sparse copy changed data or length")
	}
}

func TestMalformedManifestsNeverBeginInstallation(t *testing.T) {
	for _, variant := range []string{"version", "path", "duplicate", "permissions", "trailing"} {
		t.Run(variant, func(t *testing.T) {
			m := fixture(t)
			ctx := context.Background()
			if _, e := m.Create(ctx, "safe"); e != nil {
				t.Fatal(e)
			}
			p := filepath.Join(m.Root, "checkpoints/safe/manifest.json")
			data, e := os.ReadFile(p)
			if e != nil {
				t.Fatal(e)
			}
			var manifest Manifest
			if e = json.Unmarshal(data, &manifest); e != nil {
				t.Fatal(e)
			}
			switch variant {
			case "version":
				manifest.Version = 99
			case "path":
				manifest.Files[0].Path = "../../unrelated"
			case "duplicate":
				manifest.Files[1] = manifest.Files[0]
			}
			data, e = json.Marshal(manifest)
			if e != nil {
				t.Fatal(e)
			}
			if variant == "trailing" {
				data = append(data, []byte(" {}")...)
			}
			if e = os.WriteFile(p, data, 0600); e != nil {
				t.Fatal(e)
			}
			if variant == "permissions" {
				if e = os.Chmod(p, 0644); e != nil {
					t.Fatal(e)
				}
			}
			original := read(t, m, paths[0])
			if _, e = m.Restore(ctx, "safe"); e == nil {
				t.Fatal("malformed checkpoint accepted")
			}
			if read(t, m, paths[0]) != original {
				t.Fatal("live disk changed")
			}
			if _, e = os.Stat(filepath.Join(m.Root, Recovery)); !os.IsNotExist(e) {
				t.Fatal("unsafe installation began")
			}
		})
	}
}
func TestCheckpointDirectorySymlinkCannotReachAnotherMachine(t *testing.T) {
	m, other := fixture(t), fixture(t)
	before := read(t, other, paths[0])
	if e := os.Symlink(other.Root, filepath.Join(m.Root, "checkpoints")); e != nil {
		t.Fatal(e)
	}
	if _, e := m.Create(context.Background(), "unsafe"); e == nil {
		t.Fatal("external namespace followed")
	}
	if read(t, other, paths[0]) != before {
		t.Fatal("another machine modified")
	}
}

func TestInterruptedConfirmationCleanupHasDurableReceipt(t *testing.T) {
	for _, phase := range []string{"confirmed", "cleanup:recovery"} {
		t.Run(phase, func(t *testing.T) {
			m := fixture(t)
			ctx := context.Background()
			if _, e := m.Create(ctx, "safe"); e != nil {
				t.Fatal(e)
			}
			if _, e := m.Restore(ctx, "safe"); e != nil {
				t.Fatal(e)
			}
			m.Fault = func(point string) error {
				if point == phase {
					return errors.New("power loss during cleanup")
				}
				return nil
			}
			if e := m.Confirm(); e == nil {
				t.Fatal("fault not injected")
			}
			m.Fault = nil
			j, e := m.Pending()
			if e != nil || j.Phase != "confirmed_cleanup" {
				t.Fatal(j, e)
			}
			if e = m.Recover(ctx, false); e != nil {
				t.Fatal(e)
			}
			if j, e = m.Pending(); e != nil || j != nil {
				t.Fatal("cleanup receipt not recovered", j, e)
			}
		})
	}
}
func TestInterruptedCreationBeforeManifestCanBeExplicitlyCleaned(t *testing.T) {
	m := fixture(t)
	name := ".creating-" + protocol.ID()
	if e := os.MkdirAll(filepath.Join(m.Root, "checkpoints", name), 0700); e != nil {
		t.Fatal(e)
	}
	list, e := m.List()
	if e != nil || len(list) != 1 || list[0].Status != "incomplete" {
		t.Fatal(list, e)
	}
	if e = m.Delete(list[0].Name); e != nil {
		t.Fatal(e)
	}
}

func TestUnknownFilesAndIncompatibleRuntimeFailClosed(t *testing.T) {
	m := fixture(t)
	ctx := context.Background()
	if _, e := m.Create(ctx, "safe"); e != nil {
		t.Fatal(e)
	}
	other := m
	other.Compatibility = strings.Repeat("b", 64)
	if _, e := other.Restore(ctx, "safe"); e == nil {
		t.Fatal("incompatible runtime accepted")
	}
	extra := filepath.Join(m.Root, "checkpoints/safe/personal.txt")
	if e := os.WriteFile(extra, []byte("keep"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := m.Delete("safe"); e == nil {
		t.Fatal("unknown owner file removed")
	}
	if _, e := os.Stat(extra); e != nil {
		t.Fatal("unrelated file touched")
	}
}
func TestOversizedConfigurationRefusedBeforeCopy(t *testing.T) {
	m := fixture(t)
	if e := os.Truncate(filepath.Join(m.Root, "machine.yaml"), maxSize("machine.yaml")+1); e != nil {
		t.Fatal(e)
	}
	if _, e := m.Create(context.Background(), "large"); e == nil || !strings.Contains(e.Error(), "size bound") {
		t.Fatal("oversized file copied", e)
	}
}
