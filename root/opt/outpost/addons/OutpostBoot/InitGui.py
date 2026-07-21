# Outpost boot addon. FreeCAD loads InitGui.py from every Mod/<Name>/ at GUI startup,
# on the main thread — which is exactly where GitPDM's register_sigterm_handler must
# run. This replaces the old command-line-script approach (a .py argument makes FreeCAD
# run-and-exit; an addon loads without exiting).
#
# Wires GitPDM's shipped checkpoint SIGTERM hook (R2.5 / deviation 2.5): a container
# stop or serverless sleep (SIGTERM) saves the active document and pushes it to the
# gitpdm/recovery branch before exit. Outpost supplies save_if_dirty and CALLS the
# hook — it does not reimplement checkpoint logic. Everything is best-effort and must
# never raise out of here: a wiring failure cannot be allowed to break FreeCAD startup.
import os


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

    def _save_if_dirty():
        # Persist every open document that has a file path (an unnamed doc has
        # nowhere to save to).
        for doc in list(App.listDocuments().values()):
            try:
                if getattr(doc, "FileName", ""):
                    doc.save()
            except Exception as e:
                App.Console.PrintError(f"[outpost] save failed for a document: {e}\n")

    def _handler():
        try:
            run_shutdown_checkpoint(client, repo_root, _save_if_dirty)
        except Exception as e:
            App.Console.PrintError(f"[outpost] shutdown checkpoint failed: {e}\n")

    if register_sigterm_handler(_handler):
        App.Console.PrintMessage("[outpost] SIGTERM checkpoint handler installed\n")
    else:
        App.Console.PrintWarning(
            "[outpost] could not install SIGTERM handler on this platform/thread\n"
        )


try:
    _wire_checkpoint()
except Exception as _e:  # never break FreeCAD startup
    try:
        import FreeCAD as App

        App.Console.PrintWarning(f"[outpost] checkpoint wiring skipped: {_e}\n")
    except Exception:
        pass
