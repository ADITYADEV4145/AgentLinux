// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/config"
)

// start deliberately composes existing lifecycle/readiness and viewer paths; it
// neither resets machines nor treats a spawned process as ready.
func runStart(ctx context.Context, args []string, out, errout io.Writer) int {
	f := flag.NewFlagSet("start", flag.ContinueOnError)
	f.SetOutput(errout)
	f.Usage = func() {
		fmt.Fprintln(errout, "Usage: agent-linux start [options]\nStart/resume persistent local Linux, wait for real readiness, then open an interactive terminal viewer.\nKitty: F2 hands input between you and agents; Ctrl+] exits without stopping Linux.\nOther terminals: secured interactive loopback web fallback.\nNAT is the default for this convenience command and can reach host/LAN services.\nNo cloud resource is allocated implicitly; --cloud-config opens an existing cloud session.")
		f.PrintDefaults()
	}
	root := f.String("state-dir", DefaultRoot(), "private persistent local machine namespace; never reset by start")
	lima := f.String("lima", "limactl", "reviewed Lima executable")
	cpus := f.Int("cpus", config.Default().CPUs, "creation-only CPU count")
	memory := f.Int("memory", config.Default().MemoryGiB, "creation-only memory GiB")
	disk := f.Int("disk", config.Default().DiskGiB, "creation-only disk GiB")
	timeout := f.Duration("timeout", 20*time.Minute, "startup deadline; no startup-time promise")
	metrics := f.String("metrics", "", "optional NEW viewer metrics JSON file")
	web := f.Bool("web", false, "interactive loopback web viewer")
	viewOnly := f.Bool("view-only", false, "keep human input disabled")
	cloudConfig := f.String("cloud-config", "", "existing private cloud session config; never allocates a cloud resource")
	if e := f.Parse(args); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if f.NArg() != 0 {
		fmt.Fprintln(errout, "unexpected start arguments")
		return 1
	}
	// Reject an invalid output destination before starting/creating a VM.
	if *metrics != "" {
		if !filepath.IsAbs(*metrics) {
			fmt.Fprintln(errout, "metrics path must be absolute; no startup was attempted")
			return 1
		}
		if _, e := os.Lstat(*metrics); !os.IsNotExist(e) {
			fmt.Fprintln(errout, "metrics path must be a new file; no startup was attempted")
			return 1
		}
	}
	viewerArgs := []string{}
	if *cloudConfig != "" {
		mixed := false
		f.Visit(func(v *flag.Flag) {
			if v.Name == "state-dir" || v.Name == "lima" || v.Name == "cpus" || v.Name == "memory" || v.Name == "disk" {
				mixed = true
			}
		})
		if mixed {
			fmt.Fprintln(errout, "cloud and local flags are mutually exclusive")
			return 1
		}
		viewerArgs = append(viewerArgs, "--cloud-config", *cloudConfig)
	} else {
		fmt.Fprintln(errout, "Starting persistent Linux with NAT (host/LAN reachable); closing the viewer leaves Linux running.")
		up := []string{"up", "--local", "--state-dir", *root, "--lima", *lima, "--timeout", timeout.String()}
		_, stateError := os.Lstat(filepath.Join(*root, "state.json"))
		explicitCreation := false
		f.Visit(func(v *flag.Flag) {
			if v.Name == "cpus" || v.Name == "memory" || v.Name == "disk" {
				explicitCreation = true
			}
		})
		if os.IsNotExist(stateError) || explicitCreation {
			up = append(up, "--network=nat", "--cpus", strconv.Itoa(*cpus), "--memory", strconv.Itoa(*memory), "--disk", strconv.Itoa(*disk))
		}
		if code := Run(ctx, up, io.Discard, errout); code != 0 {
			return code
		}
		viewerArgs = append(viewerArgs, "--state-dir", *root, "--lima", *lima)
	}
	if !*viewOnly {
		viewerArgs = append(viewerArgs, "--interactive")
	}
	if *web {
		viewerArgs = append(viewerArgs, "--web")
	}
	if *metrics != "" {
		viewerArgs = append(viewerArgs, "--metrics", *metrics)
	}
	return runView(ctx, viewerArgs, out, errout)
}
