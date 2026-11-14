// SPDX-License-Identifier: Apache-2.0

// Package cli implements the initial local-only command surface.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/ADITYADEV4145/AgentLinux/internal/machine"
	"golang.org/x/term"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/ADITYADEV4145/AgentLinux/internal/buildinfo"
	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/host"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/lima"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

const Help = `agent-linux: an agent-ready Linux machine (local developer release candidate)

Usage: agent-linux COMMAND [options]
  checkpoint      Stopped-VM disk checkpoints and explicit recovery
  desktop restart Explicit graphical-session restart (unsaved work warning)
  helper upgrade  Explicit checkpoint-backed guest-helper upgrade
  machines        Named local inventory, registration and aggregate resource budgets
  dashboard       Local web console; optional private development cloud config
  doctor          Read-only host, runtime and resource diagnostics
  start           Start/resume Linux and open its interactive viewer (NAT)
  cloud           Development cloud prototype; explicit provider opt-in
  billing         Sandbox-only plans, usage ledger, limits and payment setup
  up --local      Create/resume the local VM; wait for guest handshake + probe
  status          Reconcile runtime state and check the actual guest service
  stop            Request graceful shutdown and confirm runtime stopped
  logs            Show bounded, sanitized lifecycle or backend diagnostics
  version         Report release, commit, guest and protocol versions
  copy-in/out     Explicit bounded one-way regular-file transfers
  delete/reset    Guarded destructive actions; require selected machine ID

Local selectors: --machine NAME or --state-dir PATH (mutually exclusive).
Use COMMAND --help for options. Cloud is an opt-in development prototype, not a deployed service.
  view            Live view-only desktop: Kitty graphics or secure loopback web
  mcp             Serve the existing local machine over MCP stdio
  mcp-config      Print client configuration; never modifies client settings
Software: browsers list/install; packages install/refresh/upgrade/profiles.
Computer commands: screenshot, move, click, drag, scroll, type, key, open, exec.
All input/commands target Linux; exec never runs its argv on the Mac.
`

type Options struct {
	Command, Root, Lima                     string
	JSON, Local, CreationFlags, BackendLogs bool
	Timeout                                 time.Duration
	Spec                                    config.Spec
	Output, App                             string
	Input                                   protocol.Input
	Execution                               protocol.ExecRequest
}

