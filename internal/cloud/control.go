// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"context"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"time"
)

type controlLease struct {
	Owner, Generation string
	Expires           time.Time
}
type LeaseRequest struct {
	Action     string `json:"action"`
	Owner      string `json:"owner"`
	Generation string `json:"generation"`
}

func (m *Manager) Control(i Identity, id string, q LeaseRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, e := m.owner(i, id)
	if e != nil {
		return e
	}
	if !protocol.ValidID(q.Owner) {
		return protocol.Fail("INVALID_ARGUMENT", "invalid viewer ownership nonce")
	}
	l, held := m.leases[id]
	live := held && m.now().Before(l.Expires)
	if q.Action == "release" {
		if l.Owner == q.Owner {
			delete(m.leases, id)
		}
		return nil
	}
	if s.State != "ready" || !m.now().Before(s.Expires) || s.Generation != q.Generation {
		return protocol.Fail("STALE_FRAME", "session changed; observe before taking control")
	}
	if q.Action != "acquire" && q.Action != "renew" {
		return protocol.Fail("INVALID_ARGUMENT", "invalid lease action")
	}
	if live && l.Owner != q.Owner {
		return protocol.Fail("INPUT_OWNED", "another viewer owns input")
	}
	if q.Action == "renew" && (!live || l.Owner != q.Owner || l.Generation != q.Generation) {
		return protocol.Fail("INPUT_NOT_OWNED", "lease expired/revoked; explicitly retake control")
	}
	if m.busy[id] {
		return protocol.Fail("MACHINE_BUSY", "an action is still completing; observe before takeover")
	}
	m.leases[id] = controlLease{q.Owner, q.Generation, m.now().Add(15 * time.Second)}
	return nil
}
func (c *Client) ControlLease(ctx context.Context, action, generation string) error {
	return c.request(ctx, "POST", "/v1/sessions/"+c.Config.Session+"/control", "", LeaseRequest{action, c.ControlOwner, generation}, nil)
}
