// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/ADITYADEV4145/AgentLinux/internal/guest"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

func main() {
	if runtime.GOOS != "linux" {
		fmt.Fprintln(os.Stderr, "guest helper must run inside Linux")
		os.Exit(1)
	}
	f := flag.NewFlagSet("agent-linux-guest", flag.ExitOnError)
	mode := f.String("mode", "serve", "serve or connect")
	id := f.String("machine-id", "", "provisioned machine identity")
	f.Parse(os.Args[1:])
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	var e error
	if *mode == "connect" {
		e = guest.Bridge(ctx, os.Stdin, os.Stdout, guest.Socket)
	} else if *mode == "serve" {
		s, err := guest.New(*id)
		e = err
		if e == nil {
			e = s.Serve(ctx, guest.Socket)
		}
	} else {
		e = fmt.Errorf("unknown mode")
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, "guest service failed; inspect systemd unit status inside guest")
		os.Exit(1)
	}
}
