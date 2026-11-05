// SPDX-License-Identifier: Apache-2.0
package viewer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

var ErrUnsupported = errors.New("Kitty graphics or terminal pixel dimensions unavailable")

type Terminal interface {
	Write(context.Context, []byte) error
	Size() (Size, error)
	Read(context.Context) ([]byte, error)
	Close() error
}
type Session struct {
	in, out *os.File
	saved   *term.State
	flags   int
	entered bool
	once    sync.Once
}

func OpenTerminal(ctx context.Context, in, out *os.File) (*Session, error) {
	if !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) {
		return nil, ErrUnsupported
	}
	saved, e := term.MakeRaw(int(in.Fd()))
	if e != nil {
		return nil, e
	}
	t := &Session{in: in, out: out, saved: saved}
	flags, e := unix.FcntlInt(out.Fd(), unix.F_GETFL, 0)
	if e != nil {
		_ = term.Restore(int(in.Fd()), saved)
		return nil, e
	}
	t.flags = flags
	if _, e = unix.FcntlInt(out.Fd(), unix.F_SETFL, flags|unix.O_NONBLOCK); e != nil {
		t.Close()
		return nil, e
	}
	fail := func(e error) (*Session, error) { t.Close(); return nil, e }
	probe, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if e = t.Write(probe, []byte(Query)); e != nil {
		return fail(e)
	}
	var response []byte
	for len(response) < 4096 {
		b, e := t.Read(probe)
		if e != nil {
			return fail(ErrUnsupported)
		}
		response = append(response, b...)
		if Supported(response) {
			break
		}
	}
	if !Supported(response) {
		return fail(ErrUnsupported)
	}
	s, e := t.Size()
	if e != nil || s.PixelWidth == 0 || s.PixelHeight == 0 {
		return fail(ErrUnsupported)
	}
	t.entered = true
	if e = t.Write(ctx, []byte("\x1b[?1049h\x1b[?25l\x1b[2J")); e != nil {
		return fail(e)
	}
	return t, nil
}
func (t *Session) Size() (Size, error) {
	w, e := unix.IoctlGetWinsize(int(t.out.Fd()), unix.TIOCGWINSZ)
	if e != nil {
		return Size{}, e
	}
	return Size{int(w.Col), int(w.Row), int(w.Xpixel), int(w.Ypixel)}, nil
}
func (t *Session) Write(ctx context.Context, b []byte) error {
	for len(b) > 0 {
		if e := ctx.Err(); e != nil {
			return e
		}
		n, e := unix.Write(int(t.out.Fd()), b)
		if n > 0 {
			b = b[n:]
		}
		if e != nil && !errors.Is(e, unix.EAGAIN) && !errors.Is(e, unix.EINTR) {
			return e
		}
		if n <= 0 {
			fds := []unix.PollFd{{Fd: int32(t.out.Fd()), Events: unix.POLLOUT}}
			_, e = unix.Poll(fds, 100)
			if e != nil && !errors.Is(e, unix.EINTR) {
				return e
			}
		}
	}
	return nil
}
func (t *Session) Read(ctx context.Context) ([]byte, error) {
	for ctx.Err() == nil {
		p := []unix.PollFd{{Fd: int32(t.in.Fd()), Events: unix.POLLIN}}
		n, e := unix.Poll(p, 100)
		if errors.Is(e, unix.EINTR) {
			continue
		}
		if e != nil {
			return nil, e
		}
		if n == 0 {
			continue
		}
		if p[0].Revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			return nil, errors.New("terminal disconnected")
		}
		buf := make([]byte, 256)
		n, e = unix.Read(int(t.in.Fd()), buf)
		if errors.Is(e, unix.EAGAIN) || errors.Is(e, unix.EINTR) {
			continue
		}
		if e != nil {
			return nil, e
		}
		if n == 0 {
			return nil, errors.New("terminal closed")
		}
		return buf[:n], nil
	}
	return nil, ctx.Err()
}
func (t *Session) Close() error {
	var result error
	t.once.Do(func() {
		if t.entered {
			c, cancel := context.WithTimeout(context.Background(), time.Second)
			_ = t.Write(c, []byte(DeleteImage()+"\x1b[?25h\x1b[?1049l"))
			cancel()
		}
		_, _ = unix.FcntlInt(t.out.Fd(), unix.F_SETFL, t.flags)
		if t.saved != nil {
			result = term.Restore(int(t.in.Fd()), t.saved)
		}
	})
	return result
}

