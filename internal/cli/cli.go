// SPDX-License-Identifier: Apache-2.0

// Package cli implements the initial local-only command surface.
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
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/host"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/lima"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

const Help = `agent-linux: an agent-ready Linux machine (development build)

Usage: agent-linux COMMAND [options]
  doctor          Read-only host, runtime and resource diagnostics
  up --local      Create/resume the local VM; wait for guest handshake + probe
  status          Reconcile runtime state and check the actual guest service
  stop            Request graceful shutdown and confirm runtime stopped
  logs            Show bounded, sanitized lifecycle or backend diagnostics

Use COMMAND --help for options. Cloud, desktop viewing and MCP are not implemented.
`

type Options struct {
	Command, Root, Lima                     string
	JSON, Local, CreationFlags, BackendLogs bool
	Timeout                                 time.Duration
	Spec                                    config.Spec
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
	case "doctor", "up", "status", "stop", "logs":
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
		ex, _ := os.Executable()
		f.StringVar(&o.Spec.GuestBinary, "guest-binary", filepath.Join(filepath.Dir(ex), "agent-linux-guest"), "creation only: static Linux ARM64 guest helper")
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
	if f.NArg() != 0 {
		return o, errors.New("unexpected positional arguments; use command --help")
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
			return o, errors.New("up requires --local; no cloud backend exists")
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

func Run(ctx context.Context, args []string, out, errout io.Writer) int {
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
	backend := &lima.Backend{Root: root, Executable: executable}
	c := lifecycle.Controller{Root: root, Backend: backend, Preflight: func(s config.Spec, newDisk bool) error { return host.Inspect(root, executable).Validate(s, newDisk) }}
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
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
