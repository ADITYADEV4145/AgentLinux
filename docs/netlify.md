# Public website on Netlify

Public landing: **https://agent-linux.netlify.app**.
Production deployment verified 2026-11-14 (America/Chicago), deploy
`6af8063452f7f7c04fb078cc`:
https://6af8063452f7f7c04fb078cc--agent-linux.netlify.app.

This is the marketing website and local developer setup guide. It does not host
a VM control plane, cloud desktops, local dashboard, paid checkout or subscriptions.
Get started links to public setup instructions. `/dashboard` redirects to those
instructions; use `agent-linux dashboard --local` on your Mac for VM controls.
Pricing remains explicitly a preview, not on sale. No live billing, custom domain,
DNS changes, paid add-on or downloadable binary release was activated.

## Build and publish

No npm dependency installation is needed for the site build:

```sh
python3 scripts/build_site.py
python3 -m unittest discover -s scripts -p test_build_site.py
netlify deploy --prod --no-build --dir dist/netlify --site agent-linux
```

Authenticate through Netlify's official login flow if needed. Never put a token in
Git, a web page or a chat message. `.netlify/` is ignored; it contains local project
association, not public website source. The initial deployment used the existing
signed-in account and created a new project, leaving other projects unchanged.
Continuous deployment from Git has **not** been connected; pushing to GitHub does
not automatically update this site. Run the explicit deploy command after building
reviewed changes, or configure Git integration separately.

`netlify.toml` declares the build command, public output directory, pricing rewrite,
dashboard redirect and security headers. Unknown paths retain genuine 404s;
there is no catch-all rewrite that makes missing assets or APIs look successful.
See Netlify's official [configuration reference](https://docs.netlify.com/build/configure-builds/file-based-configuration/).

The builder allowlists only reviewed landing asset directories and original
public integration/setup files. It never copies the repository root, `.env`, VM
state, guest disks, private evidence, Go dashboard assets or local capabilities.
Repeat builds are deterministic. Unexpected or modified files in previous output
cause a refusal instead of being silently removed or uploaded. Generated output
is ignored. Dependencies/Framer export retain their original notices; the owner's
supplied logo is unchanged. Review [landing notices](../third_party/landing-export.NOTICE.md)
when changing the exported design or its distribution.

## Verification

- Netlify CLI reported successful production publication with the URL above.
- Live browser checks: landing hero, original logo, local Manrope font, public
  Get started navigation and actual setup page; pricing route and monthly selector.
- Desktop 1440px and mobile 390px layouts checked without horizontal overflow.
  Private screenshots remain under ignored artifacts, not in Git.
- Public bundle/allowlist and deterministic rebuild/overwrite-refusal tests passed;
  JS syntax and Git whitespace checks passed. The new Python tests are in `make test`.
- HTTP probes verify route status, security headers and missing local API/state
  paths. No guest desktop operation was exposed through the public site.

The local Go application's authenticated server remains a separate deployment
boundary. Hosting the landing page does not establish cloud product readiness,
production security, payment availability or broader Mac compatibility.
