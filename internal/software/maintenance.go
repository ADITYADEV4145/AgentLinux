// SPDX-License-Identifier: Apache-2.0
package software

type Result struct {
	Action   string   `json:"action"`
	Packages []string `json:"packages,omitempty"`
	Output   string   `json:"output"`
	Note     string   `json:"note"`
}

// AdminArgs is an explicit guest-only operation. No host shell or environment forwarding.
func AdminArgs(action string, packages []string) ([]string, error) {
	if e := Validate(action, packages); e != nil {
		return nil, e
	}
	args := []string{"shell", "--workdir=/", "default", "sudo", "-n", "/usr/bin/systemd-run", "--unit=agent-linux-software", "--collect", "--wait", "--pipe", "--service-type=exec", "--property=RuntimeMaxSec=540", "--property=TimeoutStopSec=15", "--property=KillMode=control-group", "/usr/local/sbin/agent-linux-software", action}
	return append(args, packages...), nil
}
