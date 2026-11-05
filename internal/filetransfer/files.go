// SPDX-License-Identifier: Apache-2.0
// Package filetransfer copies explicit bounded regular files, never archives or mounts.
package filetransfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

func SafeParents(p string) error {
	if !filepath.IsAbs(p) || filepath.Clean(p) != p || strings.ContainsAny(p, "\x00\r\n") {
		return errors.New("expected absolute clean path, without symlink components")
	}
	for a := p; a != "/"; a = filepath.Dir(a) {
		st, e := os.Lstat(a)
		if e != nil {
			return e
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return errors.New("parent must be a real directory, not a symlink")
		}
	}
	return nil
}
func regular(f *os.File) (os.FileInfo, error) {
	st, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() || st.Size() > protocol.MaxFile {
		return nil, errors.New("only regular files up to 4 MiB are supported")
	}
	if s, ok := st.Sys().(*syscall.Stat_t); ok && s.Nlink != 1 {
		return nil, errors.New("hard-linked files are not accepted")
	}
	return st, nil
}
func digest(f *os.File) (string, error) {
	if _, e := f.Seek(0, 0); e != nil {
		return "", e
	}
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(f, protocol.MaxFile+1))
	if e != nil {
		return "", e
	}
	if n > protocol.MaxFile {
		return "", errors.New("file grew beyond limit")
	}
	_, e = f.Seek(0, 0)
	return hex.EncodeToString(h.Sum(nil)), e
}

