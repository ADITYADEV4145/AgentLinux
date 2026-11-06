// SPDX-License-Identifier: Apache-2.0
package viewer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

// EnableInput only exposes authenticated guest operations with an ownership lease.
// No host clipboard, host command or VM lifecycle route is added.
func (w *Web) EnableInput() { w.human = NewHuman(w.hub.Machine) }
func (w *Web) input(out http.ResponseWriter, r *http.Request) {
	out.Header().Set("Content-Type", "application/json")
	if w.human == nil || w.human.Control == nil {
		out.WriteHeader(501)
		_ = json.NewEncoder(out).Encode(map[string]string{"code": "UNSUPPORTED_OPERATION"})
		return
	}
	r.Body = http.MaxBytesReader(out, r.Body, protocol.MaxRequest)
	defer r.Body.Close()
	var q struct {
		Action     string          `json:"action"`
		Generation string          `json:"generation"`
		Input      *protocol.Input `json:"input"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || d.Decode(new(any)) != io.EOF {
		http.Error(out, "invalid input", 400)
		return
	}
	w.inputMu.Lock()
	defer w.inputMu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	var e error
	switch q.Action {
	case "acquire", "renew":
		e = w.human.Control.ControlLease(ctx, q.Action, q.Generation)
		if e == nil {
			w.human.Owned = true
			w.human.Generation = q.Generation
			w.human.Renewed = time.Now()
		}
	case "release":
		w.human.Release()
	case "input":
		if !w.human.Owned {
			e = protocol.Fail("INPUT_NOT_OWNED", "take control before typing or clicking")
			break
		}
		if q.Input == nil {
			e = protocol.Fail("INVALID_ARGUMENT", "input required")
			break
		}
		if e = q.Input.Validate(); e == nil {
			_, e = w.human.Machine.Input(ctx, *q.Input)
		}
		if e == nil {
			w.hub.Invalidate()
		} else {
			w.human.Release()
		}
	default:
		e = protocol.Fail("INVALID_ARGUMENT", "unknown input action")
	}
	if e != nil {
		out.WriteHeader(409)
		_ = json.NewEncoder(out).Encode(map[string]string{"code": Code(e)})
		return
	}
	_ = json.NewEncoder(out).Encode(map[string]any{"ok": true, "owned": w.human.Owned})
}
