// SPDX-License-Identifier: Apache-2.0
package viewer

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestInteractiveWebAuthorizationOwnershipAndValidation(t *testing.T) {
	c := &humanComputer{}
	w := &Web{host: "127.0.0.1:47777", token: "synthetic-test-capability", hub: NewHub(c, time.Second), gate: make(chan struct{}, 4)}
	request := func(body, origin, token string) int {
		r := httptest.NewRequest("POST", "http://"+w.host+"/api/input", strings.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("Authorization", token)
		out := httptest.NewRecorder()
		w.handle(out, r)
		return out.Code
	}
	auth := "Bearer " + w.token
	if code := request(`{"action":"acquire","generation":"test"}`, "", ""); code != 401 {
		t.Fatal(code)
	}
	if code := request(`{"action":"acquire","generation":"test"}`, "https://untrusted.test", auth); code != 403 {
		t.Fatal(code)
	}
	if code := request(`{"action":"acquire","generation":"test"}`, "", auth); code != 501 {
		t.Fatal("default view-only unexpectedly accepts input", code)
	}
	w.EnableInput()
	if code := request(`{"action":"input","input":{"frameId":"0123456789abcdef0123456789abcdef","kind":"key","key":"ENTER"}}`, "", auth); code != 409 {
		t.Fatal("input without lease", code)
	}
	if code := request(`{"action":"acquire","generation":"test"}`, "", auth); code != 200 {
		t.Fatal(code)
	}
	if code := request(`{"action":"input","input":{"frameId":"0123456789abcdef0123456789abcdef","kind":"click","x":1280,"y":0}}`, "", auth); code != 409 || c.inputs != 0 {
		t.Fatal("invalid coordinate dispatched", code, c.inputs)
	}
	if c.held {
		t.Fatal("failed input retained ownership")
	}
	if code := request(`{"action":"acquire","generation":"test"} {}`, "", auth); code != 400 {
		t.Fatal(code)
	}
	if code := request(`{"action":"acquire","generation":"test"}`, "", auth); code != 200 {
		t.Fatal(code)
	}
	if code := request(`{"action":"input","input":{"frameId":"0123456789abcdef0123456789abcdef","kind":"key","key":"ENTER"}}`, "", auth); code != 200 || c.inputs != 1 {
		t.Fatal(code, c.inputs)
	}
	if code := request(`{"action":"release"}`, "", auth); code != 200 || c.held {
		t.Fatal(code, c.held)
	}
}
