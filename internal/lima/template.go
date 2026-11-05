// SPDX-License-Identifier: Apache-2.0

package lima

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/software"
	"go.yaml.in/yaml/v3"
	"os"
	"reflect"
	"strings"
)

func Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// Template emits YAML block scalars; no upstream templates/mount defaults are inherited.
func Template(s config.Spec, id, image, hostKey, authorized string, binary []byte) ([]byte, error) {
	var packed bytes.Buffer
	zw := gzip.NewWriter(&packed)
	if _, err := zw.Write(binary); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	unit := fmt.Sprintf(`[Unit]
Description=Agent Linux guest control
After=local-fs.target agent-linux-desktop.service
[Service]
User=agent
Group=agent
RuntimeDirectory=agent-linux
RuntimeDirectoryMode=0700
ExecStart=/usr/local/bin/agent-linux-guest --machine-id=%s
Restart=always
RestartSec=1
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=no
ReadWritePaths=/home/agent /tmp /run/agent-linux
Delegate=yes
PrivateTmp=no
Environment=HOME=/home/agent
Environment=LANG=C.UTF-8
UMask=0077
[Install]
WantedBy=multi-user.target
`, id)
	// All variable shell data is base64. Inputs are never executable shell fragments.
	script := `#!/bin/sh
set -eu
umask 077
id agent >/dev/null 2>&1 || useradd --create-home --shell /bin/bash agent
export DEBIAN_FRONTEND=noninteractive
# New recipe applies only when creating a new machine; never upgrade a sealed existing template.
if [ ! -f /var/lib/agent-linux/` + software.Recipe + ` ]; then
  apt-get -o APT::Update::Error-Mode=any update
  apt-get install -y --no-install-recommends ` + strings.Join(software.DefaultPackages, " ") + `
  install -d /var/lib/agent-linux
  dpkg-query -W > /var/lib/agent-linux/packages.tsv
  printf '%s\n' '` + software.Recipe + `' > /var/lib/agent-linux/recipe
  touch /var/lib/agent-linux/` + software.Recipe + `
fi
printf '%s' '` + base64.StdEncoding.EncodeToString([]byte(softwareScript)) + `' | base64 -d > /usr/local/sbin/agent-linux-software
chmod 0755 /usr/local/sbin/agent-linux-software
# Own guest home directories; no host shares or credentials are introduced.
install -d -m 0700 -o agent -g agent /home/agent/Desktop /home/agent/Documents /home/agent/Downloads /home/agent/Projects
install -d /usr/local/libexec
printf '%s' '` + base64.StdEncoding.EncodeToString([]byte(desktopScript)) + `' | base64 -d > /usr/local/libexec/agent-linux-desktop
chmod 0755 /usr/local/libexec/agent-linux-desktop
printf '%s' '` + base64.StdEncoding.EncodeToString([]byte(desktopUnit)) + `' | base64 -d > /etc/systemd/system/agent-linux-desktop.service
usermod --password '*' agent
install -d -m 0700 -o agent -g agent /home/agent/.ssh
printf '%s' '` + base64.StdEncoding.EncodeToString([]byte(authorized)) + `' | base64 -d > /home/agent/.ssh/authorized_keys
chown agent:agent /home/agent/.ssh/authorized_keys
chmod 0600 /home/agent/.ssh/authorized_keys
printf '%s' '` + wrappedBase64(packed.Bytes()) + `' | base64 -d | gzip -d > /usr/local/bin/agent-linux-guest.new
chmod 0755 /usr/local/bin/agent-linux-guest.new
mv /usr/local/bin/agent-linux-guest.new /usr/local/bin/agent-linux-guest
printf '%s' '` + base64.StdEncoding.EncodeToString([]byte(hostKey)) + `' | base64 -d > /etc/ssh/ssh_host_agentlinux_key
chmod 0600 /etc/ssh/ssh_host_agentlinux_key
printf '%s\n' 'HostKey /etc/ssh/ssh_host_agentlinux_key' 'PasswordAuthentication no' 'KbdInteractiveAuthentication no' > /etc/ssh/sshd_config.d/00-agent-linux.conf
printf '%s' '` + base64.StdEncoding.EncodeToString([]byte(unit)) + `' | base64 -d > /etc/systemd/system/agent-linux.service
systemctl daemon-reload
systemctl enable agent-linux-desktop.service agent-linux.service
systemctl restart agent-linux-desktop.service
systemctl restart agent-linux.service
/usr/sbin/sshd -t
systemctl reload ssh.service
`
	t := map[string]any{"minimumLimaVersion": "2.2.1", "vmType": "vz", "arch": "aarch64", "os": "Linux", "plain": true, "cpus": s.CPUs, "memory": fmt.Sprintf("%dGiB", s.MemoryGiB), "disk": fmt.Sprintf("%dGiB", s.DiskGiB), "images": []any{map[string]any{"location": image, "arch": "aarch64", "digest": "sha512:" + s.Digest}}, "mounts": []any{}, "additionalDisks": []any{}, "portForwards": []any{}, "copyToHost": []any{}, "networks": []any{}, "containerd": map[string]any{"system": false, "user": false}, "ssh": map[string]any{"loadDotSSHPubKeys": false, "forwardAgent": false, "forwardX11": false, "forwardX11Trusted": false, "overVsock": false}, "user": map[string]any{"name": "agentlinux-admin", "comment": "Agent Linux bootstrap", "uid": 1000, "home": "/home/agentlinux-admin", "shell": "/bin/bash"}, "rosetta": map[string]any{"enabled": false, "binfmt": false}, "audio": map[string]any{"device": ""}, "video": map[string]any{"display": "none"}, "propagateProxyEnv": false, "hostResolver": map[string]any{"enabled": false}, "timezone": "UTC", "provision": []any{map[string]any{"mode": "system", "script": script}}, "env": map[string]any{}}
	env := t["env"].(map[string]any)
	for _, k := range []string{"http_proxy", "https_proxy", "ftp_proxy", "no_proxy"} {
		env[k] = ""
		env[strings.ToUpper(k)] = ""
	}
	encoded, err := yaml.Marshal(t)
	if err != nil {
		return nil, err
	}
	if len(encoded) > 3<<20 {
		return nil, errors.New("provisioning template exceeds 3 MiB safety budget (Lima input limit is 4 MiB)")
	}
	return encoded, nil
}
func ValidateEffective(data, original []byte) error {
	if len(data) > 16<<20 {
		return errors.New("effective config exceeds limit")
	}
	var got, want map[string]any
	if e := yaml.Unmarshal(data, &got); e != nil {
		return e
	}
	if e := yaml.Unmarshal(original, &want); e != nil {
		return e
	}
	for _, k := range []string{"vmType", "arch", "plain", "cpus", "memory", "disk", "user", "audio", "video", "mounts", "additionalDisks", "portForwards", "copyToHost", "networks", "propagateProxyEnv", "provision", "images"} {
		g := got[k]
		w := want[k]
		if list, ok := w.([]any); ok && len(list) == 0 && g == nil {
			continue
		}
		if !reflect.DeepEqual(g, w) {
			return fmt.Errorf("unsafe/unexpected effective %s", k)
		}
	}
	for section, keys := range map[string][]string{"ssh": {"loadDotSSHPubKeys", "forwardAgent", "forwardX11", "forwardX11Trusted", "overVsock"}, "containerd": {"system", "user"}, "rosetta": {"enabled", "binfmt"}, "hostResolver": {"enabled"}, "env": {"http_proxy", "https_proxy", "ftp_proxy", "no_proxy", "HTTP_PROXY", "HTTPS_PROXY", "FTP_PROXY", "NO_PROXY"}} {
		g, _ := got[section].(map[string]any)
		w := want[section].(map[string]any)
		for _, key := range keys {
			if !reflect.DeepEqual(g[key], w[key]) {
				return fmt.Errorf("unsafe effective %s.%s", section, key)
			}
		}
	}
	if b := got["base"]; b != nil {
		if a, ok := b.([]any); !ok || len(a) != 0 {
			return errors.New("unexpected inherited template")
		}
	}
	for _, k := range []string{"blockDevices", "caCerts"} {
		if g := got[k]; g != nil {
			switch v := g.(type) {
			case []any:
				if len(v) > 0 {
					return fmt.Errorf("unsafe effective %s", k)
				}
			case map[string]any:
				for _, value := range v {
					if value != nil && !reflect.DeepEqual(value, []any{}) && value != false {
						return fmt.Errorf("unsafe effective %s", k)
					}
				}
			default:
				return fmt.Errorf("unsafe effective %s", k)
			}
		}
	}
	return nil
}
func CheckBinary(path string) ([]byte, error) {
	if e := config.Regular(path); e != nil {
		return nil, e
	}
	if st, err := os.Stat(path); err != nil {
		return nil, err
	} else if st.Size() > 12<<20 {
		return nil, errors.New("guest binary exceeds size limit")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	if len(b) > 12<<20 || len(b) < 64 {
		return nil, errors.New("guest executable size invalid")
	}
	if string(b[:4]) != "\x7fELF" || b[4] != 2 || b[5] != 1 || b[18] != 0xb7 || b[19] != 0 {
		return nil, errors.New("guest executable must be Linux ARM64 ELF; run make build")
	}
	return b, nil
}

// Short base64 lines avoid YAML parser line-length limits for embedded binaries.
func wrappedBase64(b []byte) string {
	s := base64.StdEncoding.EncodeToString(b)
	var out strings.Builder
	for len(s) > 76 {
		out.WriteString(s[:76])
		out.WriteByte('\n')
		s = s[76:]
	}
	out.WriteString(s)
	return out.String()
}