func DefaultRoot() string {
	d, e := os.UserHomeDir()
	if e != nil {
		return ""
	}
	return filepath.Join(d, ".local", "share", "agent-linux", "local")
}
func Parse(args []string, help io.Writer) (Options, error) {
	o := Options{Spec: config.Default(), Root: DefaultRoot(), Lima: "limactl", Timeout: 30 * time.Second}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(help, Help)
		return o, flag.ErrHelp
	}
	o.Command = args[0]
	switch o.Command {
	case "doctor", "up", "status", "stop", "logs", "screenshot", "move", "click", "drag", "scroll", "type", "key", "open", "exec":
	default:
		return o, fmt.Errorf("unknown command %q; use --help", o.Command)
	}
	f := flag.NewFlagSet(o.Command, flag.ContinueOnError)
	f.SetOutput(help)
	f.StringVar(&o.Root, "state-dir", o.Root, "absolute private state directory (0700; never a project/home directory)")
	f.StringVar(&o.Lima, "lima", o.Lima, "pinned limactl executable path or PATH name")
	f.BoolVar(&o.JSON, "json", false, "emit one JSON result; errors have a nonzero exit status")
	if o.Command == "up" {
		o.Timeout = 20 * time.Minute
		f.BoolVar(&o.Local, "local", false, "explicitly select the local backend")
		f.IntVar(&o.Spec.CPUs, "cpus", o.Spec.CPUs, "creation only: 1..16, bounded by host CPUs")
		f.IntVar(&o.Spec.MemoryGiB, "memory", o.Spec.MemoryGiB, "creation only: GiB, 1..64 and at most half host RAM")
		f.IntVar(&o.Spec.DiskGiB, "disk", o.Spec.DiskGiB, "creation only: disk GiB, 8..128")
		f.StringVar(&o.Spec.Network, "network", "", "creation only: explicitly choose nat (host/LAN reachable)")
		f.StringVar(&o.Spec.Image, "image", o.Spec.Image, "creation only: reviewed default URL or absolute local cloud image")
		f.StringVar(&o.Spec.Digest, "image-sha512", o.Spec.Digest, "creation only: trusted image SHA-512 (required for custom local image)")
		f.StringVar(&o.Spec.GuestBinary, "guest-binary", defaultGuestBinary(), "creation only: static Linux ARM64 guest helper")
	}
	if o.Command == "screenshot" {
		f.StringVar(&o.Output, "output", "", "new PNG output path; existing files are never overwritten")
	}
	if isInput(o.Command) {
		f.StringVar(&o.Input.FrameID, "frame-id", "", "explicit observed frame ID (default: capture immediately before action)")
		f.StringVar(&o.Input.Button, "button", "left", "left, middle or right")
		f.IntVar(&o.Input.Clicks, "clicks", 1, "click count: 1 or 2")
		f.StringVar(&o.Input.PasteMode, "paste-mode", "standard", "standard Ctrl+V or terminal Ctrl+Shift+V")
	}
	commandTimeout := 20 * time.Second
	if o.Command == "exec" {
		o.Execution.Env = map[string]string{}
		f.StringVar(&o.Execution.Cwd, "cwd", "/home/agent", "absolute Linux guest working directory")
		f.IntVar(&o.Execution.OutputLimit, "output-limit", 64<<10, "bytes per output stream, at most 262144")
		f.DurationVar(&commandTimeout, "command-timeout", commandTimeout, "guest process deadline, at most 60s")
		f.Func("env", "guest KEY=value environment override; repeatable", func(v string) error {
			k, value, ok := strings.Cut(v, "=")
			if !ok {
				return errors.New("env requires KEY=value")
			}
			o.Execution.Env[k] = value
			return nil
		})
	}
	if o.Command == "logs" {
		f.BoolVar(&o.BackendLogs, "backend", false, "show sanitized backend diagnostics instead of lifecycle events")
	}
	if o.Command != "logs" {
		f.DurationVar(&o.Timeout, "timeout", o.Timeout, "overall operation deadline (not a startup-time promise)")
	}
	if e := f.Parse(args[1:]); e != nil {
		return o, e
	}
	argsLeft := f.Args()
	number := func(index int) (int, error) {
		if index >= len(argsLeft) {
			return 0, errors.New("missing numeric argument")
		}
		n, e := strconv.Atoi(argsLeft[index])
		if e != nil {
			return 0, errors.New("coordinates/deltas must be integers")
		}
		return n, nil
	}
	var parseErr error
	switch o.Command {
	case "move", "click", "drag", "scroll":
		expected := 2
		if o.Command == "drag" || o.Command == "scroll" {
			expected = 4
		}
		if len(argsLeft) != expected {
			return o, fmt.Errorf("%s requires %d numeric arguments", o.Command, expected)
		}
		o.Input.Kind = o.Command
		o.Input.X, parseErr = number(0)
		if parseErr != nil {
			return o, parseErr
		}
		o.Input.Y, parseErr = number(1)
		if parseErr != nil {
			return o, parseErr
		}
		if expected == 4 {
			a, e := number(2)
			if e != nil {
				return o, e
			}
			b, e := number(3)
			if e != nil {
				return o, e
			}
			if o.Command == "drag" {
				o.Input.ToX = a
				o.Input.ToY = b
			} else {
				o.Input.DX = a
				o.Input.DY = b
			}
		}
	case "type", "key", "open":
		if len(argsLeft) != 1 {
			return o, errors.New("command requires exactly one quoted text, key combination or app name")
		}
		if o.Command == "type" {
			o.Input.Kind = "text"
			o.Input.Text = argsLeft[0]
		} else if o.Command == "key" {
			o.Input.Kind = "key"
			o.Input.Key = argsLeft[0]
		} else {
			o.App = argsLeft[0]
			if !protocol.ValidApp(o.App) {
				return o, errors.New("open supports editor, terminal, browser/chromium, firefox, files, settings or launcher")
			}
		}
	case "exec":
		o.Execution.Argv = argsLeft
		o.Execution.TimeoutMS = int(commandTimeout / time.Millisecond)
		if e := o.Execution.Validate(); e != nil {
			return o, e
		}
	default:
		if len(argsLeft) != 0 {
			return o, errors.New("unexpected positional arguments; use command --help")
		}
	}
	if isInput(o.Command) {
		check := o.Input
		if check.FrameID == "" {
			check.FrameID = strings.Repeat("0", 32)
		}
		if e := check.Validate(); e != nil {
			return o, e
		}
	}
	if o.Command == "screenshot" && o.Output == "" {
		return o, errors.New("screenshot requires --output; PNG is never written to protocol stdout")
	}
	if o.Root == "" || !filepath.IsAbs(o.Root) || filepath.Clean(o.Root) != o.Root {
		return o, errors.New("--state-dir must be an absolute clean path")
	}
	if o.Timeout < time.Second || o.Timeout > time.Hour {
		return o, errors.New("--timeout must be 1s..1h")
	}
	seen := map[string]bool{}
	f.Visit(func(v *flag.Flag) {
		seen[v.Name] = true
		switch v.Name {
		case "cpus", "memory", "disk", "network", "image", "image-sha512", "guest-binary":
			o.CreationFlags = true
		}
	})
	if o.Command == "up" {
		if !o.Local {
			return o, errors.New("local up requires --local; see cloud --help for the separate development prototype")
		}
		if seen["image"] && o.Spec.Image != config.ImageURL && !seen["image-sha512"] {
			return o, errors.New("custom image requires an explicit trusted --image-sha512")
		}
		if o.Spec.CPUs < 1 || o.Spec.CPUs > 16 || o.Spec.MemoryGiB < 1 || o.Spec.MemoryGiB > 64 || o.Spec.DiskGiB < 8 || o.Spec.DiskGiB > 128 {
			return o, errors.New("invalid resources: CPUs 1..16, memory 1..64 GiB, disk 8..128 GiB")
		}
		if o.Spec.Network != "" && o.Spec.Network != "nat" {
			return o, errors.New("only explicit --network=nat is supported; offline isolation is not validated")
		}
		if !filepath.IsAbs(o.Spec.GuestBinary) {
			p, e := filepath.Abs(o.Spec.GuestBinary)
			if e != nil {
				return o, e
			}
			o.Spec.GuestBinary = p
		}
	}
	return o, nil
}

