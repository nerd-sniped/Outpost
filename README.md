# Outpost

**A browser-based workstation for FreeCAD — identity-gated, git-native, killable
everywhere.** Auth, versioning, and durability are one system: deploy it anywhere,
kill the server or the client, and lose at most a minute of work. Durable state lives
in a git host (via [GitPDM](https://github.com/nerd-sniped/GitPDM)), so the machine is
disposable by design.

> Not "FreeCAD in a browser" — that's a commodity ([linuxserver/freecad] already does
> it). Outpost is the *workflow*: one auth that is also your git credential, automatic
> checkpointing to a recovery branch, and a stateless box you can throw away.

[linuxserver/freecad]: https://github.com/linuxserver/docker-freecad

---

## Status

**Phase 1 — the image.** Buildable and boot-testable now. Rung 1 runs on `localhost`;
the Tailscale sidecar (Phase 2), gatekeeper door-key (Phase 3), and Railway template
(Phase 4) follow. See `docs/OUTPOST_DEV_PLAN.md` for the full phase plan and
`docs/DECISIONS.md` for the choices behind this build.

What works today:

- FreeCAD 1.0.2 in a browser tab via the Selkies base image (software-rendered GL).
- GitPDM v0.6.3 + HistoryWorkbench v0.1.0 baked in as pinned addons.
- Clone-on-boot from `GIT_REMOTE_URL`; first-run panel flow when it's unset.
- SIGTERM → save + checkpoint to `gitpdm/recovery` (GitPDM's shipped hook).
- `/healthz` on :8080 and an `outpost-authcheck` credential probe.

Not yet measured: the **llvmpipe benchmark** (Phase 1.5) — see `scripts/benchmark/`.

## Quick start (rung 1, localhost)

Requires Docker.

```bash
cp .env.example .env      # set GITPDM_TOKEN + optionally GIT_REMOTE_URL
docker compose up -d --build
```

Open <http://localhost:3000>. FreeCAD loads in the tab. If you set `GIT_REMOTE_URL`,
your repo is already cloned at `/config/repo`; otherwise open GitPDM's panel and
clone/create from there. Work → save → commit → push, all from inside the session.

Check credentials resolved:

```bash
docker exec outpost outpost-authcheck
# GitPDM auth check: OK — source=env provider=github host=github.com login=<you>
```

## How auth works (read before exposing this to a network)

Outpost requests `repo` scope because **the same token is the door key and the git
credential**. A valid session includes FreeCAD's Python console — i.e. arbitrary code
execution as the container user, including reading that token. Consequences:

- On `localhost`/Tailscale (rung 1) the network *is* the auth boundary — do not
  publish port 3000 to the internet without the gatekeeper (Phase 3).
- Never branch-protect `gitpdm/recovery` or `gitpdm/presence` on your forge — GitPDM
  force-resets those refs and protection breaks its pruning.

## Configuration

All via `.env` (see `.env.example`). The load-bearing values: `GITPDM_PROVIDER`,
`GITPDM_TOKEN`, `GIT_REMOTE_URL`, `PUID`/`PGID`. Provider support day one is **GitHub**
or **generic** (any git remote via PAT-in-URL / ambient SSH); other named hosts are on
the roadmap.

## Pins

| Component        | Pin      | Notes                                            |
|------------------|----------|--------------------------------------------------|
| FreeCAD          | 1.0.2    | GitPDM's 1.0 baseline; .2 fixes py311 compat; SHA256-verified. |
| GitPDM           | v0.6.3   | CI-green tag (see `docs/PHASE0_VERIFICATION.md`).|
| HistoryWorkbench | v0.1.0   | LGPL-2.1, runtime interop only, never vendored.  |
| Base image       | linuxserver `baseimage-selkies:ubuntunoble` | Display stack + RESTART_APP. |

Bump via the `ARG`s in the `Dockerfile` (or `docker-compose.yml` build args).

## License

TBD. GitPDM (MIT) and HistoryWorkbench (LGPL-2.1) are consumed as separate addons,
not vendored.
