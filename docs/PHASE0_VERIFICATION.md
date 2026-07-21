# Phase 0 — GitPDM Dependency Verification (results)

**Status:** gates 0.1 and 0.3 **PASS**; gate 0.2 (live token-in-container) **open** —
structurally requires a throwaway PAT + the built image; wired as a CI/manual step
(`scripts/authcheck.sh` + `.github/workflows/ci.yml`). Do not treat Phase 0 as fully
closed until 0.2 runs green against a real token.

Verified against real GitPDM source cloned at tag **v0.6.3** (commit
`864263a09b3031b13d3ebc17102434079e6f2891`) on 2026-07-21.

## 0.1 — Pinned tag is CI-green ✅

`v0.6.3` is the latest GitHub release. Both workflows on its commit are green:

| Workflow          | Result  | Run         |
|-------------------|---------|-------------|
| CI/CD Pipeline    | success | 29847090771 |
| Release           | success | 29846838084 |

Recorded pin: `ARG GITPDM_VERSION=v0.6.3` in the Dockerfile. Not pinned on tag
existence alone (the v0.4.0 failure mode).

## 0.3 — Headless contract surface confirmed ✅

Each fact Outpost hard-depends on, checked against source (not the spec):

- **Env-var precedence** `GITPDM_TOKEN_FILE > GITPDM_TOKEN` — documented in
  `auth/check.py` and implemented in `auth/credential_chain.py`
  (`ENV_TOKEN_FILE`, `ENV_TOKEN`).
- **`core.checkpoint.register_sigterm_handler(handler)`** — takes a *no-arg*
  callable; installs it as the SIGTERM handler; main-thread only; returns bool;
  failures logged & swallowed. (`checkpoint.py:242`)
- **`core.checkpoint.run_shutdown_checkpoint(git_client, repo_root, save_if_dirty)`**
  — synchronous save+checkpoint+push for an external supervisor. (`checkpoint.py:222`)
- **`git.client.GitClient()`** — no-arg constructor; `clone_repo`, `commit`,
  `push_ref`, `rev_parse` all present. `RECOVERY_REF = "refs/heads/gitpdm/recovery"`.
  `DEFAULT_SHALLOW_CLONE_DEPTH = 20`.
- **`python -m freecad_gitpdm.auth.check`** — `main(argv=None) -> int`; prints
  `GitPDM auth check: OK — source=… provider=… host=… login=…` (exit 0) or
  `… FAILED — …` (exit 1). Import chain is **pure stdlib** (urllib/json/os/
  dataclasses) — no `requests`, no FreeCAD, no keyring at import time. Runs under a
  bare `python3 ≥3.11` with `PYTHONPATH` at the addon clone.
- **Lazy keyring imports** — `secretstorage`/`keyring` are imported *inside
  functions* (`token_store_linux.py:37,76`, `token_store_macos.py:35`) and are
  never reached on the `GITPDM_TOKEN_FILE`/`GITPDM_TOKEN` path. **Consequence:**
  Outpost installs GitPDM by cloning the addon into FreeCAD's `Mod/` with **no pip
  dependencies** for the env-var credential path.
- **Install shape** — `pyproject.toml`, distribution `freecad-gitpdm`, version
  `0.6.3`, `requires-python >=3.11`. Legacy addon layout (`freecad_gitpdm/` +
  `InitGui.py` at repo root), so dropping the repo into `Mod/GitPDM/` gives both the
  workbench UI *and* an importable `freecad_gitpdm` via FreeCAD's addon `sys.path`.

## 0.2 — Live auth smoke test (OPEN)

`docker run -e GITPDM_TOKEN=<pat> -e GITPDM_PROVIDER=github <img> outpost-authcheck`
must print `OK — source=env provider=github …` and exit 0. Needs a scoped throwaway
PAT this environment doesn't hold. The wrapper (`/opt/outpost/authcheck.sh`) and a
CI job are in place; run it before declaring Phase 0 closed.

## Known GitPDM-side follow-up (does not block Phase 1)

`interactive_resolver` seam is unwired (deviation 2.9): GitPDM's chain won't
auto-launch device flow on a full miss. Rung 2 (gatekeeper pre-populates the token)
doesn't care; rung 1's first-run flow (R2.4) does — tracked for Phase 2.
