// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsing(t *testing.T) {
	for _, args := range [][]string{{"cloud"}, {"up"}, {"up", "--local", "--cpus=0"}, {"up", "--local", "--memory=-1"}, {"up", "--local", "--disk=129"}, {"up", "--local", "--network=off"}, {"status", "extra"}, {"status", "--timeout=0"}, {"up", "--local", "--image=/a"}, {"status", "--state-dir=relative"}, {"stop", "--force"}} {
		if _, e := Parse(args, &bytes.Buffer{}); e == nil {
			t.Errorf("accepted invalid arguments: %v", args)
		}
	}
	o, e := Parse([]string{"up", "--local", "--network=nat", "--cpus=3", "--json"}, &bytes.Buffer{})
	if e != nil || o.Spec.CPUs != 3 || !o.JSON || !o.CreationFlags {
		t.Fatalf("valid options: %#v %v", o, e)
	}
	if _, e = Parse([]string{"up", "--help"}, &bytes.Buffer{}); !errors.Is(e, flag.ErrHelp) {
		t.Fatal(e)
	}
}
func TestStructuredParseError(t *testing.T) {
	var out, err bytes.Buffer
	code := Run(context.Background(), []string{"up", "--local", "--cpus=0", "--json"}, &out, &err)
	var result Envelope
	if e := json.Unmarshal(out.Bytes(), &result); e != nil || code == 0 || result.OK || result.Error == nil {
		t.Fatalf("invalid JSON failure: %s %v", out.String(), e)
	}
}

func TestComputerArguments(t *testing.T) {
	valid := [][]string{{"click", "--button=right", "--clicks=2", "50", "60"}, {"drag", "1", "2", "300", "400"}, {"scroll", "50", "60", "-1", "2"}, {"type", "café 日本語 👋"}, {"key", "CTRL+SHIFT+S"}, {"exec", "--cwd=/home/agent", "--env=TEST=value", "--", "/bin/echo", "hello"}, {"screenshot", "--output=/private/tmp/guest.png"}}
	for _, args := range valid {
		if _, e := Parse(args, &bytes.Buffer{}); e != nil {
			t.Errorf("valid args %v: %v", args, e)
		}
	}
	for _, args := range [][]string{{"click", "-1", "20"}, {"key", "CMD+C"}, {"exec", "--", "/bin/echo\x00"}, {"screenshot"}, {"move", "1280", "0"}} {
		if _, e := Parse(args, &bytes.Buffer{}); e == nil {
			t.Errorf("invalid args accepted %v", args)
		}
	}
	if out := safeGuestText([]byte("hello\x1b]52;c;attack\a")); strings.ContainsRune(out, '\x1b') || strings.ContainsRune(out, '\a') {
		t.Fatal("guest output can invoke host clipboard")
	}
}

func TestInstalledLauncherFindsVersionedGuest(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	version := filepath.Join(root, "releases", "version", "bin")
	if e := os.MkdirAll(version, 0700); e != nil {
		t.Fatal(e)
	}
	real := filepath.Join(version, "agent-linux")
	os.WriteFile(real, []byte("fixture"), 0700)
	link := filepath.Join(root, "agent-linux")
	os.Symlink(real, link)
	if got := siblingGuest(link); got != filepath.Join(version, "agent-linux-guest") {
		t.Fatal("launcher helper discovery", got)
	}
}

func TestStartHelpAndInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"start", "--help"}, {"cloud", "--help"}} {
		var out, errout bytes.Buffer
		if code := Run(context.Background(), args, &out, &errout); code != 0 {
			t.Fatal(code, errout.String())
		}
		if out.Len()+errout.Len() == 0 {
			t.Fatal("missing help")
		}
	}
	for _, args := range [][]string{{"start", "unexpected"}, {"start", "--cloud-config", "/nonexistent", "--cpus", "4"}, {"cloud", "serve", "--listen", "0.0.0.0:8787"}} {
		var out, errout bytes.Buffer
		if code := Run(context.Background(), args, &out, &errout); code == 0 {
			t.Fatal("invalid input accepted", args)
		}
	}
}
