// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Root is a private application directory, not an arbitrary existing data directory.
func Root(path string) (string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return "", fmt.Errorf("state directory must be an absolute clean application path")
	}
	home, _ := os.UserHomeDir()
	if path == home {
		return "", errors.New("refusing to use home itself as state directory")
	}
	// Reject symlink components including ancestor substitutions. /tmp is resolved by callers first.
	for p := path; p != "/"; p = filepath.Dir(p) {
		i, e := os.Lstat(p)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return "", e
		}
		if i.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink in state path: %s", p)
		}
		if !i.IsDir() {
			return "", errors.New("non-directory state path")
		}
	}
	if i, e := os.Stat(path); e == nil {
		entries, e := os.ReadDir(path)
		if e != nil {
			return "", e
		}
		if _, e = os.Stat(filepath.Join(path, ".agent-linux-owner")); e != nil && len(entries) > 0 {
			return "", errors.New("refusing nonempty directory without application ownership marker")
		}
		if st, ok := i.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
			return "", errors.New("state directory belongs to another user")
		}
		if i.Mode().Perm()&0077 != 0 {
			return "", errors.New("state directory must have mode 0700")
		}
	}
	if e := os.MkdirAll(path, 0700); e != nil {
		return "", e
	}
	if e := Write(path, ".agent-linux-owner", []byte("agent-linux state v1\n")); e != nil {
		return "", e
	}
	return path, nil
}
func SafeFile(root, name string) error {
	if filepath.Base(name) != name {
		return errors.New("invalid state filename")
	}
	i, e := os.Lstat(filepath.Join(root, name))
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if !i.Mode().IsRegular() {
		return errors.New("state file must be regular, never symlink")
	}
	if i.Mode().Perm()&0077 != 0 {
		return errors.New("state file permissions must be 0600")
	}
	if st, ok := i.Sys().(*syscall.Stat_t); ok && (st.Nlink != 1 || int(st.Uid) != os.Getuid()) {
		return errors.New("state file has unsafe owner/link count")
	}
	return nil
}
func Write(root, name string, b []byte) error {
	if e := SafeFile(root, name); e != nil {
		return e
	}
	f, e := os.CreateTemp(root, ".write-")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return os.Rename(f.Name(), filepath.Join(root, name))
}
func JSON(root, name string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return Write(root, name, append(b, '\n'))
}
func Read(root, name string, v any) error {
	if e := SafeFile(root, name); e != nil {
		return e
	}
	f, e := os.Open(filepath.Join(root, name))
	if e != nil {
		return e
	}
	defer f.Close()
	i, e := f.Stat()
	if e != nil {
		return e
	}
	if i.Size() > 1<<20 {
		return errors.New("oversized state file")
	}
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	var trailing any
	if e := d.Decode(&trailing); e != io.EOF {
		return errors.New("unexpected trailing state data")
	}
	return nil
}

var ErrBusy = errors.New("MACHINE_BUSY: another process controls this machine; retry after it finishes")

func Lock(root string) (func(), error) {
	if e := SafeFile(root, "lock"); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(filepath.Join(root, "lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, ErrBusy
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}

// LockWait bounds contention without queuing a request past its caller deadline.
func LockWait(ctx context.Context, root string, limit time.Duration) (func(), error) {
	deadline := time.NewTimer(limit)
	defer deadline.Stop()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		unlock, e := Lock(root)
		if !errors.Is(e, ErrBusy) {
			return unlock, e
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, ErrBusy
		case <-tick.C:
		}
	}
}

// ExistingRoot never creates or adopts a path. Destructive/copy operations require
// an existing, private application namespace and its exact regular ownership marker.
func ExistingRoot(path string) (string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return "", errors.New("expected existing absolute application state path")
	}
	home, _ := os.UserHomeDir()
	if path == home {
		return "", errors.New("refusing home directory")
	}
	for p := path; p != "/"; p = filepath.Dir(p) {
		st, e := os.Lstat(p)
		if e != nil {
			return "", e
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("state path contains symlink or non-directory")
		}
	}
	st, e := os.Stat(path)
	if e != nil {
		return "", e
	}
	if st.Mode().Perm()&0077 != 0 {
		return "", errors.New("state directory must be private (0700)")
	}
	if v, ok := st.Sys().(*syscall.Stat_t); ok && int(v.Uid) != os.Getuid() {
		return "", errors.New("state belongs to another user")
	}
	if e = SafeFile(path, ".agent-linux-owner"); e != nil {
		return "", e
	}
	b, e := os.ReadFile(filepath.Join(path, ".agent-linux-owner"))
	if e != nil {
		return "", e
	}
	if string(b) != "agent-linux state v1\n" {
		return "", errors.New("invalid application ownership marker")
	}
	return path, nil
}
