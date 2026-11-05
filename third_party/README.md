# Third-party licenses

Original AgentLinux code is Apache-2.0 (root LICENSE). No terminal-mac, vmette, Vitrine, Tart, E2B, or Daytona source/assets have been incorporated.

- `go.yaml.in/yaml/v3` v3.0.4 is the only external runtime Go module. Its upstream LICENSE covers libyaml-derived files under MIT and remaining files under Apache-2.0; exact text is retained in go-yaml.LICENSE. Source: https://github.com/yaml/go-yaml/tree/v3.0.4.
- Go 1.26.1 standard-library license is retained in go.LICENSE. Toolchain third-party notices also apply when redistributing toolchain components; we do not bundle the compiler.
- `gopkg.in/check.v1` is a dependency module's test dependency in go.sum, not linked into our shipped commands.
- Lima 2.2.1 is an external user-installed executable, Apache-2.0. Development runtime downloads are ignored and not bundled. Releases that bundle Lima must retain its complete license/notice inventory and audit its SBOM.
- OpenSSH is supplied by the host and guest OS, with its own component licenses.
- Debian image packages retain their individual licenses; the image is downloaded at runtime, not committed or redistributed here. A redistribution review and corresponding-source obligations remain necessary before publishing a guest image.

The local build currently produces development binaries only, not a cleared release package. License copies here are notices, not an assertion that a future binary/image bundle has passed a complete license audit.
