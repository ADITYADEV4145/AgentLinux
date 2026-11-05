// SPDX-License-Identifier: Apache-2.0
// Package software describes the reviewed guest recipe and explicit maintenance requests.
package software

import (
	"fmt"
	"regexp"
	"strings"
)

const Recipe = "desktop-3.5.0"

var DefaultPackages = []string{
	"xvfb", "xauth", "x11-utils", "xfce4-session", "xfwm4", "xfdesktop4", "xfce4-panel", "xfce4-settings", "xfce4-appfinder", "thunar", "xfce4-terminal", "mousepad", "chromium", "chromium-sandbox", "firefox-esr", "xdotool", "scrot", "xclip", "dbus-x11", "fonts-dejavu", "fonts-noto-core", "fonts-noto-cjk", "fonts-noto-color-emoji", "adwaita-icon-theme", "desktop-file-utils", "xdg-utils", "bash", "bash-completion", "git", "openssh-client", "curl", "ca-certificates", "python3", "python3-venv", "less", "nano", "man-db", "procps", "util-linux", "gvfs", "tumbler", "at-spi2-core",
}
var Profiles = map[string][]string{
	"web-dev": {"nodejs", "npm"},
	"build":   {"build-essential", "pkg-config", "python3-dev"},
}
var packageName = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]{0,99}$`)

func Validate(action string, packages []string) error {
	switch action {
	case "refresh", "upgrade":
		if len(packages) != 0 {
			return fmt.Errorf("%s takes no package names", action)
		}
	case "install":
		if len(packages) < 1 || len(packages) > 32 {
			return fmt.Errorf("install requires 1..32 Debian package names")
		}
		for _, p := range packages {
			if !packageName.MatchString(p) {
				return fmt.Errorf("invalid Debian package name; options, URLs, paths and version selectors are not accepted")
			}
		}
	default:
		return fmt.Errorf("choose install, refresh or upgrade")
	}
	return nil
}

type Browser struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Package      string `json:"package"`
	Availability string `json:"availability"`
	Installed    bool   `json:"installed"`
	Version      string `json:"version,omitempty"`
	Installable  bool   `json:"installableThroughAgentLinux"`
	Note         string `json:"note"`
}

// Availability records researched architecture support, not a runtime launch test.
func Browsers(arch, inventory string) []Browser {
	rows := []Browser{
		{ID: "chromium", Name: "Chromium", Package: "chromium", Installable: true, Note: "Debian package; default browser, not Google Chrome"},
		{ID: "firefox", Name: "Firefox ESR", Package: "firefox-esr", Installable: true, Note: "Debian security-supported ESR package; second default browser"},
		{ID: "chrome", Name: "Google Chrome", Package: "google-chrome-stable", Note: "Vendor installation/license required; not installed or tested by this recipe; never substituted with Chromium"},
		{ID: "edge", Name: "Microsoft Edge", Package: "microsoft-edge-stable", Note: "Vendor installation/license required; Linux ARM64 build unavailable in reviewed sources"},
	}
	installed := map[string]string{}
	for _, line := range strings.Split(inventory, "\n") {
		f := strings.Split(line, "\t")
		if len(f) == 3 && f[2] == "installed" {
			installed[strings.Split(f[0], ":")[0]] = f[1]
		}
	}
	for i := range rows {
		r := &rows[i]
		r.Availability = "not_yet_investigated"
		if arch == "arm64" || arch == "amd64" {
			r.Availability = "expected_to_work_not_yet_tested"
		}
		if arch == "arm64" && r.ID == "edge" {
			r.Availability = "unavailable_on_this_architecture"
		}
		r.Version = installed[r.Package]
		r.Installed = r.Version != ""
		if arch != "arm64" && arch != "amd64" {
			r.Installable = false
		}
	}
	return rows
}
func BrowserPackage(id string) (string, error) {
	switch id {
	case "chromium":
		return "chromium", nil
	case "firefox":
		return "firefox-esr", nil
	}
	return "", fmt.Errorf("only Debian chromium and firefox are automated; vendor Chrome/Edge require separate official installation and license review, never substitution")
}
