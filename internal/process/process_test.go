// SPDX-License-Identifier: Apache-2.0

package process

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestTimeoutAndOutputLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, e := Run(ctx, os.Environ(), "/bin/sh", "-c", "sleep 30 & wait")
	if !errors.Is(e, context.DeadlineExceeded) || time.Since(start) > 3*time.Second {
		t.Fatalf("child group not canceled: %v", e)
	}
	var b Buffer
	b.Limit = 4
	n, e := b.Write([]byte("123456789"))
	if e != nil || n != 9 || string(b.Bytes()) != "1234" || !b.Truncated {
		t.Fatal("unbounded capture")
	}
}
