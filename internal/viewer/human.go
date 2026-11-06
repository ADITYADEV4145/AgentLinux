// SPDX-License-Identifier: Apache-2.0
package viewer

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ADITYADEV4145/AgentLinux/internal/machine"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

type HumanEvent struct {
	Action      string
	Input       protocol.Input
	Column, Row int
}
type HumanKeys struct {
	buffer []byte
	paste  bool
	data   []byte
}

// Feed is incremental and bounded. Terminal replies never become guest text.
func (k *HumanKeys) Feed(b []byte) []HumanEvent {
	if len(k.buffer)+len(b) > protocol.MaxText+4096 {
		k.buffer = nil
		k.data = nil
		k.paste = false
		return []HumanEvent{{Action: "overflow"}}
	}
	k.buffer = append(k.buffer, b...)
	out := []HumanEvent{}
	for len(k.buffer) > 0 {
		if k.paste {
			end := strings.Index(string(k.buffer), "\x1b[201~")
			if end < 0 {
				// Retain the suffix which may be the beginning of a split closing marker.
				n := len(k.buffer) - 5
				if n <= 0 {
					break
				}
				k.data = append(k.data, k.buffer[:n]...)
				k.buffer = k.buffer[n:]
				if len(k.data) > protocol.MaxText {
					k.data = nil
					k.buffer = nil
					k.paste = false
					out = append(out, HumanEvent{Action: "overflow"})
				}
				break
			}
			k.data = append(k.data, k.buffer[:end]...)
			k.buffer = k.buffer[end+6:]
			k.paste = false
			if len(k.data) <= protocol.MaxText && utf8.Valid(k.data) && !strings.ContainsRune(string(k.data), 0) {
				out = append(out, HumanEvent{Input: protocol.Input{Kind: "text", Text: string(k.data)}})
			} else {
				out = append(out, HumanEvent{Action: "overflow"})
			}
			k.data = nil
			continue
		}
		if k.buffer[0] == 27 {
			if len(k.buffer) < 2 {
				break
			}
			if k.buffer[1] == '_' || k.buffer[1] == ']' || k.buffer[1] == 'P' {
				end := strings.Index(string(k.buffer), "\x1b\\")
				if end < 0 {
					if len(k.buffer) > 4096 {
						k.buffer = nil
					}
					break
				}
				k.buffer = k.buffer[end+2:]
				continue
			}
			if strings.HasPrefix(string(k.buffer), "\x1b[200~") {
				k.paste = true
				k.buffer = k.buffer[6:]
				continue
			}
			end := 0
			if k.buffer[1] == '[' {
				for n := 2; n < len(k.buffer); n++ {
					if k.buffer[n] >= 0x40 && k.buffer[n] <= 0x7e {
						end = n + 1
						break
					}
				}
			} else if k.buffer[1] == 'O' {
				if len(k.buffer) >= 3 {
					end = 3
				}
			} else {
				k.buffer = k.buffer[1:]
				out = append(out, HumanEvent{Input: protocol.Input{Kind: "key", Key: "ESC"}})
				continue
			}
			if end == 0 {
				if len(k.buffer) > 128 {
					k.buffer = nil
				}
				break
			}
			seq := string(k.buffer[:end])
			k.buffer = k.buffer[end:]
			if seq == "\x1bOQ" || seq == "\x1b[12~" {
				out = append(out, HumanEvent{Action: "toggle"})
				continue
			}
			if seq == "\x1bOR" || seq == "\x1b[13~" {
				out = append(out, HumanEvent{Action: "paste-mode"})
				continue
			}
			keys := map[string]string{"\x1b[A": "UP", "\x1b[B": "DOWN", "\x1b[C": "RIGHT", "\x1b[D": "LEFT", "\x1b[H": "HOME", "\x1b[F": "END", "\x1b[1~": "HOME", "\x1b[4~": "END", "\x1b[3~": "DELETE", "\x1b[5~": "PAGEUP", "\x1b[6~": "PAGEDOWN", "\x1b[Z": "SHIFT+TAB", "\x1bOP": "F1", "\x1bOS": "F4", "\x1b[15~": "F5", "\x1b[17~": "F6", "\x1b[18~": "F7", "\x1b[19~": "F8", "\x1b[20~": "F9", "\x1b[21~": "F10", "\x1b[23~": "F11", "\x1b[24~": "F12"}
			if key, ok := keys[seq]; ok {
				out = append(out, HumanEvent{Input: protocol.Input{Kind: "key", Key: key}})
				continue
			}
			match := mousePacket.FindStringSubmatch(seq)
			if match != nil && match[4] == "M" {
				button, _ := strconv.Atoi(match[1])
				col, _ := strconv.Atoi(match[2])
				row, _ := strconv.Atoi(match[3])
				if button&28 != 0 || col <= 0 || row <= 0 {
					continue
				}
				if button == 64 || button == 65 {
					dy := -1
					if button == 65 {
						dy = 1
					}
					out = append(out, HumanEvent{Column: col, Row: row, Input: protocol.Input{Kind: "scroll", DY: dy}})
				} else if button <= 2 {
					out = append(out, HumanEvent{Column: col, Row: row, Input: protocol.Input{Kind: "click", Button: []string{"left", "middle", "right"}[button], Clicks: 1}})
				}
			}
			continue
		}
		c := k.buffer[0]
		if c == 29 {
			k.buffer = k.buffer[1:]
			out = append(out, HumanEvent{Action: "exit"})
			continue
		}
		if c < 32 || c == 127 {
			k.buffer = k.buffer[1:]
			key := ""
			switch c {
			case 9:
				key = "TAB"
			case 10, 13:
				key = "ENTER"
			case 127:
				key = "BACKSPACE"
			default:
				if c >= 1 && c <= 26 {
					key = "CTRL+" + string(rune('A'+c-1))
				}
			}
			if key != "" {
				out = append(out, HumanEvent{Input: protocol.Input{Kind: "key", Key: key}})
			}
			continue
		}
		n := 0
		for n < len(k.buffer) && k.buffer[n] >= 32 && k.buffer[n] != 127 {
			if k.buffer[n] == 27 {
				break
			}
			_, size := utf8.DecodeRune(k.buffer[n:])
			if size == 1 && k.buffer[n] >= 128 {
				if !utf8.FullRune(k.buffer[n:]) {
					break
				}
				n++
				continue
			}
			n += size
		}
		if n == 0 {
			break
		}
		text := string(k.buffer[:n])
		k.buffer = k.buffer[n:]
		if utf8.ValidString(text) {
			out = append(out, HumanEvent{Input: protocol.Input{Kind: "text", Text: text}})
		}
	}
	return out
}

