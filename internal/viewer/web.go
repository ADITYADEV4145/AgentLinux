// SPDX-License-Identifier: Apache-2.0
package viewer

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

//go:embed web.html
var page string

//go:embed web.js
var script string

//go:embed web.css
var style string

type Web struct {
	server          *http.Server
	listener        net.Listener
	URL             string
	host, token, id string
	hub             *Hub
	gate            chan struct{}
	human           *Human
	inputMu         sync.Mutex
}

func NewWeb(h *Hub, id string) (*Web, error) {
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return nil, e
	}
	var secret [32]byte
	if _, e = rand.Read(secret[:]); e != nil {
		l.Close()
		return nil, e
	}
	w := &Web{listener: l, host: l.Addr().String(), token: base64.RawURLEncoding.EncodeToString(secret[:]), id: id, hub: h, gate: make(chan struct{}, 4)}
	w.URL = "http://" + w.host + "/#" + w.token
	var mu sync.Mutex
	connections := map[net.Conn]bool{}
	w.server = &http.Server{Handler: http.HandlerFunc(w.handle), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 12 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 4096, ConnState: func(c net.Conn, s http.ConnState) {
		mu.Lock()
		defer mu.Unlock()
		switch s {
		case http.StateNew:
			if len(connections) >= 8 {
				c.Close()
			} else {
				connections[c] = true
			}
		case http.StateClosed, http.StateHijacked:
			delete(connections, c)
		}
	}}
	return w, nil
}
func (w *Web) Serve(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			c, cancel := context.WithTimeout(context.Background(), time.Second)
			_ = w.server.Shutdown(c)
			cancel()
			_ = w.server.Close()
		case <-done:
		}
	}()
	e := w.server.Serve(w.listener)
	close(done)
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}
func (w *Web) Close() {
	_ = w.server.Close()
	_ = w.listener.Close()
	w.inputMu.Lock()
	defer w.inputMu.Unlock()
	if w.human != nil {
		w.human.Release()
	}
}
func (w *Web) handle(out http.ResponseWriter, r *http.Request) {
	out.Header().Set("Cache-Control", "no-store")
	out.Header().Set("X-Content-Type-Options", "nosniff")
	out.Header().Set("Referrer-Policy", "no-referrer")
	out.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src blob:; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	if r.Host != w.host || r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+w.host {
		http.Error(out, "forbidden origin/host", 403)
		return
	}
	if !(r.Method == "POST" && r.URL.Path == "/api/input") && (r.Method != "GET" || r.ContentLength != 0) {
		http.Error(out, "GET only", 405)
		return
	}
	switch r.URL.Path {
	case "/":
		out.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(out, page)
		return
	case "/app.js":
		out.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		fmt.Fprint(out, script)
		return
	case "/app.css":
		out.Header().Set("Content-Type", "text/css; charset=utf-8")
		fmt.Fprint(out, style)
		return
	}
	got := r.Header.Get("Authorization")
	want := "Bearer " + w.token
	if len(got) != len(want) || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		http.Error(out, "viewer authorization required", 401)
		return
	}
	select {
	case w.gate <- struct{}{}:
		defer func() { <-w.gate }()
	default:
		http.Error(out, "viewer busy", 429)
		return
	}
	switch r.URL.Path {
	case "/api/input":
		if r.Method != "POST" {
			http.Error(out, "POST only", 405)
			return
		}
		w.input(out, r)
	case "/api/config":
		out.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(out).Encode(map[string]bool{"interactive": w.human != nil})
	case "/api/metrics":
		out.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(out).Encode(w.hub.Metrics())
	case "/api/frame":
		s, e := w.hub.Next(r.Context())
		if e != nil {
			out.Header().Set("Content-Type", "application/json")
			out.WriteHeader(503)
			_ = json.NewEncoder(out).Encode(map[string]string{"code": Code(e), "message": "guest capture unavailable; viewer reconnects automatically; VM is not started or stopped by the viewer"})
			return
		}
		out.Header().Set("ETag", `"`+s.Digest+`"`)
		out.Header().Set("X-Machine-ID", w.id)
		out.Header().Set("X-Frame-ID", s.Frame.ID)
		out.Header().Set("X-Desktop-Generation", s.Frame.Generation)
		out.Header().Set("X-Capture-Time", s.Observed.UTC().Format(time.RFC3339Nano))
		out.Header().Set("X-Image-Width", strconv.Itoa(s.Frame.Width))
		out.Header().Set("X-Image-Height", strconv.Itoa(s.Frame.Height))
		if r.Header.Get("If-None-Match") == `"`+s.Digest+`"` {
			out.WriteHeader(304)
			return
		}
		out.Header().Set("Content-Type", "image/png")
		out.Header().Set("Content-Length", strconv.Itoa(len(s.Frame.PNG)))
		n, _ := out.Write(s.Frame.PNG)
		w.hub.Sent(n)
	default:
		http.NotFound(out, r)
	}
}
