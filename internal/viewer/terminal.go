// SPDX-License-Identifier: Apache-2.0
package viewer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
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
	return runTerminal(parent, h, t, machineID, nil)
}
func RunInteractiveTerminal(parent context.Context, h *Hub, t Terminal, machineID string) error {
	return runTerminal(parent, h, t, machineID, NewHuman(h.Machine))
}
func runTerminal(parent context.Context, h *Hub, t Terminal, machineID string, human *Human) error {
	defer t.Close()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	type packet struct {
		data []byte
		size Size
	}
	packets := make(chan packet, 4)
	if human != nil {
		if human.Control == nil {
			return fmt.Errorf("backend does not support input ownership")
		}
		modeTerminal, ok := t.(interface {
			InputModes(context.Context) (map[int]bool, error)
		})
		if !ok {
			return ErrUnsupported
		}
		original, e := modeTerminal.InputModes(ctx)
		if e != nil {
			return e
		}
		restore := ""
		for _, mode := range []int{1000, 1006, 2004} {
			suffix := "l"
			if original[mode] {
				suffix = "h"
			}
			restore += fmt.Sprintf("\x1b[?%d%s", mode, suffix)
		}
		defer func() {
			human.Release()
			c, done := context.WithTimeout(context.Background(), time.Second)
			defer done()
			_ = t.Write(c, []byte(restore))
		}()
		if e := t.Write(ctx, []byte("\x1b[?1000h\x1b[?1006h\x1b[?2004h")); e != nil {
			return e
		}
	}
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		var keys ExitKeys
		var exitDetector HumanKeys
		for {
			b, e := t.Read(ctx)
			if e == nil && human != nil {
				for _, event := range exitDetector.Feed(b) {
					if event.Action == "exit" {
						cancel()
						return
					}
				}
				size, _ := t.Size()
				select {
				case packets <- packet{b, size}:
				case <-ctx.Done():
					return
				}
				continue
			}
			if e != nil || keys.Feed(b) {
				cancel()
				return
			}
		}
	}()
	defer func() { cancel(); <-readDone }()
	type update struct {
		snapshot Snapshot
		err      error
	}
	updates := make(chan update, 1)
	captureDone := make(chan struct{})
	go func() {
		defer close(captureDone)
		captureTick := time.NewTicker(100 * time.Millisecond)
		defer captureTick.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			snapshot, err := h.Next(ctx)
			latest := update{snapshot, err}
			select {
			case updates <- latest:
			default:
				select {
				case <-updates:
				default:
				}
				select {
				case updates <- latest:
				case <-ctx.Done():
					return
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-captureTick.C:
			}
		}
	}()
	defer func() { cancel(); <-captureDone }()
	var renderer Renderer
	var digest string
	var size Size
	var snapshot Snapshot
	var lastErr error
	var humanKeys HumanKeys
	var keyAt time.Time
	autoTake := human != nil
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return nil
		}
		if human != nil {
			if e := human.Heartbeat(ctx); e != nil {
				human.Notice = Code(e) + "; F2 retake"
			}
		}
		select {
		case update := <-updates:
			if update.err == nil {
				snapshot = update.snapshot
				if human != nil {
					human.Observe(snapshot.Frame)
					if autoTake {
						autoTake = false
						if err := human.Handle(ctx, HumanEvent{Action: "toggle"}, size); err != nil {
							human.Notice = Code(err) + "; F2 retake"
						}
					}
				}
			}
			lastErr = update.err
		default:
		}
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

		if human != nil {
			events := []HumanEvent{}
			select {
			case packet := <-packets:
				keyAt = time.Now()
				events = humanKeys.Feed(packet.data)
				if packet.size != size {
					filtered := events[:0]
					for _, ev := range events {
						if ev.Input.Kind != "click" && ev.Input.Kind != "scroll" {
							filtered = append(filtered, ev)
						}
					}
					events = filtered
					human.Notice = "Resize: mouse event discarded"
				}
			default:
				if !keyAt.IsZero() && time.Since(keyAt) > 150*time.Millisecond {
					events = humanKeys.FlushEscape()
				}
			}
			for n := 0; n < 3; n++ {
				select {
				case packet := <-packets:
					more := humanKeys.Feed(packet.data)
					for _, event := range more {
						if packet.size != size && (event.Input.Kind == "click" || event.Input.Kind == "scroll") {
							continue
						}
						events = append(events, event)
					}
				default:
					n = 3
				}
			}
			merged := events[:0]
			for _, event := range events {
				if len(merged) > 0 && event.Action == "" && event.Input.Kind == "text" && merged[len(merged)-1].Input.Kind == "text" && len(merged[len(merged)-1].Input.Text)+len(event.Input.Text) <= protocol.MaxText {
					merged[len(merged)-1].Input.Text += event.Input.Text
				} else {
					merged = append(merged, event)
				}
			}
			events = merged
			for _, event := range events {
				if event.Action == "exit" {
					done()
					return nil
				}
				if e := human.Handle(ctx, event, size); e != nil {
					human.Notice = Code(e) + "; F2 retake"
				}
				h.Invalidate()
			}
		}
		if snapshot.Frame != nil && p.Columns > 0 && (digest != snapshot.Digest || changedSize) {
			if e = renderer.Render(writeCtx, t.Write, p, snapshot.Frame.PNG); e != nil {
				done()
				return e
			}
			digest = snapshot.Digest
			h.Sent(len(snapshot.Frame.PNG))
		}
		done()
		writeCtx, done = context.WithTimeout(ctx, 5*time.Second)
		footer := ""
		if human != nil {
			footer = human.Footer()
		}
		e = t.Write(writeCtx, []byte(statusLinesFooter(size, machineID, status, age, footer)))
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
	return statusLinesFooter(size, machineID, status, age, "")
}
func statusLinesFooter(size Size, machineID, status, age, customFooter string) string {
	if size.Columns <= 0 || size.Rows <= 0 {
		return ""
	}
	line := fmt.Sprintf("Agent Linux %s | %s | last capture %s", machineID, status, age)
	footer := "VIEW ONLY | q / Ctrl+C exits; VM stays running | agent uses MCP"
	if customFooter != "" {
		footer = customFooter
	}
	if len(line) >= size.Columns {
		line = fmt.Sprintf("%s | %s | capture %s", status, machineID, age)
	}
	if size.Columns < 32 {
		footer = "q exits; VM stays on"
		if customFooter != "" {
			footer = "Ctrl+] exits; F2 handoff"
		}
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

// InputModes records VT private modes before an interactive viewer changes them.
// DECRQM replies are bounded and consumed before starting the input reader.
func (t *Session) InputModes(parent context.Context) (map[int]bool, error) {
	ctx, cancel := context.WithTimeout(parent, 700*time.Millisecond)
	defer cancel()
	if e := t.Write(ctx, []byte("\x1b[?1000$p\x1b[?1006$p\x1b[?2004$p")); e != nil {
		return nil, e
	}
	modes := map[int]bool{}
	var data []byte
	pattern := regexp.MustCompile("\\x1b\\[\\?(1000|1006|2004);([12])\\$y")
	for len(data) < 1024 && len(modes) < 3 {
		b, e := t.Read(ctx)
		if e != nil {
			return nil, ErrUnsupported
		}
		data = append(data, b...)
		for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
			n, _ := strconv.Atoi(match[1])
			modes[n] = match[2] == "1"
		}
	}
	if len(modes) != 3 {
		return nil, ErrUnsupported
	}
	return modes, nil
}
