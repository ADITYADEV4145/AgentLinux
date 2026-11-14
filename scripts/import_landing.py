#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Import the owner's Framer export for the embedded localhost site, without analytics.
No network access. Raw export is never modified. Run from a reviewed source checkout.
"""
import argparse
import re
from pathlib import Path


def transform(text):
    text = re.sub(r'(?:\.\./)+framerusercontent\.com/', '/framerusercontent.com/', text)
    text = text.replace('../unique-structure-158589.framer.app/index.html', '/')
    text = text.replace('https://unique-structure-158589.framer.app', '')
    text = text.replace('FIRST-MONTH LAUNCH OFFER', 'SELECTED PRICES · NOT YET AVAILABLE')
    text = text.replace('Launch pricing ends one month after public launch.', 'Cloud pricing preview — not on sale.')
    text = text.replace('Annual plans remain available after the offer ends. Lock in the listed launch price for your first year, paid upfront. The post-promotion annual price has not been announced.', 'These are owner-selected planning prices. Cloud access, paid checkout and subscriptions are not active. Start with the free local Mac edition.')
    text = text.replace('No dashboard backend is connected to this website. Your cloud workspace is designed for separate Linux desktops, with readiness, runtime, viewers, control handoff, usage, and explicit stopping in one place.', 'Open the local console from this website. Hosted cloud desktops remain in development; no paid cloud backend or checkout is available here.')
    text = text.replace('Explore cloud plans', 'Open local console')
    text = text.replace('CLOUD PLANS · FIRST-MONTH LAUNCH OFFER', 'CLOUD PLANS · NOT YET AVAILABLE')
    text = text.replace('Cloud access is available through the service. The live dashboard and checkout links have not yet been connected to this website.', 'Cloud is in development. Paid access, subscriptions and checkout are not available. Use the local Mac edition today.')
    text = text.replace('A real Linux desktop for your agent. In the cloud or on your Mac.', 'A real Linux desktop for your agent on your Mac. Cloud support is in development.')
    text = text.replace('A dedicated cloud workspace: 2 vCPU, 4 GiB RAM, and 30 machine-hours refreshed each month.', 'Planned cloud workspace: 2 vCPU, 4 GiB RAM, and 30 machine-hours refreshed each month.')
    text = re.sub(r'eom8bDQh6gZMIkmm0Gyn3fClog(?:_2)?\.gif', 'eom8bDQh6gZMIkmm0Gyn3fClog_1.gif', text)
    return text


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('export', type=Path)
    args = p.parse_args()
    repo = Path(__file__).resolve().parents[1]
    source = args.export.resolve()
    target = repo/'internal/console/landing'
    home = source/'unique-structure-158589.framer.app/index.html'
    if source.is_symlink() or not home.is_file():
        p.error('expected the supplied Framer export directory')
    for path in sorted((source/'framerusercontent.com').rglob('*')):
        if path.name.startswith('.') or path.is_dir(): continue
        if path.is_symlink(): p.error('export contains a symlink')
        if path.name in ('eom8bDQh6gZMIkmm0Gyn3fClog.gif', 'eom8bDQh6gZMIkmm0Gyn3fClog_2.gif'): continue
        dest = target/path.relative_to(source)
        dest.parent.mkdir(parents=True, exist_ok=True)
        if path.suffix in ('.mjs', '.json', '.html'):
            dest.write_text(transform(path.read_text()))
        else: dest.write_bytes(path.read_bytes())
    html = transform(home.read_text())
    # Render with the exported client router; relocated SSR asset URLs otherwise
    # produce React hydration mismatches. Retain SSR markup as a no-JS fallback.
    html = re.sub(r"data-framer-hydrate-v2=(?:\"[^\"]*\"|'[^']*')", '', html)
    # Drop the analytics loader and on-page editor bootstrap, not the layout/runtime.
    html = re.sub(r'<script[^>]*src="[^"]*events\.framer\.com[^>]*></script>', '', html)
    html = re.sub(r'<script>try\{if\(localStorage\.getItem\("__framer_force_showing_editorbar_since"\)\).*?</script>', '', html, flags=re.S)
    html = html.replace('</head>', '<link rel="stylesheet" href="/landing-style.css"><script src="/landing-bridge.js"></script></head>')
    target.mkdir(parents=True, exist_ok=True)
    (target/'index.html').write_text(html)
    print('Imported landing design and local assets; analytics loader omitted; cloud claims labeled pending.')


if __name__ == '__main__': main()
