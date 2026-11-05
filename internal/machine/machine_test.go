// SPDX-License-Identifier: Apache-2.0
package machine

import (
	"context"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"testing"
)

type recording struct{ calls []protocol.Request }

func (r *recording) Status(context.Context) (lifecycle.State, error) {
	return lifecycle.State{State: "ready"}, nil
}
func (r *recording) GuestCall(_ context.Context, q protocol.Request) (*protocol.Info, error) {
	r.calls = append(r.calls, q)
	return &protocol.Info{InputResult: &protocol.InputResult{Dispatched: true}, Execution: &protocol.Execution{ExitCode: 0, CleanupConfirmed: true}}, nil
}
func TestOperationsRouteToGuestContract(t *testing.T) {
	backend := &recording{}
	m := Local{Controller: backend}
	ctx := context.Background()
	if _, e := m.Input(ctx, protocol.Input{Kind: "key", Key: "ENTER", FrameID: protocol.ID()}); e != nil {
		t.Fatal(e)
	}
	if _, e := m.Exec(ctx, protocol.ExecRequest{Argv: []string{"/bin/echo", "guest"}, TimeoutMS: 1, OutputLimit: 1}); e != nil {
		t.Fatal(e)
	}
	if e := m.Open(ctx, "editor"); e != nil {
		t.Fatal(e)
	}
	if len(backend.calls) != 3 || backend.calls[0].Method != "input" || backend.calls[1].Method != "exec" || backend.calls[1].Exec.Argv[0] != "/bin/echo" || backend.calls[2].Method != "open" {
		t.Fatalf("incorrect routing: %#v", backend.calls)
	}
	before := len(backend.calls)
	if _, e := m.Input(ctx, protocol.Input{Kind: "click", X: -1, FrameID: protocol.ID()}); e == nil || len(backend.calls) != before {
		t.Fatal("invalid input reached backend")
	}
}
