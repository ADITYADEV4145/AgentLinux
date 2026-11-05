# ADR 0008: Keep Debian/Xfce and the existing Go stack

Date: 2026-11-05. Status: accepted for Phase 3.5; measured viewer decisions remain Phase 4.

Keep Debian 13 ARM64/Xfce/Xvfb, the Lima VZ backend, Go CLI/guest/common machine interface and official Go MCP SDK. Extend the recipe with Firefox ESR and a small development baseline; expose existing desktop applications and explicit host-operator guest package maintenance. Do not grant MCP/desktop `agent` sudo. Keep signed Debian repositories; vendor browsers remain optional with explicit architecture/license limits. Existing persistent machines are not silently reprovisioned. Protocol 2.0 remains; helper 0.3.5 and legacy 0.2.0 are accepted, with capabilities/readiness reporting differences documented.

## Language recommendation

| Component | Recommendation and reason |
| --- | --- |
| CLI/lifecycle/config | Keep Go: working locking/state/recovery/tests, straightforward macOS build and Linux ARM64 cross-compilation; rewriting adds migration risk without a measured bottleneck |
| macOS virtualization | Keep external Lima/VZ integration. If native platform work becomes necessary, use a narrow Swift/Objective-C platform layer where Apple APIs fit; Go/Rust FFI do not remove macOS entitlements, deployment or API-lifecycle work |
| Linux guest service | Keep Go: bounded protocol/process/cgroup code already validated on real Linux; static ARM64 build and existing cancellation tests. Audit parsers/process boundaries rather than assume a language contains malicious guest applications |
| MCP | Keep official Go SDK and current adapter: actual SDK/Codex evidence and maintained protocol negotiation; changing languages would require revalidating clients and semantics |
| Terminal viewer | Begin Go with bounded existing PNG/control operations. Measure rendering, subprocess/SSH, encode/serialization and queue costs before choosing a codec/native module. Neither Rust nor Go establishes achieved FPS |
| Future cloud control plane | Prefer Go if implemented: reuse contracts, networking/concurrency/observability experience. No cloud backend or performance evidence exists yet |

Go uses garbage collection and can have allocation/GC latency costs; bound buffers, avoid duplicate image copies, profile with CPU/heap/runtime traces and measure tail latency. Rust offers ownership/borrowing, deterministic resource lifetimes and no tracing GC, with useful low-level libraries; it also adds ownership/async/FFI integration work, target/toolchain packaging and a second operational stack for this small team. Rust can be a sensible isolated measured-hot-path implementation, but unsafe FFI/native libraries still require audits. Both offer memory-safe high-level code; Go race tests and Rust's type system address different failure classes, neither replacing lifecycle/authorization tests.

The repository is primarily Go, with shell provisioning, Python acceptance and a read-only Swift host-observation fixture. We have not built our own Swift virtualization engine or Rust viewer. Existing real end-to-end behavior is a stronger reason to keep this stack than hypothetical language microbenchmarks. Likely bottlenecks are screenshot subprocess/PNG work, SSH startup/round trips, byte copies, terminal transport and browser memory/CPU; measure phase timings, allocations/RSS, CPU, frame age and bounded-queue behavior on actual Macs before a language change. No language rewrite is essential to this phase.

References: [Go FAQ and design tradeoffs](https://go.dev/doc/faq), [Rust target support](https://doc.rust-lang.org/rustc/platform-support.html), [desktop comparison](../desktop.md), [browser sources](../browsers.md), [verification](../verification.md). The recommendation is an engineering decision based on this codebase, not a claim that one language is universally faster or safer.
