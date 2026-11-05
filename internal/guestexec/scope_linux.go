// SPDX-License-Identifier: Apache-2.0
package guestexec

import (
	"errors"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type cgroup struct {
	path string
	fd   *os.File
}

func newScope() (scope, error) {
	b, e := os.ReadFile("/proc/self/cgroup")
	if e != nil {
		return nil, e
	}
	relative := ""
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "0::") {
			relative = strings.TrimPrefix(line, "0::")
		}
	}
	if !filepath.IsAbs(relative) || filepath.Clean(relative) != relative || relative == "/" {
		return nil, errors.New("service has no delegated unified cgroup")
	}
	base := filepath.Join("/sys/fs/cgroup", relative)
	path := filepath.Join(base, "exec-"+protocol.ID())
	if e = os.Mkdir(path, 0700); e != nil {
		return nil, e
	}
	fd, e := os.Open(path)
	if e != nil {
		os.Remove(path)
		return nil, e
	}
	if _, e = os.Stat(filepath.Join(path, "cgroup.kill")); e != nil {
		fd.Close()
		os.Remove(path)
		return nil, e
	}
	return &cgroup{path, fd}, nil
}
func (c *cgroup) Apply(cmd *exec.Cmd) {
	cmd.SysProcAttr.UseCgroupFD = true
	cmd.SysProcAttr.CgroupFD = int(c.fd.Fd())
}
func (c *cgroup) Kill() error {
	return os.WriteFile(filepath.Join(c.path, "cgroup.kill"), []byte("1"), 0600)
}
func (c *cgroup) Close() error {
	c.fd.Close()
	for i := 0; i < 100; i++ {
		if e := os.Remove(c.path); e == nil {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return errors.New("command cgroup remains populated")
}
