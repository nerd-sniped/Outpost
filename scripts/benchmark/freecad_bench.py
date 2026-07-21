"""FreeCAD-side benchmark (upper bound) for the llvmpipe measurement (Phase 1.5).

Run from FreeCAD's Python console with a document open, or:
    /opt/freecad/AppRun /path/to/freecad_bench.py <optional.FCStd>

Reports orbit FPS (FreeCAD render only) and TechDraw page regen time. NOTE: this is
the *upper bound*. The number that decides Railway messaging is browser-perceived FPS
with the Selkies stream active — capture that separately (see README.md).
"""
import sys
import time

import FreeCAD as App
import FreeCADGui as Gui


def _open_if_given():
    if len(sys.argv) > 1 and sys.argv[1].lower().endswith((".fcstd", ".fcstd1")):
        App.openDocument(sys.argv[1])


def orbit_fps(seconds=5.0, step_deg=2.0):
    """Spin the active 3D view and measure frames actually drawn."""
    view = Gui.ActiveDocument.ActiveView
    cam = view.getCameraNode()
    frames = 0
    t_end = time.time() + seconds
    ang = 0.0
    while time.time() < t_end:
        ang += step_deg
        view.viewAxonometric()
        try:
            cam.orientation.setValue(
                App.Vector(0, 0, 1), ang * 3.14159 / 180.0
            )
        except Exception:
            pass
        Gui.updateGui()
        frames += 1
    return frames / seconds


def techdraw_regen():
    """Time a TechDraw page recompute if one exists; else None."""
    doc = App.ActiveDocument
    pages = [o for o in doc.Objects if o.TypeId.startswith("TechDraw::DrawPage")]
    if not pages:
        return None
    t0 = time.time()
    for p in pages:
        p.recompute()
    doc.recompute()
    return time.time() - t0


if __name__ == "__main__":
    _open_if_given()
    if App.ActiveDocument is None:
        print("[bench] no active document — open an assembly first")
    else:
        fps = orbit_fps()
        td = techdraw_regen()
        print(f"[bench] FreeCAD-side orbit FPS (upper bound): {fps:.1f}")
        print(f"[bench] TechDraw regen: {'n/a' if td is None else f'{td:.2f}s'}")
        print("[bench] Now record BROWSER-perceived FPS with the stream active.")
