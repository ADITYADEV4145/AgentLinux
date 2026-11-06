// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"context"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"testing"
	"time"
)

func TestFleetTenantIsolationMultipleMachinesAndRevocation(t *testing.T) {
	m, _, alice, bob, _ := fixture(t)
	m.db.Limits.MaxPerOwner = 2
	m.db.Limits.MaxSessions = 4
	a := ready(t, m, alice)
	b := ready(t, m, alice)
	_ = ready(t, m, bob)
	before := m.db.Sessions[a.ID].LastAction
	f, e := m.List(alice)
	if e != nil || len(f.Sessions) != 2 || f.Limits.MaxPerOwner != 2 {
		t.Fatal(f, e)
	}
	for _, s := range f.Sessions {
		if s.Owner != alice.ID || s.Workspace != alice.Workspace || s.Policy == nil || s.IdleStopAt.IsZero() {
			t.Fatal("fleet ownership/metadata")
		}
	}
	if !m.db.Sessions[a.ID].LastAction.Equal(before) {
		t.Fatal("listing extends idle deadline")
	}
	if _, e = m.Create(alice, protocol.ID(), Create{alice.Workspace, 60, "desktop"}); e == nil {
		t.Fatal("third machine exceeds cap")
	}
	if _, e = m.Stop(alice, b.ID); e != nil {
		t.Fatal(e)
	}
	if e = m.Tick(context.Background()); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Create(alice, protocol.ID(), Create{alice.Workspace, 60, "desktop"}); e != nil {
		t.Fatal("freed slot not reusable", e)
	}
	if e = m.Revoke(alice); e != nil {
		t.Fatal(e)
	}
	if _, e = m.List(alice); e == nil {
		t.Fatal("revoked credential can list")
	}
	bob.Expires = time.Now().Add(-time.Hour) // Stored identity is authoritative, not caller fields.
	if _, e = m.List(bob); e != nil {
		t.Fatal("trusted caller fields over stored identity", e)
	}
}
