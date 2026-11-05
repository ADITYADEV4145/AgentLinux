// SPDX-License-Identifier: Apache-2.0
package lima

import (
	"context"
	"strings"

	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/software"
)

func (b *Backend) Maintain(ctx context.Context, action string, packages []string) (*software.Result, error) {
	args, e := software.AdminArgs(action, packages)
	if e != nil {
		return nil, protocol.Fail("INVALID_ARGUMENT", e.Error())
	}
	o, e := b.Observe(ctx)
	if e != nil {
		return nil, e
	}
	if o.Status != "Running" {
		return nil, protocol.Fail("NOT_READY", "start the existing VM before package maintenance")
	}
	r, e := b.run(ctx, args...)
	if e != nil {
		text := string(r.Out) + string(r.Err)
		if strings.Contains(text, "AGENTLINUX_REPOSITORY_UNAVAILABLE") {
			return nil, protocol.Fail("REPOSITORY_UNAVAILABLE", "guest package indexes could not be refreshed; check guest network, signatures and mirrors using private backend diagnostics")
		}
		if strings.Contains(text, "Unable to locate package") || strings.Contains(text, "has no installation candidate") {
			return nil, protocol.Fail("PACKAGE_UNAVAILABLE", "Debian repositories do not offer a requested package; inspect private backend diagnostics")
		}
		// The durable guest unit may still run after a dropped administrative SSH link.
		return nil, protocol.Fail("PACKAGE_OUTCOME_UNKNOWN", "guest package job failed or its connection was lost; inspect systemctl status agent-linux-software and private backend logs before retrying; old machines may lack the desktop-3.5 maintenance helper")
	}
	return &software.Result{Action: action, Packages: packages, Output: Redact(string(r.Out)), Note: "guest packages only; package versions recorded in /var/lib/agent-linux/packages.tsv; stop/up for a required reboot; no rollback guarantee"}, nil
}
