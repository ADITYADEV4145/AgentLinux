// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"context"
	"errors"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type faultProvider struct {
	*Fake
	creates                 int
	lost, offline, oversize bool
}

func (p *faultProvider) Create(ctx context.Context, d string, s Session) (Resource, error) {
	p.creates++
	r, e := p.Fake.Create(ctx, d, s)
	if p.lost {
		return Resource{}, errors.New("ambiguous transport failure")
	}
	return r, e
}
func (p *faultProvider) List(ctx context.Context, d string) ([]Resource, error) {
	if p.offline {
		return nil, errors.New("provider outage")
	}
	return p.Fake.List(ctx, d)
}
func (p *faultProvider) Probe(ctx context.Context, s Session) (Probe, error) {
	if p.oversize {
		return Probe{}, protocol.Fail("RESOURCE_LIMIT", "oversized provider machine")
	}
	return p.Fake.Probe(ctx, s)
}
func (p *faultProvider) Call(ctx context.Context, s Session, q Operation) (Output, error) {
	if p.offline {
		<-ctx.Done()
		return Output{}, ctx.Err()
	}
	return p.Fake.Call(ctx, s, q)
}
func TestAmbiguousCreateOutageAndResourceViolation(t *testing.T) {
	m, p, a, _, root := fixture(t)
	fault := &faultProvider{Fake: p, lost: true}
	m.provider = fault
	s, e := m.Create(a, protocol.ID(), Create{a.Workspace, 60, "desktop"})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Tick(context.Background()); e != nil {
		t.Fatal(e)
	}
	fault.offline = true
	if e = m.Tick(context.Background()); e == nil {
		t.Fatal("outage hidden")
	}
	if fault.creates != 1 {
		t.Fatal("ambiguous create retried")
	}
	fault.offline = false
	if e = m.Tick(context.Background()); e != nil {
		t.Fatal(e)
	}
	got, _ := m.Get(a, s.ID)
	if got.State != "ready" || fault.creates != 1 {
		t.Fatal(got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	fault.offline = true
	if _, e = m.Call(ctx, a, s.ID, Operation{Method: "screenshot"}); e == nil {
		t.Fatal("timeout hidden")
	}
	fault.offline = false
	fault.oversize = true
	if e = m.Tick(context.Background()); e != nil {
		t.Fatal(e)
	}
	got, _ = m.Get(a, s.ID)
	if got.State != "stopping" {
		t.Fatal("oversized resource not queued for cleanup", got.State)
	}
	if e = m.Tick(context.Background()); e != nil {
		t.Fatal(e)
	}
	got, _ = m.Get(a, s.ID)
	if got.State != "stopped" {
		t.Fatal(got)
	}
	// Audit must never contain provider exception text or input payloads.
	log, e := os.ReadFile(filepath.Join(root, "cloud-events.jsonl"))
	if e != nil {
		t.Fatal(e)
	}
	if len(log) == 0 {
		t.Fatal("no audit trail")
	}
}

func TestCredentialOutputAndCorruptNamespaceBoundaries(t *testing.T) {
	_, p, _, _, root := fixture(t)
	if e := PrivateOutput(filepath.Join(root, "new-session.json")); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(root, filepath.Join(root, "linked")); e != nil {
		t.Fatal(e)
	}
	if e := PrivateOutput(filepath.Join(root, "linked", "secret.json")); e == nil {
		t.Fatal("symlink parent accepted")
	}
	if e := os.WriteFile(filepath.Join(root, "state.json"), []byte("{}"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := PrivateOutput(filepath.Join(root, "new-session.json")); e == nil {
		t.Fatal("local namespace accepted for credential output")
	}
	var db Database
	if e := storage.Read(root, "cloud.json", &db); e != nil {
		t.Fatal(e)
	}
	for id, i := range db.Identities {
		i.TokenHash = "bad"
		db.Identities[id] = i
		break
	}
	if e := storage.JSON(root, "cloud.json", db); e != nil {
		t.Fatal(e)
	}
	if _, e := New(root, p); e == nil {
		t.Fatal("corrupt stored identity accepted")
	}
}
func TestStrictRequestBodyLimitIncludingTrailingWhitespace(t *testing.T) {
	r := httptest.NewRequest("POST", "http://127.0.0.1/v1/sessions", strings.NewReader(`{}`+strings.Repeat(" ", protocol.MaxRequest)))
	var q Create
	code(t, decode(r, &q), "INVALID_MESSAGE")
}
