#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Opt-in Codex agent test. Attaches to an existing disposable VM; no global config edits.

Approves only the named guest desktop tools for this invocation. Requires an
already authenticated Codex CLI. Does not grant host shell/sandbox exceptions.
Evidence can include screenshots and guest content: keep it private.
"""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('cli', 'lima', 'state-dir', 'evidence'):
        parser.add_argument('--' + name, type=Path, required=True)
    parser.add_argument('--codex', default='codex')
    args = parser.parse_args()
    os.umask(0o077)
    evidence = args.evidence.resolve()
    evidence.mkdir(parents=True, exist_ok=False)
    work = tempfile.mkdtemp(prefix='agentlinux-codex-client-', dir='/private/tmp')
    tools = ['computer_status', 'computer_screenshot', 'computer_open',
             'computer_click', 'computer_key', 'computer_type']
    command = [args.codex, 'exec', '--ignore-user-config', '--ignore-rules',
               '--ephemeral', '--skip-git-repo-check', '--sandbox', 'read-only',
               '-C', work, '--json', '-o', str(evidence / 'result.txt')]
    settings = {
        'approval_policy': 'never',
        'mcp_servers.agent_linux.command': str(args.cli.resolve()),
        'mcp_servers.agent_linux.args': ['mcp', '--state-dir', str(args.state_dir.resolve()),
                                         '--lima', str(args.lima.resolve())],
        'mcp_servers.agent_linux.tool_timeout_sec': 75,
        'mcp_servers.agent_linux.enabled_tools': tools,
    }
    for tool in tools:
        settings['mcp_servers.agent_linux.tools.' + tool + '.approval_mode'] = 'approve'
    for key, value in settings.items():
        command += ['-c', key + '=' + json.dumps(value)]
    command += [
        'Use only the agent_linux MCP tools for this acceptance test. Do not use '
        'host shell, host files, web search, or other tools. Check machine status, '
        'capture a screenshot, open the browser in Linux, visit https://example.com '
        'using guest keys/text, capture another screenshot, and tell me the title '
        'and main visible text you actually see. Observe -> act -> verify. Input '
        'frameId must come from an observed screenshot captured in the last 10 '
        'seconds; recapture on STALE_FRAME. Do not run a guest command. Stop after '
        'verifying the page or report the exact failure.'
    ]
    try:
        result = subprocess.run(command, capture_output=True, text=True, timeout=450)
        (evidence / 'events.jsonl').write_text(result.stdout)
        (evidence / 'stderr.txt').write_text(result.stderr)
        print('Codex exit:', result.returncode)
        if (evidence / 'result.txt').exists():
            print((evidence / 'result.txt').read_text())
        if result.returncode:
            raise SystemExit(result.returncode)
        print('Inspect tool events and screenshots; exit 0 alone is not proof of success.')
    except subprocess.TimeoutExpired as exc:
        (evidence / 'events.jsonl').write_bytes(exc.stdout or b'')
        (evidence / 'stderr.txt').write_bytes(exc.stderr or b'')
        raise
    finally:
        shutil.rmtree(work)


if __name__ == '__main__':
    main()
