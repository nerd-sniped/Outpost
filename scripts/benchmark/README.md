# llvmpipe benchmark (Phase 1.5) — the plan's main measurement

**The one methodology rule:** measure perceived FPS *in the browser with the stream
active*, not FreeCAD-side render FPS. Rendering and Selkies' software encode contend
for the same vCPUs, so FreeCAD-only numbers overstate reality by whatever the encoder
steals. `1.2`'s encoder-CPU number quantifies that steal.

## Standard models — FreeCAD's own shipped examples

No need to source or generate assemblies: FreeCAD ships real, version-matched examples
at `/opt/freecad/usr/share/examples/` (all verified to open clean in 1.0.2). Use these
as the standard tiers so the benchmark is reproducible on every FreeCAD bump:

| Tier | File | Complexity | Notes |
| --- | --- | --- | --- |
| ~50-part | `AssemblyExample.FCStd` | 53 objects / 52 solids | a genuine assembly |
| heavy (>200) | `BIMExample.FCStd` | 361 objects / **1891 solids** | the make-or-break FPS case |
| mechanical | `EngineBlock.FCStd` | 36 / 31 | dense B-rep; TechDraw/PartDesign |
| tiny | `PartDesignExample.FCStd` | 16 / 5 | round-trip / visual-diff / checkpoint smoke |

`AssemblyExample`, `EngineBlock`, and `PartDesignExample` are also committed to the
test repo under `fixtures/` for edit→commit→diff→checkpoint cycles.

## What to run

On ~4 vCPU, no GPU (the Railway-representative case):

1. Run the image, open `http://localhost:3000`.
2. File → Open `AssemblyExample.FCStd` (50-part tier), then `BIMExample.FCStd` (heavy).
3. Run `freecad_bench.py` from FreeCAD's Python console for the FreeCAD-side orbit FPS
   and TechDraw regen — the *upper bound*. Then record the **browser-perceived** FPS
   with a screen capture while orbiting; that is the number that decides the messaging
   (rendering + Selkies software-encode contend for the same vCPUs).

## Decision (messaging only — the project ships in every branch)

| Browser orbit FPS on 200-part | Railway template messaging                          |
|-------------------------------|-----------------------------------------------------|
| ≥ 20 fps                      | Story stands as written.                            |
| 10–20 fps                     | "Small/medium assemblies" framing.                  |
| < 10 fps                      | GPU becomes a documented requirement; light-use rung. |

Record raw numbers in `RESULTS.md` next to this file, re-run on every FreeCAD bump.
