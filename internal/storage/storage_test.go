// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func owned(t *testing.T) string {
	t.Helper()
	p, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(p, 0700); e != nil {
		t.Fatal(e)
	}
	r, e := Root(p)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestLockAndAtomicState(t *testing.T) {
	r := owned(t)
	unlock, e := Lock(r)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Lock(r); e == nil {
		t.Fatal("concurrent lock accepted")
	}
	unlock()
	unlock, e = Lock(r)
	if e != nil {
		t.Fatal(e)
	}
	defer unlock()
	if e = JSON(r, "state.json", map[string]int{"a": 1}); e != nil {
		t.Fatal(e)
	}
	var s map[string]int
	if e = Read(r, "state.json", &s); e != nil || s["a"] != 1 {
		t.Fatalf("state not persisted: %v", e)
	}
}
func TestRejectUnsafePaths(t *testing.T) {
	r := owned(t)
	target := filepath.Join(r, "outside")
	if e := os.WriteFile(target, []byte("keep"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(target, filepath.Join(r, "state.json")); e != nil {
		t.Fatal(e)
	}
	if Write(r, "state.json", []byte("replace")) == nil {
		t.Fatal("symlink overwritten")
	}
	b, _ := os.ReadFile(target)
	if string(b) != "keep" {
		t.Fatal("user file changed")
	}
	if _, e := Root(filepath.Join(r, "..")); e == nil {
		t.Fatal("unclean root accepted")
	}
	if e := Write(r, "../escape", nil); e == nil {
		t.Fatal("path traversal accepted")
	}
	if _, e := Root(target); e == nil {
		t.Fatal("file root accepted")
	}
}
func TestRejectUnownedDirectory(t *testing.T) {
	r, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	_ = os.Chmod(r, 0700)
	_ = os.WriteFile(filepath.Join(r, "user-data"), []byte("keep"), 0600)
	if _, e = Root(r); e == nil {
		t.Fatal("claimed unrelated directory")
	}
}

func TestBoundedLockWaitAndCancellation(t *testing.T) {
	r := t.TempDir()
	unlock, e := Lock(r)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = LockWait(ctx, r, time.Second); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e = LockWait(context.Background(), r, 20*time.Millisecond); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	unlock()
	release, e := LockWait(context.Background(), r, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	release()
}
