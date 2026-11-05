// SPDX-License-Identifier: Apache-2.0
package viewer

import (
	"context"
	"errors"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"strings"
)

// Codes are rendered, never arbitrary guest/log text or terminal escapes.
func Code(e error) string {
	if e == nil {
		return "LIVE"
	}
	var f *protocol.Failure
	if errors.As(e, &f) && len(f.Code) <= 64 && f.Code != "" {
		for _, r := range f.Code {
			if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
				return "UNAVAILABLE"
			}
		}
		return f.Code
	}
	if strings.Contains(e.Error(), "MACHINE_BUSY") {
		return "MACHINE_BUSY"
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return "TIMEOUT"
	}
	if errors.Is(e, context.Canceled) {
		return "CANCELED"
	}
	return "UNAVAILABLE"
}
