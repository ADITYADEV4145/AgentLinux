// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

type envelope struct {
	OK    bool              `json:"ok"`
	Data  json.RawMessage   `json:"data,omitempty"`
	Error *protocol.Failure `json:"error,omitempty"`
}
type rate struct {
	tokens   float64
	at       time.Time
	inflight int
}
type API struct {
	Manager *Manager
	mu      sync.Mutex
	rates   map[string]*rate
	gate    chan struct{}
}

func NewAPI(m *Manager) *API {
	return &API{Manager: m, rates: map[string]*rate{}, gate: make(chan struct{}, 8)}
}
func writeJSON(w http.ResponseWriter, code int, data any, e error) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Agent-Linux-Cloud-Version", Version)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	v := envelope{OK: e == nil}
	if e != nil {
		v.Error = &protocol.Failure{Code: "SERVICE_UNAVAILABLE", Message: "cloud operation unavailable; do not blindly retry mutations"}
		var f *protocol.Failure
		if errors.As(e, &f) {
			v.Error = f
		}
		if errors.Is(e, context.DeadlineExceeded) {
			v.Error = &protocol.Failure{Code: "DEADLINE_EXCEEDED", Message: "operation deadline exceeded; outcome may be unknown"}
		}
	} else {
		v.Data, _ = json.Marshal(data)
	}
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func decode(r *http.Request, v any) error {
	raw, err := io.ReadAll(io.LimitReader(r.Body, protocol.MaxRequest+1))
	if err != nil || len(raw) > protocol.MaxRequest {
		return protocol.Fail("INVALID_MESSAGE", "request exceeds cloud body limit")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return protocol.Fail("INVALID_MESSAGE", "invalid JSON request")
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return protocol.Fail("INVALID_MESSAGE", "exactly one bounded JSON object required")
	}
	return nil
}
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "" || r.URL.RawQuery != "" || len(r.Header.Get("Authorization")) > 128 {
		writeJSON(w, 403, nil, protocol.Fail("FORBIDDEN", "browser origins and URL credentials are not accepted"))
		return
	}
	if r.URL.Path == "/v1/stripe/webhook" {
		a.billingWebhook(w, r)
		return
	}
	// Authenticate before resolving session IDs or revealing method/path existence.
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == r.Header.Get("Authorization") {
		token = ""
	}
	who, e := a.Manager.Authenticate(token)
	if e != nil {
		writeJSON(w, 401, nil, e)
		return
	}
	select {
	case a.gate <- struct{}{}:
		defer func() { <-a.gate }()
	default:
		writeJSON(w, 429, nil, protocol.Fail("RATE_LIMITED", "control-plane concurrency limit"))
		return
	}
	a.mu.Lock()
	now := time.Now()
	b := a.rates[who.ID]
	if b == nil {
		b = &rate{tokens: 16, at: now}
		a.rates[who.ID] = b
	}
	b.tokens = min(16, b.tokens+now.Sub(b.at).Seconds()*4)
	b.at = now
	if b.tokens < 1 || b.inflight >= 2 {
		a.mu.Unlock()
		writeJSON(w, 429, nil, protocol.Fail("RATE_LIMITED", "identity request/concurrency limit"))
		return
	}
	b.tokens--
	b.inflight++
	a.mu.Unlock()
	defer func() { a.mu.Lock(); b.inflight--; a.mu.Unlock() }()
	r.Body = http.MaxBytesReader(w, r.Body, protocol.MaxRequest)
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 70*time.Second)
	defer cancel()
	var result any
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case r.URL.Path == "/v1/billing" && r.Method == "GET":
		result, e = a.Manager.Billing(who)
	case r.URL.Path == "/v1/billing" && r.Method == "POST":
		var q struct {
			Action    string `json:"action"`
			RequestID string `json:"requestId"`
			Confirm   bool   `json:"confirm"`
		}
		e = decode(r, &q)
		if e == nil {
			result, e = a.Manager.BillingAction(who, q.RequestID, q.Action, q.Confirm)
		}
	case r.URL.Path == "/v1/sessions" && r.Method == "GET":
		result, e = a.Manager.List(who)
	case r.URL.Path == "/v1/sessions" && r.Method == "POST":
		var q Create
		e = decode(r, &q)
		if e == nil {
			result, e = a.Manager.Create(who, r.Header.Get("Idempotency-Key"), q)
		}
	case r.URL.Path == "/v1/revoke" && r.Method == "POST":
		e = a.Manager.Revoke(who)
	case len(parts) >= 3 && parts[0] == "v1" && parts[1] == "sessions":
		id := parts[2]
		// Authorize every route, including unavailable file/stream routes.
		if _, e = a.Manager.Get(who, id); e != nil {
			break
		}
		switch {
		case len(parts) == 3 && r.Method == "GET":
			result, e = a.Manager.Get(who, id)
		case len(parts) == 4 && parts[3] == "stop" && r.Method == "POST":
			result, e = a.Manager.Stop(who, id)
		case len(parts) == 4 && parts[3] == "control" && r.Method == "POST":
			var q LeaseRequest
			e = decode(r, &q)
			if e == nil {
				e = a.Manager.Control(who, id, q)
			}
		case len(parts) == 4 && parts[3] == "call" && r.Method == "POST":
			var q Operation
			e = decode(r, &q)
			if e == nil {
				result, e = a.Manager.Call(ctx, who, id, q)
			}
		default:
			e = protocol.Fail("UNSUPPORTED_OPERATION", "route not supported; no cloud files or raw VNC/stream credentials")
		}
	default:
		e = protocol.Fail("NOT_FOUND", "route not found")
	}
	status := 200
	if e != nil {
		status = 400
		var f *protocol.Failure
		if errors.As(e, &f) {
			switch f.Code {
			case "UNAUTHORIZED":
				status = 401
			case "FORBIDDEN":
				status = 403
			case "NOT_FOUND":
				status = 404
			case "MACHINE_BUSY", "LIMIT_EXCEEDED", "IDEMPOTENCY_CONFLICT":
				status = 409
			case "NOT_READY":
				status = 503
			case "UNSUPPORTED_OPERATION":
				status = 501
			}
		} else {
			status = 503
		}
	}
	writeJSON(w, status, result, e)
}
