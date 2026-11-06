// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ADITYADEV4145/AgentLinux/internal/cloud"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/lima"
	"github.com/ADITYADEV4145/AgentLinux/internal/machine"
	"github.com/ADITYADEV4145/AgentLinux/internal/mcpserver"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func runMCP(ctx context.Context, args []string, out, errout io.Writer) int {
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	f.SetOutput(errout)
	root := f.String("state-dir", DefaultRoot(), "existing private local machine state directory")
	runtime := f.String("lima", "limactl", "pinned Lima executable")
	cloudConfig := f.String("cloud-config", "", "private cloud session config; mutually exclusive with local state/runtime flags")
	debug := f.Bool("debug", false, "log tool names, outcome codes and duration to stderr; never arguments/results")
	client := f.String("client", "codex", "mcp-config only: codex, claude or vscode")
	if e := f.Parse(args[1:]); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	fail := func(e error) int { fmt.Fprintln(errout, e); return 1 }
	if f.NArg() != 0 {
		return fail(errors.New("unexpected positional arguments"))
	}
	if *cloudConfig != "" {
		c, e := cloud.LoadClient(*cloudConfig)
		if e != nil {
			return fail(e)
		}
		mixed := false
		f.Visit(func(v *flag.Flag) {
			if v.Name == "state-dir" || v.Name == "lima" {
				mixed = true
			}
		})
		if mixed {
			return fail(errors.New("cloud and local flags are mutually exclusive"))
		}
		if args[0] == "mcp-config" {
			exe, e := os.Executable()
			if e != nil {
				return fail(e)
			}
			if e = printMCPArguments(out, *client, exe, []string{"mcp", "--cloud-config", *cloudConfig}); e != nil {
				return fail(e)
			}
			return 0
		}
		state, e := c.Status(ctx)
		if e != nil {
			return fail(e)
		}
		var capabilities *protocol.Capabilities
		if state.Guest != nil {
			capabilities = state.Guest.Capabilities
		}
		opts := mcpserver.Options{EnabledTools: cloud.Tools(capabilities)}
		if *debug {
			opts.Debug = errout
		}
		if e = mcpserver.New(c, opts).Run(ctx, &mcp.StdioTransport{MaxLineLength: 256 << 10}); e != nil && !errors.Is(e, context.Canceled) {
			return fail(errors.New("cloud MCP transport closed"))
		}
		return 0
	}
	if !filepath.IsAbs(*root) {
		return fail(errors.New("state-dir must be absolute"))
	}
	if args[0] == "mcp-config" {
		exe, e := os.Executable()
		if e != nil {
			return fail(e)
		}
		runtimePath, e := lima.Find(*runtime)
		if e != nil {
			return fail(e)
		}
		if e = printMCPConfig(out, *client, exe, *root, runtimePath); e != nil {
			return fail(e)
		}
		return 0
	}
	// Refuse accidental empty-state creation. MCP never creates or starts a VM.
	if _, e := os.Stat(filepath.Join(*root, "state.json")); e != nil {
		return fail(errors.New("MCP requires an existing machine; run agent-linux up --local first"))
	}
	private, e := storage.Root(*root)
	if e != nil {
		return fail(e)
	}
	runtimePath, e := lima.Find(*runtime)
	if e != nil {
		return fail(e)
	}
	c := &lifecycle.Controller{Root: private, Backend: &lima.Backend{Root: private, Executable: runtimePath}}
	opts := mcpserver.Options{}
	if *debug {
		opts.Debug = errout
	}
	server := mcpserver.New(machine.Local{Controller: c}, opts)
	if e = server.Run(ctx, &mcp.StdioTransport{MaxLineLength: 256 << 10}); e != nil && !errors.Is(e, context.Canceled) {
		return fail(errors.New("MCP transport closed with an error; inspect client diagnostics"))
	}
	return 0
}

func printMCPConfig(out io.Writer, client, exe, root, runtime string) error {
	args := []string{"mcp", "--state-dir", root, "--lima", runtime}
	return printMCPArguments(out, client, exe, args)
}
func printMCPArguments(out io.Writer, client, exe string, args []string) error {
	server := map[string]any{"command": exe, "args": args}
	switch client {
	case "codex":
		fmt.Fprintln(out, "[mcp_servers.agent_linux]")
		fmt.Fprintf(out, "command = %s\nargs = [", tomlString(exe))
		for i, a := range args {
			if i > 0 {
				fmt.Fprint(out, ", ")
			}
			fmt.Fprint(out, tomlString(a))
		}
		fmt.Fprintln(out, "]\ntool_timeout_sec = 75")
	case "claude":
		server["type"] = "stdio"
		e := json.NewEncoder(out)
		e.SetIndent("", "  ")
		return e.Encode(map[string]any{"mcpServers": map[string]any{"agent_linux": server}})
	case "vscode":
		server["type"] = "stdio"
		e := json.NewEncoder(out)
		e.SetIndent("", "  ")
		return e.Encode(map[string]any{"servers": map[string]any{"agent_linux": server}})
	default:
		return errors.New("unknown client: choose codex, claude or vscode")
	}
	return nil
}

// JSON string escaping is compatible with TOML basic strings.
func tomlString(s string) string { b, _ := json.Marshal(s); return string(b) }
