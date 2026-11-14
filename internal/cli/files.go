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

	"github.com/ADITYADEV4145/AgentLinux/internal/filetransfer"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/lima"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

func runFiles(ctx context.Context, args []string, out, errout io.Writer) int {
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	f.SetOutput(errout)
	f.Usage = func() {
		fmt.Fprintln(errout, "Explicit one-way regular-file copy; 4 MiB maximum, no mounts or archive extraction.\ncopy-in --source /absolute/project.tar copies to a new guest Transfers directory.\ncopy-out --source /home/agent/result --output /absolute/new-file refuses overwrites.\nCopies contain no ownership/executable metadata; files are mode 0600. Archives remain opaque.")
		f.PrintDefaults()
	}
	root := f.String("state-dir", DefaultRoot(), "existing private VM state")
	runtime := f.String("lima", "limactl", "pinned runtime executable")
	source := f.String("source", "", "explicit absolute source file")
	output := f.String("output", "", "copy-out only: new absolute host file")
	jsonMode := f.Bool("json", false, "structured result")
	timeout := f.Duration("timeout", 5*time.Minute, "total deadline, 1s..15m")
	e := f.Parse(args[1:])
	if errors.Is(e, flag.ErrHelp) {
		return 0
	}
	fail := func(e error) int {
		code := "FILE_TRANSFER_FAILED"
		var p *protocol.Failure
		if errors.As(e, &p) {
			code = p.Code
		}
		if *jsonMode {
			json.NewEncoder(out).Encode(Envelope{Error: &protocol.Failure{Code: code, Message: e.Error()}})
		} else {
			fmt.Fprintln(errout, e)
		}
		return 1
	}
	if e != nil {
		return fail(e)
	}
	if f.NArg() != 0 || *source == "" || *timeout < time.Second || *timeout > 15*time.Minute || (args[0] == "copy-out" && *output == "") || (args[0] == "copy-in" && *output != "") {
		return fail(errors.New("invalid copy options; see --help"))
	}
	r, e := storage.ExistingRoot(*root)
	if e != nil {
		return fail(e)
	}
	ex, e := lima.Find(*runtime)
	if e != nil {
		return fail(e)
	}
	c := lifecycle.Controller{Root: r, Backend: &lima.Backend{Root: r, Executable: ex}}
	pinController(ctx, &c)
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	call := func(ctx context.Context, q protocol.FileRequest) (*protocol.FileResult, error) {
		info, e := c.GuestCall(ctx, protocol.Request{Method: "files", File: &q})
		if e != nil {
			return nil, e
		}
		if info.File == nil {
			return nil, errors.New("guest returned no file result; old guests need explicit reset/new state")
		}
		return info.File, nil
	}
	var v *protocol.FileResult
	if args[0] == "copy-in" {
		v, e = filetransfer.In(ctx, *source, call)
	} else {
		v, e = filetransfer.Out(ctx, *source, *output, call)
	}
	if e != nil {
		return fail(e)
	}
	if *jsonMode {
		json.NewEncoder(out).Encode(Envelope{OK: true, Result: v})
	} else {
		b, _ := json.MarshalIndent(v, "", "  ")
		fmt.Fprintln(out, string(b))
	}
	return 0
}
