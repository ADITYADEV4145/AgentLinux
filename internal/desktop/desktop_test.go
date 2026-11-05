// SPDX-License-Identifier: Apache-2.0
package desktop

import (
	"bytes"
	"context"
	"fmt"
	"github.com/ADITYADEV4145/AgentLinux/internal/process"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (*Desktop, *string, *[]string) {
	t.Helper()
	gen := strings.Repeat("a", 36)
	commands := []string{}
	px, py := 0, 0
	d := New()
	d.run = func(_ context.Context, name string, args ...string) (process.Result, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
		if len(args) == 3 && args[0] == "mousemove" {
			fmt.Sscan(args[1], &px)
			fmt.Sscan(args[2], &py)
		}
		if len(args) == 1 && args[0] == "getmouselocation" {
			return process.Result{Out: []byte(fmt.Sprintf("x:%d y:%d screen:0 window:1", px, py))}, nil
		}
		switch strings.Join(args, " ") {
		case "getdisplaygeometry":
			return process.Result{Out: []byte("1280 800\n")}, nil
		case "-root _AGENT_LINUX_GENERATION":
			return process.Result{Out: []byte(fmt.Sprintf("_AGENT_LINUX_GENERATION(STRING) = %q", gen))}, nil
		case "-root _NET_SUPPORTING_WM_CHECK":
			return process.Result{Out: []byte("window id # 0x1")}, nil
		}
		return process.Result{}, nil
	}
	var b bytes.Buffer
	if e := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, protocol.Width, protocol.Height))); e != nil {
		t.Fatal(e)
	}
	data := b.Bytes()
	d.capture = func(context.Context) ([]byte, error) { return data, nil }
	return d, &gen, &commands
}
func TestFreshFramesAndInputRouting(t *testing.T) {
	d, gen, commands := fixture(t)
	ctx := context.Background()
	s := protocol.ID()
	f, e := d.Screenshot(ctx, s)
	if e != nil || f.Scale != 1 || f.Width != 1280 || f.SessionID != s || f.CapturedAt.IsZero() {
		t.Fatalf("frame metadata: %#v %v", f, e)
	}
	next, e := d.Screenshot(ctx, s)
	if e != nil || next.ID == f.ID || next.Sequence <= f.Sequence {
		t.Fatal("stale frame reused")
	}
	if _, e = d.Input(ctx, protocol.Input{Kind: "click", FrameID: f.ID, X: 70, Y: 80, Button: "right", Clicks: 2}); e != nil {
		t.Fatal(e)
	}
	joined := strings.Join(*commands, "\n")
	if !strings.Contains(joined, "mousemove 70 80") || !strings.Contains(joined, "--repeat 2 --delay 100 3") || !strings.Contains(joined, "keyup ctrl alt shift super") {
		t.Fatal("input or modifier release not routed to guest X11", joined)
	}
	*gen = strings.Repeat("b", 36)
	if _, e = d.Input(ctx, protocol.Input{Kind: "click", FrameID: f.ID}); e == nil {
		t.Fatal("old display generation accepted")
	}
}
func TestUnavailableExpiredAndOversized(t *testing.T) {
	d, _, _ := fixture(t)
	ctx := context.Background()
	f, e := d.Screenshot(ctx, protocol.ID())
	if e != nil {
		t.Fatal(e)
	}
	d.frames[f.ID] = observation{f.Generation, time.Now().Add(-11 * time.Second)}
	if _, e = d.Input(ctx, protocol.Input{Kind: "key", FrameID: f.ID, Key: "ENTER"}); e == nil {
		t.Fatal("expired frame accepted")
	}
	d.capture = func(context.Context) ([]byte, error) { return make([]byte, protocol.MaxPNG+1), nil }
	if _, e = d.Screenshot(ctx, protocol.ID()); e == nil {
		t.Fatal("oversized PNG accepted")
	}
	d.run = func(context.Context, string, ...string) (process.Result, error) {
		return process.Result{}, fmt.Errorf("no X server")
	}
	if d.Status(ctx).Ready {
		t.Fatal("missing X server claimed ready")
	}
	if _, e = d.Screenshot(ctx, protocol.ID()); e == nil {
		t.Fatal("stale image returned after display loss")
	}
}
