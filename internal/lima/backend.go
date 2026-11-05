// SPDX-License-Identifier: Apache-2.0

package lima

import (
	"context"
	"crypto/sha512"
	"debug/elf"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/process"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Backend struct {
	Root, Executable string
	Progress         func(string)
}

func (b *Backend) home() string { return filepath.Join(b.Root, "lima") }
func (b *Backend) env() []string {
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + b.Root, "LIMA_HOME=" + b.home(), "TMPDIR=" + b.Root + "/tmp", "LANG=C", "LC_ALL=C"}
	return env
}
func (b *Backend) run(ctx context.Context, args ...string) (process.Result, error) {
	r, e := process.Run(ctx, b.env(), b.Executable, args...)
	if len(r.Err) > 0 {
		if logErr := b.diagnostic(args[0], r.Err); logErr != nil {
			return r, logErr
		}
	}
	if e != nil {
		return r, fmt.Errorf("Lima %s failed (%w); inspect agent-linux logs --backend", args[0], e)
	}
	if r.Truncated {
		return r, errors.New("Lima output exceeded safety limit")
	}
	return r, nil
}
func (b *Backend) Observe(ctx context.Context) (lifecycle.Observation, error) {
	if _, e := os.Stat(b.home()); os.IsNotExist(e) {
		return lifecycle.Observation{}, nil
	}
	if e := b.noOverrides(); e != nil {
		return lifecycle.Observation{}, e
	}
	r, e := b.run(ctx, "list", "--format", `{{printf "{\"name\":%q,\"status\":%q,\"dir\":%q,\"sshLocalPort\":%d,\"vmType\":%q}" .Name .Status .Dir .SSHLocalPort .VMType}}`)
	if e != nil {
		return lifecycle.Observation{}, e
	}
	d := json.NewDecoder(strings.NewReader(string(r.Out)))
	var found lifecycle.Observation
	for {
		var v struct {
			Name         string `json:"name"`
			Status       string `json:"status"`
			Dir          string `json:"dir"`
			SSHLocalPort int    `json:"sshLocalPort"`
			VMType       string `json:"vmType"`
		}
		e = d.Decode(&v)
		if e == io.EOF {
			break
		}
		if e != nil {
			return found, errors.New("malformed Lima JSON inventory")
		}
		if v.Name != "default" {
			return found, errors.New("unexpected machine in private Lima namespace")
		}
		if found.Exists {
			return found, errors.New("duplicate runtime machine")
		}
		if v.Dir != filepath.Join(b.home(), "default") {
			return found, errors.New("runtime returned a path outside owned machine")
		}
		if v.VMType != "vz" {
			return found, errors.New("unexpected VM driver; QEMU fallback is prohibited")
		}
		found = lifecycle.Observation{Exists: true, Status: v.Status, Port: v.SSHLocalPort}
	}
	return found, nil
}
func (b *Backend) noOverrides() error {
	for _, path := range []string{b.home(), filepath.Join(b.home(), "_config"), filepath.Join(b.home(), "default"), filepath.Join(b.Root, "tmp")} {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("runtime directory must not be a symlink or file")
		}
	}
	if entries, err := os.ReadDir(filepath.Join(b.home(), "_config")); err == nil {
		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink != 0 {
				return errors.New("symlinks in runtime global config are prohibited")
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	for _, name := range []string{"default.yaml", "override.yaml", "base.yaml"} {
		if _, e := os.Lstat(filepath.Join(b.home(), "_config", name)); e == nil {
			return errors.New("global runtime overrides are prohibited in the private namespace")
		} else if !os.IsNotExist(e) {
			return e
		}
	}
	return nil
}
func (b *Backend) Create(ctx context.Context, s lifecycle.State) error {
	if e := s.Spec.Validate(); e != nil {
		return e
	}
	for _, dir := range []string{"lima", "tmp"} {
		p := filepath.Join(b.Root, dir)
		if i, e := os.Lstat(p); e == nil {
			if !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
				return errors.New("unsafe runtime directory")
			}
		}
		if e := os.MkdirAll(p, 0700); e != nil {
			return e
		}
	}
	if e := b.noOverrides(); e != nil {
		return e
	}
	bin, e := CheckBinary(s.Spec.GuestBinary)
	if e != nil {
		return e
	}
	f, e := elf.Open(s.Spec.GuestBinary)
	if e != nil {
		return e
	}
	if len(f.Progs) == 0 {
		f.Close()
		return errors.New("guest ELF has no loadable program")
	}
	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP {
			f.Close()
			return errors.New("guest binary must be statically linked")
		}
	}
	f.Close()
	image, e := b.obtainImage(ctx, s.Spec)
	if e != nil {
		return e
	}
	for _, name := range []string{"guest_host", "control"} {
		p := filepath.Join(b.Root, name)
		if _, e := os.Lstat(p); os.IsNotExist(e) {
			if _, e = process.Run(ctx, b.env(), "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "agent-linux", "-f", p); e != nil {
				return errors.New("failed to generate per-machine SSH identity")
			}
		} else if e != nil {
			return e
		}
		if e = storage.SafeFile(b.Root, name); e != nil {
			return e
		}
	}
	hostkey, e := os.ReadFile(filepath.Join(b.Root, "guest_host"))
	if e != nil {
		return e
	}
	pub, e := os.ReadFile(filepath.Join(b.Root, "control.pub"))
	if e != nil {
		return e
	}
	hostpub, e := os.ReadFile(filepath.Join(b.Root, "guest_host.pub"))
	if e != nil {
		return e
	}
	auth := `restrict,command="/usr/local/bin/agent-linux-guest --mode=connect" ` + strings.TrimSpace(string(pub)) + "\n"
	template, e := Template(s.Spec, s.MachineID, image, string(hostkey), auth, bin)
	if e != nil {
		return e
	}
	if e = storage.Write(b.Root, "machine.yaml", template); e != nil {
		return e
	}
	if e = storage.Write(b.Root, "known_hosts", []byte("agent-linux-"+s.MachineID+" "+strings.TrimSpace(string(hostpub))+"\n")); e != nil {
		return e
	}
	if _, e = b.run(ctx, "create", "--tty=false", "--name=default", filepath.Join(b.Root, "machine.yaml")); e != nil {
		return e
	}
	return b.seal()
}
func (b *Backend) seal() error {
	got, e := os.ReadFile(filepath.Join(b.home(), "default", "lima.yaml"))
	if e != nil {
		return e
	}
	original, e := os.ReadFile(filepath.Join(b.Root, "machine.yaml"))
	if e != nil {
		return e
	}
	if e = ValidateEffective(got, original); e != nil {
		return e
	}
	return storage.Write(b.Root, "config.sha256", []byte(Hash(got)))
}
func (b *Backend) Validate(ctx context.Context) error {
	if e := b.noOverrides(); e != nil {
		return e
	}
	for _, p := range []string{filepath.Join(b.home(), "default"), b.home()} {
		i, e := os.Lstat(p)
		if e != nil {
			return e
		}
		if !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
			return errors.New("unsafe runtime state path")
		}
	}
	path := filepath.Join(b.home(), "default", "lima.yaml")
	i, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !i.Mode().IsRegular() || i.Size() > 16<<20 {
		return errors.New("unsafe runtime configuration file")
	}
	got, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	original, e := os.ReadFile(filepath.Join(b.Root, "machine.yaml"))
	if e != nil {
		return e
	}
	if e = ValidateEffective(got, original); e != nil {
		return e
	}
	hash, e := os.ReadFile(filepath.Join(b.Root, "config.sha256"))
	if os.IsNotExist(e) {
		return b.seal()
	}
	if e != nil {
		return e
	}
	if strings.TrimSpace(string(hash)) != Hash(got) {
		return errors.New("runtime configuration changed since creation; refusing start/control")
	}
	return nil
}
func (b *Backend) Start(ctx context.Context) error {
	_, e := b.run(ctx, "start", "--tty=false", "--timeout=20m", "default")
	return e
}
func (b *Backend) Stop(ctx context.Context) error { _, e := b.run(ctx, "stop", "default"); return e }
func (b *Backend) obtainImage(ctx context.Context, s config.Spec) (string, error) {
	path := s.Image
	if s.Image == config.ImageURL {
		path = filepath.Join(b.Root, "debian-arm64.qcow2")
		if _, e := os.Lstat(path); os.IsNotExist(e) {
			tmp, e := os.CreateTemp(b.Root, ".download-")
			if e != nil {
				return "", e
			}
			defer os.Remove(tmp.Name())
			defer tmp.Close()
			req, e := http.NewRequestWithContext(ctx, http.MethodGet, config.ImageURL, nil)
			if e != nil {
				return "", e
			}
			client := http.Client{Timeout: 15 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) > 3 || req.URL.Scheme != "https" || !allowedImageHost(req.URL.Hostname()) {
					return errors.New("unexpected image redirect")
				}
				return nil
			}}
			res, e := client.Do(req)
			if e != nil {
				return "", fmt.Errorf("image download failed: %w", e)
			}
			defer res.Body.Close()
			if res.StatusCode != 200 {
				return "", fmt.Errorf("pinned image unavailable (HTTP %d); select a reviewed image, no daily fallback", res.StatusCode)
			}
			if b.Progress != nil {
				b.Progress(fmt.Sprintf("downloading pinned Debian image (%d content-length bytes; -1 means unknown); verifying SHA-512 before use", res.ContentLength))
			}
			reader := &downloadProgress{r: res.Body, report: b.Progress, last: time.Now()}
			n, e := io.Copy(tmp, io.LimitReader(reader, (4<<30)+1))
			if e != nil {
				return "", e
			}
			if n > 4<<30 {
				return "", errors.New("image exceeds 4 GiB download limit")
			}
			if e = tmp.Sync(); e != nil {
				return "", e
			}
			if e = tmp.Close(); e != nil {
				return "", e
			}
			if e = verifyImage(tmp.Name(), s.Digest); e != nil {
				return "", e
			}
			if e = os.Rename(tmp.Name(), path); e != nil {
				return "", e
			}
		} else if e != nil {
			return "", e
		}
	}
	if e := verifyImage(path, s.Digest); e != nil {
		return "", e
	}
	if e := inspectImage(path, s.DiskGiB); e != nil {
		return "", e
	}
	return path, nil
}
func verifyImage(path, digest string) error {
	if e := config.Regular(path); e != nil {
		return e
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	if st, err := f.Stat(); err != nil {
		return err
	} else if st.Size() > 4<<30 {
		return errors.New("image exceeds 4 GiB safety limit")
	}
	h := sha512.New()
	if _, e = io.Copy(h, io.LimitReader(f, (4<<30)+1)); e != nil {
		return e
	}
	if hex.EncodeToString(h.Sum(nil)) != digest {
		return errors.New("IMAGE_VERIFICATION_FAILED: image SHA-512 differs from pinned trust value")
	}
	return nil
}

// Pinning is independent from Lima's relaxed bootstrap SSH policy.
func (b *Backend) exchange(ctx context.Context, s lifecycle.State, o lifecycle.Observation) (*protocol.Info, error) {
	return b.exchangeCall(ctx, s, o, nil)
}

func (b *Backend) exchangeCall(ctx context.Context, s lifecycle.State, o lifecycle.Observation, operation *protocol.Request) (*protocol.Info, error) {
	if o.Port < 1 || o.Port > 65535 {
		return nil, protocol.Fail("DISCONNECTED", "runtime SSH endpoint not available")
	}
	args := []string{"-F", "/dev/null", "-T", "-p", fmt.Sprint(o.Port), "-i", filepath.Join(b.Root, "control"), "-o", "IdentitiesOnly=yes", "-o", "IdentityAgent=none", "-o", "ForwardAgent=no", "-o", "ForwardX11=no", "-o", "ClearAllForwardings=yes", "-o", "StrictHostKeyChecking=yes", "-o", "NoHostAuthenticationForLocalhost=no", "-o", "UserKnownHostsFile=" + filepath.Join(b.Root, "known_hosts"), "-o", "GlobalKnownHostsFile=/dev/null", "-o", "HostKeyAlias=agent-linux-" + s.MachineID, "-o", "HostKeyAlgorithms=ssh-ed25519", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "ServerAliveInterval=2", "-o", "ServerAliveCountMax=2", "agent@127.0.0.1"}
	cmd := process.Prepare(ctx, "ssh", args...)
	cmd.Env = b.env()
	in, e := cmd.StdinPipe()
	if e != nil {
		return nil, e
	}
	out, e := cmd.StdoutPipe()
	if e != nil {
		return nil, e
	}
	errbuf := &process.Buffer{Limit: 4096}
	cmd.Stderr = errbuf
	if e = cmd.Start(); e != nil {
		return nil, e
	}
	defer func() {
		in.Close()
		if cmd.Process != nil {
			_ = cmd.Cancel()
		}
		_ = cmd.Wait()
	}()
	reader := protocol.Reader(out)
	q := protocol.Request{Version: protocol.Version, ID: protocol.ID(), MachineID: s.MachineID, Method: "hello"}
	if e = protocol.WriteRequest(in, q); e != nil {
		return nil, e
	}
	var p protocol.Response
	if e = protocol.Read(reader, &p); e != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if strings.Contains(string(errbuf.Bytes()), "Host key verification failed") || strings.Contains(string(errbuf.Bytes()), "REMOTE HOST IDENTIFICATION HAS CHANGED") {
			return nil, protocol.Fail("HOST_KEY_MISMATCH", "pinned guest host key does not match; do not disable verification")
		}
		return nil, protocol.Fail("DISCONNECTED", "guest disconnected before handshake; inspect provisioning/service and host-key enrollment")
	}
	if e = checkResponse(q, p); e != nil {
		return nil, e
	}
	g := p.Result
	if !protocol.CompatibleGuestVersion(g.GuestVersion) || !protocol.ValidID(g.SessionID) || g.BootID == "" || g.OS != "linux" || g.Arch != "arm64" {
		return nil, errors.New("guest version/architecture/session not compatible")
	}
	q.ID = protocol.ID()
	q.SessionID = g.SessionID
	q.Method = "probe"
	q.Nonce = protocol.ID()
	if e = protocol.WriteRequest(in, q); e != nil {
		return nil, e
	}
	p = protocol.Response{}
	if e = protocol.Read(reader, &p); e != nil {
		return nil, e
	}
	if e = checkResponse(q, p); e != nil {
		return nil, e
	}
	if p.Result.SessionID != g.SessionID {
		return nil, errors.New("guest session changed during probe")
	}
	h := sha256Probe(q.Nonce)
	if p.Result.NonceSHA256 != h {
		return nil, errors.New("guest probe response mismatch")
	}
	if operation != nil {
		q = *operation
		q.Version = protocol.Version
		q.ID = protocol.ID()
		q.MachineID = s.MachineID
		if q.SessionID == "" {
			q.SessionID = g.SessionID
		}
		if e = protocol.WriteRequest(in, q); e != nil {
			return nil, protocol.Fail("OUTCOME_UNKNOWN", "guest request write failed; do not replay mutations blindly")
		}
		p = protocol.Response{}
		if e = protocol.Read(reader, &p); e != nil {
			return nil, protocol.Fail("OUTCOME_UNKNOWN", "guest disconnected or deadline elapsed; command cancellation requested, outcome/cleanup unconfirmed")
		}
		if e = checkResponse(q, p); e != nil {
			return nil, e
		}
		if p.Result.SessionID != q.SessionID {
			return nil, protocol.Fail("STALE_SESSION", "guest session changed")
		}
		if p.Result.Frame != nil {
			if p.Result.Frame.SessionID != q.SessionID {
				return nil, protocol.Fail("STALE_FRAME", "screenshot session mismatch")
			}
			if e = p.Result.Frame.Validate(); e != nil {
				return nil, e
			}
		}
	}
	return p.Result, nil
}
func checkResponse(q protocol.Request, p protocol.Response) error {
	if p.Version != protocol.Version {
		return protocol.Fail("VERSION_MISMATCH", "guest protocol mismatch; use a fresh --state-dir for Phase 2, existing disks are not implicitly upgraded")
	}
	if p.ID != q.ID {
		return errors.New("guest response request ID mismatch")
	}
	if (p.Result == nil) == (p.Error == nil) {
		return errors.New("invalid guest result/error envelope")
	}
	if p.Error != nil {
		return protocol.Fail(Redact(p.Error.Code), Redact(p.Error.Message))
	}
	if p.Result.MachineID != q.MachineID || p.Result.Protocol != protocol.Version {
		return errors.New("guest machine/protocol identity mismatch")
	}
	return nil
}
func sha256Probe(nonce string) string { return Hash([]byte(nonce)) }
func (b *Backend) Handshake(ctx context.Context, s lifecycle.State) (*protocol.Info, error) {
	var last error
	for {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("guest handshake deadline/interruption; VM may still run: %w", ctx.Err())
		}
		o, e := b.Observe(ctx)
		if e != nil {
			return nil, e
		}
		if o.Status != "Running" {
			return nil, errors.New("VM is not running; guest cannot be ready")
		}
		attempt, cancel := context.WithTimeout(ctx, 8*time.Second)
		g, e := b.exchange(attempt, s, o)
		cancel()
		if e == nil {
			if g.Desktop != nil && g.Desktop.Ready {
				return g, nil
			}
			e = errors.New("guest service is ready but managed desktop is unavailable")
		}
		last = e
		var p *protocol.Failure
		if errors.As(e, &p) {
			return nil, e
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("guest not ready (%v): %w", last, ctx.Err())
		case <-time.After(time.Second):
		}
	}
}

