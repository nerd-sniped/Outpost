# Outpost

**Outpost is a Full FreeCAD Deployment that runs in the "cloud" & auto-saves to your own Github Account** Sign in, design, close the tab — nothing to install, nothing lost.

<p align="center">
  <a href="https://railway.com/deploy/outpost-1?referralCode=D4kUtS&utm_medium=integration&utm_source=template&utm_campaign=generic">
    <img src="https://railway.com/button.svg" alt="Deploy on Railway">
  </a>
</p>

New here? Click the button above and you'll have your own private FreeCAD
workspace in about 5 minutes. To get started youll need a github account, and a paid Railway account. The paid account is necessary because running FreeCAD exceeds the free trial RAM limit and the program will just continually crash and close quietly in a way that makes it look like nothing is happening, ASK me how I know....

Prefer to run it on your own computer for free
instead? Both paths are covered step by step, in **[`docs/DEPLOY_GUIDE.md`](docs/DEPLOY_GUIDE.md)**.
Start there if any of the words below are unfamiliar.

---

**A browser-based workstation for FreeCAD — identity-gated, git-native, killable
everywhere.** Auth, versioning, and durability are one system: deploy it anywhere,
kill the server or the client, and lose at most a minute of work. Durable state lives
in a git host (via [GitPDM](https://github.com/nerd-sniped/GitPDM)), so the machine is
disposable by design.

---
There are a handful of core technologies that enable this to work.
## Tech Stack 
- FreeCAD Free and Open Source CAD Software
- GitPDM [GitPDM](https://github.com/nerd-sniped/GitPDM) Lets you connect FreeCAD to your Githost (Github in our case) to automatically save and upload FreeCAD files to a repository. 
- Docker An Open-Source platform that automates the process of building deployments inside lightweight portable software packages called containers.
- Selkies Open-Source Low-Latency Remote desktop Streaming Platform
- Tailscale a zero config VPN client that lets you connect machines with an easy SSO process.
- Railway A minimal configuration server host.

## Overview
Ultimately the goal is to have access to FreeCAD anywhere, anytime, from any device. Currently no CAD system is usable via the browser AND standalone program, it's one or the other, and there isnt a great reason for it besides money.  Outpost is a step in that direction. You can use your Desktop instance of FreeCAD like you normally would, sync your files up to any githost, and then access them while youre on the go from a mobile version of the same desktop software. 
This is not configured as a Monthly SaaS, its an easy to copy and paste template that helps you set the system up and then gets out of your way. By design theres no vendor lockin, recuring subscription or account to create 

Currently there are two main ways to do launch an Outpost.

Option 1 is to fully self host it. You run the software on your own hardware, and just connect to your home machine. This is outlined in the Tailscale install instructions. The pros are its next to free, and you control the hardware. The cons are, you own everything, if that machine goes offline, so does your deployment. If you're not home or onsite with the machine to turn the machine back on, you're potentially SOL. 

Option 2 which I think is the more interesting one.  Full disclosure, it does cost money, but its not the typical Saas nonsense.  FreeCAD is run in the cloud and you access it directly, and pay only for the compute you use. Your files live in github, and the FreeCAD instance is treated as disposable.  With this method you can access FreeCAD from any device with a web browser,  Cell phones, Ipad, a Smart Fridge...  All the compute is done in the cloud. The hosting minimum is $5 a month and with my rough testing, It seems like normal continuous use for 8 hours is about $1. So about $30 a month if youre using it every day. When the machine is idle you pay less because youre not using as much compute. If you need a more powerful machine you can pay for access to more CPU cores at a time. 

Costs change all the time but in the realm of $0.002 per minute is a safe estimate while you're using the system. If you use the system 8 hours a day all year, your compute costs will likely approach a subscription cost to fusion360, but most hobbiests are much less frequent CAD users. Plus Open Source Software is cool, and will never get worse or more expensive unlike OTHER offerings. 

To get started you'll need a Github Account, and if you're going to host this on Railway you also need a railway account (I use my github account to access Railway so I only need to remember one login)  The Railway account does need to be a paid hobby license 

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
| FreeCAD          | 1.0.2    | GitPDM's 1.0 baseline; .2 fixes py311 compat; SHA256-verified.  |
| GitPDM           | v0.6.3   | CI-green tag (see `docs/PHASE0_VERIFICATION.md`).|
| HistoryWorkbench | v0.1.0   | LGPL-2.1, runtime interop only, never vendored.  |
| Base image       | linuxserver `baseimage-selkies:ubuntunoble` | Display stack + RESTART_APP. |

Bump via the `ARG`s in the `Dockerfile` (or `docker-compose.yml` build args).

## License


