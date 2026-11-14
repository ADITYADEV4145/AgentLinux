// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"net/http"
	"strings"
	"time"
)

type LocalRecovery interface {
	Checkpoints(context.Context, string) (any, error)
	Recovery(context.Context, string, string, string, string) (any, error)
}

func (s *Server) localRecoveryRequest(w http.ResponseWriter, r *http.Request, ctx context.Context, id, route string) bool {
	if route != "checkpoints" && route != "recovery" {
		return false
	}
	local, ok := s.local.(LocalRecovery)
	if !ok {
		send(w, nil, protocol.Fail("UNSUPPORTED_OPERATION", "Local backend does not support checkpoint/recovery operations."))
		return true
	}
	if route == "checkpoints" && r.Method == "GET" {
		result, e := local.Checkpoints(ctx, id)
		send(w, result, e)
		return true
	}
	if route != "recovery" || r.Method != "POST" {
		http.NotFound(w, r)
		return true
	}
	var q struct {
		Action  string `json:"action"`
		Name    string `json:"name"`
		Mode    string `json:"mode"`
		Confirm bool   `json:"confirm"`
	}
	if e := decode(w, r, &q); e != nil {
		send(w, nil, e)
		return true
	}
	switch q.Action {
	case "create", "restore", "delete", "recover", "desktop-restart", "helper-upgrade":
	default:
		send(w, nil, protocol.Fail("INVALID_ARGUMENT", "Unknown explicit recovery action."))
		return true
	}
	if q.Action != "create" && !q.Confirm {
		send(w, nil, protocol.Fail("CONFIRMATION_REQUIRED", "Review the recovery warning and confirm explicitly."))
		return true
	}
	s.mu.Lock()
	if old, exists := s.jobs[id]; exists && old.State == "starting" {
		s.mu.Unlock()
		send(w, nil, protocol.Fail("MACHINE_BUSY", "Another operation is already pending for this machine."))
		return true
	}
	if len(s.jobs) >= 64 {
		if _, exists := s.jobs[id]; !exists {
			s.mu.Unlock()
			send(w, nil, protocol.Fail("LIMIT_EXCEEDED", "Dashboard job limit reached; restart the dashboard."))
			return true
		}
	}
	operation := strings.ReplaceAll(q.Action, "-", " ")
	job := localJob{ID: id, State: "starting", Operation: operation}
	s.jobs[id] = job
	s.mu.Unlock()
	go func() {
		deadline, cancel := context.WithTimeout(s.ctx, 20*time.Minute)
		defer cancel()
		_, e := local.Recovery(deadline, id, q.Action, q.Name, q.Mode)
		finished := localJob{ID: id, State: "complete", Operation: operation}
		if e != nil {
			finished.State = "error"
			finished.Error = "Recovery operation failed. Inspect checkpoint list and machine logs before retrying; rollback data is retained when recovery is pending."
			if pe, ok := e.(*protocol.Failure); ok {
				finished.Error = pe.Error()
			}
		}
		s.mu.Lock()
		s.jobs[id] = finished
		s.mu.Unlock()
	}()
	w.WriteHeader(http.StatusAccepted)
	send(w, job, nil)
	return true
}
