// SPDX-License-Identifier: Apache-2.0

package host

import (
	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"testing"
)

func TestResourceBounds(t *testing.T) {
	r := Report{CPUs: 8, MemoryBytes: 16 * config.GiB, FreeDiskBytes: 40 * config.GiB}
	s := config.Default()
	if e := r.Validate(s, true); e != nil {
		t.Fatal(e)
	}
	s.CPUs = 9
	if r.Validate(s, true) == nil {
		t.Fatal("host CPU limit ignored")
	}
	s = config.Default()
	s.MemoryGiB = 9
	if r.Validate(s, true) == nil {
		t.Fatal("memory limit ignored")
	}
	s = config.Default()
	r.FreeDiskBytes = 10 * config.GiB
	if r.Validate(s, true) == nil {
		t.Fatal("disk headroom ignored")
	}
	if r.Validate(s, false) != nil {
		t.Fatal("restart should not require another whole disk")
	}
	r.Problems = []string{"virtualization unsupported"}
	if r.Validate(s, false) == nil {
		t.Fatal("missing prerequisite ignored")
	}
}
