// SPDX-License-Identifier: Apache-2.0
// Package guestexec is linked only into the Linux guest service, never the host CLI.
package guestexec

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/desktop"
	"github.com/ADITYADEV4145/AgentLinux/internal/process"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

type scope interface {
	Apply(*exec.Cmd)
	Kill() error
	Close() error
}

func Run(ctx context.Context, r protocol.ExecRequest) (*protocol.Execution, error) {
	if e := r.Validate(); e != nil {
		return nil, e
	}
	group, e := newScope()
	if e != nil {
		return nil, protocol.Fail("EXEC_UNAVAILABLE", "delegated cgroup cleanup is unavailable; inspect guest service")
	}
	return run(ctx, r, group)
}

type limited struct {
	b      process.Buffer
	cancel context.CancelFunc
}

func (l *limited) Write(p []byte) (int, error) {
	n, e := l.b.Write(p)
	if l.b.Truncated {
		l.cancel()
	}
	return n, e
}
func run(parent context.Context, r protocol.ExecRequest, group scope) (*protocol.Execution, error) {
	if e := r.Validate(); e != nil {
		_ = group.Close()
		return nil, e
	}
	started := time.Now()
	ctx, cancel := context.WithTimeout(parent, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
	env := map[string]string{}
	for _, entry := range desktop.Env() {
		k, v, _ := strings.Cut(entry, "=")
		env[k] = v
	}
	for k, v := range r.Env {
		env[k] = v
	}
	executable := r.Argv[0]
	if !strings.ContainsRune(executable, '/') {
		executable = ""
		for _, dir := range filepath.SplitList(env["PATH"]) {
			if !filepath.IsAbs(dir) {
				continue
			}
			candidate := filepath.Join(dir, r.Argv[0])
			if i, e := os.Stat(candidate); e == nil && !i.IsDir() && i.Mode()&0111 != 0 {
				executable = candidate
				break
			}
		}
		if executable == "" {
			cleaned := group.Close() == nil
			return &protocol.Execution{ExitCode: 127, Error: "guest executable not found in PATH", CleanupConfirmed: cleaned}, nil
		}
	} else if !filepath.IsAbs(executable) {
		_ = group.Close()
		return nil, protocol.Fail("INVALID_ARGUMENT", "executable path must be absolute or a PATH command name")
	}
	cmd := process.Prepare(ctx, executable, r.Argv[1:]...)
	cmd.Dir = r.Cwd
	if cmd.Dir == "" {
		cmd.Dir = "/home/agent"
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		cmd.Env = append(cmd.Env, k+"="+env[k])
	}
	group.Apply(cmd)
	oldCancel := cmd.Cancel
	cmd.Cancel = func() error { _ = group.Kill(); return oldCancel() }
	out := &limited{b: process.Buffer{Limit: r.OutputLimit}, cancel: cancel}
	errout := &limited{b: process.Buffer{Limit: r.OutputLimit}, cancel: cancel}
	cmd.Stdout = out
	cmd.Stderr = errout
	err := cmd.Run()
	killErr := group.Kill()
	closeErr := group.Close()
	result := &protocol.Execution{ExitCode: 0, Stdout: out.b.Bytes(), Stderr: errout.b.Bytes(), Truncated: out.b.Truncated || errout.b.Truncated, DurationMS: time.Since(started).Milliseconds(), CleanupConfirmed: killErr == nil && closeErr == nil}
	if cmd.ProcessState == nil {
		result.ExitCode = 127
		result.Error = "guest process could not start; verify executable, cwd and permissions"
	} else {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.TimedOut = true
		result.Error = "guest command deadline exceeded"
	}
	if parent.Err() != nil {
		result.Canceled = true
		result.Error = "guest command canceled or control connection closed"
	}
	if result.Truncated {
		result.Error = "output limit exceeded; guest command terminated"
	}
	if !result.CleanupConfirmed {
		result.Error = "guest command cleanup unconfirmed"
	}
	if err != nil && result.ExitCode == 0 {
		result.ExitCode = -1
	}
	return result, nil
}
