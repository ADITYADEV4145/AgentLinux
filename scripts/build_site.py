#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Build only reviewed public landing assets. Never package the local dashboard."""
import hashlib
import json
from pathlib import Path
import shutil
import tempfile

REPO = Path(__file__).resolve().parents[1]
SOURCE = REPO / 'internal/console/landing'
OUTPUT = REPO / 'dist/netlify'
DIRECTORIES = ('brand', 'fonts', 'framerusercontent.com')


def public_text(text):
    return text.replace('Open local console', 'Get started').replace('/landing-bridge.js', '/public-bridge.js')


def build(output):
    output.mkdir(parents=True, exist_ok=False)
    for directory in DIRECTORIES:
        for path in sorted((SOURCE/directory).rglob('*')):
            if path.is_symlink():
                raise ValueError('symlink in public asset tree')
            if not path.is_file():
                continue
            dest = output/path.relative_to(SOURCE)
            dest.parent.mkdir(parents=True, exist_ok=True)
            if path.suffix in ('.mjs', '.html', '.json'):
                dest.write_text(public_text(path.read_text()))
            else:
                shutil.copyfile(path, dest)
    (output/'index.html').write_text(public_text((SOURCE/'index.html').read_text()))
    shutil.copyfile(SOURCE/'style.css', output/'landing-style.css')
    shutil.copyfile(REPO/'website/public-bridge.js', output/'public-bridge.js')
    shutil.copyfile(REPO/'website/setup.css', output/'setup.css')
    (output/'get-started').mkdir()
    shutil.copyfile(REPO/'website/get-started.html', output/'get-started/index.html')
    (output/'404.html').write_text('<!doctype html><html lang="en"><meta charset="utf-8"><title>Page not found — Agent Linux</title><h1>Page not found</h1><a href="/">Return to Agent Linux</a></html>')
    notices = (REPO/'third_party/landing-export.NOTICE.md').read_text()
    (output/'third-party-notices.txt').write_text(notices+'\n'+(SOURCE/'fonts/OFL.txt').read_text())
    manifest = {str(p.relative_to(output)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(output.rglob('*')) if p.is_file()}
    (output/'.build-manifest.json').write_text(json.dumps(manifest, indent=2)+'\n')
    return manifest


def main():
    OUTPUT.parent.mkdir(exist_ok=True)
    if OUTPUT.is_symlink() or OUTPUT.parent.is_symlink():
        raise ValueError('refusing symlink output')
    if OUTPUT.exists():
        manifest = json.loads((OUTPUT/'.build-manifest.json').read_text())
        actual = {str(p.relative_to(OUTPUT)) for p in OUTPUT.rglob('*') if p.is_file()}
        if actual != set(manifest)|{'.build-manifest.json'} or any(p.is_symlink() for p in OUTPUT.rglob('*')):
            raise ValueError('unexpected files in old generated output; inspect before rebuilding')
        for name, digest in manifest.items():
            path = OUTPUT/name
            if not path.is_relative_to(OUTPUT) or '..' in Path(name).parts or hashlib.sha256(path.read_bytes()).hexdigest()!=digest:
                raise ValueError('modified generated output; refusing to overwrite')
    with tempfile.TemporaryDirectory(prefix='.site-',dir=OUTPUT.parent) as stage:
        ready=Path(stage)/'ready';manifest=build(ready)
        if OUTPUT.exists():
            OUTPUT.rename(Path(stage)/'previous')
        ready.rename(OUTPUT)
    print(f'Public static site: {len(manifest)} files, {sum((OUTPUT/p).stat().st_size for p in manifest):,} bytes. No local APIs or machine state.')


if __name__ == '__main__':
    main()
