// SPDX-License-Identifier: Apache-2.0
package filetransfer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

func TestRoundTripAndBoundaries(t *testing.T) {
	home := t.TempDir()
	host, _ := filepath.EvalSymlinks(t.TempDir())
	g := &Guest{Home: home}
	call := func(_ context.Context, q protocol.FileRequest) (*protocol.FileResult, error) { return g.Handle(q) }
	source := filepath.Join(host, "project.tar")
	data := []byte(strings.Repeat("café\x00", 30000))
	os.WriteFile(source, data, 0600)
	v, e := In(context.Background(), source, call)
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(host, "result.tar")
	if _, e = Out(context.Background(), v.Path, out, call); e != nil {
		t.Fatal(e)
	}
	got, _ := os.ReadFile(out)
	if string(got) != string(data) {
		t.Fatal("corrupt copy")
	}
	st, _ := os.Stat(out)
	if st.Mode().Perm() != 0600 {
		t.Fatal("unsafe output permissions")
	}
	if _, e = Out(context.Background(), v.Path, out, call); e == nil {
		t.Fatal("overwrote existing output")
	}
	for _, p := range []string{"/etc/passwd", "/home/agent/../etc/passwd", "/home/agent/Transfers/../../secret"} {
		if _, e = Out(context.Background(), p, filepath.Join(host, "no"), call); e == nil {
			t.Fatal("unsafe guest path", p)
		}
	}
	os.Symlink(source, filepath.Join(host, "link"))
	if _, e = In(context.Background(), filepath.Join(host, "link"), call); e == nil {
		t.Fatal("copied symlink")
	}
	os.Link(source, filepath.Join(host, "hard"))
	if _, e = In(context.Background(), source, call); e == nil {
		t.Fatal("copied hardlink")
	}
	os.Symlink(host, filepath.Join(host, "parent"))
	if _, e = Out(context.Background(), v.Path, filepath.Join(host, "parent", "escape"), call); e == nil {
		t.Fatal("followed host parent symlink")
	}
	os.Symlink("/etc/passwd", filepath.Join(home, "link"))
	if _, e = g.Handle(protocol.FileRequest{Action: "stat", Path: "link"}); e == nil {
		t.Fatal("followed guest symlink")
	}
}
func TestMalformedGuestAndCleanup(t *testing.T) {
	for _, kind := range []string{"oversize", "empty-chunk", "bad-hash", "changed-size", "disconnect"} {
		t.Run(kind, func(t *testing.T) {
			host, _ := filepath.EvalSymlinks(t.TempDir())
			output := filepath.Join(host, "out")
			call := func(_ context.Context, q protocol.FileRequest) (*protocol.FileResult, error) {
				if q.Action == "stat" {
					size := int64(3)
					if kind == "oversize" {
						size = protocol.MaxFile + 1
					}
					return &protocol.FileResult{Size: size, SHA256: strings.Repeat("0", 64)}, nil
				}
				if kind == "disconnect" {
					return nil, protocol.Fail("DISCONNECTED", "lost guest")
				}
				v := &protocol.FileResult{Size: 3, Data: []byte("abc")}
				if kind == "empty-chunk" {
					v.Data = nil
				}
				if kind == "changed-size" {
					v.Size = 4
				}
				return v, nil
			}
			if _, e := Out(context.Background(), "/home/agent/result", output, call); e == nil {
				t.Fatal("accepted malformed guest")
			}
			entries, _ := os.ReadDir(host)
			if len(entries) != 0 {
				t.Fatal("left partial/claimed output", entries)
			}
		})
	}
}
func TestGuestWriteOrderingCommitAndAbandonment(t *testing.T) {
	g := &Guest{Home: t.TempDir()}
	id := protocol.ID()
	if _, e := g.Handle(protocol.FileRequest{Action: "begin", TransferID: id, Name: "project"}); e != nil {
		t.Fatal(e)
	}
	if _, e := g.Handle(protocol.FileRequest{Action: "begin", TransferID: id, Name: "project"}); e == nil {
		t.Fatal("reused transfer ID")
	}
	if _, e := g.Handle(protocol.FileRequest{Action: "write", TransferID: id, Offset: 1, Data: []byte("x")}); e == nil {
		t.Fatal("out-of-order write")
	}
	if _, e := g.Handle(protocol.FileRequest{Action: "commit", TransferID: id, Name: "project", SHA256: strings.Repeat("0", 64)}); e == nil {
		t.Fatal("accepted wrong checksum")
	}
	if _, e := g.Handle(protocol.FileRequest{Action: "abort", TransferID: id}); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(g.Home, "Transfers", id)); !os.IsNotExist(e) {
		t.Fatal("partial remained")
	}
	for _, q := range []protocol.FileRequest{{Action: "begin", TransferID: protocol.ID(), Name: "data.partial"}, {Action: "read", Path: "../secret"}, {Action: "read", Path: "/etc/passwd"}, {Action: "begin", TransferID: protocol.ID(), Name: "../escape"}, {Action: "write", TransferID: protocol.ID(), Data: make([]byte, protocol.FileChunk+1)}} {
		if _, e := g.Handle(q); e == nil {
			t.Fatal("accepted unsafe request", q.Action)
		}
	}
}

func TestOversizeEmptyAndCancellation(t *testing.T) {
	host, _ := filepath.EvalSymlinks(t.TempDir())
	g := &Guest{Home: t.TempDir()}
	call := func(_ context.Context, q protocol.FileRequest) (*protocol.FileResult, error) { return g.Handle(q) }
	empty := filepath.Join(host, "empty")
	os.WriteFile(empty, nil, 0600)
	v, e := In(context.Background(), empty, call)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Out(context.Background(), v.Path, filepath.Join(host, "empty-result"), call); e != nil {
		t.Fatal(e)
	}
	huge := filepath.Join(host, "huge")
	f, e := os.Create(huge)
	if e != nil {
		t.Fatal(e)
	}
	f.Truncate(protocol.MaxFile + 1)
	f.Close()
	if _, e = In(context.Background(), huge, call); e == nil {
		t.Fatal("accepted oversized source")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	timed := func(c context.Context, q protocol.FileRequest) (*protocol.FileResult, error) { return nil, c.Err() }
	if _, e = Out(ctx, "/home/agent/result", filepath.Join(host, "cancelled"), timed); e == nil {
		t.Fatal("ignored cancellation")
	}
	if _, e = os.Stat(filepath.Join(host, "cancelled")); !os.IsNotExist(e) {
		t.Fatal("published cancelled file")
	}
}
