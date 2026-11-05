// SPDX-License-Identifier: Apache-2.0

package lima

import (
	"crypto/rand"
	"encoding/binary"
	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"go.yaml.in/yaml/v3"
	"os"
	"strings"
	"testing"
)

func TestRejectUnsafeEffectiveConfig(t *testing.T) {
	base, e := Template(config.Default(), protocol.ID(), "/private/test/image", "fixture-host-key", "fixture-authorized", []byte("fixture-binary"))
	if e != nil {
		t.Fatal(e)
	}
	if e = ValidateEffective(base, base); e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{"mounts", "portForwards", "vmType", "provision"} {
		var bad map[string]any
		_ = yaml.Unmarshal(base, &bad)
		bad[key] = "unexpected"
		b, _ := yaml.Marshal(bad)
		if ValidateEffective(b, base) == nil {
			t.Errorf("accepted unsafe %s", key)
		}
	}
}
func TestResponseValidation(t *testing.T) {
	q := protocol.Request{Version: protocol.Version, ID: protocol.ID(), MachineID: protocol.ID()}
	p := protocol.Response{Version: protocol.Version, ID: q.ID, Result: &protocol.Info{MachineID: q.MachineID, Protocol: protocol.Version}}
	if e := checkResponse(q, p); e != nil {
		t.Fatal(e)
	}
	p.Version = "99.0"
	if checkResponse(q, p) == nil {
		t.Fatal("version mismatch accepted")
	}
	p.Version = protocol.Version
	p.ID = protocol.ID()
	if checkResponse(q, p) == nil {
		t.Fatal("mismatched request ID accepted")
	}
}
func TestRedaction(t *testing.T) {
	text := "-----BEGIN OPENSSH PRIVATE KEY-----\nSECRET\n-----END OPENSSH PRIVATE KEY-----\napi_key=supersecret " + strings.Repeat("a", 200)
	text += " https://fixture-user:fixture-pass@repo.example.test/debian"
	redacted := Redact(text)
	if strings.Contains(redacted, "fixture-user") || strings.Contains(redacted, "fixture-pass") || strings.Contains(redacted, "SECRET") || strings.Contains(redacted, "supersecret") || strings.Contains(redacted, strings.Repeat("a", 100)) {
		t.Fatal("secret not redacted")
	}
}

func TestImageHeaderConstraints(t *testing.T) {
	r := t.TempDir()
	path := r + "/image.qcow2"
	header := make([]byte, 104)
	copy(header, "QFI\xfb")
	binary.BigEndian.PutUint32(header[4:8], 3)
	binary.BigEndian.PutUint64(header[24:32], 2*config.GiB)
	if e := os.WriteFile(path, header, 0600); e != nil {
		t.Fatal(e)
	}
	if e := inspectImage(path, 8); e != nil {
		t.Fatal(e)
	}
	for _, offset := range []int{8, 16, 32, 72} {
		bad := append([]byte(nil), header...)
		bad[offset] = 1
		_ = os.WriteFile(path, bad, 0600)
		if inspectImage(path, 8) == nil {
			t.Errorf("unsafe header offset %d accepted", offset)
		}
	}
	binary.BigEndian.PutUint64(header[24:32], 100*config.GiB)
	_ = os.WriteFile(path, header, 0600)
	if inspectImage(path, 8) == nil {
		t.Fatal("oversized virtual disk accepted")
	}
}
func TestProvisioningCompressionBudget(t *testing.T) {
	payload := make([]byte, 4<<20)
	base, e := Template(config.Default(), protocol.ID(), "/private/test/image", "fixture", "fixture", payload)
	if e != nil || len(base) > 3<<20 {
		t.Fatalf("compressible binary did not fit Lima limit: %d %v", len(base), e)
	}
	if _, e = rand.Read(payload); e != nil {
		t.Fatal(e)
	}
	if _, e = Template(config.Default(), protocol.ID(), "/private/test/image", "fixture", "fixture", payload); e == nil {
		t.Fatal("oversized template was allowed to be truncated by Lima")
	}
}
func TestMirrorRedirectPolicy(t *testing.T) {
	for _, host := range []string{"cloud.debian.org", "laotzu.ftp.acc.umu.se"} {
		if !allowedImageHost(host) {
			t.Fatal("official mirror rejected")
		}
	}
	for _, host := range []string{"localhost", "cloud.debian.org.attacker.test", "169.254.169.254"} {
		if allowedImageHost(host) {
			t.Fatal("unexpected mirror accepted")
		}
	}
}

func TestRejectRuntimeDirectorySymlinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if e := os.Symlink(outside, root+"/lima"); e != nil {
		t.Fatal(e)
	}
	b := Backend{Root: root}
	if b.noOverrides() == nil {
		t.Fatal("symlinked runtime namespace accepted")
	}
}

func TestCompleteRecipeAndMaintenance(t *testing.T) {
	data, e := Template(config.Default(), protocol.ID(), "/private/test/image", "fixture", "fixture", []byte("binary"))
	if e != nil {
		t.Fatal(e)
	}
	var parsed map[string]any
	if e = yaml.Unmarshal(data, &parsed); e != nil {
		t.Fatal(e)
	}
	provision := parsed["provision"].([]any)[0].(map[string]any)["script"].(string)
	for _, required := range []string{"firefox-esr", "thunar", "xfce4-appfinder", "python3-venv", "openssh-client", "desktop-3.5.0", "/usr/local/sbin/agent-linux-software"} {
		if !strings.Contains(provision, required) {
			t.Fatal("missing recipe component", required)
		}
	}
	if strings.Contains(provision, "NOPASSWD") || strings.Contains(provision, "usermod -aG sudo") {
		t.Fatal("guest agent granted administrator privileges")
	}
	if !strings.Contains(softwareScript, "APT::Update::Error-Mode=any") || !strings.Contains(softwareScript, "flock -n") {
		t.Fatal("maintenance does not fail closed or serialize")
	}
	if !protocol.CompatibleGuestVersion("0.2.0") || !protocol.CompatibleGuestVersion("0.3.5") || protocol.CompatibleGuestVersion("0.1.0") {
		t.Fatal("legacy v2 compatibility")
	}
}
