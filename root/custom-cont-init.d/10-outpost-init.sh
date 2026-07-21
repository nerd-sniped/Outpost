#!/usr/bin/with-contenv bash
# Outpost boot init — runs once at container start (as root, before services), via
# linuxserver's custom-cont-init.d hook. Idempotent: safe across restarts and a
# persisted /config volume.
set -uo pipefail

log() { echo "[outpost-init] $*"; }
log "starting"

# Chown by NAME (abc), never a hardcoded uid: this base ships abc as uid 911, and
# PUID only remaps it when explicitly set. An earlier version chowned to ${PUID:-1000}
# and handed FreeCAD's cache to uid 1000 — abc (911) then couldn't create its IPC lock
# and the GUI exited after the splash (blank stream). "abc" always resolves correctly.
OWNER="abc:abc"
FC_MOD="/config/.local/share/FreeCAD/Mod"
ADDONS_DIR="${OUTPOST_ADDONS_DIR:-/opt/outpost/addons}"
REPO_ROOT="${OUTPOST_REPO_ROOT:-/config/repo}"

# 1. Seed FreeCAD's Mod dir with the baked addons (symlink; survives a /config volume).
#    OutpostBoot is our own startup addon (wires the SIGTERM checkpoint hook).
mkdir -p "$FC_MOD"
for addon in GitPDM HistoryWorkbench OutpostBoot; do
  if [ -d "$ADDONS_DIR/$addon" ] && [ ! -e "$FC_MOD/$addon" ]; then
    ln -s "$ADDONS_DIR/$addon" "$FC_MOD/$addon" && log "linked addon $addon"
  fi
done

# 2. Clone-on-boot (shallow depth 20 = GitPDM's DEFAULT_SHALLOW_CLONE_DEPTH).
#    Without GIT_REMOTE_URL, start empty and let GitPDM's first-run panel drive setup.
if [ -n "${GIT_REMOTE_URL:-}" ] && [ ! -d "$REPO_ROOT/.git" ]; then
  log "cloning $GIT_REMOTE_URL (depth 20)"
  if git clone --depth 20 "$GIT_REMOTE_URL" "$REPO_ROOT"; then
    log "clone ok"
  else
    log "clone FAILED — starting with first-run flow"
  fi
fi

# 3. Ownership: git/init run as root; FreeCAD runs as abc. FreeCAD writes config to
#    ~/.config/FreeCAD and its single-instance IPC lock to ~/.cache/FreeCAD/Cache/ —
#    both MUST be abc-writable or the GUI exits right after the splash (blank stream).
#    The FreeCAD AppImage's build-time --version probe can bake these dirs as root
#    (the base sets HOME=/config even at build), so chown RECURSIVELY, by name. This is
#    also volume-safe.
for d in /config/.config /config/.cache /config/.local; do
  mkdir -p "$d"
  chown -R "$OWNER" "$d" 2>/dev/null || true
done
[ -d "$REPO_ROOT" ] && chown -R "$OWNER" "$REPO_ROOT" 2>/dev/null || true

# Credential note: rung 1 passes GITPDM_TOKEN via env (documented .env contract);
# GitPDM reads it directly. The tmpfs token-file handoff (GITPDM_TOKEN_FILE) is the
# gatekeeper's job in Phase 3 — not wired here to avoid a half-measure.
log "done"
