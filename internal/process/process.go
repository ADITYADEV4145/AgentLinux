// SPDX-License-Identifier: Apache-2.0

// Package process runs owned subprocess groups with bounded output and cancellation.
package process

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type Buffer struct {
	mu        sync.Mutex
	Data      []byte
	Limit     int
	Truncated bool
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	room := b.Limit - len(b.Data)
	if room < 0 {
		room = 0
	}
	if len(p) > room {
		b.Truncated = true
		p = p[:room]
	}
	b.Data = append(b.Data, p...)
	return n, nil
}
func (b *Buffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.Data...)
}

type Result struct {
	Out, Err  []byte
	Truncated bool
}

func Prepare(ctx context.Context, name string, args ...string) *exec.Cmd {
	c := exec.CommandContext(ctx, name, args...)
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error {
		if c.Process == nil {
			return nil
		}
		err := syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	c.WaitDelay = 2 * time.Second
	return c
}
func Run(ctx context.Context, env []string, name string, args ...string) (Result, error) {
	c := Prepare(ctx, name, args...)
	c.Env = env
	out := &Buffer{Limit: 1 << 20}
	errout := &Buffer{Limit: 64 << 10}
	c.Stdout = out
	c.Stderr = errout
	err := c.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return Result{out.Bytes(), errout.Bytes(), out.Truncated || errout.Truncated}, err
}
