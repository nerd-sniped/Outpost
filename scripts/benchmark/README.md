# llvmpipe benchmark (Phase 1.5) — the plan's main measurement

**The one methodology rule:** measure perceived FPS *in the browser with the stream
active*, not FreeCAD-side render FPS. Rendering and Selkies' software encode contend
for the same vCPUs, so FreeCAD-only numbers overstate reality by whatever the encoder
steals. `1.2`'s encoder-CPU number quantifies that steal.

## What to run

On ~4 vCPU, no GPU (the Railway-representative case):

1. Build + run the image, open `http://localhost:3000`.
2. Load a representative assembly — one ~50-part, one ~200-part. Drop the files in
   `scripts/benchmark/models/` (gitignored) or clone a repo with them.
3. Run `freecad_bench.py` from FreeCAD's Python console (or pass it on the command
   line). It reports FreeCAD-side orbit FPS and TechDraw regen time — the *upper
   bound*. Then record the **browser-perceived** FPS with a screen capture while
   orbiting; that is the number that decides the messaging.

## Decision (messaging only — the project ships in every branch)

| Browser orbit FPS on 200-part | Railway template messaging                          |
|-------------------------------|-----------------------------------------------------|
| ≥ 20 fps                      | Story stands as written.                            |
| 10–20 fps                     | "Small/medium assemblies" framing.                  |
| < 10 fps                      | GPU becomes a documented requirement; light-use rung. |

Record raw numbers in `RESULTS.md` next to this file, re-run on every FreeCAD bump.
