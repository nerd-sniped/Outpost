#!/usr/bin/with-contenv bash
# Headless auth/credential probe — GitPDM's own CLI, run under a bare system python3
# (the auth.check import chain is pure stdlib; no FreeCAD, no pip deps needed on the
# GITPDM_TOKEN_FILE/GITPDM_TOKEN path). This is Phase 0.2's smoke test and the
# container's auth health check.
#
#   Prints "GitPDM auth check: OK — source=… provider=… host=… login=…" + exit 0, or
#          "GitPDM auth check: FAILED — …" + exit 1.
#
# Usage: outpost-authcheck   (symlinked onto PATH at /usr/local/bin/outpost-authcheck)
exec env PYTHONPATH="${OUTPOST_ADDONS_DIR:-/opt/outpost/addons}/GitPDM" \
     python3 -m freecad_gitpdm.auth.check "$@"
