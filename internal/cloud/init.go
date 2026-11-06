// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

// Init creates random, expiring development identities, not production accounts.
// API credentials are stored hashed; client files are private bearer credentials.
func Init(root, endpoint, provider string) (map[string]string, error) {
	return InitWithLimits(root, endpoint, provider, DefaultLimits())
}

// InitWithLimits changes only a fresh deployment; billing concurrency still applies.
func InitWithLimits(root, endpoint, provider string, limits Limits) (map[string]string, error) {
	if !limits.valid() {
		return nil, errors.New("invalid cloud resource limits")
	}

	if provider != "fake-development" && provider != "e2b-desktop-2.7.2" {
		return nil, errors.New("select fake-development or e2b-desktop-2.7.2")
	}
	if _, e := os.Lstat(filepath.Join(root, "cloud.json")); !os.IsNotExist(e) {
		return nil, errors.New("cloud state already exists; init never overwrites identities or sessions")
	}
	entries, e := os.ReadDir(root)
	if e != nil {
		return nil, e
	}
	for _, entry := range entries {
		if entry.Name() != ".agent-linux-owner" && entry.Name() != "lock" {
			return nil, errors.New("cloud init requires a fresh empty namespace; never use local VM or user-data directories")
		}
	}
	db := Database{Version: Version, Deployment: protocol.ID(), Provider: provider, Limits: limits, Identities: map[string]Identity{}, Sessions: map[string]Session{}}
	result := map[string]string{}
	for _, name := range []string{"alice", "bob"} {
		raw := make([]byte, 32)
		if _, e := rand.Read(raw); e != nil {
			return nil, e
		}
		token := hex.EncodeToString(raw)
		workspace := protocol.ID()
		c := ClientConfig{Version: Version, URL: endpoint, Token: token, Workspace: workspace, Development: true}
		if _, e := NewClient(c); e != nil {
			return nil, e
		}
		db.Identities[name] = Identity{ID: name, Workspace: workspace, TokenHash: HashToken(token), Expires: time.Now().UTC().Add(8 * time.Hour)}
		if e := storage.JSON(root, name+".json", c); e != nil {
			return nil, e
		}
		result[name] = filepath.Join(root, name+".json")
	}
	if e := storage.JSON(root, "cloud.json", db); e != nil {
		return nil, e
	}
	return result, nil
}
func Tools(c *protocol.Capabilities) map[string]bool {
	tools := map[string]bool{"computer_status": true}
	if c == nil {
		return tools
	}
	if c.Displays == 1 && c.Format == "png" {
		tools["computer_screenshot"] = true
	}
	for _, k := range c.Input {
		if k == "text" {
			k = "type"
		}
		tools["computer_"+k] = true
	}
	if c.GuestExec {
		tools["computer_run_command"] = true
	}
	if len(c.Apps) > 0 {
		tools["computer_open"] = true
	}
	return tools
}
