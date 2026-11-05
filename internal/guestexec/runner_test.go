// SPDX-License-Identifier: Apache-2.0
package guestexec

import (
	"context"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

type testScope struct{ killed bool }

func (s *testScope) Apply(*exec.Cmd) {}
func (s *testScope) Kill() error     { s.killed = true; return nil }
func (s *testScope) Close() error    { return nil }
func request(t *testing.T, argv ...string) protocol.ExecRequest {
	t.Helper()
	cwd, _ := filepath.EvalSymlinks(t.TempDir())
	return protocol.ExecRequest{Argv: argv, Cwd: cwd, Env: map[string]string{"TEST_VALUE": "café 日本語"}, TimeoutMS: 1000, OutputLimit: 1024}
}
func TestResultAndEnvironment(t *testing.T) {
	q := request(t, "/bin/sh", "-c", "printf '%s' \"$TEST_VALUE\"; printf 'problem' >&2; exit 7")
	group := &testScope{}
	r, e := run(context.Background(), q, group)
	if e != nil || r.ExitCode != 7 || string(r.Stdout) != "café 日本語" || string(r.Stderr) != "problem" || !r.CleanupConfirmed || !group.killed {
		t.Fatalf("result: %#v %v", r, e)
	}
}
func TestTimeoutCancellationAndOutput(t *testing.T) {
	q := request(t, "/bin/sh", "-c", "sleep 30 & wait")
	q.TimeoutMS = 50
	r, e := run(context.Background(), q, &testScope{})
	if e != nil || !r.TimedOut || !r.CleanupConfirmed {
		t.Fatalf("timeout: %#v %v", r, e)
	}
	q = request(t, "/bin/sh", "-c", "while :; do printf 'flood'; done")
	q.OutputLimit = 64
	r, e = run(context.Background(), q, &testScope{})
	if e != nil || !r.Truncated || len(r.Stdout) > 64 {
		t.Fatalf("output bound: %#v %v", r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	q = request(t, "/bin/sleep", "30")
	r, e = run(ctx, q, &testScope{})
	if e != nil || !r.Canceled {
		t.Fatalf("cancel: %#v %v", r, e)
	}
}
