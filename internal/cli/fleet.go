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
	"strings"

	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/fleet"
	"github.com/ADITYADEV4145/AgentLinux/internal/host"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/lima"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
	"golang.org/x/term"
)

type selectionKey struct{}
type selection struct {
	Manager *fleet.Manager
	Entry   fleet.Entry
	Pin     string
}

func catalogRoot() string { return filepath.Join(filepath.Dir(DefaultRoot()), "catalog") }
func fleetManager(root, runtime string) *fleet.Manager {
	report := host.Inspect(root, runtime)
	return &fleet.Manager{Root: root, DefaultRoot: DefaultRoot(), HostCPUs: report.CPUs, HostMemoryGiB: int(report.MemoryBytes / config.GiB), Observe: func(ctx context.Context, v fleet.Entry) (lifecycle.Observation, error) {
		exe, e := lima.Find(runtime)
		if e != nil {
			return lifecycle.Observation{}, e
		}
		return (&lima.Backend{Root: v.Root, Executable: exe}).Observe(ctx)
	}}
}
func pinController(ctx context.Context, c *lifecycle.Controller) {
	if selected, ok := ctx.Value(selectionKey{}).(selection); ok {
		c.ExpectedID = selected.Pin
		if selected.Manager != nil {
			c.Reserve = func(ctx context.Context, s config.Spec) (func(), error) {
				return selected.Manager.Reserve(ctx, selected.Entry, s)
			}
		}
	}
}

// Resolve once at the CLI boundary, before command-specific parsers. Stop at --
// so guest argv and arbitrary command strings are never treated as host selectors.
func selectMachine(ctx context.Context, args []string) (context.Context, []string, error) {
	if _, ok := ctx.Value(selectionKey{}).(selection); ok {
		return ctx, args, nil
	}
	if len(args) == 0 {
		return ctx, args, nil
	}
	cmd := args[0]
	eligible := map[string]bool{"start": true, "up": true, "status": true, "stop": true, "logs": true, "doctor": true, "view": true, "mcp": true, "mcp-config": true, "copy-in": true, "copy-out": true, "delete": true, "reset": true, "screenshot": true, "move": true, "click": true, "drag": true, "scroll": true, "type": true, "key": true, "open": true, "exec": true, "browsers": true, "packages": true}
	if !eligible[cmd] {
		return ctx, args, nil
	}
	name, root, pin, cat, runtime := "", "", "", catalogRoot(), "limactl"
	cleaned := []string{cmd}
	hasCloud, help := false, false
	machineFlag, rootFlag := false, false
	for i := 1; i < len(args); i++ {
		token := args[i]
		if token == "--" {
			cleaned = append(cleaned, args[i:]...)
			break
		}
		key, value, hasValue := strings.Cut(token, "=")
		if token == "--help" || token == "-h" {
			help = true
		}
		if key == "--machine" || key == "--catalog-dir" || key == "--expected-machine-id" {
			if !hasValue {
				if i+1 >= len(args) {
					return ctx, args, errors.New(key + " requires a value")
				}
				i++
				value = args[i]
			}
			switch key {
			case "--machine":
				if machineFlag {
					return ctx, args, errors.New("duplicate --machine")
				}
				machineFlag = true
				name = value
			case "--catalog-dir":
				cat = value
			case "--expected-machine-id":
				pin = value
			}
			continue
		}
		if key == "--state-dir" || key == "--lima" || key == "--cloud-config" {
			if !hasValue && i+1 < len(args) {
				value = args[i+1]
			}
			if key == "--state-dir" {
				rootFlag = true
				root = value
			}
			if key == "--lima" {
				runtime = value
			}
			if key == "--cloud-config" {
				hasCloud = true
			}
			cleaned = append(cleaned, token)
			if !hasValue && i+1 < len(args) {
				i++
				cleaned = append(cleaned, args[i])
			}
			continue
		}
		cleaned = append(cleaned, token)
	}
	if help {
		return ctx, cleaned, nil
	}
	if hasCloud {
		if name != "" || pin != "" {
			return ctx, args, errors.New("cloud and named local selection are mutually exclusive")
		}
		return ctx, cleaned, nil
	}
	if machineFlag && rootFlag {
		return ctx, args, errors.New("--machine and --state-dir are mutually exclusive")
	}
	if pin != "" && !protocol.ValidID(pin) {
		return ctx, args, errors.New("invalid expected machine identity")
	}
	if machineFlag && !fleet.ValidName(name) {
		return ctx, args, errors.New("invalid machine name")
	}
	if rootFlag {
		// Explicit legacy namespaces remain supported. MCP/view pin the current identity
		// below even when the path is deliberately outside the managed catalog.
		if _, e := os.Stat(filepath.Join(cat, "catalog.json")); e == nil {
			manager := fleetManager(cat, runtime)
			entries, e := manager.ReadEntries()
			if e != nil {
				return ctx, args, e
			}
			for _, entry := range entries {
				if entry.Root == root {
					if pin != "" && pin != entry.ID {
						return ctx, args, errors.New("registered namespace identity mismatch")
					}
					return context.WithValue(ctx, selectionKey{}, selection{Manager: manager, Entry: entry, Pin: entry.ID}), cleaned, nil
				}
			}
		}
		return context.WithValue(ctx, selectionKey{}, selection{Pin: pin}), cleaned, nil
	}
	if name == "" {
		if cmd != "up" && cmd != "start" {
			if _, e := os.Stat(filepath.Join(cat, "catalog.json")); os.IsNotExist(e) {
				return ctx, cleaned, nil
			}
		}
		name = "default"
	}
	manager := fleetManager(cat, runtime)
	var entry fleet.Entry
	var e error
	if cmd == "up" || cmd == "start" {
		entry, e = manager.Resolve(ctx, name, true)
	} else {
		entry, e = manager.Lookup(name)
	}
	if e != nil && !machineFlag {
		var pe *protocol.Failure
		if errors.As(e, &pe) && pe.Code == "MACHINE_NOT_FOUND" {
			return ctx, cleaned, nil
		}
	}
	if e != nil {
		return ctx, args, e
	}
	if pin != "" && pin != entry.ID {
		return ctx, args, protocol.Fail("MACHINE_IDENTITY_CHANGED", "Machine name now refers to a different identity.")
	}
	selected := selection{Manager: manager, Entry: entry, Pin: entry.ID}
	at := 1
	if cmd == "browsers" || cmd == "packages" {
		at = min(2, len(cleaned))
	}
	result := append([]string{}, cleaned[:at]...)
	result = append(result, "--state-dir", entry.Root)
	result = append(result, cleaned[at:]...)
	return context.WithValue(ctx, selectionKey{}, selected), result, nil
}
func runMachines(ctx context.Context, args []string, out, errout io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(out, "Usage: agent-linux machines list|register|budget [options]\nSelectors: --machine NAME on local commands; --state-dir PATH remains explicit legacy mode.\nregister --name NAME --state-dir PATH; budget --cpus N --memory N; use --json for automation.")
		return 0
	}
	f := flag.NewFlagSet("machines "+args[0], flag.ContinueOnError)
	f.SetOutput(errout)
	cat := f.String("catalog-dir", catalogRoot(), "private catalog (advanced/test isolation)")
	runtime := f.String("lima", "limactl", "pinned runtime")
	name := f.String("name", "", "registration name")
	root := f.String("state-dir", "", "existing namespace to register without moving disks")
	cpu := f.Int("cpus", 0, "aggregate vCPU budget; zero selects default")
	ram := f.Int("memory", 0, "aggregate RAM budget in GiB; zero selects default")
	structured := f.Bool("json", false, "structured result")
	if e := f.Parse(args[1:]); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	fail := func(e error) int {
		if *structured {
			_ = json.NewEncoder(out).Encode(Envelope{Error: &protocol.Failure{Code: "FLEET_ERROR", Message: e.Error()}})
		} else {
			fmt.Fprintln(errout, e)
		}
		return 1
	}
	if f.NArg() != 0 {
		return fail(errors.New("unexpected arguments"))
	}
	m := fleetManager(*cat, *runtime)
	var result any
	var e error
	switch args[0] {
	case "list":
		result, e = (&localDashboard{manager: m, runtime: *runtime, planned: map[string]config.Spec{}}).List(ctx)
	case "register":
		result, e = m.Register(ctx, *name, *root)
	case "budget":
		result, e = m.Configure(ctx, fleet.Budget{CPUs: *cpu, MemoryGiB: *ram})
	default:
		return fail(errors.New("unknown machines action"))
	}
	if e != nil {
		return fail(e)
	}
	_ = json.NewEncoder(out).Encode(Envelope{OK: true, Result: result})
	return 0
}
func afterReplacement(ctx context.Context, command string) error {
	s, ok := ctx.Value(selectionKey{}).(selection)
	if !ok || s.Manager == nil {
		return nil
	}
	id := protocol.ID()
	if command == "reset" {
		var state lifecycle.State
		if e := storage.Read(s.Entry.Root, "state.json", &state); e != nil {
			return e
		}
		id = state.MachineID
	}
	return s.Manager.Rebind(ctx, s.Entry, id)
}

