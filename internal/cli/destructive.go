// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/host"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/lima"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

func runDestructive(ctx context.Context, args []string, out, errout io.Writer) int {
	action := args[0]
	f := flag.NewFlagSet(action, flag.ContinueOnError)
	f.SetOutput(errout)
	f.Usage = func() {
		fmt.Fprintln(errout, "Usage: agent-linux "+action+" --state-dir PATH --machine-id ID [--yes]\nStop first. Without --yes, prints a removal plan and changes nothing.\n--yes explicitly authorizes permanent loss of all guest data, keys, VM disk and cached image.\nReset reprovisions the pinned base image, rotates identity/keys and waits for a real handshake. It is not a snapshot.")
		f.PrintDefaults()
	}
	root := f.String("state-dir", DefaultRoot(), "existing selected private state directory")
	runtime := f.String("lima", "limactl", "pinned runtime executable")
	id := f.String("machine-id", "", "required exact machine ID from status")
	yes := f.Bool("yes", false, "explicitly confirm permanent removal described in the plan")
	structured := f.Bool("json", false, "structured output; progress uses stderr")
	guest := defaultGuestBinary()
	if action == "reset" {
		f.StringVar(&guest, "guest-binary", guest, "new static Linux ARM64 helper, outside selected state")
	}
	timeout := f.Duration("timeout", 25*time.Minute, "overall deadline 1s..1h; reset includes reprovisioning")
	e := f.Parse(args[1:])
	if errors.Is(e, flag.ErrHelp) {
		return 0
	}
	fail := func(e error) int {
		if *structured {
			json.NewEncoder(out).Encode(Envelope{Error: &protocol.Failure{Code: "DESTRUCTIVE_OPERATION_FAILED", Message: e.Error()}})
		} else {
			fmt.Fprintln(errout, e)
		}
		return 1
	}
	if e != nil {
		return fail(e)
	}
	if f.NArg() != 0 || !protocol.ValidID(*id) || *timeout < time.Second || *timeout > time.Hour {
		return fail(errors.New("exact --machine-id and valid timeout are required; see --help"))
	}
	r, e := storage.ExistingRoot(*root)
	if e != nil {
		return fail(e)
	}
	// Plan preview is read-only and works even if the runtime executable is unavailable.
	c := lifecycle.Controller{Root: r}
	plan, e := c.PlanRemoval(*id, action)
	if e != nil {
		return fail(e)
	}
	if *structured {
		json.NewEncoder(errout).Encode(plan)
	} else {
		b, _ := json.MarshalIndent(plan, "", "  ")
		fmt.Fprintln(errout, string(b))
	}
	if !*yes {
		if *structured {
			json.NewEncoder(out).Encode(Envelope{Result: plan, Error: &protocol.Failure{Code: "CONFIRMATION_REQUIRED", Message: "review the removal plan, then explicitly pass --yes with the same machine ID"}})
		} else {
			fmt.Fprintln(errout, "Nothing removed. Review the plan, then pass --yes to authorize it.")
		}
		return 2
	}
	exe, e := lima.Find(*runtime)
	if e != nil {
		return fail(e)
	}
	progress := func(s string) { fmt.Fprintln(errout, "agent-linux:", s) }
	c.Backend = &lima.Backend{Root: r, Executable: exe, Progress: progress}
	c.Progress = progress
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	c.Preflight = func(s config.Spec, newDisk bool) error { return host.Inspect(r, exe).Validate(s, newDisk) }
	v, e := c.Remove(ctx, *id, action == "reset", guest)
	if e != nil {
		return fail(e)
	}
	if *structured {
		json.NewEncoder(out).Encode(Envelope{OK: true, Result: v})
	} else {
		b, _ := json.MarshalIndent(v, "", "  ")
		fmt.Fprintln(out, string(b))
	}
	return 0
}
