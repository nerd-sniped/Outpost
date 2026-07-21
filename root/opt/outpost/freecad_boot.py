# Runs inside FreeCAD's GUI process (main thread) at startup — passed on the FreeCAD
# command line by /defaults/autostart.
#
# Wires GitPDM's shipped checkpoint SIGTERM hook (R2.5 / deviation 2.5): a container
# stop or serverless sleep (SIGTERM) saves the active document and pushes it to the
# gitpdm/recovery branch before the process exits. This is what makes scale-to-zero
# safe for unsaved work. Outpost supplies save_if_dirty and CALLS the hook — it does
# not reimplement checkpoint logic.
#
# register_sigterm_handler must run on the main thread; FreeCAD executes a command-line
# script there during GUI init, so this file is the right place. Everything is
# best-effort and swallows its own errors — a checkpoint-wiring failure must never
# block FreeCAD from starting.
import os

try:
    import FreeCAD as App
except Exception:  # pragma: no cover — only meaningful inside FreeCAD
    App = None


def _msg(fn, text):
    if App is not None:
        getattr(App.Console, fn)(f"[outpost] {text}\n")


REPO_ROOT = os.environ.get("OUTPOST_REPO_ROOT", "/config/repo")


def _save_if_dirty():
    """GitPDM's checkpoint asks us to persist in-memory work. Save every open
    document that has a file path (an unsaved/unnamed doc has nowhere to go)."""
    if App is None:
        return
    for doc in list(App.listDocuments().values()):
        try:
            if getattr(doc, "FileName", "") and doc.Label:
                doc.save()
        except Exception as e:
            _msg("PrintError", f"save failed for a document: {e}")


def _install():
    try:
        from freecad_gitpdm.git.client import GitClient
        from freecad_gitpdm.core.checkpoint import (
            register_sigterm_handler,
            run_shutdown_checkpoint,
        )
    except Exception as e:
        _msg("PrintWarning", f"GitPDM checkpoint unavailable ({e}); SIGTERM hook not installed")
        return

    if not os.path.isdir(os.path.join(REPO_ROOT, ".git")):
        _msg("PrintMessage", f"no git repo at {REPO_ROOT} yet; SIGTERM hook deferred to first-run")
        return

    client = GitClient()

    def handler():
        try:
            run_shutdown_checkpoint(client, REPO_ROOT, _save_if_dirty)
        except Exception as e:
            _msg("PrintError", f"shutdown checkpoint failed: {e}")

    if register_sigterm_handler(handler):
        _msg("PrintMessage", "SIGTERM checkpoint handler installed")
    else:
        _msg("PrintWarning", "could not install SIGTERM handler on this platform/thread")


_install()
