# Reproducible local demo

This script uses a real local Linux guest and an image-capable MCP agent. Cloud is in development and is omitted. Do not show simulated frames as cloud Linux. Use synthetic data, an isolated machine namespace and the approved tool/runtime versions; initial downloads and model usage can consume bandwidth or an existing model allowance.

## Short presentation script

1. Say: “Give your agent a Linux computer. This demo runs locally on an Apple silicon Mac; our cloud backend is still a prototype.”
2. In terminal A run `agent-linux start --view-only` (add the reviewed `--lima` path when needed). Wait for the actual desktop, not just a spawned process. Unsupported terminals show a private live loopback URL. Leave this viewer running.
3. In terminal B run `agent-linux mcp-config --client codex` and connect the chosen agent using [MCP setup](mcp.md). Review permissions; this grants powerful guest access. The command only prints configuration.
4. Copy the synthetic form into Linux: `agent-linux copy-in --source "$PWD/examples/demo/project-budget.html" --json`. Read its returned guest path. This is a one-way regular-file copy, not a host mount.
5. Ask: “Check the Linux machine and take a screenshot. Open its browser at file://RETURNED_GUEST_PATH. Use the visible form to estimate 6 hours at $45/hour plus $30 expenses. Press Calculate, capture the changed result, and tell me the amount you actually see. Use fresh screenshots before and after actions.” Replace the path with the exact returned guest path. Expected arithmetic is $300.00, but report a result only when observed.
6. Show the live desktop changing, the form's displayed result and the agent's response. There is no payment, checkout, external website or real customer data in this form. If the action fails, show the error and do not present the expected number as an observed outcome.
7. Exit a view-only terminal with q/Ctrl+C, then run `agent-linux stop` in terminal B. Explain that exiting the viewer alone keeps Linux running and stopping preserves guest files. For a disposable machine, inspect `delete --machine-id ID`, then add `--yes` only deliberately after stopping.

For a human turn, use `start` or `view --interactive`: F2 gives/takes input, Ctrl+] exits, and Ctrl+C/q are guest input. Give the agent control before file copy or MCP mutations. Screenshot coordinates remain original 1280×800 PNG pixels, not preview coordinates. No Mac pointer or clipboard operation is part of this flow.

## Exact opt-in installed-product acceptance

From a reviewed source checkout, build the package and provide its complete bundle, reviewed runtime, Kitty and a NEW private evidence directory:

```sh
python3 scripts/launch_acceptance.py \
 --bundle /absolute/reviewed/agent-linux-0.5.0-rc.1-darwin-arm64 \
 --lima /absolute/reviewed/limactl \
 --kitty /absolute/kitty.app/Contents/MacOS/kitty \
 --kitten /absolute/kitty.app/Contents/MacOS/kitten \
 --agent --evidence "$PWD/artifacts/phase8/my-new-run"
```

Requires an unlocked supported Mac, Python/Pillow, an already authenticated Codex CLI, adequate disk/RAM and normal image/APT networking. `--image /absolute/pinned-base.qcow2` explicitly uses a verified cached public base and labels the measurement accordingly. `--chrome /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome` optionally exercises the secured web fallback, real human widgets and capture-age metrics using an isolated browser profile. Neither flag accesses a personal VM/profile. The harness creates only its own temporary VM/tools/Kitty socket, installs the synthetic page inside Linux, runs the real model through six approved guest MCP tools, verifies the changed guest title, measures lifecycle/capture/viewing, verifies persistence/reset, deletes its exact selected machine and uninstalls its temporary tools.

Without `--agent`, the harness explicitly uses a deterministic official SDK client and does not claim an AI-agent demo. Raw screenshots, model output, paths, recordings and private viewer links remain in ignored `artifacts`; review before any sharing. Window capture timestamps are preserved. No recording should be accelerated or relabeled as measured full-motion desktop performance. Publication/upload requires a separate decision. See [performance](performance.md) and [verification](verification.md) for actual observations.
