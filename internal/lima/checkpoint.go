// SPDX-License-Identifier: Apache-2.0
package lima

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/lifecycle"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
	"go.yaml.in/yaml/v3"
)

// CheckpointCompatibility pins the exact VZ instance and hardware/image contract,
// while allowing an explicitly checkpoint-backed helper/provisioning update.
func (b *Backend) CheckpointCompatibility(ctx context.Context, s lifecycle.State) (string, error) {
	if e := b.noOverrides(); e != nil {
		return "", e
	}
	root, e := os.OpenRoot(b.Root)
	if e != nil {
		return "", e
	}
	defer root.Close()
	for _, name := range []string{"lima/default/disk", "lima/default/vz-efi", "lima/default/vz-identifier"} {
		i, e := root.Lstat(name)
		if e != nil {
			return "", e
		}
		if !i.Mode().IsRegular() || i.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("unsupported/symlink VZ checkpoint file")
		}
	}
	disk, e := root.Open("lima/default/disk")
	if e != nil {
		return "", e
	}
	var sector [1024]byte
	_, e = disk.ReadAt(sector[:], 0)
	disk.Close()
	if e != nil {
		return "", e
	}
	if sector[510] != 0x55 || sector[511] != 0xaa {
		return "", errors.New("only the verified raw VZ disk layout supports checkpoints; ASIF/qcow backing chains are not accepted")
	}
	identifier, e := root.ReadFile("lima/default/vz-identifier")
	if e != nil {
		return "", e
	}
	if len(identifier) > 4096 {
		return "", errors.New("oversized VZ identifier")
	}
	return Hash([]byte(fmt.Sprintf("vz/%s/arm64/%s/%d/%d/%d/%s/%s", config.LimaVersion, s.MachineID, s.Spec.CPUs, s.Spec.MemoryGiB, s.Spec.DiskGiB, s.Spec.Digest, Hash(identifier)))), nil
}
func (b *Backend) RestartDesktop(ctx context.Context) error {
	_, e := b.run(ctx, "shell", "default", "sudo", "--non-interactive", "systemctl", "restart", "agent-linux-desktop.service")
	return e
}

var helperPayload = regexp.MustCompile(`(?s)(printf '%s' ')([A-Za-z0-9+/=\n]+)(' \| base64 -d \| gzip -d > /usr/local/bin/agent-linux-guest\.new)`)

func replaceHelper(data, binary []byte) ([]byte, error) {
	var tree map[string]any
	if e := yaml.Unmarshal(data, &tree); e != nil {
		return nil, e
	}
	provision, ok := tree["provision"].([]any)
	if !ok {
		return nil, errors.New("unsupported provisioning shape")
	}
	var packed bytes.Buffer
	zw := gzip.NewWriter(&packed)
	if _, e := zw.Write(binary); e != nil {
		return nil, e
	}
	if e := zw.Close(); e != nil {
		return nil, e
	}
	replaced := 0
	for _, entry := range provision {
		item, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		script, ok := item["script"].(string)
		if !ok {
			continue
		}
		matches := helperPayload.FindAllStringSubmatchIndex(script, -1)
		if len(matches) == 0 {
			continue
		}
		if len(matches) != 1 || item["mode"] != "system" {
			return nil, errors.New("ambiguous helper provisioning payload")
		}
		loc := matches[0]
		script = script[:loc[4]] + wrappedBase64(packed.Bytes()) + script[loc[5]:]
		item["script"] = script
		replaced++
	}
	if replaced != 1 {
		return nil, errors.New("existing guest template does not support a helper-only upgrade")
	}
	result, e := yaml.Marshal(tree)
	if e != nil {
		return nil, e
	}
	if len(result) > 3<<20 {
		return nil, errors.New("upgraded template exceeds provisioning budget")
	}
	return result, nil
}
func (b *Backend) UpgradeHelper(ctx context.Context, binaryPath string) error {
	if e := b.Validate(ctx); e != nil {
		return e
	}
	binary, e := CheckBinary(binaryPath)
	if e != nil {
		return e
	}
	original, e := os.ReadFile(filepath.Join(b.Root, "machine.yaml"))
	if e != nil {
		return e
	}
	effective, e := os.ReadFile(filepath.Join(b.home(), "default/lima.yaml"))
	if e != nil {
		return e
	}
	nextOriginal, e := replaceHelper(original, binary)
	if e != nil {
		return e
	}
	nextEffective, e := replaceHelper(effective, binary)
	if e != nil {
		return e
	}
	if e = ValidateEffective(nextEffective, nextOriginal); e != nil {
		return e
	}
	// Caller already established a synced recovery journal and rollback disk. Each
	// metadata write is atomic; interrupted multi-file updates cannot be booted.
	if e = storage.Write(b.Root, "machine.yaml", nextOriginal); e != nil {
		return e
	}
	dir := filepath.Join(b.home(), "default")
	if e = writeRuntimeTemplate(dir, nextEffective); e != nil {
		return e
	}
	return storage.Write(b.Root, "config.sha256", []byte(strings.TrimSpace(Hash(nextEffective))))
}

func writeRuntimeTemplate(dir string, data []byte) error {
	r, e := os.OpenRoot(dir)
	if e != nil {
		return e
	}
	defer r.Close()
	i, e := r.Lstat("lima.yaml")
	if e != nil {
		return e
	}
	if !i.Mode().IsRegular() || i.Mode()&os.ModeSymlink != 0 {
		return errors.New("unsafe runtime template")
	}
	name := ".helper-" + protocol.ID()
	f, e := r.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer r.Remove(name)
	if _, e = f.Write(data); e == nil {
		e = f.Sync()
	}
	f.Close()
	if e != nil {
		return e
	}
	if e = r.Rename(name, "lima.yaml"); e != nil {
		return e
	}
	d, e := r.Open(".")
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}
