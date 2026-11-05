// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestViewHelpAndArguments(t *testing.T) {
	var out, errout bytes.Buffer
	if Run(context.Background(), []string{"view", "--help"}, &out, &errout) != 0 || !strings.Contains(errout.String(), "VIEW-ONLY") || !strings.Contains(errout.String(), "VM") {
		t.Fatal(errout.String())
	}
	for _, args := range [][]string{{"view", "--interval=0s"}, {"view", "--interval=10s"}, {"view", "--terminal=iterm"}, {"view", "--state-dir=relative"}, {"view", "--unknown"}, {"view", "--state-dir", t.TempDir() + "/missing"}} {
		out.Reset()
		errout.Reset()
		if Run(context.Background(), args, &out, &errout) == 0 || out.Len() != 0 {
			t.Fatal("invalid/missing-state viewer invocation", args, out.String())
		}
	}
}
