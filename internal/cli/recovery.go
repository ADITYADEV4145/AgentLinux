// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/host"
	"io"
	"strings"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/lima"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

func runRecovery(ctx context.Context, args []string, out, errout io.Writer) int {
	if len(args) < 2 || args[1] == "--help" || args[1] == "-h" {
		fmt.Fprintln(out, "Usage: agent-linux checkpoint create|list|restore|delete NAME [options]\n       agent-linux checkpoint recover --mode resume|rollback [options]\n       agent-linux desktop restart [options]\n       agent-linux helper upgrade [options]\nSelect --machine NAME or --state-dir PATH. Disk operations/upgrade require a stopped VM.\nRestore/delete/recover/restart/upgrade require exact --machine-id ID and --yes.\nCheckpoints save disk/configuration, not memory or unsaved application buffers.\nNo command silently stops a VM. helper upgrade explicitly boots after staging; restore does not.")
		return 0
	}
	group, action := args[0], args[1]
	f := flag.NewFlagSet(group+" "+action, flag.ContinueOnError)
	f.SetOutput(errout)
	root := f.String("state-dir", DefaultRoot(), "selected existing private namespace")
	runtime := f.String("lima", "limactl", "pinned runtime")
	id := f.String("machine-id", "", "exact current machine identity for destructive/recovery actions")
	yes := f.Bool("yes", false, "confirm the displayed plan; can lose unsaved/since-checkpoint guest work")
	mode := f.String("mode", "", "recover only: resume or rollback")
	binary := f.String("guest-binary", defaultGuestBinary(), "helper upgrade only: static Linux ARM64 helper")
	jsonMode := f.Bool("json", false, "structured result")
	timeout := f.Duration("timeout", 20*time.Minute, "bounded copy/boot operation deadline (1s..1h)")
	rest := args[2:]
	name := ""
	if group == "checkpoint" && len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		name = rest[0]
		rest = rest[1:]
	}
	if e := f.Parse(rest); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	fail := func(e error) int {
		code := "RECOVERY_FAILED"
		var pe *protocol.Failure
		if errors.As(e, &pe) {
			code = pe.Code
		}
		if *jsonMode {
			_ = json.NewEncoder(out).Encode(Envelope{Error: &protocol.Failure{Code: code, Message: e.Error()}})
		} else {
			fmt.Fprintln(errout, e)
		}
		return 1
	}
	if f.NArg() == 1 && name == "" {
		name = f.Arg(0)
	} else if f.NArg() != 0 {
		return fail(errors.New("unexpected recovery arguments"))
	}
	if *timeout < time.Second || *timeout > time.Hour {
		return fail(errors.New("timeout must be 1s..1h"))
	}
	valid := group == "checkpoint" && (action == "create" || action == "list" || action == "restore" || action == "delete" || action == "recover") || group == "desktop" && action == "restart" || group == "helper" && action == "upgrade"
	if !valid {
		return fail(errors.New("unknown recovery action; use checkpoint --help"))
	}
	if group == "checkpoint" && (action == "create" || action == "restore" || action == "delete") && name == "" {
		return fail(errors.New("checkpoint name is required"))
	}
	private, e := storage.ExistingRoot(*root)
	if e != nil {
		return fail(e)
	}
	var state lifecycle.State
	if e = storage.Read(private, "state.json", &state); e != nil {
		return fail(e)
	}
	destructive := action != "list" && action != "create"
	if destructive {
		warning := "Replace saved guest disk/configuration; changes since the checkpoint can be lost. VM must already be stopped."
		if action == "delete" {
			warning = "Permanently delete only this selected disk checkpoint."
		}
		if group == "desktop" {
			warning = "Restart the guest graphical session. Unsaved documents and application buffers can be lost; VM stays running."
		}
		if group == "helper" {
			warning = "Create a stopped-disk checkpoint, stage a helper-only provisioning upgrade, then explicitly boot and verify it. Saved projects/identity are preserved; rollback data stays until verification."
		}
		plan := map[string]any{"action": group + " " + action, "checkpoint": name, "machineId": state.MachineID, "stateDirectory": private, "warning": warning}
		b, _ := json.MarshalIndent(plan, "", "  ")
		fmt.Fprintln(errout, string(b))
		if !*yes {
			if *jsonMode {
				_ = json.NewEncoder(out).Encode(Envelope{Result: plan, Error: &protocol.Failure{Code: "CONFIRMATION_REQUIRED", Message: "Review the plan, then pass --machine-id and --yes explicitly."}})
			}
			return 2
		}
		if !protocol.ValidID(*id) || *id != state.MachineID {
			return fail(errors.New("exact --machine-id must match the selected namespace"))
		}
	}
	exe, e := lima.Find(*runtime)
	if e != nil {
		return fail(e)
	}
	backend := &lima.Backend{Root: private, Executable: exe}
	c := &lifecycle.Controller{Root: private, Backend: backend, Progress: func(message string) { fmt.Fprintln(errout, "agent-linux:", message) }}
	c.Preflight = func(spec config.Spec, newDisk bool) error { return host.Inspect(private, exe).Validate(spec, newDisk) }
	pinController(ctx, c)
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	var result any
	switch {
	case group == "desktop":
		result, e = c.RestartDesktop(ctx, *id)
	case group == "helper":
		if _, e = lima.CheckBinary(*binary); e == nil {
			result, e = c.UpgradeHelper(ctx, *id, *binary)
		}
	case action == "list":
		var list any
		var pending any
		l, j, err := c.Checkpoints(ctx)
		list = l
		pending = j
		e = err
		result = map[string]any{"checkpoints": list, "recovery": pending}
	default:
		result, e = c.Checkpoint(ctx, action, name, *mode, *id)
	}
	if e != nil {
		return fail(e)
	}
	if *jsonMode {
		_ = json.NewEncoder(out).Encode(Envelope{OK: true, Result: result})
	} else {
		b, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(out, string(b))
	}
	return 0
}
