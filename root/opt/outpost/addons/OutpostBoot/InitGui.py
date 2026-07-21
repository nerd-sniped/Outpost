# Outpost boot addon. FreeCAD loads InitGui.py from every Mod/<Name>/ at GUI startup,
# on the main thread — where GitPDM's register_sigterm_handler must run. This replaces
# the old command-line-script approach (a .py argument makes FreeCAD run-and-exit; an
# addon loads without exiting).
#
# Wires GitPDM's shipped checkpoint SIGTERM hook (R2.5 / deviation 2.5): a container
# stop or serverless sleep (SIGTERM) saves the active document and pushes it to the
# gitpdm/recovery branch before exit. Outpost supplies save_if_dirty and CALLS the
# hook — it does not reimplement checkpoint logic. Best-effort throughout: a wiring
# failure must never break FreeCAD startup.
#
# THE SIGNAL PUMP (why the QTimer exists): CPython runs a signal handler only when the
# interpreter regains control between bytecodes. FreeCAD sits blocked in Qt's C++ event
# loop, so a bare signal.signal() handler is *registered but never executed* — verified
# live: SIGTERM was caught (non-fatal) yet the checkpoint never ran and docker stop hit
# its full grace period. A short QTimer ticking inside the Qt loop hands control back to
# the interpreter regularly, so the pending SIGTERM handler fires within ~one tick.
import os

_timer = None  # module-level ref so Qt doesn't garbage-collect the signal-pump timer


def _wire_checkpoint():
    import FreeCAD as App

    repo_root = os.environ.get("OUTPOST_REPO_ROOT", "/config/repo")
    if not os.path.isdir(os.path.join(repo_root, ".git")):
        App.Console.PrintMessage(
            f"[outpost] no git repo at {repo_root} yet; SIGTERM checkpoint deferred\n"
        )
        return

    from freecad_gitpdm.git.client import GitClient
    from freecad_gitpdm.core.checkpoint import (
        register_sigterm_handler,
        run_shutdown_checkpoint,
    )

    client = GitClient()
    _ran = {"done": False}

    def _save_if_dirty():
        # Persist every open document that has a file path (an unnamed doc has nowhere
        # to save to).
        for doc in list(App.listDocuments().values()):
            try:
                if getattr(doc, "FileName", ""):
                    doc.save()
            except Exception as e:
                App.Console.PrintError(f"[outpost] save failed for a document: {e}\n")

    def _handler():
        if _ran["done"]:
            return
        _ran["done"] = True
        App.Console.PrintMessage("[outpost] SIGTERM received; running shutdown checkpoint\n")
        try:
            res = run_shutdown_checkpoint(client, repo_root, _save_if_dirty)
            App.Console.PrintMessage(f"[outpost] checkpoint ok={getattr(res, 'ok', '?')}\n")
        except Exception as e:
            App.Console.PrintError(f"[outpost] shutdown checkpoint failed: {e}\n")
        # Work is now safe on gitpdm/recovery — exit promptly so `docker stop` (and
        # serverless sleep) completes well under the grace period instead of waiting
        # for SIGKILL.
        os._exit(0)

    if not register_sigterm_handler(_handler):
        App.Console.PrintWarning(
            "[outpost] could not install SIGTERM handler on this platform/thread\n"
        )
        return

    global _timer
    try:
        from PySide2 import QtCore
    except Exception:
        try:
            from PySide6 import QtCore
        except Exception:
            from PySide import QtCore  # very old FreeCAD fallback
    _timer = QtCore.QTimer()
    _timer.timeout.connect(lambda: None)  # firing hands control back to the interpreter
    _timer.start(200)
    App.Console.PrintMessage(
        "[outpost] SIGTERM checkpoint handler installed (signal pump active)\n"
    )


try:
    _wire_checkpoint()
except Exception as _e:  # never break FreeCAD startup
    try:
        import FreeCAD as App

        App.Console.PrintWarning(f"[outpost] checkpoint wiring skipped: {_e}\n")
    except Exception:
        pass