type Envelope struct {
	OK      bool              `json:"ok"`
	Result  any               `json:"result,omitempty"`
	Error   *protocol.Failure `json:"error,omitempty"`
	Machine *lifecycle.State  `json:"machine,omitempty"`
}

func runCommand(ctx context.Context, args []string, out, errout io.Writer) int {
	// No implicit boot when piped, scripted or invoked by a noninteractive client.
	if len(args) == 0 {
		if f, ok := out.(*os.File); ok && term.IsTerminal(int(f.Fd())) && term.IsTerminal(int(os.Stdin.Fd())) {
			return runStart(ctx, nil, out, errout)
		}
	}
	if len(args) > 0 && (args[0] == "checkpoint" || args[0] == "desktop" || args[0] == "helper") {
		return runRecovery(ctx, args, out, errout)
	}
	if len(args) > 0 && args[0] == "dashboard" {
		return runDashboard(ctx, args[1:], out, errout)
	}

	if len(args) > 0 && (args[0] == "delete" || args[0] == "reset") {
		return runDestructive(ctx, args, out, errout)
	}
	if len(args) > 0 && (args[0] == "copy-in" || args[0] == "copy-out") {
		return runFiles(ctx, args, out, errout)
	}
	if len(args) > 0 && args[0] == "version" {
		if len(args) > 2 || (len(args) == 2 && args[1] != "--json") {
			fmt.Fprintln(errout, "usage: agent-linux version [--json]")
			return 1
		}
		json.NewEncoder(out).Encode(map[string]string{"version": buildinfo.Version, "commit": buildinfo.Commit, "guest": protocol.GuestVersion, "protocol": protocol.Version, "lima": config.LimaVersion})
		return 0
	}
	if len(args) > 0 && args[0] == "view" {
		return runView(ctx, args[1:], out, errout)
	}
	if len(args) > 0 && (args[0] == "mcp" || args[0] == "mcp-config") {
		return runMCP(ctx, args, out, errout)
	}
	if len(args) > 0 && (args[0] == "browsers" || args[0] == "packages") {
		return runSoftware(ctx, args, out, errout)
	}
	if len(args) > 0 && args[0] == "start" {
		return runStart(ctx, args[1:], out, errout)
	}
	if len(args) > 0 && args[0] == "billing" {
		return runBilling(ctx, args[1:], out, errout)
	}
	if len(args) > 0 && args[0] == "cloud" {
		return runCloud(ctx, args[1:], out, errout)
	}
	o, e := Parse(args, errout)
	if errors.Is(e, flag.ErrHelp) {
		return 0
	}
	jsonMode := o.JSON
	for _, a := range args {
		if a == "--json" || a == "--json=true" {
			jsonMode = true
		}
	}
	fail := func(e error, machine *lifecycle.State) int {
		code := "OPERATION_FAILED"
		var pe *protocol.Failure
		if errors.As(e, &pe) {
			code = pe.Code
		}
		if errors.Is(e, context.DeadlineExceeded) {
			code = "DEADLINE_EXCEEDED"
		}
		if errors.Is(e, context.Canceled) {
			code = "INTERRUPTED"
		}
		if strings.Contains(e.Error(), "MACHINE_BUSY") {
			code = "MACHINE_BUSY"
		}
		if jsonMode {
			_ = json.NewEncoder(out).Encode(Envelope{Error: &protocol.Failure{Code: code, Message: e.Error()}, Machine: machine})
		} else {
			fmt.Fprintln(errout, code+": "+e.Error())
		}
		return 1
	}
	if e != nil {
		return fail(e, nil)
	}
	emit := func(result any) int {
		if o.JSON {
			_ = json.NewEncoder(out).Encode(Envelope{OK: true, Result: result})
		} else {
			b, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(out, string(b))
		}
		return 0
	}
	if o.Command == "doctor" {
		r := host.Inspect(o.Root, o.Lima)
		if _, err := lima.CheckBinary(defaultGuestBinary()); err != nil {
			r.Problems = append(r.Problems, "guest helper missing/invalid beside CLI; reinstall the complete bundle or run make build")
			r.Capabilities = []string{}
		}
		if len(r.Problems) > 0 {
			if o.JSON {
				_ = json.NewEncoder(out).Encode(Envelope{Result: r, Error: &protocol.Failure{Code: "PREREQUISITES_MISSING", Message: strings.Join(r.Problems, "; ")}})
			} else {
				emit(r)
			}
			return 1
		}
		return emit(r)
	}
	root, e := storage.Root(o.Root)
	if e != nil {
		return fail(e, nil)
	}
	if o.Command == "logs" {
		unlock, e := storage.Lock(root)
		if e != nil {
			return fail(e, nil)
		}
		defer unlock()
		name := "events.jsonl"
		if o.BackendLogs {
			name = "backend.jsonl"
		}
		if e = storage.SafeFile(root, name); e != nil {
			return fail(e, nil)
		}
		b, e := os.ReadFile(filepath.Join(root, name))
		if os.IsNotExist(e) {
			return emit([]any{})
		}
		if e != nil {
			return fail(e, nil)
		}
		if len(b) > 2<<20 {
			return fail(errors.New("log exceeds size limit"), nil)
		}
		var records []any
		for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if line == "" {
				continue
			}
			var record any
			if e = json.Unmarshal([]byte(line), &record); e != nil {
				return fail(errors.New("invalid structured log; inspect private state locally"), nil)
			}
			records = append(records, record)
		}
		return emit(records)
	}
	executable, e := lima.Find(o.Lima)
	if e != nil {
		return fail(e, nil)
	}
	var progressMu sync.Mutex
	progress := func(s string) { progressMu.Lock(); defer progressMu.Unlock(); fmt.Fprintln(errout, "agent-linux:", s) }
	backend := &lima.Backend{Root: root, Executable: executable}
	if o.Command == "up" {
		backend.Progress = progress
	}
	c := lifecycle.Controller{Root: root, Backend: backend, Preflight: func(s config.Spec, newDisk bool) error { return host.Inspect(root, executable).Validate(s, newDisk) }}
	pinController(ctx, &c)
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	if o.Command == "up" {
		c.Progress = progress
		done := make(chan struct{})
		workerDone := make(chan struct{})
		defer func() { close(done); <-workerDone }()
		go func() {
			defer close(workerDone)
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ctx.Done():
					return
				case <-ticker.C:
					progress(fmt.Sprint("setup is still running; package provisioning can take several minutes (deadline ", o.Timeout, ")"))
				}
			}
		}()
	}
	local := machine.Local{Controller: &c}
	switch o.Command {
	case "screenshot":
		frame, e := local.Screenshot(ctx)
		if e != nil {
			return fail(e, nil)
		}
		file, e := os.OpenFile(o.Output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return fail(e, nil)
		}
		_, e = file.Write(frame.PNG)
		if e == nil {
			e = file.Sync()
		}
		closeErr := file.Close()
		if e != nil {
			return fail(e, nil)
		}
		if closeErr != nil {
			return fail(closeErr, nil)
		}
		frame.PNG = nil
		return emit(frame)
	case "move", "click", "drag", "scroll", "type", "key":
		r, e := local.Input(ctx, o.Input)
		if e != nil {
			return fail(e, nil)
		}
		return emit(r)
	case "open":
		if e := local.Open(ctx, o.App); e != nil {
			return fail(e, nil)
		}
		return emit(map[string]string{"app": o.App, "status": "visible"})
	case "exec":
		r, e := local.Exec(ctx, o.Execution)
		if e != nil {
			return fail(e, nil)
		}
		if o.JSON {
			emit(r)
		} else {
			fmt.Fprint(out, safeGuestText(r.Stdout))
			fmt.Fprint(errout, safeGuestText(r.Stderr))
			if r.Error != "" {
				fmt.Fprintln(errout, "guest exec: "+r.Error)
			}
		}
		if !r.CleanupConfirmed {
			return 1
		}
		if r.TimedOut {
			return 124
		}
		if r.Canceled || r.Truncated {
			return 1
		}
		if r.ExitCode != 0 {
			if r.ExitCode > 0 && r.ExitCode <= 125 {
				return r.ExitCode
			}
			return 1
		}
		return 0
	}

	var s lifecycle.State
	switch o.Command {
	case "up":
		s, e = c.Up(ctx, o.Spec, o.CreationFlags)
	case "status":
		s, e = c.Status(ctx)
	case "stop":
		s, e = c.Stop(ctx)
	}
	if e != nil {
		return fail(e, &s)
	}
	return emit(s)
}

func isInput(command string) bool {
	switch command {
	case "move", "click", "drag", "scroll", "type", "key":
		return true
	}
	return false
}

// Escape guest terminal controls; never let guest output invoke host OSC/clipboard operations.
func safeGuestText(data []byte) string {
	var b strings.Builder
	for _, r := range strings.ToValidUTF8(string(data), "�") {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			fmt.Fprintf(&b, "\\u%04x", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Install launchers/current are symlinks; discover the helper beside the actual
// host binary in its immutable version directory, never beside the launcher.
func siblingGuest(executable string) string {
	if resolved, e := filepath.EvalSymlinks(executable); e == nil {
		executable = resolved
	}
	return filepath.Join(filepath.Dir(executable), "agent-linux-guest")
}
func defaultGuestBinary() string { ex, _ := os.Executable(); return siblingGuest(ex) }
