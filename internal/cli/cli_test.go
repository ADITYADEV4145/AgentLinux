// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
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
