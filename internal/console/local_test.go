// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"github.com/ADITYADEV4145/AgentLinux/internal/machine"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"sync"
	"testing"
	"time"
)

type localDouble struct {
	mu            sync.Mutex
	starts, stops int
	wait          chan struct{}
}

func (d *localDouble) List(context.Context) (any, error) {
	return map[string]any{"machines": []any{}, "budget": map[string]int{"cpus": 4, "memoryGiB": 8}}, nil
}
func (d *localDouble) Create(context.Context, string, int, int, int) (string, error) {
	return "0123456789abcdef0123456789abcdef", nil
}
func (d *localDouble) Start(ctx context.Context, _ string) error {
	d.mu.Lock()
	d.starts++
	d.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-d.wait:
		return nil
	}
}
func (d *localDouble) Stop(context.Context, string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stops++
	return nil
}
func (d *localDouble) Computer(context.Context, string) (machine.Machine, error) {
	return nil, protocol.Fail("NOT_READY", "not ready")
}
func TestLocalDashboardAuthStartDedupStopConfirmationAndNoCapture(t *testing.T) {
	s, e := New(nil)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	d := &localDouble{wait: make(chan struct{})}
	s.EnableLocal(d)
	id := "0123456789abcdef0123456789abcdef"
	if request(t, s, "GET", "/api/local/machines", "", false).Code != 401 {
		t.Fatal("unauthenticated local inventory")
	}
	if request(t, s, "GET", "/api/local/machines", "", true).Code != 200 {
		t.Fatal("inventory unavailable")
	}
	for i := 0; i < 2; i++ {
		if request(t, s, "POST", "/api/local/machines/"+id+"/start", "{}", true).Code != 202 {
			t.Fatal("start not asynchronous")
		}
	}
	deadline := time.Now().Add(time.Second)
	for {
		d.mu.Lock()
		n := d.starts
		d.mu.Unlock()
		if n > 0 {
			if n != 1 {
				t.Fatal("start replayed")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("start not dispatched")
		}
		time.Sleep(time.Millisecond)
	}
	if request(t, s, "POST", "/api/local/machines/"+id+"/stop", "{}", true).Code != 409 {
		t.Fatal("unconfirmed stop")
	}
	if request(t, s, "POST", "/api/local/machines/"+id+"/stop", `{"confirm":true}`, true).Code != 200 {
		t.Fatal("explicit stop failed")
	}
	if request(t, s, "POST", "/api/local/machines/"+id+"/view", "{}", true).Code != 409 || len(s.viewers) != 0 {
		t.Fatal("unready viewer created")
	}
	close(d.wait)
}

func (d *localDouble) Checkpoints(context.Context, string) (any, error) {
	return map[string]any{"checkpoints": []any{}, "recovery": nil}, nil
}
func (d *localDouble) Recovery(context.Context, string, string, string, string) (any, error) {
	return nil, nil
}
func TestLocalRecoveryRequiresAuthAndExplicitConsent(t *testing.T) {
	s, e := New(nil)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	s.EnableLocal(&localDouble{wait: make(chan struct{})})
	id := "0123456789abcdef0123456789abcdef"
	path := "/api/local/machines/" + id
	if request(t, s, "GET", path+"/checkpoints", "", false).Code != 401 {
		t.Fatal("unauthorized checkpoints")
	}
	if request(t, s, "GET", path+"/checkpoints", "", true).Code != 200 {
		t.Fatal("checkpoint list failed")
	}
	if request(t, s, "POST", path+"/recovery", `{"action":"restore","name":"safe"}`, true).Code != 409 {
		t.Fatal("restore without consent")
	}
	if request(t, s, "POST", path+"/recovery", `{"action":"restore","name":"safe","confirm":true,"stateDirectory":"/"}`, true).Code != 409 {
		t.Fatal("arbitrary host path accepted")
	}
}
