# Third-party licenses

Original AgentLinux code is Apache-2.0 (root LICENSE). No terminal-mac, vmette, Vitrine, Tart, E2B, or Daytona source/assets have been incorporated.

- `go.yaml.in/yaml/v3` v3.0.4 is a runtime Go module. Its upstream LICENSE covers libyaml-derived files under MIT and remaining files under Apache-2.0; exact text is retained in go-yaml.LICENSE. Source: https://github.com/yaml/go-yaml/tree/v3.0.4.
- Go 1.26.1 standard-library license is retained in go.LICENSE. Toolchain third-party notices also apply when redistributing toolchain components; we do not bundle the compiler.
- `gopkg.in/check.v1` is a dependency module's test dependency in go.sum, not linked into our shipped commands.
- Lima 2.2.1 is an external user-installed executable, Apache-2.0. Development runtime downloads are ignored and not bundled. Releases that bundle Lima must retain its complete license/notice inventory and audit its SBOM.
- OpenSSH is supplied by the host and guest OS, with its own component licenses.
- Debian image packages retain their individual licenses; the image is downloaded at runtime, not committed or redistributed here. A redistribution review and corresponding-source obligations remain necessary before publishing a guest image.

The local build now produces an unpublished developer RC bundle with linked dependency inventory and these notices; it is not a cleared redistributed VM/runtime image. License copies here are notices, not an assertion that a future binary/image bundle has passed a complete license audit.


Phase 2's guest package recipe installs Xvfb/Xorg (component X11/MIT-style notices), Xfce (component GPL/LGPL), Mousepad (GPL), Chromium (BSD-style core plus extensive third-party notices), xdotool (BSD-style), xclip (GPLv2), scrot (MIT-feh acknowledgement terms), and Noto/DejaVu fonts (font-specific licenses). The screenshot adapter uses scrot. Review exact Debian `/usr/share/doc/<package>/copyright` and source obligations before redistributing an image. The recipe/runtime installation is not a statement that a bundled image has distribution clearance. No new Go dependency was added for desktop or cgroup control.


## Phase 3 Go dependencies

MCP integration adds the pinned modules below. Exact upstream LICENSE/NOTICE files are retained alongside this document. SDK dependencies are downloaded by Go with go.sum verification; source pins are in go.mod/go.sum. The MCP SDK is undergoing MIT → Apache-2.0 relicensing: retain its complete mixed-license notice, rather than label all files Apache-only. Documentation/specification licenses mentioned in that notice do not make our original code CC-BY.

| Module | Version | Upstream terms |
| --- | --- | --- |
| github.com/modelcontextprotocol/go-sdk | v1.8.0 | Apache-2.0/new and consenting contributions; legacy MIT; exact notice retained |
| github.com/google/jsonschema-go | v0.4.3 | MIT |
| github.com/segmentio/encoding | v0.5.4 | MIT |
| github.com/segmentio/asm | v1.1.3 | MIT |
| github.com/yosida95/uritemplate/v3 | v3.0.2 | BSD-3-Clause |
| golang.org/x/oauth2 | v0.35.0 | BSD-3-Clause |
| golang.org/x/sync | v0.20.0 | BSD-3-Clause |
| golang.org/x/sys | v0.41.0 | BSD-3-Clause |
| golang.org/x/time | v0.15.0 | BSD-3-Clause |

Some SDK modules in go.sum are upstream/test dependencies, not necessarily linked into our binaries. No SDK HTTP/OAuth listener is enabled by this stdio adapter. Phase 5 generates linked dependency inventory in DEPENDENCIES.txt and retains the exact notices; that inventory is not a complete formal SBOM or clearance to redistribute a VM/runtime image. Full runtime/image compliance still requires separate review.


Phase 3.5 adds Debian Firefox ESR (MPL-2.0 core and other components/trademark terms), GVFS/tumbler/accessibility/Xfce utilities (package-specific GPL/LGPL and other notices), Git (GPL-2.0), OpenSSH (component BSD-style/permissive), curl (curl license), Python (PSF plus components), Bash (GPL), Nano (GPL), and their dependencies through apt. Optional Node.js/npm and build profiles add MIT/BSD/GPL and other package-specific terms; they are not bundled by this repository. No new Go module dependency was introduced. Google Chrome and Microsoft Edge are proprietary optional vendor products, not included or rebranded; vendor redistribution/installer terms need separate review. Exact `/usr/share/doc/PACKAGE/copyright`, source offers and aggregate image obligations must be audited before a distributed image/release. None is published by this phase.


Phase 4 adds `golang.org/x/term` v0.40.0 (BSD-3-Clause; exact x-term.LICENSE retained), reusing pinned x/sys v0.41.0. Kitty 0.50.0 is an external GPL-3.0 terminal used from an ignored local test download, not bundled or linked into the core; bundling it would need its own full distribution review/notices/source compliance. The local fallback uses Go net/http and static original HTML/CSS/JS, no web runtime dependency. Browser acceptance optionally imports externally installed test-only websocket-client 1.9.0; it is not a shipped Go/runtime dependency. No Kitty source/assets were copied into the product.


Phase 5 adds only original Go/Python code using the standard libraries and existing pinned modules. The local package contains our CLI/helper, documentation, Go module/build dependency inventory and retained licenses. It excludes Lima, Kitty, Debian disks, installed guest packages, test browser binaries and the Go compiler. The original copyright/license notice must accompany redistribution; do not remove mixed upstream notices. Python/Pillow/websocket/ffmpeg remain optional externally installed acceptance tools, not product components. A broader formal SPDX/CycloneDX and source-obligation audit remains before image/runtime bundling. No such image or binary release is published here.


The desktop-quality evaluation installs Ubuntu/GNOME, PipeWire, GStreamer and normal desktop applications only in an isolated research guest. The original experiments/gnome probe uses external D-Bus APIs and distro GI bindings; no upstream GNOME implementation/assets are copied into the core and no new Go dependency is added. Its use of a private Mutter API is experimental. Full desktop image redistribution still requires component/source/trademark review; no Ubuntu/Fedora image or proprietary browser is bundled or published. The CLI default remains Debian/Xfce.


Phase 5.5 installs Debian's Greybird 3.23.3 theme (Shimmer project, dual GPL-2.0-or-later / CC-BY-SA-3.0-or-later) and librsvg2-common for SVG rendering (package/component terms, including LGPL). No upstream theme source or artwork is incorporated into our original code; packages remain external signed APT installations. The original geometric wallpaper in internal/lima/assets/wallpaper.svg is Apache-2.0. Retain Adwaita and Noto/DejaVu package notices and inspect exact Debian copyright/source obligations before image redistribution. Experimental Plasma/Qt/Breeze packages likewise have component-specific GPL/LGPL/artwork terms; they are not bundled or a second supported production image. See the official sources and limits in docs/workstation.md.

Phase 6 optional provider integration: E2B Desktop Python SDK 2.7.2 declares MIT ([source and dependency ranges](https://github.com/e2b-dev/E2B/blob/main/packages/desktop-python/pyproject.toml)); it is not bundled or installed automatically. Complete transitive pinning/license review remains pending before distribution of an SDK environment. Our adapter is original Apache-2.0 code. The managed E2B service has separate [terms](https://e2b.dev/terms), including commercial access/resale restrictions; SDK licensing is not hosted-service authorization. No provider image or hosting infrastructure is redistributed in this phase.
