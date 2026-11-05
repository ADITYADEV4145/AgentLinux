// SPDX-License-Identifier: Apache-2.0
package protocol

import (
	"encoding/hex"
	"path"
	"strings"
	"unicode"
)

const MaxFile = 4 << 20
const FileChunk = 96 << 10

type FileRequest struct {
	Action     string `json:"action"`
	TransferID string `json:"transferId,omitempty"`
	Name       string `json:"name,omitempty"`
	Path       string `json:"path,omitempty"` // Relative to the guest's /home/agent only.
	Offset     int64  `json:"offset,omitempty"`
	Size       int64  `json:"size,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	Data       []byte `json:"data,omitempty"`
}
type FileResult struct {
	Path   string `json:"path,omitempty"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
	Data   []byte `json:"data,omitempty"`
}

func CleanRelative(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return s != "" && s != "." && !strings.HasPrefix(s, "/") && path.Clean(s) == s && !strings.ContainsAny(s, "\\\x00\r\n") && s != ".." && !strings.HasPrefix(s, "../") && len(s) <= 1024
}
func (q FileRequest) Validate() error {
	bad := func() error {
		return Fail("INVALID_ARGUMENT", "invalid file operation/path/size; regular files only, 4 MiB maximum")
	}
	if q.Offset < 0 || q.Size < 0 || q.Size > MaxFile || q.Offset > MaxFile || len(q.Data) > FileChunk || q.Offset+int64(len(q.Data)) > MaxFile {
		return bad()
	}
	switch q.Action {
	case "begin", "write", "commit", "abort":
		if !ValidID(q.TransferID) {
			return bad()
		}
	case "stat", "read":
		if !CleanRelative(q.Path) {
			return bad()
		}
	default:
		return bad()
	}
	if q.Action == "begin" || q.Action == "commit" {
		if !CleanRelative(q.Name) || path.Base(q.Name) != q.Name || len(q.Name) > 128 || (strings.HasPrefix(q.Name, ".") || q.Name == "data.partial") {
			return bad()
		}
	}
	if q.Action == "commit" {
		h, e := hex.DecodeString(q.SHA256)
		if e != nil || len(h) != 32 || strings.ToLower(q.SHA256) != q.SHA256 {
			return bad()
		}
	}
	if q.Action == "write" && len(q.Data) == 0 {
		return bad()
	}
	return nil
}
