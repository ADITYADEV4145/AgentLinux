# Disk checkpoints and explicit recovery

These are application-managed **disk-only checkpoints**, not VM memory snapshots.
They save the writable raw disk, EFI variable store and matching sealed provisioning
and state metadata for the **same** named machine. They preserve saved files and
configuration, not running processes or unsaved application buffers. No operation
silently stops a VM. Nothing is exposed as an autonomous MCP recovery tool.

## Commands

With the selected VM already stopped:

```sh
agent-linux checkpoint create before-update --machine work
agent-linux checkpoint list --machine work --json
```

Three checkpoints maximum per machine, including incomplete creation attempts.
Additional creation is refused; older checkpoints are never silently removed.
Creation can take time even when APFS cloning succeeds because every file receives
a SHA-256 integrity check. It is not promised instantaneous or space-free.

Restore first prints a plan and changes nothing without explicit confirmation:

```sh
agent-linux checkpoint restore before-update --machine work
# Review the printed machine identity and data-loss warning, then:
agent-linux checkpoint restore before-update --machine work --machine-id EXACT_ID --yes
agent-linux up --local --machine work
```

Restore stages and validates files while stopped. It does not automatically boot.
Changes made after the checkpoint are replaced. The pre-restore disk/configuration
remain as private rollback data until an explicit boot completes the actual guest
handshake. A failed boot retains that data. Stable machine identity is preserved;
service session and desktop generation are freshly established at boot, so earlier
observations/input leases do not remain valid.

To delete just one checkpoint, review its plan, then supply the exact ID and `--yes`:

```sh
agent-linux checkpoint delete before-update --machine work --machine-id EXACT_ID --yes
```

Deleting a checkpoint does not reset the VM and may be done while it is running,
unless a recovery operation is pending. Reset/deletion of the machine removes all
its checkpoints and recovery data; the existing destructive plan states that scope.
Checkpoints are not an independent backup against host disk failure or machine deletion.

## Interrupted operations

A private durable journal records staging, installation, rollback and waiting for
readiness. Unfinished installation/helper metadata blocks boot and guest operations;
status reports the recovery condition without rewriting staged state. No automatic
VM restart, reset or rollback is performed.

```sh
agent-linux checkpoint list --machine work --json
# VM must be stopped; choose exactly one explicit action:
agent-linux checkpoint recover --mode resume --machine work --machine-id EXACT_ID --yes
agent-linux checkpoint recover --mode rollback --machine work --machine-id EXACT_ID --yes
```

Resume finishes an interrupted restore installation. Rollback reinstalls the
pre-operation files. Interrupted staging can only be explicitly discarded through
rollback because originals have not yet changed. After either installation path,
start explicitly; rollback remains until verified readiness.

After verified readiness, a durable confirmation receipt permits interrupted
backup cleanup to be resumed even if the recovery journal was already removed.
At this point rollback has been released; `resume` finishes cleanup. A malformed
manifest, receipt, unsafe path, unknown file or incompatible identity fails closed;
inspect the private namespace rather than deleting files by guesswork.

Interrupted checkpoint creation is listed as incomplete and counts toward the
limit. Explicitly delete it by its listed name, then retry creation. A crash before
its first manifest commits is listed as `interrupted-…` with a generated name.
This deletes only the identified incomplete checkpoint tree, not the live disk.

## Graphical session restart

```sh
agent-linux desktop restart --machine work --machine-id EXACT_ID --yes
```

The plan warns about unsaved work. This uses guest administration to restart the
managed X11 session in the running VM, waits for actual readiness and checks for a
new desktop generation. It does not restart the VM or send input to the Mac.

## Explicit guest-helper upgrade

```sh
agent-linux stop --machine work
agent-linux helper upgrade --machine work --machine-id EXACT_ID --yes
```

This creates `before-helper-…` (subject to the same three-checkpoint limit), stages
rollback data, replaces only the embedded helper payload in both original and
runtime provisioning templates, updates the seal and records matching host state.
It then explicitly boots and verifies the installed guest binary's SHA-256 through
the pinned authenticated channel before releasing rollback data. The machine ID,
SSH identity, project files, resource configuration and desktop recipe are retained.
The normal existing boot provisioning still runs; this is not a distribution,
package, desktop-recipe or arbitrary protocol migration.

`--guest-binary /absolute/helper` deliberately selects a bounded static Linux ARM64
helper; the default is the helper beside the CLI. Unsupported old template shapes,
invalid binaries, failed provisioning or digest mismatch produce an error and
preserve recovery data. An interrupted helper metadata update requires explicit
rollback; resume is not a blind re-execution of the upgrade. Host/guest protocol
compatibility must be supported by the running host CLI. Existing machines are
never upgraded automatically.

## Storage, compatibility and trust

- Verified layout: Lima **2.2.1**, VZ Linux ARM64, raw `disk`, `vz-efi`, and the
  selected machine's original/effective provisioning, seal and state. Other disk
  containers (including ASIF/qcow backing chains) are refused until validated.
- Runtime version, exact machine identity, VZ instance identifier and CPU/RAM/disk/
  image contract are pinned in compatibility metadata. These are not machine clones
  and cannot be imported into another namespace or VZ instance.
- APFS file cloning is attempted using the open source descriptor. Otherwise a
  bounded sparse copy uses at most two 1-MiB buffers and checks conservative free
  space (file logical size plus 64 MiB headroom) before writing. Cancellation stops
  the copy; partial state stays identifiable. Free-space checks are not reservations;
  later writes and unrelated host activity can still exhaust storage.
- File data and directory changes are synced before journal phase transitions.
  Replacement uses atomic per-file renames; multi-file atomicity comes from the
  durable journal and explicit resume/rollback. Original rollback files are never
  consumed by installation. Hashes detect accidental corruption, not tampering by
  another program with the same macOS user's access.
- Source/destination boundaries reject traversal, symlink ancestors and unsafe
  hard links. Metadata is private, bounded and schema-checked. Checkpoint trees are
  under the private owned machine directory; unknown entries prevent destructive
  cleanup. Copies may contain project data and embedded guest SSH server keys:
  treat them as private. They do not include the host's unrelated files or clipboard.
- The lifecycle lock coordinates Agent Linux processes. Do not separately control
  its private Lima namespace or edit disk files while operating on checkpoints.

## Local dashboard

`agent-linux dashboard --local` exposes the same operations behind the existing
loopback capability and exact machine selection. **Recovery** lists checkpoints,
shows action-specific warnings, and requires explicit consent for destructive or
restart/upgrade actions. Long operations run as bounded dashboard jobs, without
replay. It accepts no arbitrary host path; web helper upgrade uses only the bundled
helper. Closing the dashboard does not stop the VM; interrupted jobs remain
inspectable/recoverable. The Netlify landing site contains none of these APIs.

See [verification](local-hardening-verification.md) for actual Mac evidence and
remaining platform limits.
