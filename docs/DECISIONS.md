# Outpost — Decision Log

Running record of load-bearing choices, newest first. Each entry: what, why, and
what would reverse it (mirrors the scope doc's escape-hatch philosophy).

---

## D3 — Base image: `baseimage-selkies` + own pinned FreeCAD AppImage (Phase 1.1)

**Decision:** `FROM lscr.io/linuxserver/baseimage-selkies` and layer in a pinned
FreeCAD AppImage, rather than extending `lscr.io/linuxserver/freecad`.

**Why:** the whole project ethos is honest pinning to *tested pairs* (GitPDM ↔
HistoryWorkbench, R5.5c). `linuxserver/freecad` pins FreeCAD on *their* cadence; we
need `ARG FREECAD_VERSION` under our control so the image only ever ships a FreeCAD
we've paired-tested. The baseimage still provides the display stack, single-app
launch, and `RESTART_APP` crash-relaunch — we do **not** hand-build Xvfb/Openbox/
Selkies.

**Reverses if:** maintaining the AppImage extraction becomes a burden and
`linuxserver/freecad`'s version cadence ends up matching our tested pairs anyway —
then extending their image drops the extraction step. Low likelihood.

## D2 — FreeCAD pinned to 1.0.1 (not latest 1.1.x)

**Decision:** `ARG FREECAD_VERSION=1.0.1`, x86_64 conda AppImage, SHA256
`5a3fc405771b4fbb5f270110d055bc135123c9fc91bb7dbb5d068ac7fbe50f6e`.

**Why:** GitPDM's `package.xml` declares `<freecadmin>1.0</freecadmin>` — 1.0 is its
stated baseline. FreeCAD 1.1.x is stable and available, but there is **no GitPDM↔HW
pair test yet** (the HW G8 adapter is still a planned spike), so chasing 1.1.x buys
risk with no verification behind it. 1.0.1 is proven and matches the declared
baseline.

**Bump path:** when the GitPDM↔HistoryWorkbench pair test lands (GitPDM G8 / R5.5c),
re-run the benchmark script on a 1.1.x candidate and bump the ARG + SHA256 if green.
CalVer means this is routine, not exceptional.

## D1 — Addons baked image-internal, symlinked into `Mod/` at boot

**Decision:** clone GitPDM (`v0.6.3`, MIT) and HistoryWorkbench (`v0.1.0`, LGPL-2.1)
into `/opt/outpost/addons/` at build; a `custom-cont-init.d` script symlinks them
into `$HOME/.local/share/FreeCAD/Mod/` at boot.

**Why:** linuxserver's home is `/config`, the conventional volume mount point. Baking
addons *directly* into `/config/...` would be wiped by a user-mounted `/config`
volume. Baking them image-internal at `/opt/outpost/addons` and seeding the Mod dir
at boot survives the volume case (scope: "volume is opt-in, never required").
HistoryWorkbench is LGPL-2.1 → **runtime interop only, never vendored** into our
source tree; cloning a pinned tag at build satisfies this.