// Open rejects symlinks in every component, special files and hard links.
// Root methods additionally confine race-time path resolution beneath the selected directory.
func Open(r *os.Root, p string, flags int) (*os.File, error) {
	if !protocol.CleanRelative(p) {
		return nil, errors.New("unsafe relative file path")
	}
	parts := strings.Split(p, "/")
	for i := range parts {
		st, e := r.Lstat(strings.Join(parts[:i+1], "/"))
		if e != nil {
			return nil, e
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("symlink components are not supported")
		}
	}
	f, e := r.OpenFile(p, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if e != nil {
		return nil, e
	}
	if _, e = regular(f); e != nil {
		f.Close()
		return nil, e
	}
	return f, nil
}

type Guest struct {
	Home string
	mu   sync.Mutex
}

func (g *Guest) Handle(q protocol.FileRequest) (*protocol.FileResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if e := q.Validate(); e != nil {
		return nil, e
	}
	root, e := os.OpenRoot(g.Home)
	if e != nil {
		return nil, e
	}
	defer root.Close()
	fail := func(e error) (*protocol.FileResult, error) {
		return nil, protocol.Fail("FILE_TRANSFER_FAILED", "guest file rejected or changed; no host files are mounted: "+e.Error())
	}
	if q.Action == "stat" || q.Action == "read" {
		f, e := Open(root, q.Path, os.O_RDONLY)
		if e != nil {
			return fail(e)
		}
		defer f.Close()
		st, e := regular(f)
		if e != nil {
			return fail(e)
		}
		v := &protocol.FileResult{Size: st.Size()}
		if q.Action == "stat" {
			v.SHA256, e = digest(f)
		} else {
			if q.Offset > st.Size() {
				return fail(errors.New("offset beyond file"))
			}
			v.Data = make([]byte, min(protocol.FileChunk, int(st.Size()-q.Offset)))
			_, e = f.ReadAt(v.Data, q.Offset)
			if errors.Is(e, io.EOF) {
				e = nil
			}
		}
		if e != nil {
			return fail(e)
		}
		return v, nil
	}
	// No startup traversal of user files. Create a private transfer directory only on demand.
	if e = root.Mkdir("Transfers", 0700); e != nil && !os.IsExist(e) {
		return fail(e)
	}
	st, e := root.Lstat("Transfers")
	if e != nil {
		return fail(e)
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return fail(errors.New("Transfers must be a real directory"))
	}
	tr, e := root.OpenRoot("Transfers")
	if e != nil {
		return fail(e)
	}
	defer tr.Close()
	id := q.TransferID
	if q.Action == "begin" {
		// Bound abandoned transfer directories; successful copies remain user-owned until explicitly removed.
		d, e := tr.Open(".")
		if e != nil {
			return fail(e)
		}
		entries, e := d.ReadDir(65)
		d.Close()
		if e != nil && !errors.Is(e, io.EOF) {
			return fail(e)
		}
		if len(entries) >= 64 {
			return fail(errors.New("64 transfer directories reached; remove unwanted guest Transfers explicitly"))
		}
		if e = tr.Mkdir(id, 0700); e != nil {
			return fail(e)
		}
		f, e := tr.OpenFile(id+"/data.partial", os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
		if e != nil {
			return fail(e)
		}
		e = f.Close()
		if e != nil {
			return fail(e)
		}
		return &protocol.FileResult{}, nil
	}
	// A confined directory handle prevents substitutions from escaping the guest home.
	st, e = tr.Lstat(id)
	if e != nil {
		return fail(e)
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return fail(errors.New("invalid transfer directory"))
	}
	part, e := tr.OpenRoot(id)
	if e != nil {
		return fail(e)
	}
	defer part.Close()
	if q.Action == "abort" {
		// Never recursively remove arbitrary guest additions; only our partial file and empty directory.
		_ = part.Remove("data.partial")
		_ = tr.Remove(id)
		return &protocol.FileResult{}, nil
	}
	flags := os.O_RDONLY
	if q.Action == "write" {
		flags = os.O_WRONLY | os.O_APPEND
	}
	f, e := Open(part, "data.partial", flags)
	if e != nil {
		return fail(e)
	}
	defer f.Close()
	st, e = regular(f)
	if e != nil {
		return fail(e)
	}
	if q.Action == "write" {
		if st.Size() != q.Offset {
			return fail(errors.New("offset mismatch; do not replay a write"))
		}
		if _, e = f.Write(q.Data); e != nil {
			return fail(e)
		}
		return &protocol.FileResult{Size: q.Offset + int64(len(q.Data))}, nil
	}
	h, e := digest(f)
	if e != nil {
		return fail(e)
	}
	if st.Size() != q.Size || h != q.SHA256 {
		return fail(errors.New("length/checksum mismatch"))
	}
	if e = f.Sync(); e != nil {
		return fail(e)
	}
	// Link is atomic and refuses to replace an existing destination. Never extract archive entries.
	if e = part.Link("data.partial", q.Name); e != nil {
		return fail(e)
	}
	if e = part.Remove("data.partial"); e != nil {
		return fail(e)
	}
	return &protocol.FileResult{Path: "/home/agent/Transfers/" + id + "/" + q.Name, Size: q.Size, SHA256: h}, nil
}

type Call func(context.Context, protocol.FileRequest) (*protocol.FileResult, error)

func In(ctx context.Context, source string, call Call) (*protocol.FileResult, error) {
	if !filepath.IsAbs(source) || filepath.Clean(source) != source {
		return nil, errors.New("source must be an absolute clean path")
	}
	if e := SafeParents(filepath.Dir(source)); e != nil {
		return nil, e
	}
	r, e := os.OpenRoot(filepath.Dir(source))
	if e != nil {
		return nil, e
	}
	defer r.Close()
	f, e := Open(r, filepath.Base(source), os.O_RDONLY)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := regular(f)
	if e != nil {
		return nil, e
	}
	h, e := digest(f)
	if e != nil {
		return nil, e
	}
	id := protocol.ID()
	name := filepath.Base(source)
	q := protocol.FileRequest{Action: "begin", TransferID: id, Name: name}
	if e = q.Validate(); e != nil {
		return nil, e
	}
	if _, e = call(ctx, q); e != nil {
		return nil, e
	}
	completed := false
	defer func() {
		if !completed {
			c, cancel := context.WithTimeout(context.Background(), 2e9)
			defer cancel()
			_, _ = call(c, protocol.FileRequest{Action: "abort", TransferID: id})
		}
	}()
	buf := make([]byte, protocol.FileChunk)
	var offset int64
	for {
		n, re := f.Read(buf)
		if n > 0 {
			if offset+int64(n) > st.Size() {
				return nil, errors.New("source changed during copy")
			}
			v, e := call(ctx, protocol.FileRequest{Action: "write", TransferID: id, Offset: offset, Data: buf[:n]})
			if e != nil {
				return nil, e
			}
			offset += int64(n)
			if v == nil || v.Size != offset {
				return nil, errors.New("invalid guest write acknowledgement")
			}
		}
		if re == io.EOF {
			break
		}
		if re != nil {
			return nil, re
		}
	}
	v, e := call(ctx, protocol.FileRequest{Action: "commit", TransferID: id, Name: name, Size: st.Size(), SHA256: h})
	if e != nil {
		return nil, e
	}
	expected := "/home/agent/Transfers/" + id + "/" + name
	if v == nil || v.Path != expected || v.Size != st.Size() || v.SHA256 != h {
		return nil, errors.New("invalid guest commit acknowledgement; outcome may be unknown")
	}
	completed = true
	return v, nil
}
func Out(ctx context.Context, guestPath, output string, call Call) (*protocol.FileResult, error) {
	if !filepath.IsAbs(output) || filepath.Clean(output) != output {
		return nil, errors.New("output must be an absolute clean path")
	}
	if !strings.HasPrefix(guestPath, "/home/agent/") {
		return nil, errors.New("guest source must be under /home/agent")
	}
	rel := strings.TrimPrefix(guestPath, "/home/agent/")
	q := protocol.FileRequest{Action: "stat", Path: rel}
	if e := q.Validate(); e != nil {
		return nil, e
	}
	if e := SafeParents(filepath.Dir(output)); e != nil {
		return nil, e
	}
	r, e := os.OpenRoot(filepath.Dir(output))
	if e != nil {
		return nil, e
	}
	defer r.Close()
	name := filepath.Base(output)
	if !protocol.CleanRelative(name) {
		return nil, errors.New("unsafe output name")
	}
	if _, e = r.Lstat(name); !os.IsNotExist(e) {
		return nil, errors.New("output already exists or is inaccessible; refusing overwrite")
	}
	v, e := call(ctx, q)
	if e != nil {
		return nil, e
	}
	if v == nil || v.Size < 0 || v.Size > protocol.MaxFile || len(v.Data) != 0 {
		return nil, errors.New("invalid guest file metadata")
	}
	h, e := hex.DecodeString(v.SHA256)
	if e != nil || len(h) != 32 {
		return nil, errors.New("invalid guest file digest")
	}
	tmp := ".agent-linux-copy-" + protocol.ID()
	f, e := r.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	defer r.Remove(tmp)
	defer f.Close()
	hash := sha256.New()
	var offset int64
	for offset < v.Size {
		b, e := call(ctx, protocol.FileRequest{Action: "read", Path: rel, Offset: offset})
		if e != nil {
			return nil, e
		}
		if b == nil || b.Size != v.Size || len(b.Data) != min(protocol.FileChunk, int(v.Size-offset)) || offset+int64(len(b.Data)) > v.Size {
			return nil, errors.New("invalid/truncated/oversized guest file chunk")
		}
		if _, e = f.Write(b.Data); e != nil {
			return nil, e
		}
		hash.Write(b.Data)
		offset += int64(len(b.Data))
	}
	if hex.EncodeToString(hash.Sum(nil)) != v.SHA256 {
		return nil, errors.New("guest file changed or checksum mismatch; partial output removed")
	}
	if e = f.Sync(); e != nil {
		return nil, e
	}
	if e = f.Close(); e != nil {
		return nil, e
	}
	if e = r.Link(tmp, name); e != nil {
		return nil, fmt.Errorf("cannot publish new output without overwriting: %w", e)
	}
	return &protocol.FileResult{Path: output, Size: v.Size, SHA256: v.SHA256}, nil
}
