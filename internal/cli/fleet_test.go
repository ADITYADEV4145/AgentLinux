// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCentralMachineSelectionConflictsAndGuestArgv(t *testing.T) {
	root, e := os.MkdirTemp("/private/tmp", "af-cli-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(root)
	ctx := context.Background()
	cat := filepath.Join(root, "catalog")
	if _, _, e = selectMachine(ctx, []string{"status", "--machine", "work", "--state-dir", root}); e == nil {
		t.Fatal("conflicting selectors accepted")
	}
	if _, _, e = selectMachine(ctx, []string{"status", "--machine", "../work", "--catalog-dir", cat}); e == nil {
		t.Fatal("traversal accepted")
	}
	selected, args, e := selectMachine(ctx, []string{"up", "--machine", "work", "--catalog-dir", cat, "--local"})
	if e != nil {
		t.Fatal(e)
	}
	s := selected.Value(selectionKey{}).(selection)
	if s.Entry.Name != "work" || !strings.Contains(strings.Join(args, " "), s.Entry.Root) {
		t.Fatal("name not resolved centrally")
	}
	_, args, e = selectMachine(ctx, []string{"exec", "--state-dir", root, "--", "printf", "--machine", "literal"})
	if e != nil {
		t.Fatal(e)
	}
	if strings.Join(args[len(args)-4:], " ") != "-- printf --machine literal" {
		t.Fatal("guest argv parsed as host flags", args)
	}
}
