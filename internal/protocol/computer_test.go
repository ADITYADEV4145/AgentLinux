// SPDX-License-Identifier: Apache-2.0
package protocol

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"
)

func TestKeysAndUnicode(t *testing.T) {
	for key, want := range map[string]string{"ENTER": "Return", "CTRL+SHIFT+S": "ctrl+shift+s", "ALT+F4": "alt+F4", "SUPER+SPACE": "super+space", "PAGEDOWN": "Next"} {
		got, e := TranslateKey(key)
		if e != nil || got != want {
			t.Fatalf("%s -> %s %v", key, got, e)
		}
	}
	for _, key := range []string{"CTRL+CTRL+A", "CMD+C", "ctrl+;rm -rf /", "CTRL+", "é", "--clearmodifiers"} {
		if _, e := TranslateKey(key); e == nil {
			t.Fatal("malformed key accepted", key)
		}
	}
	text := "ASCII café — 日本語 👋"
	i := Input{Kind: "text", FrameID: ID(), Text: text}
	if e := i.Validate(); e != nil || i.Text != text {
		t.Fatal("Unicode damaged", e)
	}
	i.Text = "bad\x00"
	if i.Validate() == nil {
		t.Fatal("NUL accepted")
	}
	i.Text = strings.Repeat("x", MaxText+1)
	if i.Validate() == nil {
		t.Fatal("oversized text accepted")
	}
}
func TestCoordinatesAndCommandValidation(t *testing.T) {
	for _, i := range []Input{{Kind: "click", FrameID: ID(), X: -1}, {Kind: "move", FrameID: ID(), X: Width}, {Kind: "drag", FrameID: ID(), ToY: Height}, {Kind: "click", FrameID: ID(), Button: "side"}, {Kind: "click", FrameID: ID(), Clicks: 3}, {Kind: "scroll", FrameID: ID(), DY: 101}} {
		if i.Validate() == nil {
			t.Fatalf("invalid input accepted: %#v", i)
		}
	}
	if e := (Input{Kind: "click", FrameID: ID(), X: 1279, Y: 799, Clicks: 2, Button: "right"}).Validate(); e != nil {
		t.Fatal(e)
	}
	valid := ExecRequest{Argv: []string{"/bin/echo", "hello"}, Cwd: "/home/agent", TimeoutMS: 100, OutputLimit: 1024}
	if e := valid.Validate(); e != nil {
		t.Fatal(e)
	}
	for _, q := range []ExecRequest{{Argv: []string{"/bin/echo"}, TimeoutMS: 60001, OutputLimit: 10}, {Argv: []string{"/bin/echo"}, TimeoutMS: 1, OutputLimit: MaxOutput + 1}, {Argv: []string{"/bin/echo"}, TimeoutMS: 1, OutputLimit: 10, Cwd: "relative"}} {
		if q.Validate() == nil {
			t.Fatal("invalid exec accepted")
		}
	}
}

func TestScreenshotMetadataAndPNG(t *testing.T) {
	var b bytes.Buffer
	if e := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, Width, Height))); e != nil {
		t.Fatal(e)
	}
	f := Frame{ID: ID(), SessionID: ID(), Generation: "display-test", Sequence: 1, CapturedAt: time.Now(), Width: Width, Height: Height, Scale: 1, Format: "png", PNG: b.Bytes()}
	if e := f.Validate(); e != nil {
		t.Fatal(e)
	}
	bad := f
	bad.Width = 640
	if bad.Validate() == nil {
		t.Fatal("mismatched dimensions accepted")
	}
	bad = f
	bad.Scale = 2
	if bad.Validate() == nil {
		t.Fatal("unimplemented scaling accepted")
	}
	bad = f
	bad.PNG = bad.PNG[:20]
	if bad.Validate() == nil {
		t.Fatal("truncated PNG accepted")
	}
	bad = f
	bad.CapturedAt = time.Time{}
	if bad.Validate() == nil {
		t.Fatal("missing capture time accepted")
	}
}
func TestSeparateRequestBudget(t *testing.T) {
	var out bytes.Buffer
	q := Request{Version: Version, ID: ID(), MachineID: ID(), Method: "probe", Nonce: strings.Repeat("x", MaxRequest)}
	if WriteRequest(&out, q) == nil || out.Len() != 0 {
		t.Fatal("oversized request was partially transmitted")
	}
	var request Request
	if Read(RequestReader(strings.NewReader(strings.Repeat(" ", MaxRequest)+"\n")), &request) == nil {
		t.Fatal("request parser exceeded its request budget")
	}
}
