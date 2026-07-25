# Outpost

**A full CAD program, running in your browser, that auto-saves to your own
GitHub.** Sign in, design, close the tab — nothing to install, nothing lost.

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/outpost-1?referralCode=D4kUtS&utm_medium=integration&utm_source=template&utm_campaign=generic)

New here? Click the button above and you'll have your own private FreeCAD
workspace in about 5 minutes. Prefer to run it on your own computer for free
instead? Both paths — plus everything in between — are covered step by step,
no jargon, no assumed experience, in **[`docs/DEPLOY_GUIDE.md`](docs/DEPLOY_GUIDE.md)**.
Start there if any of the words below are unfamiliar — that's exactly what it's for.

---

**A browser-based workstation for FreeCAD — identity-gated, git-native, killable
everywhere.** Auth, versioning, and durability are one system: deploy it anywhere,
kill the server or the client, and lose at most a minute of work. Durable state lives
in a git host (via [GitPDM](https://github.com/nerd-sniped/GitPDM)), so the machine is
disposable by design.

> Not "FreeCAD in a browser" — that's a commodity ([linuxserver/freecad] already does
> it). Outpost is the *workflow*: one auth that is also your git credential, automatic
> checkpointing to a recovery branch, and a stateless box you can throw away. Call it
> the free, open-source answer to Onshape: real version control instead of a
> proprietary vault, and a workstation you host, own, and can walk away from any time.

[linuxserver/freecad]: https://github.com/linuxserver/docker-freecad

**Everything below this point is the engineering-level README** — architecture,
config reference, and internals for contributors and self-hosters who want the
full picture.

---

## Status

**Short version: rung 1 (self-host) has been solid for a while, and rung 2 (Railway)
is now live and running continuously too.** The longer version, for anyone who wants
the receipts:

Railway deploy works end-to-end on a real public URL — gatekeeper sign-in, device-flow
login, and GitPDM clone/edit/commit/push all verified working
(`railway.json` + `docs/PHASE4_DEPLOY.md`). Railway's own sleep/wake never reliably
triggers (`docs/DECISIONS.md` D11); rather than chase that further or eat a
latency-costing workaround, Outpost just runs continuously, with an owner-only manual
shutdown control as the escape hatch (D12). Four real findings turned up along the way
and got fixed (D9–D12) — that's what live testing against a real account is for. Next
up is Phase 6: cost validation over real use, a touch-input pass, and a
stranger-deploys-from-the-listing test — each of those needs calendar time or other
people, not another coding session.

Rung 1 (self-host) has been done and tagged for a while: FreeCAD in a browser,
reachable from a phone on cellular over Tailscale with zero open ports, confirmed on a
second machine (see `docs/PHASE2_VERIFICATION.md`). The gatekeeper door-key —
device-flow login, identity-pinned session, token handoff to GitPDM — passed a full
adversarial test pass against a real GitHub OAuth App and two real accounts
(see `docs/PHASE3_VERIFICATION.md`): wrong-account rejection, tampered/expired cookies,
restart/recreate token self-heal, `SESSION_SECRET` rotation, and the two-step panic
procedure all confirmed working.

Curious about the reasoning, not just the result? `docs/OUTPOST_DEV_PLAN.md` has the
full phase plan, and `docs/DECISIONS.md` walks through the choices behind this build.

What works today:

- FreeCAD 1.0.2 in a browser tab via the Selkies base image (software-rendered GL).
- GitPDM v0.6.3 + HistoryWorkbench v0.1.0 baked in as pinned addons.
- Clone-on-boot from `GIT_REMOTE_URL`; first-run panel flow when it's unset.
- SIGTERM → save + checkpoint to `gitpdm/recovery` (GitPDM's shipped hook).
- `/healthz` on :8090 and an `outpost-authcheck` credential probe.
- Tailscale sidecar: `https://freecad.<tailnet>.ts.net`, zero host ports (Phase 2.1).
- Gatekeeper: GitHub device-flow login, identity-pinned to one account, encrypted
  session cookie, token handoff to GitPDM — see "Reach it from the public internet"
  below. Adversarial test pass complete (Phase 3 exit gate met).

Not yet measured: the **llvmpipe benchmark** (Phase 1.5) — see `scripts/benchmark/`.

## Quick start (rung 1, localhost)

The fastest way to see it running. Requires Docker.

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

Want it on your phone too? The Tailscale sidecar makes the session reachable from your
phone or any tailnet device as an HTTPS URL — with **zero ports published on the host**
and no gatekeeper. Add the overlay to the same command:

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
private; the public-URL story is the gatekeeper, next.
Config lives in `compose.tailscale.yml` + `tailscale/serve.json`; rationale in
`docs/DECISIONS.md` D5.

## Reach it from the public internet (rung 2, gatekeeper)

Ready to give it a real public URL instead of a tailnet-only one? The gatekeeper is a
small Go shim in front of Selkies: GitHub device-flow login, identity-pinned to one
account, and a proxy that only lets an authenticated session through. It's what makes
a public URL (Railway, Phase 4) safe — but it also runs fine standalone, as a local
stand-in for that, right now:

```bash
# GitHub OAuth App (device flow enabled, no client secret needed):
# https://github.com/settings/developers -> New OAuth App -> Enable Device Flow
cp .env.example .env   # set GITHUB_CLIENT_ID, ALLOWED_GITHUB_USER, SESSION_SECRET
docker compose -f docker-compose.yml -f compose.gatekeeper.yml up -d --build
```

Open `http://localhost:8081` — you'll see a device code and a link to
`github.com/login/device`. Enter the code there; if the account matches
`ALLOWED_GITHUB_USER`, the page reloads into FreeCAD, already GitPDM-authenticated with
no second prompt. Anyone else who completes device flow with a different account is
rejected outright — no cookie, no token written anywhere. Only the gatekeeper's port is
published; Selkies and `/healthz` are not reachable except through it.

Session cookies default to 24–48 h and are self-contained (encrypted, not stored
server-side) — see `docs/DECISIONS.md` D6. If a logged-in device is lost or stolen,
follow the panic procedure in `SECURITY.md` immediately.

## How auth works (read before exposing this to a network)

Worth two minutes before you point this at anything but your own tailnet. Outpost
requests `repo` scope because **the same token is the door key and the git
credential**. A valid session includes FreeCAD's Python console — i.e. arbitrary code
execution as the container user, including reading that token. What that means in
practice:

- On `localhost`/Tailscale (rung 1) the network *is* the auth boundary — do not publish
  port 3000 to the internet without the gatekeeper.
- On a public deployment (rung 2), the gatekeeper is the auth boundary — it is the only
  thing that should ever be publicly reachable; Selkies must not be independently
  published (Railway's own routing enforces this in Phase 4; `compose.gatekeeper.yml`
  enforces it locally today).
- Never branch-protect `gitpdm/recovery` or `gitpdm/presence` on your forge — GitPDM
  force-resets those refs and protection breaks its pruning.
- If a device with a live session is lost or stolen, see `SECURITY.md`'s panic
  procedure: rotate `SESSION_SECRET` (kills all sessions instantly) and revoke the
  GitHub OAuth App's authorization (kills the token itself).

## Configuration

Nothing hidden — all via `.env` (see `.env.example`). The load-bearing values: `GITPDM_PROVIDER`,
`GITPDM_TOKEN`, `GIT_REMOTE_URL`, `PUID`/`PGID` for rung 1; `GITHUB_CLIENT_ID`,
`ALLOWED_GITHUB_USER`, `SESSION_SECRET` for the gatekeeper (rung 2 / local rung-2-style
testing). Provider support day one is **GitHub** or **generic** (any git remote via
PAT-in-URL / ambient SSH) on rung 1 — the gatekeeper's device flow is GitHub-only by
design; other named hosts are on the roadmap.

## Pins

Every version below is pinned deliberately, not just whatever was latest on build day.

| Component        | Pin      | Notes                                            |
|------------------|----------|--------------------------------------------------|
| FreeCAD          | 1.0.2    | GitPDM's 1.0 baseline; .2 fixes py311 compat; SHA256-verified. |
| GitPDM           | v0.6.3   | CI-green tag (see `docs/PHASE0_VERIFICATION.md`).|
| HistoryWorkbench | v0.1.0   | LGPL-2.1, runtime interop only, never vendored.  |
| Base image       | linuxserver `baseimage-selkies:ubuntunoble` | Display stack + RESTART_APP. |

Bump via the `ARG`s in the `Dockerfile` (or `docker-compose.yml` build args).

## License


