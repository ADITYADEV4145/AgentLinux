// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/cloud"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

func runCloud(ctx context.Context, args []string, out, errout io.Writer) int {
	fail := func(e error) int { fmt.Fprintln(errout, e); return 1 }
	if len(args) == 0 || args[0] == "--help" {
		fmt.Fprintln(out, `Usage: agent-linux cloud ACTION [flags]
  init     Create two random 8-hour DEVELOPMENT identities (no provider calls)
  serve    Run the DEVELOPMENT control plane on literal 127.0.0.1 only
  up       Create one ephemeral session, write a NEW private client config, wait ready
  status   Show authenticated session metadata, expiry, limits and capabilities
  stop     Terminate the selected ephemeral machine; guest data is discarded
  revoke   Revoke this development identity and stop all its active sessions
Use mcp/view --cloud-config FILE for the common agent/viewer workflow.
Fake provider is SIMULATED, not a Linux desktop. No live billing or cloud login.`)
		return 0
	}
	action := args[0]
	f := flag.NewFlagSet("cloud "+action, flag.ContinueOnError)
	f.SetOutput(errout)
	root := f.String("state-dir", "", "explicit private development control-plane namespace")
	config := f.String("config", "", "private cloud client config file")
	output := f.String("output", "", "up: NEW private session config file (never printed as JSON)")
	endpoint := f.String("endpoint", "http://127.0.0.1:8787", "init: loopback development endpoint")
	listen := f.String("listen", "127.0.0.1:8787", "serve: literal IPv4 loopback only")
	provider := f.String("provider", "fake-development", "init/serve: fake-development or e2b-desktop-2.7.2")
	dev := f.Bool("dev", false, "explicitly enable development identities/control plane")
	allow := f.Bool("allow-provider-create", false, "operator opt-in to real E2B API calls; may incur charges; never implicit")
	python := f.String("python", "", "E2B: absolute Python 3.11+ venv executable with reviewed SDK installed")
	template := f.String("template", "", "E2B: REQUIRED reviewed 2..4 CPU / 4..8 GiB desktop template; not local image parity")
	seconds := f.Int("seconds", 300, "up: maximum lifetime 30..900 seconds; stop discards guest data")
	view := f.Bool("view", false, "up: immediately open the interactive viewer after readiness")
	key := f.String("request-id", "", "up: required stable 32-hex idempotency key; retry same intent with same key")
	if e := f.Parse(args[1:]); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if f.NArg() != 0 {
		return fail(errors.New("unexpected positional arguments"))
	}
	emit := func(v any) int { _ = json.NewEncoder(out).Encode(v); return 0 }
	if action == "init" || action == "serve" {
		if !*dev {
			return fail(errors.New("only explicit --dev control-plane mode is implemented; production deployment/auth are disabled"))
		}
		private, e := storage.Root(*root)
		if e != nil {
			return fail(e)
		}
		unlock, e := storage.Lock(private)
		if e != nil {
			return fail(e)
		}
		defer unlock()
		if action == "init" {
			files, e := cloud.Init(private, *endpoint, *provider)
			if e != nil {
				return fail(e)
			}
			return emit(map[string]any{"development": true, "credentialFiles": files, "credentialsExpireAfter": "8h", "provider": *provider, "providerResourcesCreated": false})
		}
		host, _, e := net.SplitHostPort(*listen)
		if e != nil || host != "127.0.0.1" {
			return fail(errors.New("development server must bind literal 127.0.0.1; public/TLS deployment is not implemented"))
		}
		var p cloud.Provider
		switch *provider {
		case "fake-development":
			p = cloud.NewFake()
		case "e2b-desktop-2.7.2":
			if !*allow {
				return fail(errors.New("E2B API disabled; requires explicit --allow-provider-create and separately authorized credentials"))
			}
			p, e = cloud.NewE2B(*python, *template)
			if e != nil {
				return fail(e)
			}
		default:
			return fail(errors.New("unknown cloud provider"))
		}
		m, e := cloud.New(private, p)
		if e != nil {
			return fail(e)
		}
		ln, e := net.Listen("tcp4", *listen)
		if e != nil {
			return fail(e)
		}
		defer ln.Close()
		api := cloud.NewAPI(m)
		server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Host != ln.Addr().String() {
				http.Error(w, "invalid host", 403)
				return
			}
			api.ServeHTTP(w, r)
		}), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 75 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 8192}
		runctx, cancel := context.WithCancel(ctx)
		defer cancel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-runctx.Done():
					return
				case <-ticker.C:
					if e := m.Tick(runctx); e != nil {
						fmt.Fprintln(errout, "cloud reconciliation unavailable; retained reservations/cleanup intents; provider TTL was requested; verify cleanup with the provider during outages")
					}
				}
			}
		}()
		go func() {
			<-runctx.Done()
			stop, done := context.WithTimeout(context.Background(), 3*time.Second)
			defer done()
			_ = server.Shutdown(stop)
		}()
		fmt.Fprintf(errout, "DEVELOPMENT cloud control plane %s; provider=%s; no production billing. Provider credentials never go to clients.\n", ln.Addr(), p.Name())
		e = server.Serve(ln)
		cancel()
		<-done
		if e != nil && !errors.Is(e, http.ErrServerClosed) {
			return fail(errors.New("development cloud server stopped unexpectedly"))
		}
		return 0
	}
	c, e := cloud.LoadClient(*config)
	if e != nil {
		return fail(e)
	}
	switch action {
	case "status":
		s, e := c.Session(ctx)
		if e != nil {
			return fail(e)
		}
		return emit(s)
	case "stop":
		s, e := c.Stop(ctx)
		if e != nil {
			return fail(e)
		}
		return emit(s)
	case "revoke":
		if e = c.Revoke(ctx); e != nil {
			return fail(e)
		}
		return emit(map[string]bool{"identityRevoked": true})
	case "up":
		if e = cloud.PrivateOutput(*output); e != nil {
			return fail(e)
		}
		if !filepath.IsAbs(*output) || strings.Contains(*output, "\n") {
			return fail(errors.New("up requires a NEW absolute --output path for private session credentials"))
		}
		if e = storage.SafeFile(filepath.Dir(*output), filepath.Base(*output)); e != nil {
			return fail(e)
		}
		if _, e = os.Lstat(*output); !os.IsNotExist(e) {
			return fail(errors.New("output already exists; reuse that session config for status/stop; never overwrite credentials"))
		}
		s, e := c.Create(ctx, *key, *seconds)
		if e != nil {
			return fail(e)
		}
		c.Config.Session = s.ID
		file, e := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return fail(errors.New("session created but config could not be saved; use same request-id to recover, or wait for expiry"))
		}
		e = json.NewEncoder(file).Encode(c.Config)
		if e == nil {
			e = file.Sync()
		}
		ce := file.Close()
		if e != nil || ce != nil {
			return fail(errors.New("private session config save failed; session expiry still applies"))
		}
		fmt.Fprintf(errout, "Cloud session %s; ephemeral; hard expiry %s; simulated=%t. Idle viewing does not extend lifetime.\n", s.ID, s.Expires.Format(time.RFC3339), s.Simulated)
		wait, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		for {
			s, e = c.Session(wait)
			if e != nil {
				return fail(e)
			}
			if s.State == "ready" {
				if *view {
					return runView(ctx, []string{"--cloud-config", *output, "--interactive"}, out, errout)
				}
				return emit(s)
			}
			if s.State == "stopped" {
				return fail(errors.New("session stopped before readiness; guest data is ephemeral"))
			}
			select {
			case <-wait.Done():
				return fail(errors.New("cloud readiness pending; inspect saved session config, do not create another session blindly"))
			case <-time.After(time.Second):
			}
		}
	default:
		return fail(errors.New("unknown cloud action; use cloud --help"))
	}
}
