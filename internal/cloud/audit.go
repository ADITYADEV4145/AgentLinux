// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"encoding/json"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
	"os"
	"path/filepath"
)

func (m *Manager) audit(event string, s Session) error {
	if e := storage.SafeFile(m.root, "cloud-events.jsonl"); e != nil {
		return e
	}
	path := filepath.Join(m.root, "cloud-events.jsonl")
	if st, e := os.Stat(path); e == nil && st.Size() > 1<<20 {
		if e = storage.SafeFile(m.root, "cloud-events.previous.jsonl"); e != nil {
			return e
		}
		if e = os.Rename(path, filepath.Join(m.root, "cloud-events.previous.jsonl")); e != nil {
			return e
		}
	}
	file, e := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if e != nil {
		return e
	}
	defer file.Close()
	if e = json.NewEncoder(file).Encode(map[string]any{"at": m.now().UTC(), "event": event, "session": s.ID, "owner": s.Owner, "workspace": s.Workspace, "providerResource": s.ProviderID, "state": s.State, "errorCode": s.Error, "simulated": s.Simulated}); e != nil {
		return e
	}
	return file.Sync()
}
