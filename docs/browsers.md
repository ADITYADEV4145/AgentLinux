# Browser compatibility and installation

Researched 2026-11-05 against official distribution/vendor documentation. Architecture availability and actual AgentLinux acceptance are separate. Chromium is not Google Chrome. No personal account/sign-in is required for the basic browser tests. This repository provisions packages; it does not redistribute a browser binary or prebuilt guest image.

| Browser | Debian ARM64 local | Linux x86_64 future cloud | Installation and policy |
| --- | --- | --- | --- |
| Chromium | Supported and tested in the real guest; preinstalled | Expected to work, not yet tested | Signed Debian `chromium` + `chromium-sandbox`; default `browser`/`chromium` launch target |
| Firefox ESR | Supported and tested in the real guest; preinstalled since recipe 3.5, retained in workstation-5.5.0 | Expected to work, not yet tested | Signed Debian `firefox-esr`; `open firefox`; ESR rather than silently selecting Mozilla rapid release |
| Google Chrome | Expected to work, not yet tested; optional vendor install | Expected to work, not yet tested | Current official Linux requirements include ARM and Debian; vendor download/terms required, not Debian main. AgentLinux does not automate or redistribute it |
| Microsoft Edge | Unavailable on this architecture in reviewed sources | Expected to work, not yet tested | Official Linux downloads/repository reviewed are amd64; Arm's current guide explicitly excludes Arm Linux. Optional vendor install only on a matching supported architecture |

The Chrome result differs from older advice that Linux Chrome is x86-only. Google's current system requirements list ARM; that is **not** proof that an arbitrary installer works here. Attempts to retrieve Google apt repository metadata through the research fetch tool were unreachable, so a repository/package binary was not independently validated. Use only the vendor's official matching-architecture installer, review its terms, and record a real launch test before claiming support. Edge's generic “64-bit Linux” requirement does not establish ARM64 availability. Do not install unofficial repacks, enable x86 emulation silently, or call Chromium a Chrome substitute.

`browsers list` queries installed package versions from the actual guest and separately labels researched architecture expectations. It does not launch each browser and therefore deliberately does not infer “tested” from an installed package. Phase 5.5 additionally exercised Chromium 156.0.8078.79 and Firefox ESR 153.6.0esr through real MCP Unicode forms, drag/scroll and focus checks on both comparison desktops; see workstation.md. These are recorded configurations, not every version or website. The fresh Xfce candidate had one initial Chromium launch failure followed by a successful explicit retry; it is not hidden by the tested label.

```sh
bin/agent-linux browsers list --json
bin/agent-linux browsers install firefox
bin/agent-linux browsers install chromium
bin/agent-linux open firefox
bin/agent-linux open chromium
```

Options (`--lima`, `--state-dir`, `--json`, `--timeout`) precede the browser name. Install uses explicit host-operator guest administration, needs working guest apt networking/signatures and may take minutes. It refreshes indexes and installs signed Debian packages without disabling browser sandboxes. `browsers install chrome` or `edge` returns unsupported with context; it never substitutes. On older recipes use the [documented legacy maintenance route](../images/debian/README.md) or create a separate fresh namespace without deleting the old disk.

For other supported browsers, consult [Mozilla's official Linux download/install guide](https://www.firefox.com/en-US/download/linux/), [Google's official Chrome download](https://www.google.com/chrome/), or [Microsoft's official Edge download](https://www.microsoft.com/en-us/edge/download). Download **inside the guest**, select Linux and the guest architecture, review the vendor terms, and install using guest administrator access. Do not run a Linux package installer on macOS or bypass package signatures. Third-party repository enrollment, signing-key rotation and vendor auto-update policy are deliberately not automated here. Vendor browsers and arbitrary new applications are not fixed MCP launch targets; use their desktop menu entry or explicit guest commands after installation.

## Profiles, updates and licensing

Chromium uses its normal guest `~/.config/chromium` profile; Firefox ESR uses `~/.mozilla/firefox`. Guest stop/up preserves these directories. Phase 2/3's old helper used `~/.config/agent-linux/chromium`; old machines are not silently converted or reset. Keep that profile and explicitly select it when moving data; do not assume cross-version/cross-architecture password/profile portability. Browser shutdown or profile recovery can affect the last unsaved session even though the disk persists.

Debian browser updates require `packages upgrade` or explicit operator apt maintenance; restart is not an automatic security update. Reopen a browser after upgrading it. DRM/codecs, extensions, hardware acceleration, WebGL, video/audio, downloads with external handlers, browser debugging/WebDriver and every site's behavior are not certified by a simple page-navigation test. The image contains ordinary sandbox-enabled graphical browsers; no `--no-sandbox` fallback is used.

Chromium has BSD-style core and many third-party licenses; Firefox has MPL-2.0 core plus other components and trademark conditions. Google Chrome/Edge add proprietary vendor terms. Preserve package-specific notices and review corresponding-source/trademark/redistribution requirements before publishing an image. Debian main availability is not blanket distribution clearance for a future bundle.

Official sources: [Debian Firefox ESR architectures/security](https://packages.debian.org/trixie/firefox-esr), [Debian Chromium architectures/security](https://packages.debian.org/trixie/chromium), [Mozilla Linux installation](https://support.mozilla.org/en-US/kb/install-firefox-linux), [Chrome Linux/ARM installers and requirements](https://support.google.com/chrome/answer/95346?hl=en&co=GENIE.Platform%3DDesktop), [Edge OS requirements](https://learn.microsoft.com/en-us/deployedge/microsoft-edge-supported-operating-systems), [Microsoft's stable package directory](https://packages.microsoft.com/repos/edge/pool/main/m/microsoft-edge-stable/), [Arm's Edge guide](https://learn.arm.com/install-guides/browsers/edge/).
