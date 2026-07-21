# llvmpipe benchmark — results

Phase 1.5 measurement. Re-run on every FreeCAD bump. **Read the caveats** — the number
that gates the Railway messaging is the *constrained* one, not the local baseline.

## Run 1 — local baseline (2026-07-21, FreeCAD 1.0.2)

**Verdict: comfortably usable; llvmpipe is not a dealbreaker.** Both the ~50-part
assembly and the heavy 1891-solid BIM model orbit/zoom/pan responsively.

| Model | Objects / solids | Perceived result (browser, stream active) |
| --- | --- | --- |
| `AssemblyExample.FCStd` | 53 / 52 | responsive, very usable |
| `BIMExample.FCStd` | 361 / **1891** | responsive, still very usable |

- Stream: ~80 FPS; input-to-photon latency ~16–35 ms typical, still responsive at ~60 ms.
- Against the plan's kill/branch criteria (≥20 fps → story stands as written): **passes
  by a wide margin.**

**Caveats that keep this honest — this is NOT the Railway tier:**

- **Hardware:** the container saw **24 vCPUs**, unlimited RAM (host: 24 CPU / 64 GB).
  Railway's target is ~4 vCPU, no GPU. This baseline over-states the Railway case by
  whatever headroom those extra cores provide.
- **Network:** measured over **localhost** — ~0 WAN latency. Railway adds real internet
  RTT on top of the input-to-photon numbers above.
- So this run answers "does the architecture work and feel good at all?" (yes,
  emphatically) — not "how does the Railway free tier feel?" That needs Run 2 (below)
  for compute and Phase 4.3 for the network + real-bill half.

## Run 2 — constrained to ~4 vCPU (Railway-compute proxy), 2026-07-21

Container hard-capped at 4 CPUs (`docker update --cpus=4`, `cpu.max: 400000 100000`),
still localhost. Isolates Railway's compute constraint from its network constraint.

**Verdict: passes clearly — the Railway story holds on compute.** Even the heavy
1891-solid `BIMExample` stayed very usable at **75+ FPS** under the 4-core cap; the
50-part `AssemblyExample` likewise.

**CPU is bursty (on-demand rendering), so a single number misleads:**

- FreeCAD renders only while the view moves; Selkies encodes only on screen change.
- Peak during an active orbit-drag: ~3.5–3.6 cores (observed ~15% of the 24-core host
  in Task Manager) — i.e. it *will* use most of the 4-core budget in bursts.
- Windowed average (15 s, cgroup `usage_usec`, imperfectly-continuous orbit): ~0.39
  cores (~10% of the cap) — the gaps between drags dominate the average.
- Idle: ~0.1 cores. Good for scale-to-zero economics — a static scene barely streams.

**Implication for the template:** at 4 vCPU the *compute* is comfortable even on the
heavy model, so "small/medium assemblies" framing is not needed on compute grounds.
The open question is no longer llvmpipe — it's **egress + WAN latency on real Railway**
(Run 3 / Phase 4.3). Sustained-load encoder CPU is best measured there, under a real
continuous session, not via remote sampling of a local container.

## Run 3 — real Railway (Phase 4.3) — PENDING

The only run with both constrained compute *and* WAN latency *and* a real egress bill.
