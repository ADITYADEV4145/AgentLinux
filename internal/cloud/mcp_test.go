// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"context"
	"encoding/json"
	"github.com/ADITYADEV4145/AgentLinux/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// This exercises a real HTTP remote adapter and official MCP client. The
// provider is explicitly simulated; it proves neither VM nor E2B acceptance.
func TestOfficialMCPClientThroughDevelopmentCloud(t *testing.T) {
	m, _, a, _, root := fixture(t)
	s := ready(t, m, a)
	api := httptest.NewServer(NewAPI(m))
	defer api.Close()
	c, e := LoadClient(filepath.Join(root, "alice.json"))
	if e != nil {
		t.Fatal(e)
	}
	c.Config.URL = api.URL
	c.Config.Session = s.ID
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	left, right := mcp.NewInMemoryTransports()
	server, e := mcpserver.New(c, mcpserver.Options{EnabledTools: Tools(s.Capabilities)}).Connect(ctx, left, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer server.Close()
	client, e := mcp.NewClient(&mcp.Implementation{Name: "cloud-contract-test", Version: "1"}, nil).Connect(ctx, right, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	tools, e := client.ListTools(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "computer_run_command" || tool.Name == "computer_open" {
			t.Fatal("unsupported capability advertised", tool.Name)
		}
	}
	call := func(name string, args any) *mcp.CallToolResult {
		t.Helper()
		r, e := client.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if e != nil || r.IsError {
			t.Fatal(name, r, e)
		}
		return r
	}
	status := call("computer_status", nil)
	b, _ := json.Marshal(status.StructuredContent)
	if !strings.Contains(string(b), "SIMULATED") {
		t.Fatal(string(b))
	}
	first := call("computer_screenshot", nil)
	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	b, _ = json.Marshal(first.StructuredContent)
	if json.Unmarshal(b, &result) != nil || result.Data.ID == "" {
		t.Fatal(string(b))
	}
	if len(first.Content) != 2 {
		t.Fatal("PNG content missing")
	}
	if _, ok := first.Content[1].(*mcp.ImageContent); !ok {
		t.Fatal(first.Content)
	}
	call("computer_click", map[string]any{"x": 0, "y": 0, "frameId": result.Data.ID})
	second := call("computer_screenshot", nil)
	b, _ = json.Marshal(second.StructuredContent)
	if strings.Contains(string(b), result.Data.ID) {
		t.Fatal("stale frame returned")
	}
}
