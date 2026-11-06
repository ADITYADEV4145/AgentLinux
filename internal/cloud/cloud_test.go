// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

func fixture(t *testing.T) (*Manager, *Fake, Identity, Identity, string) {
	t.Helper()
	path, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(path, 0700); e != nil {
		t.Fatal(e)
	}
	root, e := storage.Root(path)
	if e != nil {
		t.Fatal(e)
	}
	files, e := Init(root, "http://127.0.0.1:8787", "fake-development")
	if e != nil {
		t.Fatal(e)
	}
	a, e := LoadClient(files["alice"])
	if e != nil {
		t.Fatal(e)
	}
	b, e := LoadClient(files["bob"])
	if e != nil {
		t.Fatal(e)
	}
	p := NewFake()
	m, e := New(root, p)
	if e != nil {
		t.Fatal(e)
	}
	alice, e := m.Authenticate(a.Config.Token)
	if e != nil {
		t.Fatal(e)
	}
	bob, e := m.Authenticate(b.Config.Token)
	if e != nil {
		t.Fatal(e)
	}
	return m, p, alice, bob, root
}
func ready(t *testing.T, m *Manager, a Identity) Session {
	t.Helper()
	s, e := m.Create(a, protocol.ID(), Create{a.Workspace, 60, "desktop"})
	if e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 2; n++ {
		if e = m.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
	s, e = m.Get(a, s.ID)
	if e != nil || s.State != "ready" {
		t.Fatal(s, e)
	}
	return s
}
func code(t *testing.T, e error, want string) {
	t.Helper()
	if e == nil || !strings.Contains(e.Error(), want) {
		t.Fatalf("got %v; want %s", e, want)
	}
}
func TestLifecycleIsolationIdempotencyAndBounds(t *testing.T) {
	m, p, a, b, _ := fixture(t)
	ctx := context.Background()
	key := protocol.ID()
	q := Create{a.Workspace, 60, "desktop"}
	s, e := m.Create(a, key, q)
	if e != nil {
		t.Fatal(e)
	}
	again, e := m.Create(a, key, q)
	if e != nil || again.ID != s.ID {
		t.Fatal(again, e)
	}
	q.Seconds = 61
	_, e = m.Create(a, key, q)
	code(t, e, "IDEMPOTENCY_CONFLICT")
	_, e = m.Create(b, protocol.ID(), Create{a.Workspace, 60, "desktop"})
	code(t, e, "FORBIDDEN")
	for _, method := range []string{"screenshot", "input", "exec", "open", "files", "stream"} {
		_, e = m.Call(ctx, b, s.ID, Operation{Method: method})
		code(t, e, "NOT_FOUND")
	}
	_, e = m.Stop(b, s.ID)
	code(t, e, "NOT_FOUND")
	_, e = m.Get(b, s.ID)
	code(t, e, "NOT_FOUND")
	if e = m.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	if e = m.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	out, e := m.Call(ctx, a, s.ID, Operation{Method: "screenshot"})
	if e != nil || out.Frame == nil || out.Frame.Validate() != nil {
		t.Fatal(out, e)
	}
	in := protocol.Input{Kind: "click", FrameID: out.Frame.ID, X: 0, Y: 0, Button: "left", Clicks: 1}
	out, e = m.Call(ctx, a, s.ID, Operation{Method: "input", Input: &in})
	if e != nil || !out.Input.Dispatched {
		t.Fatal(out, e)
	}
	in.X = 1280
	_, e = m.Call(ctx, a, s.ID, Operation{Method: "input", Input: &in})
	code(t, e, "INVALID_ARGUMENT")
	_, e = m.Call(ctx, a, s.ID, Operation{Method: "exec", Exec: &protocol.ExecRequest{}})
	code(t, e, "UNSUPPORTED_OPERATION")
	_, e = m.Create(a, protocol.ID(), Create{a.Workspace, 60, "desktop"})
	code(t, e, "LIMIT_EXCEEDED")
	_, e = m.Stop(a, s.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	s, e = m.Get(a, s.ID)
	if e != nil || s.State != "stopped" {
		t.Fatal(s, e)
	}
	rs, _ := p.List(ctx, "")
	if len(rs) != 0 {
		t.Fatal(rs)
	}
	_, e = m.Call(ctx, a, s.ID, Operation{Method: "screenshot"})
	code(t, e, "NOT_READY")
}
func TestCloudLeasesRevocationExpiryAndNoViewerKeepalive(t *testing.T) {
	m, _, a, _, _ := fixture(t)
	ctx := context.Background()
	s := ready(t, m, a)
	owner := protocol.ID()
	if e := m.Control(a, s.ID, LeaseRequest{"acquire", owner, s.Generation}); e != nil {
		t.Fatal(e)
	}
	out, e := m.Call(ctx, a, s.ID, Operation{Method: "screenshot"})
	if e != nil {
		t.Fatal(e)
	}
	in := protocol.Input{Kind: "text", FrameID: out.Frame.ID, Text: "café 日本語 👋"}
	_, e = m.Call(ctx, a, s.ID, Operation{Method: "input", Input: &in})
	code(t, e, "INPUT_OWNED")
	_, e = m.Call(ctx, a, s.ID, Operation{Method: "input", Input: &in, ControlNonce: owner})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Control(a, s.ID, LeaseRequest{"release", owner, s.Generation}); e != nil {
		t.Fatal(e)
	}
	_, e = m.Call(ctx, a, s.ID, Operation{Method: "input", Input: &in, ControlNonce: owner})
	code(t, e, "INPUT_NOT_OWNED")
	if e = m.Revoke(a); e != nil {
		t.Fatal(e)
	}
	_, e = m.Get(a, s.ID)
	code(t, e, "UNAUTHORIZED")
	if e = m.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	m, _, a, _, _ = fixture(t)
	s = ready(t, m, a)
	future := s.Expires.Add(time.Second)
	m.now = func() time.Time { return future }
	_, e = m.Call(ctx, a, s.ID, Operation{Method: "screenshot"})
	code(t, e, "NOT_READY")
	if e = m.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	s, _ = m.Get(a, s.ID)
	if s.State != "stopped" {
		t.Fatal(s)
	}
	m, _, a, _, _ = fixture(t)
	m.db.Limits.IdleSeconds = 10
	s = ready(t, m, a)
	m.now = func() time.Time { return s.LastAction.Add(11 * time.Second) }
	_, e = m.Call(ctx, a, s.ID, Operation{Method: "screenshot"})
	code(t, e, "NOT_READY")
}
func TestDurabilityBudgetOrphansAndInterruptedAllocation(t *testing.T) {
	m, p, a, _, root := fixture(t)
	ctx := context.Background()
	s := ready(t, m, a)
	recovered, e := New(root, p)
	if e != nil {
		t.Fatal(e)
	}
	again, e := recovered.Get(a, s.ID)
	if e != nil || again.ProviderID != s.ProviderID {
		t.Fatal(again, e)
	}
	// Unknown, deployment-owned resources are deleted; foreign deployments survive.
	orphan := Session{ID: protocol.ID()}
	_, _ = p.Create(ctx, m.db.Deployment, orphan)
	foreign, _ := p.Create(ctx, protocol.ID(), Session{ID: protocol.ID()})
	if e = m.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	rs, _ := p.List(ctx, "")
	if len(rs) != 2 {
		t.Fatal(rs)
	}
	found := false
	for _, r := range rs {
		found = found || r.ID == foreign.ID
	}
	if !found {
		t.Fatal("foreign resource removed")
	}
	// Lost create reply: persist attempted intent, then discover labelled resource.
	m2, p2, a2, _, _ := fixture(t)
	pending, e := m2.Create(a2, protocol.ID(), Create{a2.Workspace, 60, "desktop"})
	if e != nil {
		t.Fatal(e)
	}
	pending.Attempted = true
	m2.db.Sessions[pending.ID] = pending
	_ = m2.save()
	r, _ := p2.Create(ctx, m2.db.Deployment, pending)
	if e = m2.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	got, _ := m2.Get(a2, pending.ID)
	if got.ProviderID != r.ID || got.State != "ready" {
		t.Fatal(got)
	}
	m2.db.Limits.BudgetSeconds = 60
	_, _ = m2.Stop(a2, pending.ID)
	future := pending.Created.Add(65 * time.Second)
	m2.now = func() time.Time { return future }
	if e = m2.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	_, e = m2.Create(a2, protocol.ID(), Create{a2.Workspace, 30, "desktop"})
	code(t, e, "LIMIT_EXCEEDED")
}
func TestHTTPMachineClientAndEveryCrossUserRoute(t *testing.T) {
	m, _, a, _, root := fixture(t)
	s := ready(t, m, a)
	server := httptest.NewServer(NewAPI(m))
	defer server.Close()
	load := func(name string) *Client {
		c, e := LoadClient(filepath.Join(root, name+".json"))
		if e != nil {
			t.Fatal(e)
		}
		c.Config.URL = server.URL
		c.Config.Session = s.ID
		return c
	}
	c := load("alice")
	b := load("bob")
	ctx := context.Background()
	status, e := c.Status(ctx)
	if e != nil || status.Observed != "SIMULATED DEVELOPMENT; NO LINUX VM" {
		t.Fatal(status, e)
	}
	f, e := c.Screenshot(ctx)
	if e != nil || f.Validate() != nil {
		t.Fatal(e)
	}
	_, e = c.Input(ctx, protocol.Input{Kind: "click", FrameID: f.ID, X: 0, Y: 0, Clicks: 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{"", "/stop", "/call", "/control", "/files", "/stream", "/frame"} {
		method := "POST"
		if path == "" {
			method = "GET"
		}
		e = b.request(ctx, method, "/v1/sessions/"+s.ID+path, "", map[string]string{}, new(any))
		code(t, e, "NOT_FOUND")
	}
	req, _ := http.NewRequest("GET", server.URL+"/v1/sessions/"+s.ID, nil)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 401 {
		t.Fatal(resp.StatusCode)
	}
	resp.Body.Close()
	req, _ = http.NewRequest("GET", server.URL+"/v1/sessions/"+s.ID, nil)
	req.Header.Set("Authorization", "Bearer "+c.Config.Token)
	req.Header.Set("Origin", "https://untrusted.invalid")
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 403 {
		t.Fatal(resp.StatusCode)
	}
	resp.Body.Close()
	// Revocation rejects future requests even with the same session config.
	if e = c.Revoke(ctx); e != nil {
		t.Fatal(e)
	}
	_, e = c.Screenshot(ctx)
	code(t, e, "UNAUTHORIZED")
}
func TestClientTransportAndPrivateConfig(t *testing.T) {
	_, _, _, _, root := fixture(t)
	c, e := LoadClient(filepath.Join(root, "alice.json"))
	if e != nil {
		t.Fatal(e)
	}
	for _, u := range []string{"http://example.com:99", "http://localhost:99", "https://user:pass@example.com", "https://example.com/?token=secret"} {
		cfg := c.Config
		cfg.URL = u
		if _, e = NewClient(cfg); e == nil {
			t.Fatal("accepted", u)
		}
	}
	path := filepath.Join(root, "open.json")
	b, _ := json.Marshal(c.Config)
	_ = os.WriteFile(path, b, 0644)
	if _, e = LoadClient(path); e == nil {
		t.Fatal("accepted public credential file")
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "https://example.com/", 302) }))
	defer redirect.Close()
	c.Config.URL = redirect.URL
	c.Config.Session = protocol.ID()
	_, e = c.Status(context.Background())
	if e == nil {
		t.Fatal("missing session was accepted")
	}
}
func TestConcurrentDuplicateCreation(t *testing.T) {
	m, _, a, _, _ := fixture(t)
	key := protocol.ID()
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	for n := 0; n < 8; n++ {
		wg.Go(func() {
			s, e := m.Create(a, key, Create{a.Workspace, 60, "desktop"})
			if e != nil {
				t.Error(e)
			}
			ids <- s.ID
		})
	}
	wg.Wait()
	close(ids)
	id := ""
	for next := range ids {
		if id == "" {
			id = next
		}
		if next != id {
			t.Fatal("duplicate allocation intent")
		}
	}
}
