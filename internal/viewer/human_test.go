// SPDX-License-Identifier: Apache-2.0
package viewer

import (
	"context"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"strings"
	"testing"
	"time"
)

func TestHumanKeysIncrementalUnicodeMouseAndReservedKeys(t *testing.T) {
	var p HumanKeys
	var events []HumanEvent
	raw := []byte("q\x1b[200~hello café 日本語 👋\n\x1b[201~\x1bOQ\x1b[<0;50;20M\x1b[<0;50;20m\x03\x1d")
	for _, b := range raw {
		events = append(events, p.Feed([]byte{b})...)
	}
	text := ""
	toggles, clicks, exits, ctrl := 0, 0, 0, 0
	for _, e := range events {
		if e.Input.Kind == "text" {
			text += e.Input.Text
		}
		if e.Action == "toggle" {
			toggles++
		}
		if e.Input.Kind == "click" {
			clicks++
			if e.Column != 50 || e.Row != 20 {
				t.Fatal(e)
			}
		}
		if e.Action == "exit" {
			exits++
		}
		if e.Input.Key == "CTRL+C" {
			ctrl++
		}
	}
	if text != "qhello café 日本語 👋\n" || toggles != 1 || clicks != 1 || exits != 1 || ctrl != 1 {
		t.Fatal(events, text)
	}
	if e := p.Feed([]byte("\x1b_Gq=2;payload q\x1d\x1b\\")); len(e) != 0 {
		t.Fatal("terminal reply forwarded", e)
	}
	_ = p.Feed([]byte{27})
	if e := p.FlushEscape(); len(e) != 1 || e[0].Input.Key != "ESC" {
		t.Fatal(e)
	}
}
func TestHumanPasteAndFrameMappingBounds(t *testing.T) {
	var p HumanKeys
	e := p.Feed([]byte("\x1b[200~" + strings.Repeat("a", protocol.MaxText+1) + "\x1b[201~"))
	if len(e) != 1 || e[0].Action != "overflow" {
		t.Fatal(e)
	}
	for _, s := range []Size{{120, 40, 1200, 800}, {80, 30, 800, 600}, {140, 24, 1400, 480}} {
		fit := Fit(s)
		x, y, ok := MapCell(s, fit.Column+fit.Columns/2, fit.Row+fit.Rows/2)
		if !ok || x < 600 || x > 700 || y < 350 || y > 450 {
			t.Fatal(s, fit, x, y, ok)
		}
		if _, _, ok = MapCell(s, 1, 1); ok {
			t.Fatal("status bar maps to guest")
		}
	}
}

type humanComputer struct {
	computer
	held   bool
	inputs int
}

func (c *humanComputer) ControlLease(_ context.Context, action, g string) error {
	if action == "release" {
		c.held = false
		return nil
	}
	c.held = true
	return nil
}
func (c *humanComputer) Input(context.Context, protocol.Input) (*protocol.InputResult, error) {
	if !c.held {
		return nil, protocol.Fail("INPUT_NOT_OWNED", "test")
	}
	c.inputs++
	return &protocol.InputResult{Dispatched: true}, nil
}
func TestHumanOwnershipRequiresExplicitHandoffAfterRestart(t *testing.T) {
	c := &humanComputer{}
	h := NewHuman(c)
	h.Observe(validFrame(t))
	ctx := context.Background()
	if e := h.Handle(ctx, HumanEvent{Action: "toggle"}, Size{}); e != nil {
		t.Fatal(e)
	}
	if !h.Owned {
		t.Fatal("not owned")
	}
	if e := h.Handle(ctx, HumanEvent{Input: protocol.Input{Kind: "text", Text: "hello"}}, Size{}); e != nil || c.inputs != 1 {
		t.Fatal(e, c.inputs)
	}
	next := validFrame(t)
	next.Generation = "new-generation"
	next.CapturedAt = time.Now()
	h.Observe(next)
	if h.Owned || c.held {
		t.Fatal("restart did not revoke human control")
	}
	_ = h.Handle(ctx, HumanEvent{Input: protocol.Input{Kind: "text", Text: "ignored"}}, Size{})
	if c.inputs != 1 {
		t.Fatal("blind post-restart input")
	}
}

func (c *humanComputer) Status(context.Context) (lifecycle.State, error) {
	return lifecycle.State{State: "ready"}, nil
}

type interactiveTerminalDouble struct{ terminalDouble }

func (t *interactiveTerminalDouble) InputModes(context.Context) (map[int]bool, error) {
	return map[int]bool{1000: true, 1006: false, 2004: true}, nil
}

type interactiveComputerDouble struct {
	*computer
	held bool
}

func (c *interactiveComputerDouble) ControlLease(_ context.Context, action, generation string) error {
	c.held = action != "release"
	return nil
}
func TestInteractiveTerminalRestoresOriginalModesOnCancellation(t *testing.T) {
	c := &interactiveComputerDouble{computer: &computer{frame: validFrame(t)}}
	terminal := &interactiveTerminalDouble{}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_ = RunInteractiveTerminal(ctx, NewHub(c, time.Second), terminal, identity)
	if !terminal.closed || c.held {
		t.Fatal("terminal or human ownership leaked")
	}
	output := terminal.data.String()
	if !strings.Contains(output, "\x1b[?1000h\x1b[?1006l\x1b[?2004h") {
		t.Fatal("original mixed terminal modes not restored")
	}
}
func (c *interactiveComputerDouble) Status(context.Context) (lifecycle.State, error) {
	return lifecycle.State{State: "ready"}, nil
}
