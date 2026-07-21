# Outpost — Decision Log

Running record of load-bearing choices, newest first. Each entry: what, why, and
what would reverse it (mirrors the scope doc's escape-hatch philosophy).

---

## D4 — Startup wiring via a FreeCAD addon, not a command-line script (Phase 1.1)

**Decision:** launch FreeCAD **bare** (`exec /opt/freecad/AppRun`, no arguments) and
put startup wiring (the SIGTERM checkpoint hook) in a FreeCAD addon,
`Mod/OutpostBoot/InitGui.py`.

**Why:** discovered during first live bring-up — passing a `.py` on FreeCAD's command
line runs it in **console mode and then exits**. Under the base's `RESTART_APP`
watchdog that becomes an invisible relaunch loop into an immediate exit: a *blank*
Selkies stream. An addon's `InitGui.py` loads at GUI init, on the main thread (where
`register_sigterm_handler` must run), without exiting. This is also how GitPDM itself
hooks into FreeCAD.

Two adjacent facts nailed down at the same time: the base's real display is
**`DISPLAY=:1`** (Xvfb :1), already exported into the container environment; and the
base copies `/defaults/autostart` → `$HOME/.config/openbox/autostart` and runs it via
`sh` (the shebang is cosmetic there).

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

## D2 — FreeCAD pinned to 1.0.2 (not 1.0.1, not latest 1.1.x)

**Decision:** `ARG FREECAD_VERSION=1.0.2`, x86_64 conda AppImage, SHA256
`e00be00ad9fdb12b05c5002bfd1aa2ea8126f2c1d4e2fb603eb7423b72904f61`.

**Why 1.0.x not 1.1.x:** GitPDM's `package.xml` declares `<freecadmin>1.0</freecadmin>`
— 1.0 is its stated baseline. FreeCAD 1.1.x is stable and available, but there is **no
GitPDM↔HW pair test yet** (the HW G8 adapter is still a planned spike), so chasing
1.1.x buys risk with no verification behind it.

**Why .2 not .1:** the 1.0.1 conda **py311** AppImage self-reports a mismatch at
startup — *"FreeCAD version (1.0.1) must be at least 1.0.2 to work with Python 3.11 and
above."* 1.0.2 is the patch that clears it, still within the 1.0 baseline. Caught
during the first live browser bring-up (see D4).

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