// ExitKeys consumes terminal replies/escape sequences; keys are never guest input.
type ExitKeys struct {
	state, count int
	esc          bool
}

func (k *ExitKeys) Feed(data []byte) bool {
	for _, b := range data {
		k.count++
		if k.count > 4096 {
			k.state = 0
			k.count = 0
		}
		switch k.state {
		case 0:
			if b == 27 {
				k.state = 1
				k.count = 0
			} else if b == 'q' || b == 'Q' || b == 3 {
				return true
			}
		case 1:
			if b == '[' {
				k.state = 2
			} else if b == '_' || b == ']' || b == 'P' {
				k.state = 3
				k.esc = false
			} else {
				k.state = 0
			}
		case 2:
			if b >= 0x40 && b <= 0x7e {
				k.state = 0
			}
		case 3:
			if k.esc && b == '\\' || b == 7 {
				k.state = 0
			}
			k.esc = b == 27
		}
	}
	return false
}
func RunTerminal(parent context.Context, h *Hub, t Terminal, machineID string) error {
	defer t.Close()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		var keys ExitKeys
		for {
			b, e := t.Read(ctx)
			if e != nil || keys.Feed(b) {
				cancel()
				return
			}
		}
	}()
	defer func() { cancel(); <-readDone }()
	var digest string
	var size Size
	var snapshot Snapshot
	var lastErr error
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return nil
		}
		latest, e := h.Next(ctx)
		if e == nil {
			snapshot = latest
		}
		lastErr = e
		newSize, e := t.Size()
		if e != nil {
			return e
		}
		changedSize := newSize != size
		size = newSize
		p := Fit(size)
		writeCtx, done := context.WithTimeout(ctx, 5*time.Second)
		status := "WAITING"
		age := "--"
		if snapshot.Frame != nil {
			age = fmt.Sprintf("%.1fs", time.Since(snapshot.Observed).Seconds())
			status = "LIVE"
		}
		if lastErr != nil {
			status = Code(lastErr)
			if status != "MACHINE_BUSY" {
				snapshot = Snapshot{}
				digest = ""
				if e = t.Write(writeCtx, []byte(DeleteImage()+"\x1b[2J")); e != nil {
					done()
					return e
				}
			}
		}
		if p.Columns == 0 {
			status = "RESIZE: terminal too small"
			digest = ""
			_ = t.Write(writeCtx, []byte(DeleteImage()))
		}
		if snapshot.Frame != nil && p.Columns > 0 && (digest != snapshot.Digest || changedSize) {
			if e = Render(writeCtx, t.Write, p, snapshot.Frame.PNG); e != nil {
				done()
				return e
			}
			digest = snapshot.Digest
			h.Sent(len(snapshot.Frame.PNG))
		}
		e = t.Write(writeCtx, []byte(statusLines(size, machineID, status, age)))
		done()
		if e != nil {
			return e
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// Keep the actionable state visible when a window cannot fit the full identity.
func statusLines(size Size, machineID, status, age string) string {
	if size.Columns <= 0 || size.Rows <= 0 {
		return ""
	}
	line := fmt.Sprintf("Agent Linux %s | %s | last capture %s", machineID, status, age)
	footer := "VIEW ONLY | q / Ctrl+C exits; VM stays running | agent uses MCP"
	if len(line) >= size.Columns {
		line = fmt.Sprintf("%s | %s | capture %s", status, machineID, age)
	}
	if size.Columns < 32 {
		footer = "q exits; VM stays on"
	}
	clip := func(s string) string {
		// Leave the final cell unused to avoid automatic wrapping/scrolling.
		n := size.Columns - 1
		if len(s) > n {
			return s[:n]
		}
		return s
	}
	out := "\x1b[1;1H\x1b[2K" + clip(line)
	if size.Rows > 1 {
		out += fmt.Sprintf("\x1b[%d;1H\x1b[2K", size.Rows) + clip(footer)
	}
	return out
}
