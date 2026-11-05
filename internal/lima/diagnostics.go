// SPDX-License-Identifier: Apache-2.0

package lima

import (
	"encoding/json"
	"errors"
	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/host"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func Find(name string) (string, error) {
	p, e := exec.LookPath(name)
	if e != nil {
		return "", errors.New("limactl missing; install reviewed Lima " + config.LimaVersion + " or pass --lima")
	}
	p, e = filepath.Abs(p)
	if e != nil {
		return "", e
	}
	r := host.Inspect("/", p)
	if r.LimaVersion != config.LimaVersion {
		return "", errors.New("only pinned Lima " + config.LimaVersion + " is accepted")
	}
	return p, nil
}

var privateKey = regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`)
var longData = regexp.MustCompile(`[A-Za-z0-9+/=_-]{80,}`)
var credential = regexp.MustCompile(`(?i)([a-z_]*(?:token|password|secret|api_key)[a-z_]*\s*[=:]\s*)\S+`)

func Redact(s string) string {
	s = privateKey.ReplaceAllString(s, "[private key redacted]")
	s = credential.ReplaceAllString(s, "${1}[redacted]")
	s = longData.ReplaceAllString(s, "[long data redacted]")
	if h, _ := os.UserHomeDir(); h != "" {
		s = strings.ReplaceAll(s, h, "[home]")
	}
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r >= 32 && r != 127 {
			return r
		}
		return -1
	}, s)
	if len(s) > 4096 {
		s = s[len(s)-4096:]
	}
	return s
}
func (b *Backend) diagnostic(op string, stderr []byte) error {
	if e := storage.SafeFile(b.Root, "backend.jsonl"); e != nil {
		return e
	}
	p := filepath.Join(b.Root, "backend.jsonl")
	if i, e := os.Stat(p); e == nil && i.Size() > 256<<10 {
		if e = storage.Write(b.Root, "backend.jsonl", nil); e != nil {
			return e
		}
	}
	f, e := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(map[string]any{"time": time.Now().UTC(), "operation": op, "diagnostic": Redact(string(stderr))})
}
