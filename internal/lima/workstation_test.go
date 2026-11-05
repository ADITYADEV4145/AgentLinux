// SPDX-License-Identifier: Apache-2.0
package lima

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Real shell execution with a fake settings daemon: checks first-run recovery
// and preservation, not whether the theme actually renders (hardware acceptance).
func TestWorkstationDefaultsPreserveCustomizationAndRetryFailure(t *testing.T) {
	home := t.TempDir()
	bin := t.TempDir()
	log := filepath.Join(home, "calls")
	query := filepath.Join(bin, "xfconf-query")
	if e := os.WriteFile(query, []byte("#!/bin/sh\n[ ! -f \"$HOME/fail\" ] || exit 1\nprintf '%s\\n' \"$*\" >> \"$HOME/calls\"\n"), 0700); e != nil {
		t.Fatal(e)
	}
	run := func() error {
		c := exec.Command("/bin/sh", "-c", workstationScript)
		c.Env = []string{"HOME=" + home, "PATH=" + bin + ":/usr/bin:/bin"}
		return c.Run()
	}
	marker := filepath.Join(home, ".config/agent-linux/workstation-5.5.0")
	fail := filepath.Join(home, "fail")
	if e := os.WriteFile(fail, nil, 0600); e != nil {
		t.Fatal(e)
	}
	if run() == nil {
		t.Fatal("failed settings must stop first-run setup")
	}
	if _, e := os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("failed setup published marker", e)
	}
	if e := os.Remove(fail); e != nil {
		t.Fatal(e)
	}
	if e := run(); e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(log)
	if e != nil || len(before) == 0 {
		t.Fatal("no settings applied", e)
	}
	if _, e := os.Stat(marker); e != nil {
		t.Fatal("no completion marker", e)
	}
	// A later session must not overwrite preferences, even if the daemon fails.
	if e := os.WriteFile(fail, nil, 0600); e != nil {
		t.Fatal(e)
	}
	if e := run(); e != nil {
		t.Fatal(e)
	}
	after, e := os.ReadFile(log)
	if e != nil || string(before) != string(after) {
		t.Fatal("persistent user preferences changed", e)
	}
}
