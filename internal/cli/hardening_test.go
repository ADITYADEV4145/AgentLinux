// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

func TestDestructiveCLIRequiresIntentWithoutRuntime(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	os.Chmod(root, 0700)
	if _, e := storage.Root(root); e != nil {
		t.Fatal(e)
	}
	id := protocol.ID()
	storage.JSON(root, "state.json", lifecycle.State{Schema: 1, MachineID: id, State: "stopped"})
	out, errout := &bytes.Buffer{}, &bytes.Buffer{}
	code := Run(context.Background(), []string{"delete", "--state-dir", root, "--machine-id", id, "--lima", "does-not-exist", "--json"}, out, errout)
	if code != 2 || !bytes.Contains(out.Bytes(), []byte("CONFIRMATION_REQUIRED")) {
		t.Fatal("preview required a runtime or hid its plan", code, out.String(), errout.String())
	}
	if _, e := os.Stat(filepath.Join(root, "state.json")); e != nil {
		t.Fatal("preview deleted state")
	}
	out.Reset()
	errout.Reset()
	code = Run(context.Background(), []string{"delete", "--state-dir", root, "--machine-id", protocol.ID(), "--yes", "--json"}, out, errout)
	if code != 1 || !bytes.Contains(out.Bytes(), []byte("does not match")) {
		t.Fatal("wrong ID reached backend", code, out.String())
	}
}
func TestCopyAndDeleteDoNotAdoptMissingPaths(t *testing.T) {
	parent, _ := filepath.EvalSymlinks(t.TempDir())
	root := filepath.Join(parent, "absent")
	for _, args := range [][]string{{"copy-in", "--source", "/absolute/file"}, {"delete", "--machine-id", protocol.ID(), "--yes"}} {
		args = append(args, "--state-dir", root)
		if Run(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}) == 0 {
			t.Fatal("accepted missing path")
		}
		if _, e := os.Stat(root); !os.IsNotExist(e) {
			t.Fatal("created root for sensitive operation")
		}
	}
}
