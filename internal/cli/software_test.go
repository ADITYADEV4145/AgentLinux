// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestSoftwareParsing(t *testing.T) {
	o, e := parseSoftware([]string{"browsers", "install", "--json", "firefox"}, io.Discard)
	if e != nil || o.packages[0] != "firefox-esr" || !o.json {
		t.Fatal(o, e)
	}
	o, e = parseSoftware([]string{"packages", "install", "--profile", "web-dev"}, io.Discard)
	if e != nil || len(o.packages) != 2 || o.packages[0] != "nodejs" {
		t.Fatal(o, e)
	}
	for _, args := range [][]string{{"browsers", "install", "chrome"}, {"browsers", "list", "firefox"}, {"packages", "install", "--profile", "unknown"}, {"packages", "install", "--timeout", "0s", "jq"}, {"packages", "install", "--", "--evil"}, {"packages", "refresh", "jq"}} {
		if _, e = parseSoftware(args, io.Discard); e == nil {
			t.Fatal("accepted", args)
		}
	}
	var out, errout bytes.Buffer
	if Run(context.Background(), []string{"packages", "profiles", "--json"}, &out, &errout) != 0 || out.Len() == 0 {
		t.Fatal(out.String(), errout.String())
	}
	for _, app := range []string{"files", "settings", "launcher", "firefox", "chromium"} {
		if _, e = Parse([]string{"open", app}, io.Discard); e != nil {
			t.Fatal(app, e)
		}
	}
}
