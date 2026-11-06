// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/process"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

//go:embed e2b_worker.py
var e2bWorker string

type E2B struct{ Python, Template, apiKey string }

func NewE2B(python, template string) (*E2B, error) {
	if !filepath.IsAbs(python) || !regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`).MatchString(template) || os.Getenv("E2B_API_KEY") == "" {
		return nil, errors.New("E2B requires an absolute reviewed venv Python, template ID/name and server-only E2B_API_KEY; none is fetched or configured automatically")
	}
	return &E2B{python, template, os.Getenv("E2B_API_KEY")}, nil
}
func (*E2B) Name() string    { return "e2b-desktop-2.7.2" }
func (*E2B) Simulated() bool { return false }

type workerRequest struct {
	Method     string    `json:"method"`
	Deployment string    `json:"deployment"`
	Session    Session   `json:"session"`
	Resource   Resource  `json:"resource"`
	Operation  Operation `json:"operation"`
	Template   string    `json:"template"`
}
type workerResponse struct {
	OK        bool       `json:"ok"`
	Code      string     `json:"code"`
	Resource  Resource   `json:"resource"`
	Resources []Resource `json:"resources"`
	Probe     Probe      `json:"probe"`
	Output    Output     `json:"output"`
}

func (p *E2B) run(ctx context.Context, q workerRequest) (workerResponse, error) {
	q.Template = p.Template
	b, e := json.Marshal(q)
	if e != nil {
		return workerResponse{}, e
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	cmd := process.Prepare(ctx, p.Python, "-I", "-c", e2bWorker)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8", "E2B_API_KEY=" + p.apiKey, "AGENT_LINUX_ENABLE_E2B=operator-enabled"}
	cmd.Stdin = bytes.NewReader(b)
	out := &process.Buffer{Limit: protocol.MaxFrame}
	errout := &process.Buffer{Limit: 2048}
	cmd.Stdout = out
	cmd.Stderr = errout
	if e = cmd.Run(); e != nil || out.Truncated {
		return workerResponse{}, protocol.Fail("PROVIDER_UNAVAILABLE", "E2B SDK worker failed/timed out; outcome may be unknown; inspect provider resources by deployment label")
	}
	var v workerResponse
	d := json.NewDecoder(bytes.NewReader(out.Bytes()))
	d.DisallowUnknownFields()
	if d.Decode(&v) != nil {
		return v, protocol.Fail("INVALID_MESSAGE", "malformed SDK worker response")
	}
	if !v.OK {
		allowed := map[string]bool{"UNSUPPORTED_OPERATION": true, "RESOURCE_LIMIT": true, "STALE_FRAME": true, "DESKTOP_UNAVAILABLE": true}
		if allowed[v.Code] {
			return v, protocol.Fail(v.Code, "E2B operation failed its validated contract")
		}
		return v, protocol.Fail("PROVIDER_UNAVAILABLE", "E2B API/SDK unavailable; no automatic mutation retry")
	}
	return v, nil
}
func (p *E2B) Create(ctx context.Context, d string, s Session) (Resource, error) {
	v, e := p.run(ctx, workerRequest{Method: "create", Deployment: d, Session: s})
	return v.Resource, e
}
func (p *E2B) List(ctx context.Context, d string) ([]Resource, error) {
	v, e := p.run(ctx, workerRequest{Method: "list", Deployment: d})
	return v.Resources, e
}
func (p *E2B) Probe(ctx context.Context, s Session) (Probe, error) {
	v, e := p.run(ctx, workerRequest{Method: "probe", Session: s})
	return v.Probe, e
}
func (p *E2B) Call(ctx context.Context, s Session, q Operation) (Output, error) {
	v, e := p.run(ctx, workerRequest{Method: "call", Session: s, Operation: q})
	return v.Output, e
}
func (p *E2B) Delete(ctx context.Context, r Resource) error {
	_, e := p.run(ctx, workerRequest{Method: "delete", Resource: r})
	return e
}