var mousePacket = regexp.MustCompile("^\\x1b\\[<([0-9]{1,3});([0-9]{1,5});([0-9]{1,5})([Mm])$")

func (k *HumanKeys) FlushEscape() []HumanEvent {
	if string(k.buffer) == "\x1b" {
		k.buffer = nil
		return []HumanEvent{{Input: protocol.Input{Kind: "key", Key: "ESC"}}}
	}
	return nil
}
func MapCell(size Size, col, row int) (int, int, bool) {
	p := Fit(size)
	if p.Columns == 0 {
		return 0, 0, false
	}
	cw := float64(size.PixelWidth) / float64(size.Columns)
	ch := float64(size.PixelHeight) / float64(size.Rows)
	x := (float64(col) - .5 - float64(p.Column-1)) * cw
	y := (float64(row) - .5 - float64(p.Row-1)) * ch
	width := float64(p.Columns) * cw
	height := width * 800 / 1280
	if x < 0 || y < 0 || x >= width || y >= height {
		return 0, 0, false
	}
	return int(x / width * 1280), int(y / height * 800), true
}

type Human struct {
	Machine    machine.Machine
	Control    machine.InputControl
	Owned      bool
	Generation string
	Renewed    time.Time
	PasteMode  string
	Notice     string
	Frame      *protocol.Frame
}

func NewHuman(m machine.Machine) *Human {
	c, _ := m.(machine.InputControl)
	return &Human{Machine: m, Control: c, PasteMode: "standard"}
}
func (h *Human) Release() {
	if h.Control != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = h.Control.ControlLease(ctx, "release", "")
		cancel()
	}
	h.Owned = false
}
func (h *Human) Observe(f *protocol.Frame) {
	if h.Owned && h.Generation != f.Generation {
		h.Release()
		h.Notice = "Desktop restarted; F2 to retake control"
	}
	h.Frame = f
}
func (h *Human) Handle(ctx context.Context, e HumanEvent, size Size) error {
	if e.Action == "exit" {
		return context.Canceled
	}
	if e.Action == "overflow" {
		h.Release()
		h.Notice = "Input exceeded limits; discarded; F2 to retake"
		return nil
	}
	if e.Action == "paste-mode" {
		if h.PasteMode == "terminal" {
			h.PasteMode = "standard"
		} else {
			h.PasteMode = "terminal"
		}
		return nil
	}
	if e.Action == "toggle" {
		if h.Owned {
			h.Release()
			h.Notice = "Agent may now operate the guest"
			return nil
		}
		if h.Control == nil || h.Frame == nil || time.Since(h.Frame.CapturedAt) > 10*time.Second {
			return protocol.Fail("NOT_READY", "wait for a fresh image before takeover")
		}
		if e := h.Control.ControlLease(ctx, "acquire", h.Frame.Generation); e != nil {
			return e
		}
		h.Owned = true
		h.Generation = h.Frame.Generation
		h.Renewed = time.Now()
		status, e := h.Machine.Status(ctx)
		if e == nil && status.Guest != nil && status.Guest.Capabilities != nil && slices.Contains(status.Guest.Capabilities.PasteModes, "auto") {
			h.PasteMode = "auto"
		}
		h.Notice = ""
		return nil
	}
	if !h.Owned {
		return nil
	}
	if h.Frame == nil || time.Since(h.Frame.CapturedAt) > 10*time.Second {
		h.Release()
		return protocol.Fail("STALE_FRAME", "image too old; F2 after observing a fresh frame")
	}
	input := e.Input
	input.FrameID = h.Frame.ID
	if input.Kind == "click" || input.Kind == "scroll" {
		x, y, ok := MapCell(size, e.Column, e.Row)
		if !ok {
			return nil
		}
		input.X = x
		input.Y = y
	}
	if input.Kind == "text" {
		input.PasteMode = h.PasteMode
	}
	_, err := h.Machine.Input(ctx, input)
	if err != nil {
		h.Release()
	}
	return err
}
func (h *Human) Heartbeat(ctx context.Context) error {
	if h.Owned && time.Since(h.Renewed) >= 5*time.Second {
		if e := h.Control.ControlLease(ctx, "renew", h.Generation); e != nil {
			h.Release()
			return e
		}
		h.Renewed = time.Now()
	}
	return nil
}
func (h *Human) Footer() string {
	mode := "AGENT CAN CONTROL"
	if h.Owned {
		mode = "YOU CONTROL"
	}
	return fmt.Sprintf("%s | F2 handoff | F3 paste:%s | Ctrl+] exits | %s", mode, h.PasteMode, h.Notice)
}
