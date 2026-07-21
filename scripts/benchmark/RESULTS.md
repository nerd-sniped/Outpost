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

## Run 2 — constrained to ~4 vCPU (Railway-compute proxy) — PENDING

Same two models, container limited to 4 CPUs (`docker update --cpus=4 outpost`), still
localhost. Isolates the compute half of the Railway story from the network half.
Record perceived FPS here; this is the number that actually decides the template
messaging tier.

## Run 3 — real Railway (Phase 4.3) — PENDING

The only run with both constrained compute *and* WAN latency *and* a real egress bill.
