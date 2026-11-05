// SPDX-License-Identifier: Apache-2.0
package software

import (
	"strings"
	"testing"
)

func TestMaintenanceBoundary(t *testing.T) {
	for _, p := range []string{"--allow-unauthenticated", "/tmp/evil.deb", "https://evil/a.deb", "pkg;touch /host", "pkg=1", "pkg/stable", "pkg:amd64", "$(command)", "", "a\nb"} {
		if Validate("install", []string{p}) == nil {
			t.Fatalf("accepted %q", p)
		}
	}
	for _, p := range []string{"jq", "firefox-esr", "libstdc++6", "python3.13-venv"} {
		if e := Validate("install", []string{p}); e != nil {
			t.Fatal(e)
		}
	}
	args, e := AdminArgs("install", []string{"jq"})
	if e != nil {
		t.Fatal(e)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "shell --workdir=/ default sudo -n /usr/bin/systemd-run") || !strings.Contains(joined, "RuntimeMaxSec=540") || strings.Contains(joined, "--preserve-env") {
		t.Fatal(args)
	}
	if Validate("upgrade", []string{"jq"}) == nil || Validate("destroy", nil) == nil || Validate("install", nil) == nil {
		t.Fatal("invalid action accepted")
	}
}
func TestBrowserInventoryDoesNotInventCompatibility(t *testing.T) {
	rows := Browsers("arm64", "chromium\t156-test\tinstalled\nfirefox-esr:arm64\t153-test\tinstalled\ngoogle-chrome-stable\t1\tconfig-files\n")
	if !rows[0].Installed || rows[0].Version != "156-test" || !rows[1].Installed || rows[2].Installed || rows[3].Availability != "unavailable_on_this_architecture" {
		t.Fatal(rows)
	}
	if rows[2].Availability != "expected_to_work_not_yet_tested" || rows[2].Installable {
		t.Fatal("Chrome ARM support or vendor install misreported", rows[2])
	}
	if Browsers("amd64", "")[3].Availability != "expected_to_work_not_yet_tested" {
		t.Fatal("cloud architecture confused with local")
	}
	if Browsers("riscv64", "")[0].Installable {
		t.Fatal("unsupported architecture install advertised")
	}
	if _, e := BrowserPackage("chrome"); e == nil {
		t.Fatal("Chrome silently substituted")
	}
}
