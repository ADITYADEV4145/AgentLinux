// SPDX-License-Identifier: Apache-2.0

package host

import (
	"context"
	"fmt"
	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Report struct {
	OS             string   `json:"os"`
	Arch           string   `json:"arch"`
	MacOS          string   `json:"macOS,omitempty"`
	CPUs           int      `json:"cpus"`
	MemoryBytes    uint64   `json:"memoryBytes"`
	FreeDiskBytes  uint64   `json:"freeDiskBytes"`
	Virtualization bool     `json:"virtualization"`
	LimaPath       string   `json:"limaPath,omitempty"`
	LimaVersion    string   `json:"limaVersion,omitempty"`
	Capabilities   []string `json:"capabilities"`
	Problems       []string `json:"problems"`
}

func output(name string, args ...string) string {
	ctx, c := context.WithTimeout(context.Background(), 3*time.Second)
	defer c()
	b, e := exec.CommandContext(ctx, name, args...).Output()
	if e != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
func Inspect(path, lima string) Report {
	r := Report{OS: runtime.GOOS, Arch: runtime.GOARCH, CPUs: runtime.NumCPU(), Capabilities: []string{}, Problems: []string{}}
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		r.Problems = append(r.Problems, "local VZ backend requires an Apple silicon Mac")
	} else {
		r.MacOS = output("/usr/bin/sw_vers", "-productVersion")
		m, _ := strconv.Atoi(strings.Split(r.MacOS, ".")[0])
		if m < 14 {
			r.Problems = append(r.Problems, "development target requires macOS 14 or later; compatibility matrix is not established")
		}
		r.MemoryBytes, _ = strconv.ParseUint(output("/usr/sbin/sysctl", "-n", "hw.memsize"), 10, 64)
		r.Virtualization = output("/usr/sbin/sysctl", "-n", "kern.hv_support") == "1"
		if !r.Virtualization {
			r.Problems = append(r.Problems, "hardware virtualization unavailable or query denied")
		}
		if r.MemoryBytes == 0 {
			r.Problems = append(r.Problems, "cannot read host memory; run outside a restricted sandbox")
		}
	}
	p := path
	for {
		_, e := os.Stat(p)
		if e == nil {
			break
		}
		parent := strings.LastIndex(p, "/")
		if parent <= 0 {
			p = "/"
			break
		}
		p = p[:parent]
	}
	var st syscall.Statfs_t
	if e := syscall.Statfs(p, &st); e == nil {
		r.FreeDiskBytes = uint64(st.Bavail) * uint64(st.Bsize)
	} else {
		r.Problems = append(r.Problems, "cannot read available disk")
	}
	if lima == "" {
		lima = "limactl"
	}
	r.LimaPath, _ = exec.LookPath(lima)
	if r.LimaPath == "" {
		r.Problems = append(r.Problems, "limactl missing; install reviewed Lima "+config.LimaVersion+" and add it to PATH or use --lima")
	} else {
		r.LimaVersion = strings.TrimPrefix(output(r.LimaPath, "--version"), "limactl version ")
		if r.LimaVersion != config.LimaVersion {
			r.Problems = append(r.Problems, "requires pinned Lima "+config.LimaVersion+"; found "+r.LimaVersion)
		}
	}
	for _, t := range []string{"ssh", "ssh-keygen"} {
		if _, e := exec.LookPath(t); e != nil {
			r.Problems = append(r.Problems, t+" is missing")
		}
	}
	if len(r.Problems) == 0 {
		r.Capabilities = []string{"local.vz", "guest.health", "guest.probe", "persistent-disk", "network.nat"}
	}
	return r
}
func (r Report) Validate(s config.Spec, newDisk bool) error {
	if len(r.Problems) > 0 {
		return fmt.Errorf("prerequisites: %s", strings.Join(r.Problems, "; "))
	}
	if s.CPUs > r.CPUs {
		return fmt.Errorf("requested %d CPUs exceeds host %d", s.CPUs, r.CPUs)
	}
	if uint64(s.MemoryGiB)*config.GiB > r.MemoryBytes/2 {
		return fmt.Errorf("requested memory exceeds half of host RAM")
	}
	need := 2 * config.GiB
	if newDisk {
		need = uint64(s.DiskGiB+6) * config.GiB
	}
	if r.FreeDiskBytes < need {
		return fmt.Errorf("insufficient disk: need %.1f GiB free for disk/conversion/headroom, have %.1f; choose a larger state volume or free space without deleting user data", float64(need)/float64(config.GiB), float64(r.FreeDiskBytes)/float64(config.GiB))
	}
	return nil
}
