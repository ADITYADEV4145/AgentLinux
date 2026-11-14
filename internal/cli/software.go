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
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/lima"
	"github.com/ADITYADEV4145/AgentLinux/internal/machine"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/software"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

type softwareOptions struct {
	group, action, root, runtime, profile string
	json                                  bool
	timeout                               time.Duration
	packages                              []string
}

func parseSoftware(args []string, errout io.Writer) (softwareOptions, error) {
	o := softwareOptions{root: DefaultRoot(), runtime: "limactl", timeout: 10 * time.Minute}
	if len(args) < 2 || args[1] == "--help" || args[1] == "-h" {
		fmt.Fprintln(errout, "Usage: agent-linux browsers list|install [options] [firefox|chromium]\n       agent-linux packages install|refresh|upgrade|profiles [options] [Debian package names]\nOptions precede package names. Package changes require guest network and explicit guest-admin access; never host installation.\nUse ACTION --help for options.")
		return o, flag.ErrHelp
	}
	o.group, o.action = args[0], args[1]
	f := flag.NewFlagSet(o.group+" "+o.action, flag.ContinueOnError)
	f.SetOutput(errout)
	f.StringVar(&o.root, "state-dir", o.root, "existing private machine namespace")
	f.StringVar(&o.runtime, "lima", o.runtime, "pinned Lima executable")
	f.BoolVar(&o.json, "json", false, "structured output")
	f.DurationVar(&o.timeout, "timeout", o.timeout, "overall deadline, 1s..15m; cancellation may leave bounded guest job running")
	f.StringVar(&o.profile, "profile", "", "packages install only: web-dev or build")
	if e := f.Parse(args[2:]); e != nil {
		return o, e
	}
	o.packages = f.Args()
	if o.timeout < time.Second || o.timeout > 15*time.Minute {
		return o, errors.New("timeout must be 1s..15m")
	}
	if o.group == "browsers" {
		if o.profile != "" {
			return o, errors.New("browser operation does not take a profile")
		}
		switch o.action {
		case "list":
			if len(o.packages) != 0 {
				return o, errors.New("browsers list takes no names")
			}
		case "install":
			if len(o.packages) != 1 {
				return o, errors.New("browsers install requires firefox or chromium")
			}
			p, e := software.BrowserPackage(o.packages[0])
			if e != nil {
				return o, protocol.Fail("UNSUPPORTED_OPERATION", e.Error())
			}
			o.packages = []string{p}
		default:
			return o, errors.New("choose browsers list or install")
		}
	} else {
		if o.action == "profiles" {
			if len(o.packages) > 0 || o.profile != "" {
				return o, errors.New("profiles takes no arguments")
			}
			return o, nil
		}
		if o.profile != "" {
			if o.action != "install" || len(o.packages) > 0 {
				return o, errors.New("profile is an alternative to package names for packages install")
			}
			p, ok := software.Profiles[o.profile]
			if !ok {
				return o, errors.New("unknown profile; choose web-dev or build")
			}
			o.packages = append([]string(nil), p...)
		}
		if e := software.Validate(o.action, o.packages); e != nil {
			return o, e
		}
	}
	return o, nil
}
func runSoftware(ctx context.Context, args []string, out, errout io.Writer) int {
	o, e := parseSoftware(args, errout)
	if errors.Is(e, flag.ErrHelp) {
		return 0
	}
	fail := func(e error) int {
		code := "OPERATION_FAILED"
		if strings.Contains(e.Error(), "MACHINE_BUSY") {
			code = "MACHINE_BUSY"
		}
		if errors.Is(e, context.DeadlineExceeded) {
			code = "TIMEOUT"
		}
		if errors.Is(e, context.Canceled) {
			code = "CANCELED"
		}
		var p *protocol.Failure
		if errors.As(e, &p) {
			code = p.Code
		}
		if o.json {
			_ = json.NewEncoder(out).Encode(Envelope{Error: &protocol.Failure{Code: code, Message: e.Error()}})
		} else {
			fmt.Fprintln(errout, code+": "+e.Error())
		}
		return 1
	}
	if e != nil {
		var pe *protocol.Failure
		if !errors.As(e, &pe) {
			e = protocol.Fail("INVALID_ARGUMENT", e.Error())
		}
		return fail(e)
	}
	emit := func(v any) int {
		if o.json {
			_ = json.NewEncoder(out).Encode(Envelope{OK: true, Result: v})
		} else {
			b, _ := json.MarshalIndent(v, "", "  ")
			fmt.Fprintln(out, string(b))
		}
		return 0
	}
	if o.action == "profiles" {
		return emit(software.Profiles)
	}
	if !filepath.IsAbs(o.root) {
		return fail(protocol.Fail("INVALID_ARGUMENT", "state-dir must be absolute"))
	}
	if _, e = os.Stat(filepath.Join(o.root, "state.json")); e != nil {
		return fail(protocol.Fail("NOT_READY", "create and start the selected VM first"))
	}
	root, e := storage.Root(o.root)
	if e != nil {
		return fail(e)
	}
	exe, e := lima.Find(o.runtime)
	if e != nil {
		return fail(e)
	}
	c := &lifecycle.Controller{Root: root, Backend: &lima.Backend{Root: root, Executable: exe}}
	pinController(ctx, c)
	ctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()
	if o.group == "browsers" && o.action == "list" {
		local := machine.Local{Controller: c}
		r, e := local.Exec(ctx, protocol.ExecRequest{Argv: []string{"/usr/bin/dpkg-query", "-W", "-f=${binary:Package}\t${Version}\t${db:Status-Status}\n", "chromium", "firefox-esr", "google-chrome-stable", "microsoft-edge-stable"}, Cwd: "/home/agent", TimeoutMS: 5000, OutputLimit: 16384})
		if e != nil {
			return fail(e)
		}
		if !r.CleanupConfirmed || r.TimedOut || r.Canceled || r.Truncated || r.ExitCode > 1 {
			return fail(protocol.Fail("INVENTORY_FAILED", "guest package inventory failed; no install status inferred"))
		}
		return emit(map[string]any{"architecture": "arm64", "browsers": software.Browsers("arm64", string(r.Stdout)), "note": "package inventory is not a runtime/browser smoke test; see docs/browsers.md for tested evidence"})
	}
	r, e := c.Maintain(ctx, o.action, o.packages)
	if e != nil {
		return fail(e)
	}
	return emit(r)
}
