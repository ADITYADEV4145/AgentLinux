// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"bytes"
	"context"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoveryPreviewAndMalformedArguments(t *testing.T) {
	path, _ := filepath.EvalSymlinks(t.TempDir())
	_ = os.Chmod(path, 0700)
	root, e := storage.Root(path)
	if e != nil {
		t.Fatal(e)
	}
	id := protocol.ID()
	if e = storage.JSON(root, "state.json", lifecycle.State{Schema: 1, State: "stopped", MachineID: id}); e != nil {
		t.Fatal(e)
	}
	var out, errout bytes.Buffer
	if code := Run(context.Background(), []string{"checkpoint", "restore", "safe", "--state-dir", root, "--json", "--lima", "/missing/runtime"}, &out, &errout); code != 2 || !bytes.Contains(out.Bytes(), []byte("CONFIRMATION_REQUIRED")) {
		t.Fatal("preview needed runtime or mutated", code, out.String(), errout.String())
	}
	for _, args := range [][]string{{"checkpoint", "create", ""}, {"checkpoint", "wrong"}, {"desktop", "restart", "--yes", "--machine-id", "wrong"}} {
		out.Reset()
		errout.Reset()
		args = append(args, "--state-dir", root)
		if Run(context.Background(), args, &out, &errout) == 0 {
			t.Fatal("malformed recovery accepted", args)
		}
	}
	if _, e = os.Stat(filepath.Join(root, "checkpoints")); !os.IsNotExist(e) {
		t.Fatal("preview created checkpoint data")
	}
}
