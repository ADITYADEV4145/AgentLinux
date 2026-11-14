# Named local machines

One interactive command still starts/resumes the default Linux machine. Names
select independent private Lima/VZ namespaces, disks and guest services:

```sh
agent-linux start --machine work
agent-linux start --machine research
agent-linux machines list --json
agent-linux status --machine work --json
agent-linux mcp-config --machine work --client codex
agent-linux stop --machine research
agent-linux dashboard --local
```

`start` waits for real guest readiness and opens the existing interactive Kitty
viewer or live browser fallback. Closing a viewer/dashboard leaves Linux running.
Each machine has one desktop; human/agent ownership is still per-machine.
No VM is automatically restarted, reset or restored.

Names are 1–32 lowercase letters, digits and hyphens, starting with a letter.
A private version-1 catalog maps them to stable random identities and owned
state directories. New nondefault names use short generated directories under
`~/.local/share/agent-linux/m/`; names are never used as filesystem paths.
The default remains `~/.local/share/agent-linux/local`. No existing disk is moved.
Names, namespaces and IDs must be unique; socket paths must fit Lima's limit.

All local operations share the resolver, including file transfer, software
commands, viewer and MCP configuration. `--machine` conflicts with `--state-dir`.
Pass selectors before command arguments (`packages install --machine work jq`).
Guest argv after `exec --` is never parsed as host selection. Explicit legacy
`--state-dir` workflows remain valid; registered paths receive the same budget
and identity checks. Unregistered namespaces are outside the managed allocation
budget until explicitly registered:

```sh
agent-linux machines register --name existing --state-dir /absolute/private/namespace
```

Registration requires an existing owned namespace and valid machine state. It
never copies disks. Explicit registration of the same name/path also repairs a
stale catalog ID after an interrupted explicit reset; this intentionally binds
new connections to the currently stored identity. Already-open viewers and MCP
servers retain their prior ID and fail instead of following a replacement.
Successful CLI reset/deletion updates the catalog; reusing a deleted name rotates
its identity. Printed MCP configurations contain the exact path and expected ID,
so reconnect/reprint explicitly after reset or replacement.

## Managed resource budgets

Admission is serialized across processes. A durable reservation covers startup
before the VM exists; live process reservations and reconciled runtime allocations
are counted. Crashed reservations with no remaining VM are reclaimed on subsequent
admission. Unknown runtime state fails closed. The per-machine lifecycle lock
prevents two processes starting the same machine concurrently.

Defaults: 50% of host RAM, and host logical CPUs minus two (minimum one).
Explicit overrides must leave at least the greater of 4 GiB or one-third of host
RAM and cannot exceed host CPU count:

```sh
agent-linux machines budget --cpus 8 --memory 12 --json
```

Zero values select defaults. Lowering a budget never terminates existing VMs;
new admission is refused until the managed allocation fits. Existing per-machine
validation and disk-space checks also apply. Budgets are allocation policy, not
CPU scheduling, a memory-pressure guarantee or an inventory of every VM/process
on the Mac. A private catalog is a cooperative control mechanism for this owner,
not a security boundary against other programs running as the same macOS user.

`--catalog-dir /absolute/private/path` is an advanced explicit catalog/test
isolation option. New named directories are generated beside that catalog.
Separate catalogs have separate budgets; they do not establish a global host cap.
Do not register one namespace into multiple catalogs to bypass admission.

## Dashboard

`dashboard` defaults to local mode without a cloud config. The authenticated
loopback interface lists known active allocations, CPU/RAM budget and readiness.
It can create a named machine with validated CPU/RAM/disk settings, queue a bounded
startup, start/stop existing machines and open viewers on demand. Inventory does
not capture screenshots. A maximum of four viewer servers and 64 job records are
kept per dashboard; restart the dashboard to release those slots. Closing it closes
its viewers, not the VMs. An interrupted startup may leave Linux running: inspect
status before retrying. Stop asks explicitly and preserves saved guest files;
unsaved application buffers can be lost.

The capability URL authorizes this local dashboard. Exact host/origin checks,
bounded bodies, private IDs and existing guest key verification remain required.
No host path can be supplied through the web API. The frontend never receives SSH
keys or provider credentials. Resource shortages and failed readiness remain
visible; mutations are not automatically replayed. Local usage needs no payment
account. Cloud mode is separate and remains experimental.

For stopped disk checkpoints, recovery and helper upgrades, see [checkpoint guide](checkpoints.md). These operations require explicit user actions and are not autonomous MCP tools.
