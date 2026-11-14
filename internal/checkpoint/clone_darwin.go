// SPDX-License-Identifier: Apache-2.0
package checkpoint

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
)

func clone(root *os.Root, source *os.File, destination string) (bool, error) {
	dir, e := root.Open(filepath.Dir(destination))
	if e != nil {
		return false, e
	}
	defer dir.Close()
	e = unix.Fclonefileat(int(source.Fd()), int(dir.Fd()), filepath.Base(destination), unix.CLONE_NOFOLLOW)
	if e == nil {
		return true, nil
	}
	if errors.Is(e, unix.ENOTSUP) || errors.Is(e, unix.EXDEV) || errors.Is(e, unix.ENOSYS) || errors.Is(e, unix.EINVAL) {
		return false, nil
	}
	return false, e
}
