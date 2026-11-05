# ADR 0001: Integrate Lima/VZ first

Status: accepted for the first implementation, subject to Phase 1 compatibility gates.

Use a pinned Lima 2.2.1 executable with `vmType: vz`, `plain: true`, a project-owned LIMA_HOME, no inherited desktop template, and explicit guest image/configuration. Write the CLI/broker in Go. Do not write a Swift VM manager in the first slice.

Apple provides the VM devices and native VM view APIs, but we would still own installation, image conversion, signing, recovery, and lifecycle integration. Lima already provides image boot/provisioning and SSH. Plain mode disables the convenience integrations we do not want. A separate backend interface serves both the Lima implementation and meaningful test doubles; cloud has no fake implementation.

Cost: an external executable, runtime version compatibility, and explicit verification of effective configuration. No promise of a single-file install or a native view of our Xvfb session. Do not silently fall back to QEMU. If plain-mode ARM64 boot/control fails, first fix the pinned config or select another tested Lima release; reopen this ADR before replacing the runtime.

Evidence and comparisons: [architecture](../architecture.md), [sources R1–R4](../research.md).
