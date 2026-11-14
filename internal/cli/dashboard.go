// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/ADITYADEV4145/AgentLinux/internal/cloud"
	guestconfig "github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/console"
	"io"
)

func runDashboard(ctx context.Context, args []string, out, errout io.Writer) int {
	f := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	f.SetOutput(errout)
	local := f.Bool("local", false, "explicitly select local named machines (default without cloud config)")
	catalog := f.String("catalog-dir", catalogRoot(), "private local machine catalog")
	runtime := f.String("lima", "limactl", "pinned local runtime")
	config := f.String("cloud-config", "", "optional existing private development identity; credential stays server-side")
	f.Usage = func() {
		fmt.Fprintln(errout, "Usage: agent-linux dashboard [--cloud-config FILE]\nOpen the printed private loopback URL. Ctrl+C closes the console/viewers; cloud sessions continue until stop/expiry.\nNo public deployment, real compute creation or live payment occurs on launch.")
		f.PrintDefaults()
	}
	if e := f.Parse(args); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if f.NArg() != 0 {
		fmt.Fprintln(errout, "unexpected arguments; use dashboard --help")
		return 1
	}
	if *local && *config != "" {
		fmt.Fprintln(errout, "--local and --cloud-config are mutually exclusive")
		return 1
	}
	var client *cloud.Client
	var e error
	if *config != "" {
		client, e = cloud.LoadClient(*config)
		if e != nil {
			fmt.Fprintln(errout, e)
			return 1
		}
	}
	s, e := console.New(client)
	if e != nil {
		fmt.Fprintln(errout, "cannot bind private loopback console")
		return 1
	}
	if *config == "" {
		s.EnableLocal(&localDashboard{manager: fleetManager(*catalog, *runtime), runtime: *runtime, planned: map[string]guestconfig.Spec{}})
	}
	defer s.Close()
	fmt.Fprintln(out, "Agent Linux local console:", s.URL)
	if e = s.Serve(ctx); e != nil {
		fmt.Fprintln(errout, "console stopped unexpectedly")
		return 1
	}
	return 0
}
