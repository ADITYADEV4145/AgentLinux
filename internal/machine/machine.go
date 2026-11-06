// SPDX-License-Identifier: Apache-2.0
// Package machine defines the implemented common computer interface. Cloud is absent.
package machine

import (
	"context"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

type Machine interface {
	Status(context.Context) (lifecycle.State, error)
	Screenshot(context.Context) (*protocol.Frame, error)
	Input(context.Context, protocol.Input) (*protocol.InputResult, error)
	Exec(context.Context, protocol.ExecRequest) (*protocol.Execution, error)
	Open(context.Context, string) error
}
type Controller interface {
	Status(context.Context) (lifecycle.State, error)
	GuestCall(context.Context, protocol.Request) (*protocol.Info, error)
}
type Local struct{ Controller Controller }

func (l Local) Status(ctx context.Context) (lifecycle.State, error) { return l.Controller.Status(ctx) }
func (l Local) Screenshot(ctx context.Context) (*protocol.Frame, error) {
	r, e := l.Controller.GuestCall(ctx, protocol.Request{Method: "screenshot"})
	if e != nil {
		return nil, e
	}
	if r.Frame == nil {
		return nil, protocol.Fail("INVALID_FRAME", "backend returned no screenshot")
	}
	return r.Frame, r.Frame.Validate()
}
func (l Local) Input(ctx context.Context, i protocol.Input) (*protocol.InputResult, error) {
	if i.FrameID == "" {
		f, e := l.Screenshot(ctx)
		if e != nil {
			return nil, e
		}
		i.FrameID = f.ID
	}
	if e := i.Validate(); e != nil {
		return nil, e
	}
	r, e := l.Controller.GuestCall(ctx, protocol.Request{Method: "input", Input: &i})
	if e != nil {
		return nil, e
	}
	if r.InputResult == nil {
		return nil, protocol.Fail("INVALID_MESSAGE", "backend returned no input acknowledgement")
	}
	return r.InputResult, nil
}
func (l Local) Exec(ctx context.Context, q protocol.ExecRequest) (*protocol.Execution, error) {
	if e := q.Validate(); e != nil {
		return nil, e
	}
	r, e := l.Controller.GuestCall(ctx, protocol.Request{Method: "exec", Exec: &q})
	if e != nil {
		return nil, e
	}
	if r.Execution == nil {
		return nil, protocol.Fail("INVALID_MESSAGE", "backend returned no command result")
	}
	return r.Execution, nil
}
func (l Local) Open(ctx context.Context, app string) error {
	_, e := l.Controller.GuestCall(ctx, protocol.Request{Method: "open", App: app})
	return e
}

var _ Machine = Local{}

// InputControl is optional. Viewers must refuse interactive mode when absent.
type InputControl interface {
	ControlLease(context.Context, string, string) error
}

func (l Local) ControlLease(ctx context.Context, action, generation string) error {
	c, ok := l.Controller.(InputControl)
	if !ok {
		return protocol.Fail("UNSUPPORTED_OPERATION", "backend has no input ownership")
	}
	return c.ControlLease(ctx, action, generation)
}
