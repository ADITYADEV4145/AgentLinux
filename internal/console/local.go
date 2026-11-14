// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/machine"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/viewer"
)

// LocalComputer is deliberately independent of the cloud client. All operations
// resolve and pin a stable identity on the server, never an arbitrary host path.
type LocalComputer interface {
	List(context.Context) (any, error)
	Create(context.Context, string, int, int, int) (string, error)
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Computer(context.Context, string) (machine.Machine, error)
}
type localJob struct {
	ID        string `json:"machineId"`
	State     string `json:"state"`
	Error     string `json:"error,omitempty"`
	Operation string `json:"operation,omitempty"`
}

func (s *Server) EnableLocal(local LocalComputer) { s.local = local; s.jobs = map[string]localJob{} }
func (s *Server) localRequest(w http.ResponseWriter, r *http.Request, ctx context.Context) bool {
	if s.local == nil || !strings.HasPrefix(r.URL.Path, "/api/local/") {
		return false
	}
	route := strings.TrimPrefix(r.URL.Path, "/api/local/")
	if route == "machines" && r.Method == "GET" {
		v, e := s.local.List(ctx)
		s.mu.Lock()
		jobs := make(map[string]localJob, len(s.jobs))
		for k, v := range s.jobs {
			jobs[k] = v
		}
		s.mu.Unlock()
		send(w, map[string]any{"fleet": v, "jobs": jobs}, e)
		return true
	}
	if route == "machines" && r.Method == "POST" {
		var q struct {
			Name   string `json:"name"`
			CPUs   int    `json:"cpus"`
			Memory int    `json:"memoryGiB"`
			Disk   int    `json:"diskGiB"`
		}
		if e := decode(w, r, &q); e != nil {
			send(w, nil, e)
			return true
		}
		id, e := s.local.Create(ctx, q.Name, q.CPUs, q.Memory, q.Disk)
		if e != nil {
			send(w, nil, e)
			return true
		}
		s.localStart(w, id)
		return true
	}
	parts := strings.Split(route, "/")
	if len(parts) != 3 || parts[0] != "machines" || !protocol.ValidID(parts[1]) {
		http.NotFound(w, r)
		return true
	}
	id, action := parts[1], parts[2]
	if s.localRecoveryRequest(w, r, ctx, id, action) {
		return true
	}
	if r.Method != "POST" {
		http.NotFound(w, r)
		return true
	}
	var q struct {
		Confirm bool `json:"confirm"`
	}
	if e := decode(w, r, &q); e != nil {
		send(w, nil, e)
		return true
	}
	switch action {
	case "start":
		s.localStart(w, id)
	case "stop":
		if !q.Confirm {
			send(w, nil, protocol.Fail("CONFIRMATION_REQUIRED", "Explicit stop confirmation required."))
			return true
		}
		send(w, map[string]bool{"stopped": true}, s.local.Stop(ctx, id))
	case "view":
		s.mu.Lock()
		defer s.mu.Unlock()
		if v := s.viewers[id]; v != nil {
			send(w, map[string]string{"url": v.URL}, nil)
			return true
		}
		if len(s.viewers) >= 4 {
			send(w, nil, protocol.Fail("LIMIT_EXCEEDED", "Close/restart the dashboard to release viewer slots."))
			return true
		}
		computer, e := s.local.Computer(ctx, id)
		if e != nil {
			send(w, nil, e)
			return true
		}
		// No background captures until the opened viewer requests a frame.
		v, e := viewer.NewWeb(viewer.NewHub(computer, time.Second), id)
		if e != nil {
			send(w, nil, e)
			return true
		}
		v.EnableInput()
		s.viewers[id] = v
		go func() { _ = v.Serve(s.ctx) }()
		send(w, map[string]string{"url": v.URL}, nil)
	default:
		http.NotFound(w, r)
	}
	return true
}
func (s *Server) localStart(w http.ResponseWriter, id string) {
	s.mu.Lock()
	if job, ok := s.jobs[id]; ok && job.State == "starting" {
		if job.Operation != "" {
			s.mu.Unlock()
			send(w, nil, protocol.Fail("MACHINE_BUSY", "Recovery operation in progress; inspect it before starting."))
			return
		}
		s.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		send(w, job, nil)
		return
	}
	if len(s.jobs) >= 64 {
		if _, ok := s.jobs[id]; !ok {
			s.mu.Unlock()
			send(w, nil, protocol.Fail("LIMIT_EXCEEDED", "Dashboard operation history is full; restart the dashboard."))
			return
		}
	}
	job := localJob{ID: id, State: "starting"}
	s.jobs[id] = job
	s.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(s.ctx, 20*time.Minute)
		defer cancel()
		e := s.local.Start(ctx, id)
		result := localJob{ID: id, State: "ready"}
		if e != nil {
			result.State = "error"
			result.Error = "Startup failed or was interrupted; inspect machine status/logs before retrying."
			if pe, ok := e.(*protocol.Failure); ok {
				result.Error = pe.Error()
			}
		}
		s.mu.Lock()
		s.jobs[id] = result
		s.mu.Unlock()
	}()
	w.WriteHeader(http.StatusAccepted)
	send(w, job, nil)
}
