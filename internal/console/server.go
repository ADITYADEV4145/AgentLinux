// SPDX-License-Identifier: Apache-2.0
// Package console serves a local review site and authenticated cloud console.
// Provider credentials stay in Go; the browser receives a separate ephemeral capability.
package console

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/cloud"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/viewer"
)

//go:embed assets/*
var assets embed.FS

type Server struct {
	URL         string
	client      *cloud.Client
	token, host string
	listener    net.Listener
	server      *http.Server
	gate        chan struct{}
	mu          sync.Mutex
	viewers     map[string]*viewer.Web
	ctx         context.Context
	cancel      context.CancelFunc
}

func New(client *cloud.Client) (*Server, error) {
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return nil, e
	}
	var raw [32]byte
	if _, e = rand.Read(raw[:]); e != nil {
		l.Close()
		return nil, e
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{client: client, listener: l, host: l.Addr().String(), token: hex.EncodeToString(raw[:]), gate: make(chan struct{}, 4), viewers: map[string]*viewer.Web{}, ctx: ctx, cancel: cancel}
	s.URL = "http://" + s.host + "/#" + s.token
	var connectionsMu sync.Mutex
	connections := map[net.Conn]bool{}
	s.server = &http.Server{ConnState: func(c net.Conn, state http.ConnState) {
		connectionsMu.Lock()
		defer connectionsMu.Unlock()
		switch state {
		case http.StateNew:
			if len(connections) >= 16 {
				_ = c.Close()
			} else {
				connections[c] = true
			}
		case http.StateClosed, http.StateHijacked:
			delete(connections, c)
		}
	}, Handler: http.HandlerFunc(s.handle), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 12 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 4096}
	return s, nil
}
func (s *Server) Serve(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			s.Close()
		case <-done:
		}
	}()
	e := s.server.Serve(s.listener)
	close(done)
	if errors.Is(e, http.ErrServerClosed) {
		return nil
	}
	return e
}
func (s *Server) Close() {
	s.cancel()
	_ = s.server.Close()
	_ = s.listener.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.viewers {
		v.Close()
	}
	clear(s.viewers)
}
func send(w http.ResponseWriter, value any, e error) {
	w.Header().Set("Content-Type", "application/json")
	if e != nil {
		code := "UNAVAILABLE"
		var f *protocol.Failure
		if errors.As(e, &f) {
			code = f.Code
		}
		w.WriteHeader(409)
		// Protocol errors are curated. Transport errors never expose endpoints/credentials.
		message := "operation unavailable; inspect the control plane and refresh before retrying"
		if f != nil {
			message = f.Message
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"error": code, "message": message})
		return
	}
	_ = json.NewEncoder(w).Encode(value)
}
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	if r.Host != s.host || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+s.host) || r.URL.RawQuery != "" {
		http.Error(w, "forbidden host/origin/query", 403)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		if r.Method != "GET" {
			http.Error(w, "GET only", 405)
			return
		}
		path := r.URL.Path
		if path == "/" || path == "/dashboard" {
			path = "/index.html"
		}
		if path != "/index.html" && path != "/app.js" && path != "/app.css" && path != "/pricing.json" {
			http.NotFound(w, r)
			return
		}
		data, e := assets.ReadFile("assets" + path)
		if e != nil {
			http.NotFound(w, r)
			return
		}
		kinds := map[string]string{"/index.html": "text/html; charset=utf-8", "/app.js": "text/javascript; charset=utf-8", "/app.css": "text/css; charset=utf-8", "/pricing.json": "application/json"}
		w.Header().Set("Content-Type", kinds[path])
		_, _ = w.Write(data)
		return
	}
	got := r.Header.Get("Authorization")
	want := "Bearer " + s.token
	if len(got) != len(want) || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		http.Error(w, "console capability required; reopen printed private URL", 401)
		return
	}
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	default:
		http.Error(w, "console busy", 429)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	if r.URL.Path == "/api/config" && r.Method == "GET" {
		send(w, map[string]any{"connected": s.client != nil, "development": true}, nil)
		return
	}
	if s.client == nil {
		send(w, nil, protocol.Fail("NOT_CONFIGURED", "Start dashboard --cloud-config FILE to attach an existing private development identity. No cloud resources have been created."))
		return
	}
	switch {
	case r.URL.Path == "/api/sessions" && r.Method == "GET":
		f, e := s.client.List(ctx)
		send(w, f, e)
	case r.URL.Path == "/api/billing" && r.Method == "GET":
		v, e := s.client.Billing(ctx)
		send(w, v, e)
	case r.URL.Path == "/api/sessions" && r.Method == "POST":
		var q struct {
			RequestID string `json:"requestId"`
			Seconds   int    `json:"seconds"`
		}
		if e := decode(w, r, &q); e != nil {
			send(w, nil, e)
			return
		}
		v, e := s.client.Create(ctx, q.RequestID, q.Seconds)
		send(w, v, e)
	case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/sessions/"):
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/")
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}
		c, e := s.client.ForSession(parts[0])
		if e != nil {
			send(w, nil, e)
			return
		}
		// Authorize before opening listeners or allocating session resources.
		session, e := c.Session(ctx)
		if e != nil {
			send(w, nil, e)
			return
		}
		var q struct {
			Confirm bool `json:"confirm"`
		}
		if e = decode(w, r, &q); e != nil {
			send(w, nil, e)
			return
		}
		switch parts[1] {
		case "stop":
			if !q.Confirm {
				send(w, nil, protocol.Fail("CONFIRM_REQUIRED", "Stopping discards this guest's files. Explicit confirmation required."))
				return
			}
			v, e := c.Stop(ctx)
			send(w, v, e)
			s.mu.Lock()
			if v := s.viewers[session.ID]; v != nil {
				v.Close()
				delete(s.viewers, session.ID)
			}
			s.mu.Unlock()
		case "view":
			if session.State != "ready" {
				send(w, nil, protocol.Fail("NOT_READY", "Wait for the actual guest readiness handshake."))
				return
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			if v := s.viewers[session.ID]; v != nil {
				send(w, map[string]string{"url": v.URL}, nil)
				return
			}
			if len(s.viewers) >= 4 {
				send(w, nil, protocol.Fail("LIMIT_EXCEEDED", "Close/restart the console to release its four viewer slots."))
				return
			}
			c.ControlOwner = protocol.ID()
			v, e := viewer.NewWeb(viewer.NewHub(c, time.Second), session.ID)
			if e != nil {
				send(w, nil, e)
				return
			}
			v.EnableInput()
			s.viewers[session.ID] = v
			go func() { _ = v.Serve(s.ctx) }()
			send(w, map[string]string{"url": v.URL}, nil)
		default:
			http.NotFound(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		return protocol.Fail("INVALID_ARGUMENT", "Invalid or oversized console request")
	}
	return nil
}
