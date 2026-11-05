// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMCPConfigPrinter(t *testing.T) {
	for _, client := range []string{"codex", "claude", "vscode"} {
		var b bytes.Buffer
		if e := printMCPConfig(&b, client, "/path with spaces/agent-linux", "/private/state", "/runtime/limactl"); e != nil {
			t.Fatal(e)
		}
		if client == "codex" {
			if !strings.Contains(b.String(), `command = "/path with spaces/agent-linux"`) {
				t.Fatal(b.String())
			}
		} else {
			var v map[string]any
			if e := json.Unmarshal(b.Bytes(), &v); e != nil {
				t.Fatal(e)
			}
			key := "servers"
			if client == "claude" {
				key = "mcpServers"
			}
			if v[key] == nil {
				t.Fatal(v)
			}
		}
	}
	var b bytes.Buffer
	if printMCPConfig(&b, "cloud", "a", "b", "c") == nil {
		t.Fatal("invalid client accepted")
	}
}
func TestMCPDoesNotStartOrCreate(t *testing.T) {
	var out, errout bytes.Buffer
	code := Run(context.Background(), []string{"mcp", "--state-dir", t.TempDir() + "/missing"}, &out, &errout)
	if code == 0 || out.Len() != 0 || !strings.Contains(errout.String(), "existing machine") {
		t.Fatal(code, out.String(), errout.String())
	}
	code = Run(context.Background(), []string{"mcp", "--unknown"}, &out, &errout)
	if code == 0 || out.Len() != 0 {
		t.Fatal("invalid MCP options polluted stdout")
	}
}
