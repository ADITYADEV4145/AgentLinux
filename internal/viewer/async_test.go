// SPDX-License-Identifier: Apache-2.0
package viewer

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDoubleBufferPlacesReplacementBeforeDeletingPrevious(t *testing.T) {
	var r Renderer
	var out bytes.Buffer
	write := func(_ context.Context, b []byte) error { _, err := out.Write(b); return err }
	p := Placement{Column: 1, Row: 2, Columns: 80, Rows: 30}
	for n := 0; n < 6; n++ {
		out.Reset()
		if err := r.Render(context.Background(), write, p, []byte("bounded image")); err != nil {
			t.Fatal(err)
		}
		data := out.String()
		transmitted, placed, deleted := strings.Index(data, "a=t"), strings.Index(data, "a=p"), strings.Index(data, "a=d")
		if transmitted < 0 || placed < transmitted || (n > 0 && deleted < placed) {
			t.Fatal("image removed before replacement", data)
		}
		if r.current != ImageID && r.current != ImageID+1 {
			t.Fatal("unbounded image IDs")
		}
	}
	previous := r.current
	out.Reset()
	fail := func(_ context.Context, b []byte) error { out.Write(b); return errors.New("slow/disconnected terminal") }
	if err := r.Render(context.Background(), fail, p, []byte("replacement")); err == nil || r.current != previous || strings.Contains(out.String(), "a=d") {
		t.Fatal("failed transmission removed previous image")
	}
	if !strings.Contains(DeleteImage(), "i=4145") || !strings.Contains(DeleteImage(), "i=4146") {
		t.Fatal("cleanup missed a buffer")
	}
}
func TestSlowCaptureDoesNotBlockTerminalResizeOrCancellation(t *testing.T) {
	c := &computer{frame: validFrame(t), wait: make(chan struct{}), entered: make(chan struct{}, 1)}
	terminal := &terminalDouble{}
	ctx, cancel := context.WithTimeout(context.Background(), 450*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := RunTerminal(ctx, NewHub(c, time.Second), terminal, identity); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second || !terminal.closed || terminal.sizeCalls < 3 {
		t.Fatal("capture blocked controls/cleanup", time.Since(start), terminal.sizeCalls)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.calls != 1 {
		t.Fatal("multiple captures queued", c.calls)
	}
}
