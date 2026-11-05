// SPDX-License-Identifier: Apache-2.0

package lima

import (
	"encoding/binary"
	"errors"
	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"io"
	"os"
)

// inspectImage validates only the documented QCOW2 header, never disk contents.
// Lima's existing converter handles the actual image. Reject host-file references.
func inspectImage(path string, diskGiB int) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return e
	}
	if st.Size() < 104 || st.Size() > 4<<30 {
		return errors.New("QCOW2 image must be 104 bytes..4 GiB")
	}
	var h [104]byte
	if _, e = io.ReadFull(f, h[:]); e != nil {
		return e
	}
	if string(h[:4]) != "QFI\xfb" {
		return errors.New("only standalone QCOW2 cloud images are supported in Phase 1")
	}
	version := binary.BigEndian.Uint32(h[4:8])
	if version != 2 && version != 3 {
		return errors.New("unsupported QCOW2 version")
	}
	if binary.BigEndian.Uint64(h[8:16]) != 0 || binary.BigEndian.Uint32(h[16:20]) != 0 {
		return errors.New("QCOW2 backing files are prohibited")
	}
	size := binary.BigEndian.Uint64(h[24:32])
	if size == 0 || size > uint64(diskGiB)*config.GiB {
		return errors.New("image virtual size exceeds configured disk capacity")
	}
	if binary.BigEndian.Uint32(h[32:36]) != 0 {
		return errors.New("encrypted guest images are not supported")
	}
	if version == 3 && binary.BigEndian.Uint64(h[72:80]) != 0 {
		return errors.New("QCOW2 incompatible/external-data features are prohibited")
	}
	return nil
}
