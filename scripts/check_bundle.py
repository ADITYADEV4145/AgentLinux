#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Read-only package contract check: integrity plus first-run supporting files."""
import argparse
from pathlib import Path
import re
import install


def check(bundle):
    metadata=install.manifest(bundle)
    required={'examples/demo/project-budget.html','examples/cost-model.json','examples/billing.sandbox.json',
              'images/debian/manifest.json','scripts/cost_model.py','scripts/billing_acceptance.py',
              'internal/mcpserver/instructions.txt','internal/console/assets/pricing.json','docs/web-console.md','third_party/stripe-go.LICENSE','docs/launch-readiness.md'}
    if not required <= metadata['files'].keys():
        raise ValueError('package lacks documented first-run supporting files')
    # Deep development source links require the source checkout, as documented.
    # README and billing first-run links must work inside the local bundle.
    for name in ('README.md','docs/billing.md'):
        path=bundle/name
        for target in re.findall(r'\]\(([^)]+)\)',path.read_text()):
            target=target.split('#')[0]
            if not target or '://' in target or target.startswith('mailto:'):continue
            if not (path.parent/target).is_file():
                raise ValueError('broken bundled first-run link: '+name+' -> '+target)
    return metadata


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('bundle',type=Path)
    args=p.parse_args()
    m=check(args.bundle.resolve())
    print('Bundle integrity/supporting files/first-run links verified; commit '+m['commit'])


if __name__=='__main__':main()
