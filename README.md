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

**Phase 2 — rung 1 MVP, verified.** The personal MVP is done: FreeCAD in a browser,
reachable from a phone on cellular over Tailscale with zero open ports, confirmed on a
second machine (see `docs/PHASE2_VERIFICATION.md`). The gatekeeper door-key (Phase 3)
and Railway template (Phase 4) follow. See `docs/OUTPOST_DEV_PLAN.md` for the full phase
plan and `docs/DECISIONS.md` for the choices behind this build.

What works today:

- FreeCAD 1.1.1 in a browser tab via the Selkies base image (software-rendered GL).
- GitPDM v0.6.3 + HistoryWorkbench v0.1.0 baked in as pinned addons.
- Clone-on-boot from `GIT_REMOTE_URL`; first-run panel flow when it's unset.
- SIGTERM → save + checkpoint to `gitpdm/recovery` (GitPDM's shipped hook).
- `/healthz` on :8080 and an `outpost-authcheck` credential probe.
- Tailscale sidecar: `https://freecad.<tailnet>.ts.net`, zero host ports (Phase 2.1).

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

## Reach it from anywhere over Tailscale (rung 1, no open ports)

The Tailscale sidecar makes the session reachable from your phone or any tailnet
device as an HTTPS URL — with **zero ports published on the host** and no gatekeeper.
Add the overlay to the same command:

```bash
# set GITPDM_TOKEN + TS_AUTHKEY in .env (or leave TS_AUTHKEY blank for interactive login)
docker compose -f docker-compose.yml -f compose.tailscale.yml up -d --build
docker compose logs -f tailscale   # first run without an authkey: follow the login URL
```

Then browse to `https://freecad.<your-tailnet>.ts.net` from any device on the tailnet
(phone on cellular included). The sidecar owns the network namespace, so Selkies never
binds a host port — `tailscale serve` terminates a real cert on :443 and proxies to it
(WebSocket upgrade and all). Tailnet-only: no Funnel, nothing on the public internet.

On rung 1 **the tailnet is the auth boundary** (`AUTH_MODE=tailscale`, gatekeeper
absent) — anyone on your tailnet reaches a full FreeCAD Python console. Keep the tailnet
private; the public-URL story waits for the gatekeeper (Phase 3). Config lives in
`compose.tailscale.yml` + `tailscale/serve.json`; rationale in `docs/DECISIONS.md` D5.

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
| FreeCAD          | 1.1.1    | GitPDM↔HW pair verified on 1.1.x by the GitPDM owner; SHA256-verified. |
| GitPDM           | v0.6.3   | CI-green tag (see `docs/PHASE0_VERIFICATION.md`).|
| HistoryWorkbench | v0.1.0   | LGPL-2.1, runtime interop only, never vendored.  |
| Base image       | linuxserver `baseimage-selkies:ubuntunoble` | Display stack + RESTART_APP. |

Bump via the `ARG`s in the `Dockerfile` (or `docker-compose.yml` build args).

## License

TBD. GitPDM (MIT) and HistoryWorkbench (LGPL-2.1) are consumed as separate addons,
not vendored.
