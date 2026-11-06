// SPDX-License-Identifier: Apache-2.0
package protocol

import (
	"bytes"
	"errors"
	"image/png"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const Width = 1280
const Height = 800
const MaxText = 64 << 10
const MaxOutput = 256 << 10

type DesktopStatus struct {
	Ready      bool   `json:"ready"`
	Generation string `json:"generation,omitempty"`
	Display    string `json:"display"`
	Server     string `json:"server"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Reason     string `json:"reason,omitempty"`
}
type Frame struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"sessionId"`
	Generation string    `json:"generation"`
	Sequence   uint64    `json:"sequence"`
	CapturedAt time.Time `json:"capturedAt"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	Scale      int       `json:"scale"`
	Format     string    `json:"format"`
	PNG        []byte    `json:"png,omitempty"`
}

func (f Frame) Validate() error {
	if !ValidID(f.ID) || !ValidID(f.SessionID) || f.Generation == "" || f.Sequence == 0 || f.CapturedAt.IsZero() || f.Format != "png" || f.Scale != 1 || f.Width != Width || f.Height != Height {
		return Fail("INVALID_FRAME", "incompatible screenshot metadata")
	}
	if len(f.PNG) == 0 || len(f.PNG) > MaxPNG {
		return Fail("FRAME_TOO_LARGE", "PNG must be 1 byte..8 MiB")
	}
	cfg, e := png.DecodeConfig(bytes.NewReader(f.PNG))
	if e != nil || cfg.Width != f.Width || cfg.Height != f.Height {
		return Fail("INVALID_FRAME", "PNG dimensions do not match declared screenshot")
	}
	if _, e = png.Decode(bytes.NewReader(f.PNG)); e != nil {
		return Fail("INVALID_FRAME", "PNG is truncated or malformed")
	}
	return nil
}

type Input struct {
	Kind      string `json:"kind"`
	FrameID   string `json:"frameId"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	ToX       int    `json:"toX,omitempty"`
	ToY       int    `json:"toY,omitempty"`
	Button    string `json:"button,omitempty"`
	Clicks    int    `json:"clicks,omitempty"`
	DX        int    `json:"dx,omitempty"`
	DY        int    `json:"dy,omitempty"`
	Text      string `json:"text,omitempty"`
	Key       string `json:"key,omitempty"`
	PasteMode string `json:"pasteMode,omitempty"`
}
type InputResult struct {
	Dispatched bool   `json:"dispatched"`
	FrameID    string `json:"frameId"`
	Generation string `json:"generation"`
	Note       string `json:"note,omitempty"`
}

func (i Input) Validate() error {
	if !ValidID(i.FrameID) {
		return Fail("INVALID_ARGUMENT", "input requires a valid observed frame ID")
	}
	point := func(x, y int) bool { return x >= 0 && x < Width && y >= 0 && y < Height }
	switch i.Kind {
	case "move", "click", "drag", "scroll":
		if !point(i.X, i.Y) {
			return Fail("INVALID_ARGUMENT", "coordinates must lie within 1280x800 guest pixels")
		}
	case "text", "key":
	default:
		return Fail("INVALID_ARGUMENT", "unknown input action")
	}
	if i.Kind == "drag" && !point(i.ToX, i.ToY) {
		return Fail("INVALID_ARGUMENT", "drag destination outside guest screen")
	}
	if i.Button != "" && i.Button != "left" && i.Button != "middle" && i.Button != "right" {
		return Fail("INVALID_ARGUMENT", "button must be left, middle or right")
	}
	if i.Clicks < 0 || i.Clicks > 2 {
		return Fail("INVALID_ARGUMENT", "clicks must be 1 or 2")
	}
	if i.DX < -100 || i.DX > 100 || i.DY < -100 || i.DY > 100 {
		return Fail("INVALID_ARGUMENT", "scroll delta is -100..100 wheel steps")
	}
	if i.Kind == "text" {
		if !utf8.ValidString(i.Text) || len(i.Text) > MaxText || strings.ContainsRune(i.Text, 0) {
			return Fail("INVALID_ARGUMENT", "text must be valid UTF-8 up to 64 KiB, without NUL")
		}
		if i.PasteMode != "" && i.PasteMode != "standard" && i.PasteMode != "terminal" && i.PasteMode != "auto" {
			return Fail("INVALID_ARGUMENT", "paste mode must be standard, terminal or auto")
		}
	}
	if i.Kind == "key" {
		_, e := TranslateKey(i.Key)
		return e
	}
	return nil
}
func TranslateKey(s string) (string, error) {
	if len(s) == 0 || len(s) > 80 {
		return "", Fail("INVALID_ARGUMENT", "invalid key combination")
	}
	parts := strings.Split(strings.ToUpper(s), "+")
	if len(parts) > 5 {
		return "", Fail("INVALID_ARGUMENT", "too many key modifiers")
	}
	mods := map[string]string{"CTRL": "ctrl", "ALT": "alt", "SHIFT": "shift", "SUPER": "super"}
	seen := map[string]bool{}
	out := []string{}
	for _, p := range parts[:len(parts)-1] {
		m, ok := mods[p]
		if !ok || seen[m] {
			return "", Fail("INVALID_ARGUMENT", "modifiers are unique CTRL, ALT, SHIFT, SUPER")
		}
		seen[m] = true
		out = append(out, m)
	}
	last := parts[len(parts)-1]
	keys := map[string]string{"ENTER": "Return", "RETURN": "Return", "TAB": "Tab", "ESC": "Escape", "ESCAPE": "Escape", "BACKSPACE": "BackSpace", "DELETE": "Delete", "HOME": "Home", "END": "End", "PAGEUP": "Prior", "PAGEDOWN": "Next", "LEFT": "Left", "RIGHT": "Right", "UP": "Up", "DOWN": "Down", "SPACE": "space"}
	key, ok := keys[last]
	if !ok {
		if regexp.MustCompile(`^[A-Z0-9]$|^F([1-9]|1[0-2])$`).MatchString(last) {
			key = strings.ToLower(last)
			if strings.HasPrefix(last, "F") && len(last) > 1 {
				key = last
			}
		} else {
			return "", Fail("INVALID_ARGUMENT", "unsupported key; use text for Unicode")
		}
	}
	return strings.Join(append(out, key), "+"), nil
}

type ExecRequest struct {
	Argv        []string          `json:"argv"`
	Cwd         string            `json:"cwd,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	TimeoutMS   int               `json:"timeoutMs"`
	OutputLimit int               `json:"outputLimit"`
}

func (r ExecRequest) Validate() error {
	if len(r.Argv) == 0 || len(r.Argv) > 128 || r.Argv[0] == "" {
		return Fail("INVALID_ARGUMENT", "exec requires 1..128 argv values")
	}
	total := 0
	for _, a := range r.Argv {
		total += len(a)
		if strings.ContainsRune(a, 0) {
			return Fail("INVALID_ARGUMENT", "argv contains NUL")
		}
	}
	if total > MaxText {
		return Fail("INVALID_ARGUMENT", "argv exceeds 64 KiB")
	}
	if r.Cwd != "" && (!filepath.IsAbs(r.Cwd) || filepath.Clean(r.Cwd) != r.Cwd || strings.ContainsRune(r.Cwd, 0)) {
		return Fail("INVALID_ARGUMENT", "cwd must be a clean absolute guest path")
	}
	if r.TimeoutMS < 1 || r.TimeoutMS > 60000 || r.OutputLimit < 1 || r.OutputLimit > MaxOutput {
		return Fail("INVALID_ARGUMENT", "exec timeout 1..60000 ms and output limit 1..262144 bytes per stream")
	}
	if len(r.Env) > 32 {
		return errors.New("environment limited to 32 entries")
	}
	for k, v := range r.Env {
		if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(k) || strings.ContainsRune(v, 0) || len(v) > 8192 {
			return Fail("INVALID_ARGUMENT", "invalid environment variable")
		}
	}
	return nil
}

type Execution struct {
	ExitCode         int    `json:"exitCode"`
	Stdout           []byte `json:"stdout"`
	Stderr           []byte `json:"stderr"`
	TimedOut         bool   `json:"timedOut"`
	Canceled         bool   `json:"canceled"`
	Truncated        bool   `json:"truncated"`
	CleanupConfirmed bool   `json:"cleanupConfirmed"`
	DurationMS       int64  `json:"durationMs"`
	Error            string `json:"error,omitempty"`
}

// Capabilities describe supported operations; DesktopStatus reports current availability.
type Capabilities struct {
	Apps           []string `json:"apps,omitempty"`
	Displays       int      `json:"displays"`
	Format         string   `json:"format"`
	Origin         string   `json:"origin"`
	Scale          int      `json:"scale"`
	Input          []string `json:"input"`
	PasteModes     []string `json:"pasteModes"`
	MaxPNGBytes    int      `json:"maxPngBytes"`
	MaxTextBytes   int      `json:"maxTextBytes"`
	MaxOutputBytes int      `json:"maxOutputBytes"`
	MaxExecMS      int      `json:"maxExecMs"`
	GuestExec      bool     `json:"arbitraryGuestExec"`
}

func SupportedCapabilities() *Capabilities {
	return &Capabilities{Apps: Applications(), Displays: 1, Format: "png", Origin: "top-left", Scale: 1, Input: []string{"move", "click", "drag", "scroll", "text", "key"}, PasteModes: []string{"standard", "terminal", "auto"}, MaxPNGBytes: MaxPNG, MaxTextBytes: MaxText, MaxOutputBytes: MaxOutput, MaxExecMS: 60000, GuestExec: true}
}

// Applications are implemented guest launch targets, not an installed-package assertion.
func Applications() []string {
	return []string{"editor", "terminal", "browser", "chromium", "firefox", "files", "settings", "launcher"}
}
func ValidApp(name string) bool {
	for _, app := range Applications() {
		if name == app {
			return true
		}
	}
	return false
}
