// SPDX-License-Identifier: Apache-2.0
package lifecycle

import (
	"context"
	"errors"
	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
	"testing"
)

func TestPinnedIdentityAndStartupAdmission(t *testing.T) {
	c, f, spec := setup(t)
	id := protocol.ID()
	c.ExpectedID = id
	c.Reserve = func(context.Context, config.Spec) (func(), error) { return nil, errors.New("budget exceeded") }
	if _, e := c.Up(context.Background(), spec, true); e == nil || f.started != 0 || f.created != 0 {
		t.Fatal("VM started without reservation")
	}
	c.Reserve = nil
	state, e := c.Up(context.Background(), spec, false)
	if e != nil || state.MachineID != id {
		t.Fatal("reserved identity not used", e)
	}
	state.MachineID = protocol.ID()
	if e = storage.JSON(c.Root, "state.json", state); e != nil {
		t.Fatal(e)
	}
	if _, e = c.Status(context.Background()); e == nil {
		t.Fatal("long-lived controller followed replacement")
	}
	if _, e = c.GuestCall(context.Background(), protocol.Request{Method: "screenshot"}); e == nil {
		t.Fatal("input/capture routed to replacement")
	}
}
