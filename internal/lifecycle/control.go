// SPDX-License-Identifier: Apache-2.0
package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

// Input leases coordinate trusted frontends; they are not an isolation boundary
// against another process running as the same host user or malicious guest apps.
type inputLease struct {
	Owner      string    `json:"owner"`
	Machine    string    `json:"machine"`
	Session    string    `json:"session"`
	Generation string    `json:"generation"`
	Expires    time.Time `json:"expires"`
}

func (c *Controller) lease() (inputLease, error) {
	var l inputLease
	e := storage.Read(c.Root, "input-lease.json", &l)
	if os.IsNotExist(e) {
		return l, nil
	}
	return l, e
}
func leaseLive(l inputLease) bool {
	remaining := time.Until(l.Expires)
	return protocol.ValidID(l.Owner) && remaining > 0 && remaining <= 15*time.Second
}
func (c *Controller) checkInputLease(s State) error {
	l, e := c.lease()
	if e != nil {
		return e
	}
	if leaseLive(l) && l.Machine == s.MachineID && l.Owner != c.ControlOwner {
		return protocol.Fail("INPUT_OWNED", "human viewer owns input; observation is available; wait for explicit handoff")
	}
	if c.ControlOwner != "" && (!leaseLive(l) || l.Owner != c.ControlOwner || l.Machine != s.MachineID) {
		return protocol.Fail("INPUT_NOT_OWNED", "input lease lost; observe and explicitly retake control")
	}
	return nil
}
func (c *Controller) ControlLease(ctx context.Context, action, generation string) error {
	if !protocol.ValidID(c.ControlOwner) {
		return protocol.Fail("INVALID_ARGUMENT", "viewer control owner is missing")
	}
	unlock, e := storage.LockWait(ctx, c.Root, 2*time.Second)
	if e != nil {
		return e
	}
	defer unlock()
	l, e := c.lease()
	if e != nil {
		return e
	}
	if action == "release" {
		if l.Owner == c.ControlOwner {
			return os.Remove(filepath.Join(c.Root, "input-lease.json"))
		}
		return nil
	}
	if action != "acquire" && action != "renew" {
		return protocol.Fail("INVALID_ARGUMENT", "invalid lease operation")
	}
	if leaseLive(l) && l.Owner != c.ControlOwner {
		return protocol.Fail("INPUT_OWNED", "another human viewer owns input")
	}
	if action == "renew" && (!leaseLive(l) || l.Owner != c.ControlOwner) {
		return protocol.Fail("INPUT_NOT_OWNED", "lease expired or revoked")
	}
	s, e := c.load()
	if e != nil {
		return e
	}
	if s.State != "ready" {
		return protocol.Fail("NOT_READY", "machine is not ready")
	}
	b, ok := c.Backend.(interface {
		Call(context.Context, State, protocol.Request) (*protocol.Info, error)
	})
	if !ok {
		return errors.New("backend does not support input ownership")
	}
	q, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	info, e := b.Call(q, s, protocol.Request{Method: "screenshot"})
	if e != nil {
		return e
	}
	if info.Frame == nil || info.Frame.Generation != generation {
		return protocol.Fail("STALE_FRAME", "desktop generation changed; observe before retaking input")
	}
	if action == "renew" && (l.Generation != generation || l.Session != info.SessionID) {
		_ = os.Remove(filepath.Join(c.Root, "input-lease.json"))
		return protocol.Fail("INPUT_NOT_OWNED", "desktop restarted; explicit takeover required")
	}
	return storage.JSON(c.Root, "input-lease.json", inputLease{c.ControlOwner, s.MachineID, info.SessionID, generation, time.Now().Add(15 * time.Second)})
}
func (c *Controller) revokeLease() error {
	if e := storage.SafeFile(c.Root, "input-lease.json"); e != nil {
		return e
	}
	e := os.Remove(filepath.Join(c.Root, "input-lease.json"))
	if os.IsNotExist(e) {
		return nil
	}
	return e
}
