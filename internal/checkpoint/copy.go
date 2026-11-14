// SPDX-License-Identifier: Apache-2.0
package checkpoint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const GiB = int64(1 << 30)

var paths = []string{"lima/default/disk", "lima/default/vz-efi", "machine.yaml", "config.sha256", "lima/default/lima.yaml", "state.json"}

func allowed(path string) bool {
	for _, p := range paths {
		if p == path {
			return true
		}
	}
	return false
}
func maxSize(path string) int64 {
	if path == paths[0] {
		return 129 * GiB
	}
	if path == paths[1] {
		return 1 << 20
	}
	return 16 << 20
}
func safeFile(root *os.Root, name string) (*os.File, error) {
	if filepath.IsAbs(name) || filepath.Clean(name) != name || name == "." {
		return nil, errors.New("invalid rooted file path")
	}
	for p := filepath.Dir(name); p != "."; p = filepath.Dir(p) {
		i, e := root.Lstat(p)
		if e != nil {
			return nil, e
		}
		if !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("symlink/non-directory ancestor")
		}
	}
	f, e := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return nil, e
	}
	info, e := f.Stat()
	if e == nil {
		if !info.Mode().IsRegular() {
			e = errors.New("checkpoint file must be regular")
		} else if stat, ok := info.Sys().(*syscall.Stat_t); !ok || stat.Nlink != 1 || int(stat.Uid) != os.Getuid() {
			e = errors.New("unsafe checkpoint file owner or hard link")
		}
	}
	if e != nil {
		f.Close()
		return nil, e
	}
	return f, nil
}
func hashFile(ctx context.Context, f *os.File) (string, error) {
	if _, e := f.Seek(0, 0); e != nil {
		return "", e
	}
	h := sha256.New()
	buf := make([]byte, 1<<20)
	for {
		if e := ctx.Err(); e != nil {
			return "", e
		}
		n, e := f.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", e
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func verify(ctx context.Context, root *os.Root, base string, entry File) error {
	f, e := safeFile(root, filepath.Join(base, entry.Path))
	if e != nil {
		return e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return e
	}
	if info.Size() != entry.Size {
		return errors.New("checkpoint size mismatch")
	}
	digest, e := hashFile(ctx, f)
	if e != nil {
		return e
	}
	if digest != entry.SHA256 {
		return errors.New("checkpoint integrity mismatch")
	}
	return nil
}

// copyFile bounds memory, preserves holes in the portable fallback, and never
// follows a source symlink. Destination names are new files inside an owned root.
func copyFile(ctx context.Context, root *os.Root, source, destination, path string, progress func(string)) (File, error) {
	entry := File{Path: path}
	if !allowed(path) {
		return entry, errors.New("unsupported checkpoint path")
	}
	f, e := safeFile(root, source)
	if e != nil {
		return entry, e
	}
	defer f.Close()
	before, e := f.Stat()
	if e != nil {
		return entry, e
	}
	if before.Size() < 1 || before.Size() > maxSize(path) {
		return entry, errors.New("checkpoint file exceeds size bound")
	}
	entry.Size = before.Size()
	if e = f.Sync(); e != nil {
		return entry, e
	}
	if e = root.MkdirAll(filepath.Dir(destination), 0700); e != nil {
		return entry, e
	}
	if e = safeParents(root, destination); e != nil {
		return entry, e
	}
	if _, e = root.Lstat(destination); !os.IsNotExist(e) {
		return entry, errors.New("checkpoint destination already exists")
	}
	if progress != nil {
		progress(fmt.Sprintf("preparing %s (%d MiB); integrity verification follows", path, before.Size()>>20))
	}
	cloned, e := cloneFile(root, f, destination)
	if e != nil {
		return entry, e
	}
	if !cloned {
		dir, e := root.Open(filepath.Dir(destination))
		if e != nil {
			return entry, e
		}
		available, e := availableSpace(dir)
		dir.Close()
		if e != nil {
			return entry, e
		}
		// Conservative bound: sparse allocation can be smaller than virtual size.
		if available < uint64(before.Size()+(64<<20)) {
			return entry, errors.New("insufficient free space for bounded sparse copy plus 64 MiB headroom")
		}
		out, e := root.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
		if e != nil {
			return entry, e
		}
		good := false
		defer func() {
			out.Close()
			if !good {
				_ = root.Remove(destination)
			}
		}()
		buf, zeros := make([]byte, 1<<20), make([]byte, 1<<20)
		var copied int64
		last := time.Now()
		for {
			if e = ctx.Err(); e != nil {
				return entry, e
			}
			n, readErr := f.Read(buf)
			if n > 0 {
				if bytes.Equal(buf[:n], zeros[:n]) {
					_, e = out.Seek(int64(n), io.SeekCurrent)
				} else {
					var written int
					written, e = out.Write(buf[:n])
					if written != n && e == nil {
						e = io.ErrShortWrite
					}
				}
				if e != nil {
					return entry, e
				}
				copied += int64(n)
				if progress != nil && time.Since(last) > time.Second {
					progress(fmt.Sprintf("copying %s: %d / %d MiB", path, copied>>20, before.Size()>>20))
					last = time.Now()
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return entry, readErr
			}
		}
		if e = out.Truncate(before.Size()); e != nil {
			return entry, e
		}
		if e = out.Sync(); e != nil {
			return entry, e
		}
		good = true
		entry.Copy = "sparse-copy"
	} else {
		entry.Copy = "apfs-clone"
	}
	dest, e := safeFile(root, destination)
	if e != nil {
		return entry, e
	}
	defer dest.Close()
	if e = dest.Chmod(0600); e != nil {
		return entry, e
	}
	if e = dest.Sync(); e != nil {
		return entry, e
	}
	entry.SHA256, e = hashFile(ctx, dest)
	if e != nil {
		return entry, e
	}
	after, e := f.Stat()
	if e != nil {
		return entry, e
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return entry, errors.New("source changed during stopped-VM copy")
	}
	if e = syncAncestors(root, filepath.Dir(destination)); e != nil {
		return entry, e
	}
	return entry, nil
}
func syncDir(root *os.Root, path string) error {
	f, e := root.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}

func safeParents(root *os.Root, name string) error {
	for p := filepath.Dir(name); p != "."; p = filepath.Dir(p) {
		i, e := root.Lstat(p)
		if e != nil {
			return e
		}
		if !i.IsDir() || i.Mode()&os.ModeSymlink != 0 || i.Mode().Perm()&0077 != 0 {
			return errors.New("unsafe checkpoint directory permissions/path")
		}
	}
	return nil
}
func syncAncestors(root *os.Root, path string) error {
	for {
		if e := syncDir(root, path); e != nil {
			return e
		}
		if path == "." {
			return nil
		}
		path = filepath.Dir(path)
	}
}

var cloneFile = clone
var availableSpace = func(directory *os.File) (uint64, error) {
	var st syscall.Statfs_t
	if e := syscall.Fstatfs(int(directory.Fd()), &st); e != nil {
		return 0, e
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
