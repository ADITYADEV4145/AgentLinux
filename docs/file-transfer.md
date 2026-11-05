# Explicit one-way file transfer

`copy-in` and `copy-out` use the existing pinned guest control channel. They do not mount a host directory or enable SCP/admin SSH. Each operation copies one **regular file up to 4 MiB**, using 96 KiB chunks, bounded requests, a five-minute default total deadline and SHA-256 verification. Hidden/reserved upload basenames are rejected (rename deliberately before copying if needed). Files are mode 0600 with no ownership/execute metadata copied. The guest service advertises `files` from version 0.5.0; older 0.3.5/0.2.0 guests still support their existing operations but need an explicitly created/reset guest for copying. No new MCP host-filesystem tools are added.

```sh
# All local paths must be absolute, clean and have no symlink components.
agent-linux copy-in --source /absolute/project.tar --json
# Result: /home/agent/Transfers/<unique-transfer-id>/project.tar
agent-linux copy-out --source /home/agent/results.txt --output /absolute/new-results.txt --json
```

Use the same `--state-dir` and `--lima` as other commands, before positional arguments. Every incoming file gets a fresh transfer directory; nothing overwrites an existing guest name. Host output is written to a private temporary sibling, checksum checked, then atomically published with a no-replace link. Existing host destinations (including dangling symlinks), directories, FIFOs/devices, hard links, symlink components, traversal and oversized/changing files are rejected. Failed downloads remove their own temporary file. The guest never chooses a host output path. Guest input/screen/command access already gives an agent access to the copied guest data.

A project can be supplied as an **opaque archive**. Create it explicitly from the files you intend to share; exclude `.env`, keys, credentials, personal data, `.git`, large dependency/build trees and symlinks. Review the archive before copying. The tool never automatically extracts it on either machine. For example, a small reviewed project:

```sh
# Host shell, from the project directory; choose the files deliberately.
tar -cf /private/tmp/my-project.tar README.md src
agent-linux copy-in --source /private/tmp/my-project.tar --json
# After reviewing the printed guest path, explicitly extract only inside Linux:
agent-linux exec -- /bin/mkdir -p /home/agent/Projects/demo
agent-linux exec -- /bin/tar -xf /home/agent/Transfers/TRANSFER_ID/my-project.tar -C /home/agent/Projects/demo
```

`TRANSFER_ID` is a placeholder, not an auto-expanded command. Archive extraction inside Linux can modify guest files named by malicious entries; it is a separate explicitly granted guest command, not a safe archive-import feature. Export archives also remain opaque host files; inspect them before any manual extraction. Larger projects, recursive directory copying, automatic ignore rules, compression/sync, resume and metadata preservation are not supported in this RC.

An acknowledged mutation is never silently replayed. If a connection drops, a guest commit may have happened even if the host did not see it; inspect the guest Transfers directory before retrying. Failed uploads attempt a bounded abort of only their partial file/empty directory. Process death or disconnection can leave `data.partial`; remove that selected transfer explicitly inside the guest. At most 64 transfer directories can be created through this API (up to 256 MiB of bounded uploaded file bytes); remove unneeded guest Transfers directories explicitly to free slots. Arbitrary guest commands can fill the virtual disk, so this is not a general disk quota or adversarial same-user storage boundary.

There is no automatic cleanup of completed copies, and no source deletion. Guest edits to copied files do not affect the host source. Host changes after a copy do not update the guest. Results return only when an operator explicitly invokes copy-out; they are untrusted data, never executed by the tool. Reset/delete permanently loses guest-side copies; uninstall leaves VM data intact.
