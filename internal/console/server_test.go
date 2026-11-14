// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/cloud"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

func request(t *testing.T, s *Server, method, path, body string, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "http://"+s.host+path, strings.NewReader(body))
	r.Host = s.host
	if auth {
		r.Header.Set("Authorization", "Bearer "+s.token)
	}
	w := httptest.NewRecorder()
	s.handle(w, r)
	return w
}
func TestConsoleBoundaryAndEmptyState(t *testing.T) {
	s, e := New(nil)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	for _, p := range []string{"/", "/dashboard", "/app.css", "/app.js", "/pricing.json"} {
		w := request(t, s, "GET", p, "", false)
		if w.Code != 200 {
			t.Fatalf("%s: %d", p, w.Code)
		}
		if strings.Contains(w.Body.String(), s.token) {
			t.Fatal("static asset contains capability")
		}
	}
	if request(t, s, "GET", "/api/config", "", false).Code != 401 {
		t.Fatal("missing auth accepted")
	}
	if request(t, s, "GET", "/api/sessions", "", true).Code != 409 {
		t.Fatal("invented cloud connection")
	}
	for _, header := range []string{"Origin", "Host"} {
		r := httptest.NewRequest("GET", "http://"+s.host+"/api/config", nil)
		r.Host = s.host
		r.Header.Set("Authorization", "Bearer "+s.token)
		if header == "Host" {
			r.Host = "attacker.invalid"
		} else {
			r.Header.Set(header, "https://attacker.invalid")
		}
		w := httptest.NewRecorder()
		s.handle(w, r)
		if w.Code != 403 {
			t.Fatal("host/origin bypass", header)
		}
	}
	if request(t, s, "GET", "/api/config?token=secret", "", true).Code != 403 {
		t.Fatal("URL credentials accepted")
	}
}
func TestConsoleFleetCreateStopViewerAndCredentialSeparation(t *testing.T) {
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	_ = os.Chmod(root, 0700)
	root, e = storage.Root(root)
	if e != nil {
		t.Fatal(e)
	}
	limits := cloud.DefaultLimits()
	limits.MaxPerOwner = 2
	files, e := cloud.InitWithLimits(root, "http://127.0.0.1:8787", "fake-development", limits)
	if e != nil {
		t.Fatal(e)
	}
	c, e := cloud.LoadClient(files["alice"])
	if e != nil {
		t.Fatal(e)
	}
	m, e := cloud.New(root, cloud.NewFake())
	if e != nil {
		t.Fatal(e)
	}
	upstream := httptest.NewServer(cloud.NewAPI(m))
	defer upstream.Close()
	c.Config.URL = upstream.URL
	s, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	var ids []string
	for n := 0; n < 2; n++ {
		body := `{"requestId":"` + protocol.ID() + `","seconds":60}`
		w := request(t, s, "POST", "/api/sessions", body, true)
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		var session cloud.Session
		if json.Unmarshal(w.Body.Bytes(), &session) != nil {
			t.Fatal("session response")
		}
		ids = append(ids, session.ID)
	}
	if request(t, s, "POST", "/api/sessions", `{"requestId":"`+protocol.ID()+`","seconds":60}`, true).Code == 200 {
		t.Fatal("concurrency cap bypass")
	}
	for n := 0; n < 2; n++ {
		if e = m.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
	w := request(t, s, "GET", "/api/sessions", "", true)
	var fleet cloud.Fleet
	if json.Unmarshal(w.Body.Bytes(), &fleet) != nil || len(fleet.Sessions) != 2 {
		t.Fatal(w.Body.String())
	}
	if strings.Contains(w.Body.String(), c.Config.Token) || strings.Contains(w.Body.String(), s.token) {
		t.Fatal("credential leak")
	}
	w = request(t, s, "POST", "/api/sessions/"+protocol.ID()+"/view", `{}`, true)
	if w.Code == 200 || len(s.viewers) != 0 {
		t.Fatal("foreign viewer allocated")
	}
	w = request(t, s, "POST", "/api/sessions/"+ids[0]+"/view", `{}`, true)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var link map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &link)
	viewerURL, capability, ok := strings.Cut(link["url"], "#")
	if !ok || capability == s.token || strings.Contains(link["url"], c.Config.Token) {
		t.Fatal("viewer credential isolation")
	}
	client := &http.Client{Timeout: 3 * time.Second}
	r, _ := http.NewRequest("GET", viewerURL+"api/frame", nil)
	r.Header.Set("Authorization", "Bearer "+capability)
	response, e := client.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode != 200 || response.Header.Get("Content-Type") != "image/png" {
		t.Fatal("real simulated-provider capture route", response.StatusCode)
	}
	if request(t, s, "POST", "/api/sessions/"+ids[0]+"/stop", `{}`, true).Code == 200 {
		t.Fatal("unconfirmed destructive operation")
	}
	if request(t, s, "POST", "/api/sessions/"+ids[0]+"/stop", `{"confirm":true}`, true).Code != 200 {
		t.Fatal("confirmed stop")
	}
	if len(s.viewers) != 0 {
		t.Fatal("stopped viewer retained")
	}
	if request(t, s, "POST", "/api/sessions", `{"requestId":"x","seconds":60,"url":"http://attacker"}`, true).Code == 200 {
		t.Fatal("unknown request fields")
	}
}

func TestExportedLandingRoutesLocalAssetsAndSeparateConsolePolicy(t *testing.T) {
	s, e := New(nil)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	for _, route := range []string{"/", "/pricing"} {
		w := request(t, s, "GET", route, "", false)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Give") || strings.Contains(w.Body.String(), "events.framer.com/script.js") {
			t.Fatal("export route/analytics", route, w.Code)
		}
		if !strings.Contains(w.Body.String(), "/landing-bridge.js") || strings.Contains(w.Body.String(), "Cloud access is available through the service.") {
			t.Fatal("unconnected export or false cloud claim")
		}
		if !strings.Contains(w.Header().Get("Content-Security-Policy"), "font-src 'self'") {
			t.Fatal("missing local asset policy")
		}
	}
	w := request(t, s, "GET", "/dashboard", "", false)
	if strings.Contains(w.Header().Get("Content-Security-Policy"), "unsafe-inline") {
		t.Fatal("landing CSP weakened authenticated console")
	}
	for _, route := range []string{"/landing-bridge.js", "/landing-style.css", "/fonts/Manrope-variable.ttf", "/framerusercontent.com/images/eom8bDQh6gZMIkmm0Gyn3fClog_1.gif"} {
		if request(t, s, "GET", route, "", false).Code != 200 {
			t.Fatal("missing exported asset", route)
		}
	}
	for _, route := range []string{"/framerusercontent.com/../server.go", "/framerusercontent.com/missing.mjs", "/events.framer.com/script.js"} {
		if request(t, s, "GET", route, "", false).Code == 200 {
			t.Fatal("unexpected file exposure", route)
		}
	}
}