var _ lifecycle.Backend = (*Backend)(nil)

func allowedImageHost(host string) bool {
	return host == "cloud.debian.org" || host == "cdimage.debian.org" || host == "ftp.acc.umu.se" || strings.HasSuffix(host, ".ftp.acc.umu.se")
}

// Call never retries a desktop mutation or command after transmission.
func (b *Backend) Call(ctx context.Context, s lifecycle.State, q protocol.Request) (*protocol.Info, error) {
	o, e := b.Observe(ctx)
	if e != nil {
		return nil, e
	}
	if !o.Exists || o.Status != "Running" {
		return nil, protocol.Fail("NOT_READY", "local VM is not running")
	}
	return b.exchangeCall(ctx, s, o, &q)
}

// Delete is called only after the controller confirms a stopped, identity-selected VM.
func (b *Backend) Delete(ctx context.Context) error {
	_, e := b.run(ctx, "delete", "--force", "default")
	return e
}

type downloadProgress struct {
	r      io.Reader
	report func(string)
	n      int64
	last   time.Time
}

func (p *downloadProgress) Read(b []byte) (int, error) {
	n, e := p.r.Read(b)
	p.n += int64(n)
	if p.report != nil && (time.Since(p.last) > 2*time.Second || e == io.EOF) {
		p.report(fmt.Sprintf("downloaded %.1f MiB; setup is still in progress", float64(p.n)/(1<<20)))
		p.last = time.Now()
	}
	return n, e
}