// Pin legacy path clients too. An existing view/MCP connection must never follow
// a reset or external replacement even though the selected path stays the same.
func pinExisting(c *lifecycle.Controller) error {
	if c.ExpectedID != "" {
		return nil
	}
	var s lifecycle.State
	if e := storage.Read(c.Root, "state.json", &s); e != nil {
		return e
	}
	if !protocol.ValidID(s.MachineID) {
		return errors.New("invalid stored machine identity")
	}
	c.ExpectedID = s.MachineID
	return nil
}
func Run(ctx context.Context, args []string, out, errout io.Writer) int {
	if len(args) > 0 && args[0] == "machines" {
		return runMachines(ctx, args[1:], out, errout)
	}
	if len(args) == 0 {
		if f, ok := out.(*os.File); ok && term.IsTerminal(int(f.Fd())) && term.IsTerminal(int(os.Stdin.Fd())) {
			args = []string{"start"}
		}
	}
	resolved, clean, e := selectMachine(ctx, args)
	if e != nil {
		jsonMode := false
		for _, v := range args {
			if v == "--json" {
				jsonMode = true
			}
		}
		if jsonMode {
			code := "INVALID_SELECTION"
			var pe *protocol.Failure
			if errors.As(e, &pe) {
				code = pe.Code
			}
			_ = json.NewEncoder(out).Encode(Envelope{Error: &protocol.Failure{Code: code, Message: e.Error()}})
		} else {
			fmt.Fprintln(errout, e)
		}
		return 1
	}
	for _, v := range args {
		if v == "--help" || v == "-h" {
			fmt.Fprintln(errout, "Local selection: --machine NAME or --state-dir PATH; advanced --catalog-dir PATH isolates the catalog.")
			break
		}
	}
	code := runCommand(resolved, clean, out, errout)
	if code == 0 && len(args) > 0 && (args[0] == "delete" || args[0] == "reset") {
		if e = afterReplacement(resolved, args[0]); e != nil {
			fmt.Fprintln(errout, "explicit replacement completed, but catalog needs inspection:", e)
			return 1
		}
	}
	return code
}
