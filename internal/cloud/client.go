// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/machine"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

type ClientConfig struct {
	Version     string `json:"version"`
	URL         string `json:"url"`
	Token       string `json:"token"`
	Workspace   string `json:"workspace"`
	Session     string `json:"session,omitempty"`
	Development bool   `json:"development"`
}
type Client struct {
	Config       ClientConfig
	ControlOwner string
	HTTP         *http.Client
}

func LoadClient(path string) (*Client, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("cloud config must be an absolute private file")
	}
	for p := path; p != "/"; p = filepath.Dir(p) {
		st, e := os.Lstat(p)
		if e != nil || st.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("cloud config path must have no symbolic links")
		}
	}
	st, e := os.Stat(path)
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 || st.Size() > 8192 {
		return nil, errors.New("cloud config must be a bounded regular 0600 file")
	}
	if s, ok := st.Sys().(*syscall.Stat_t); !ok || int(s.Uid) != os.Getuid() || s.Nlink != 1 {
		return nil, errors.New("cloud config has unsafe owner/link count")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c ClientConfig
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e = d.Decode(&c); e != nil || d.Decode(new(any)) != io.EOF {
		return nil, errors.New("invalid cloud config")
	}
	return NewClient(c)
}
func NewClient(c ClientConfig) (*Client, error) {
	digest, tokenError := hex.DecodeString(c.Token)
	u, e := url.Parse(c.URL)
	if e != nil || tokenError != nil || len(digest) != 32 || c.Version != Version || len(c.Token) != 64 || !protocol.ValidID(c.Workspace) || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Hostname() == "" {
		return nil, errors.New("invalid cloud URL, version or credential configuration")
	}
	if u.Scheme != "https" && !(c.Development && u.Scheme == "http" && u.Hostname() == "127.0.0.1" && u.Port() != "") {
		return nil, errors.New("cloud traffic requires HTTPS; HTTP is restricted to explicit development on 127.0.0.1")
	}
	if c.Session != "" && !protocol.ValidID(c.Session) {
		return nil, errors.New("invalid cloud session ID")
	}
	transport := &http.Transport{DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, MaxIdleConnsPerHost: 2, ResponseHeaderTimeout: 70 * time.Second, IdleConnTimeout: 30 * time.Second}
	return &Client{Config: c, HTTP: &http.Client{Transport: transport, Timeout: 75 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("cloud redirects are disabled") }}}, nil
}
func (c *Client) request(ctx context.Context, method, path, key string, body, result any) error {
	var data []byte
	var e error
	if body != nil {
		data, e = json.Marshal(body)
		if e != nil {
			return e
		}
		if len(data) > protocol.MaxRequest {
			return protocol.Fail("INVALID_MESSAGE", "request exceeds cloud body limit")
		}
	}
	r, e := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.Config.URL, "/")+path, bytes.NewReader(data))
	if e != nil {
		return e
	}
	r.Header.Set("Authorization", "Bearer "+c.Config.Token)
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	response, e := c.HTTP.Do(r)
	if e != nil {
		return protocol.Fail("DISCONNECTED", "cloud transport unavailable; mutation outcome may be unknown")
	}
	defer response.Body.Close()
	if response.Header.Get("X-Agent-Linux-Cloud-Version") != Version {
		return protocol.Fail("VERSION_MISMATCH", "cloud control-plane version is incompatible")
	}
	b, e := io.ReadAll(io.LimitReader(response.Body, protocol.MaxFrame+1))
	if e != nil || len(b) > protocol.MaxFrame {
		return protocol.Fail("INVALID_MESSAGE", "cloud response unavailable or oversized")
	}
	var v envelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&v) != nil || d.Decode(new(any)) != io.EOF {
		return protocol.Fail("INVALID_MESSAGE", "invalid cloud response")
	}
	if !v.OK {
		if v.Error == nil {
			return protocol.Fail("INVALID_MESSAGE", "cloud error lacks details")
		}
		return v.Error
	}
	if response.StatusCode != 200 {
		return protocol.Fail("INVALID_MESSAGE", "cloud success has incompatible HTTP status")
	}
	if result != nil && json.Unmarshal(v.Data, result) != nil {
		return protocol.Fail("INVALID_MESSAGE", "invalid cloud result")
	}
	return nil
}
func (c *Client) Create(ctx context.Context, key string, seconds int) (Session, error) {
	var s Session
	e := c.request(ctx, "POST", "/v1/sessions", key, Create{c.Config.Workspace, seconds, "desktop"}, &s)
	return s, e
}
func (c *Client) Session(ctx context.Context) (Session, error) {
	var s Session
	if !protocol.ValidID(c.Config.Session) {
		return s, protocol.Fail("NOT_READY", "select a cloud session ID in the private client config")
	}
	e := c.request(ctx, "GET", "/v1/sessions/"+c.Config.Session, "", nil, &s)
	return s, e
}
func (c *Client) Stop(ctx context.Context) (Session, error) {
	var s Session
	if !protocol.ValidID(c.Config.Session) {
		return s, protocol.Fail("NOT_READY", "select a cloud session")
	}
	e := c.request(ctx, "POST", "/v1/sessions/"+c.Config.Session+"/stop", "", nil, &s)
	return s, e
}
func (c *Client) Revoke(ctx context.Context) error {
	return c.request(ctx, "POST", "/v1/revoke", "", nil, nil)
}
func (c *Client) call(ctx context.Context, q Operation) (Output, error) {
	q.ControlNonce = c.ControlOwner
	var o Output
	if !protocol.ValidID(c.Config.Session) {
		return o, protocol.Fail("NOT_READY", "select a cloud session")
	}
	e := c.request(ctx, "POST", "/v1/sessions/"+c.Config.Session+"/call", "", q, &o)
	return o, e
}
func (c *Client) Status(ctx context.Context) (lifecycle.State, error) {
	s, e := c.Session(ctx)
	if e != nil {
		return lifecycle.State{}, e
	}
	observed := "remote"
	if s.Simulated {
		observed = "SIMULATED DEVELOPMENT; NO LINUX VM"
	}
	return lifecycle.State{Backend: "cloud", Remote: &lifecycle.RemoteState{Provider: s.Provider, ExpiresAt: s.Expires, IdleStopAt: s.IdleStopAt, EstimatedRuntimeSeconds: s.EstimatedSeconds, Persistent: false, Simulated: s.Simulated}, Schema: 1, MachineID: s.ID, State: s.State, Observed: observed, Stage: s.Provider, BackendVersion: Version, LastError: s.Error, UpdatedAt: time.Now().UTC(), Guest: &protocol.Info{GuestVersion: s.Provider, Protocol: Version, MachineID: s.ID, SessionID: s.ID, OS: "provider-dependent", Arch: "provider-dependent", Capabilities: s.Capabilities, Desktop: &protocol.DesktopStatus{Ready: s.State == "ready", Generation: s.Generation, Server: observed, Width: 1280, Height: 800}}}, nil
}
func (c *Client) Screenshot(ctx context.Context) (*protocol.Frame, error) {
	o, e := c.call(ctx, Operation{Method: "screenshot"})
	if e != nil {
		return nil, e
	}
	if o.Frame == nil {
		return nil, protocol.Fail("INVALID_FRAME", "cloud returned no frame")
	}
	if e = o.Frame.Validate(); e != nil {
		return nil, e
	}
	if o.Frame.SessionID != c.Config.Session {
		return nil, protocol.Fail("INVALID_FRAME", "cloud frame belongs to another session")
	}
	return o.Frame, nil
}
func (c *Client) Input(ctx context.Context, i protocol.Input) (*protocol.InputResult, error) {
	if i.FrameID == "" {
		f, e := c.Screenshot(ctx)
		if e != nil {
			return nil, e
		}
		i.FrameID = f.ID
	}
	if e := i.Validate(); e != nil {
		return nil, e
	}
	o, e := c.call(ctx, Operation{Method: "input", Input: &i})
	if e != nil {
		return nil, e
	}
	if o.Input == nil || !o.Input.Dispatched || o.Input.FrameID != i.FrameID {
		return nil, protocol.Fail("INVALID_MESSAGE", "invalid cloud input acknowledgement")
	}
	return o.Input, nil
}
func (c *Client) Exec(ctx context.Context, q protocol.ExecRequest) (*protocol.Execution, error) {
	if e := q.Validate(); e != nil {
		return nil, e
	}
	o, e := c.call(ctx, Operation{Method: "exec", Exec: &q})
	if e != nil {
		return nil, e
	}
	if o.Execution == nil {
		return nil, protocol.Fail("INVALID_MESSAGE", "no cloud command result")
	}
	return o.Execution, nil
}
func (c *Client) Open(ctx context.Context, app string) error {
	_, e := c.call(ctx, Operation{Method: "open", App: app})
	return e
}

var _ machine.Machine = (*Client)(nil)

// PrivateOutput prevents credential writes through symbolic parents or into a
// local VM namespace. O_EXCL at the caller also refuses any existing destination.
func PrivateOutput(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("cloud output must be an absolute clean path")
	}
	parent := filepath.Dir(path)
	for p := parent; p != "/"; p = filepath.Dir(p) {
		st, e := os.Lstat(p)
		if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return errors.New("cloud output parents must be real directories")
		}
	}
	st, e := os.Stat(parent)
	if e != nil || st.Mode().Perm()&0077 != 0 {
		return errors.New("cloud credentials require a private 0700 parent directory")
	}
	if owner, ok := st.Sys().(*syscall.Stat_t); !ok || int(owner.Uid) != os.Getuid() {
		return errors.New("cloud output parent belongs to another user")
	}
	if _, e = os.Lstat(filepath.Join(parent, "state.json")); !os.IsNotExist(e) {
		return errors.New("do not place cloud credentials in a local VM namespace")
	}
	return nil
}
