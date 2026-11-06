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
	"runtime"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/cloud"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/lima"
	"github.com/ADITYADEV4145/AgentLinux/internal/machine"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
	"github.com/ADITYADEV4145/AgentLinux/internal/viewer"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func runView(ctx context.Context, args []string, out, errout io.Writer) int {
	f := flag.NewFlagSet("view", flag.ContinueOnError)
	f.SetOutput(errout)
	f.Usage = func() {
		fmt.Fprintln(errout, "Usage: agent-linux view [options]\nLive VIEW-ONLY guest desktop: Kitty direct PNG graphics first; authenticated loopback web fallback.\nq or Ctrl+C exits the terminal viewer. Ctrl+C closes the web server. Neither stops the VM.\nDefault: input is not forwarded; agents use MCP. With --interactive: F2 handoff, Ctrl+] exits, text/click/scroll/common keys target Linux only. No tmux/screen graphics support.\nStart the VM with up --local first; stopped/unavailable machines show reconnecting status.")
		f.PrintDefaults()
	}
	root := f.String("state-dir", DefaultRoot(), "existing private machine namespace")
	runtimePath := f.String("lima", "limactl", "pinned Lima executable")
	cloudConfig := f.String("cloud-config", "", "private cloud session configuration; no local fallback")
	interactive := f.Bool("interactive", false, "human input lease; F2 handoff; Ctrl+] exits; supported Kitty terminal")
	web := f.Bool("web", false, "use live local web viewer even in Kitty; prints a private clickable URL")
	target := f.String("terminal", "auto", "auto or kitty; unsupported terminals fall back to live web")
	interval := f.Duration("interval", time.Second, "minimum capture interval, 250ms..5s; static screens adapt up to 5s")
	metrics := f.String("metrics", "", "optional NEW absolute private JSON metrics file written on exit")
	fail := func(e error) int { fmt.Fprintln(errout, e); return 1 }
	if e := f.Parse(args); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if f.NArg() != 0 || (*target != "auto" && *target != "kitty") || *interval < 250*time.Millisecond || *interval > 5*time.Second {
		return fail(errors.New("invalid viewer arguments; use view --help"))
	}
	if !filepath.IsAbs(*root) {
		return fail(errors.New("state-dir must be absolute"))
	}
	if *metrics != "" {
		if !filepath.IsAbs(*metrics) {
			return fail(errors.New("metrics path must be absolute"))
		}
		if _, e := os.Lstat(*metrics); !os.IsNotExist(e) {
			return fail(errors.New("metrics path must be a new file"))
		}
	}
	var state lifecycle.State
	var computer machine.Machine
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
			return fail(errors.New("cloud/local viewer flags are mutually exclusive"))
		}
		if *interactive {
			c.ControlOwner = protocol.ID()
		}
		state, e = c.Status(ctx)
		if e != nil {
			return fail(e)
		}
		computer = c
		if state.Remote != nil {
			fmt.Fprintf(errout, "Cloud is ephemeral; hard expiry %s; idle stop by %s. Closing the viewer does not stop it immediately; these limits still apply. Simulated=%t.\n", state.Remote.ExpiresAt.Format(time.RFC3339), state.Remote.IdleStopAt.Format(time.RFC3339), state.Remote.Simulated)
		}
	} else {
		if _, e := os.Stat(filepath.Join(*root, "state.json")); e != nil {
			return fail(errors.New("viewer requires an existing machine; run up --local first"))
		}
		private, e := storage.Root(*root)
		if e != nil {
			return fail(e)
		}

		if e = storage.Read(private, "state.json", &state); e != nil || !protocol.ValidID(state.MachineID) {
			return fail(errors.New("viewer requires an existing machine; run up --local first"))
		}
		executable, e := lima.Find(*runtimePath)
		if e != nil {
			return fail(e)
		}
		c := &lifecycle.Controller{Root: private, Backend: &lima.Backend{Root: private, Executable: executable}}
		if *interactive {
			c.ControlOwner = protocol.ID()
		}
		computer = machine.Local{Controller: c}
	}
	hub := viewer.NewHub(computer, *interval)
	started := time.Now()
	if *metrics != "" {
		defer func() {
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			var usage unix.Rusage
			_ = unix.Getrusage(unix.RUSAGE_SELF, &usage)
			data := map[string]any{"hostOS": runtime.GOOS, "hostArch": runtime.GOARCH, "goVersion": runtime.Version(), "durationSeconds": time.Since(started).Seconds(), "goHeapBytes": mem.HeapAlloc, "goSysBytes": mem.Sys, "maxRSSNativeUnits": usage.Maxrss, "rssUnitNote": "bytes on macOS; KiB on Linux", "cpuUserSeconds": float64(usage.Utime.Sec) + float64(usage.Utime.Usec)/1e6, "cpuSystemSeconds": float64(usage.Stime.Sec) + float64(usage.Stime.Usec)/1e6, "stream": hub.Metrics()}
			file, e := os.OpenFile(*metrics, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if e != nil {
				fmt.Fprintln(errout, "could not create viewer metrics file")
				return
			}
			defer file.Close()
			_ = json.NewEncoder(file).Encode(data)
		}()
	}
	output, outputIsFile := out.(*os.File)
	candidate := outputIsFile && viewer.Candidate(term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(output.Fd())), os.Getenv("TERM"), os.Getenv("TERM_PROGRAM"), os.Getenv("KITTY_WINDOW_ID"), os.Getenv("TMUX"), os.Getenv("STY"))
	if !*web && (candidate || *target == "kitty" && outputIsFile) {
		terminal, e := viewer.OpenTerminal(ctx, os.Stdin, output)
		if e == nil {
			if *interactive {
				e = viewer.RunInteractiveTerminal(ctx, hub, terminal, state.MachineID)
			} else {
				e = viewer.RunTerminal(ctx, hub, terminal, state.MachineID)
			}
			if e == nil || errors.Is(e, context.Canceled) {
				return 0
			}
			return fail(errors.New("terminal viewer closed after an output/terminal error; terminal settings restored; VM lifecycle unchanged"))
		}
		if ctx.Err() != nil {
			return 0
		}
		fmt.Fprintln(errout, "Kitty capability/pixel query unavailable; terminal restored. Using live local web fallback.")
	} else if !*web {
		fmt.Fprintln(errout, "Terminal graphics unavailable (Kitty direct terminal required). Using live local web fallback.")
	}
	server, e := viewer.NewWeb(hub, state.MachineID)
	if e == nil && *interactive {
		server.EnableInput()
	}
	if e != nil {
		return fail(e)
	}
	defer server.Close()
	if *interactive {
		fmt.Fprintln(out, "Agent Linux interactive Linux desktop:", server.URL)
	} else {
		fmt.Fprintln(out, "Agent Linux live VIEW-ONLY desktop:", server.URL)
	}
	fmt.Fprintln(out, "Open this private URL locally. Ctrl+C closes the viewer; VM stays running. Interactive mode requires Take control; hand control back for the agent.")
	e = server.Serve(ctx)
	server.Close()
	cleanup, cancelCleanup := context.WithTimeout(context.Background(), 3*time.Second)
	cleanupErr := hub.WaitIdle(cleanup)
	cancelCleanup()
	if cleanupErr != nil {
		return fail(errors.New("viewer closed, but capture cleanup was not confirmed; VM lifecycle unchanged"))
	}
	if e != nil && !errors.Is(e, context.Canceled) {
		return fail(errors.New("local viewer server stopped unexpectedly; VM lifecycle unchanged"))
	}
	return 0
}
