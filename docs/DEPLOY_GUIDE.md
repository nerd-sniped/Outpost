# Deploying Outpost — a guide for non-technical users

Welcome! This guide assumes you've never used Docker, Git, or a cloud hosting
dashboard before — that's fine, you don't need to. It walks through getting your
own private FreeCAD workspace running in a browser tab, step by step, in plain
language. Take your time; nothing here is a one-shot deal, and every step below
is easy to undo if you get something wrong.

There are **two ways to run Outpost** — pick one:

- **Tutorial 1: Your own computer (Tailscale)** — free forever, but only reachable
  from devices you personally approve, and only while that computer is on.
- **Tutorial 2: Railway (cloud hosting)** — costs roughly $10–20/month, but it's
  always on and reachable from any device with a sign-in screen, without you
  needing to own or run any hardware.

Read "What is Outpost, in plain English?" first either way — it explains the
moving parts so the steps below make sense instead of feeling like magic.

<p align="center">
  <a href="#what-is-outpost-in-plain-english"><img src="https://img.shields.io/badge/-What%20is%20Outpost%3F-4c6ef5?style=for-the-badge" alt="Jump to: What is Outpost?"></a>
  <a href="#which-path-should-i-use"><img src="https://img.shields.io/badge/-Which%20Path%20Should%20I%20Use%3F-4c6ef5?style=for-the-badge" alt="Jump to: Which path should I use?"></a>
  <a href="#tutorial-1-run-it-on-your-own-computer-tailscale"><img src="https://img.shields.io/badge/-Tutorial%201%3A%20Own%20Computer-2f9e44?style=for-the-badge" alt="Jump to: Tutorial 1, own computer"></a>
  <a href="#tutorial-2-deploy-to-the-cloud-railway"><img src="https://img.shields.io/badge/-Tutorial%202%3A%20Cloud%20(Railway)-2f9e44?style=for-the-badge" alt="Jump to: Tutorial 2, Railway"></a>
  <a href="#technical-reference-for-docker-comfortable-users"><img src="https://img.shields.io/badge/-Technical%20Reference-868e96?style=for-the-badge" alt="Jump to: Technical reference"></a>
</p>

---

## What is Outpost, in plain English?

Think of Outpost as a CAD program (FreeCAD) running on a computer that isn't
yours, with a live video feed of its screen streamed into your browser tab —
similar to how Chrome Remote Desktop or Parsec let you watch and control a
faraway computer. You're not installing FreeCAD; you're watching and clicking on
a video of it that reacts instantly.

The twist: your actual design files never live only on that faraway computer.
Every time you save, your work gets pushed to a GitHub repository you own. That
means the computer running FreeCAD is *disposable* — it can crash, restart, or
get deleted, and at most you lose the last minute of unsaved work, because
everything else is already safely sitting in your GitHub account.

That's the whole idea: **a CAD workstation you can throw away, because your work
was never really stored on it.**

If you've used something like Onshape before, the closest comparison is: same
"CAD lives in the cloud, reach it from anywhere" convenience — except everything
here is free and open-source, your files live in a git repo you actually own
instead of a vendor's database, and there's no subscription tying your designs
to a company's continued existence.

## The moving parts (tech stack, in plain English)

You don't need to understand any of this to follow the tutorials — it's here so
you can explain the shape of the thing to someone else without hand-waving.

| Piece | Plain-English job |
|---|---|
| **FreeCAD** | The actual CAD software. Same program you'd install locally — it just happens to be running somewhere else. |
| **Docker** | A shipping container for software. It packages FreeCAD and everything it needs so it runs identically on your laptop, a friend's PC, or Railway's servers. |
| **Selkies** | The "video call" layer. It captures FreeCAD's screen and streams it to your browser, and sends your clicks/keystrokes back — this is what makes "FreeCAD in a browser tab" possible. |
| **GitPDM** | The auto-save/backup layer. Wires FreeCAD up to git (GitHub's version-control system) so Save/Commit/Push inside the app pushes your files to your own GitHub repo. |
| **The front door** | Whatever decides *who's allowed in*. Tutorial 1 uses **Tailscale** (only your own devices, via a private network). Tutorial 2 uses the **gatekeeper** (anyone can reach the URL, but only your GitHub account can sign in). |

## The basic workflow (same shape, either path)

Once it's running, using Outpost looks the same no matter which tutorial you
followed:

1. **Open a URL in your browser.** No install, no download.
2. **Prove it's you.** Either you're on the private network (Tailscale), or you
   sign in with GitHub (Railway).
3. **FreeCAD loads**, streamed live, like a video that responds to your clicks.
4. **You design.** Draw, model, orbit the 3D view — normal FreeCAD.
5. **Save → Commit → Push** from the GitPDM panel inside the app. This is the
   step that actually backs your work up to GitHub — get in the habit of doing
   it often.
6. **Close the tab whenever.** If the server ever restarts unexpectedly, it
   auto-backs-up any unsaved work first, so a dropped connection isn't scary —
   but pushing yourself is still the real safety net.

## Which path should I use?

| | Tutorial 1: Tailscale (own hardware) | Tutorial 2: Railway (cloud) |
|---|---|---|
| **Cost** | Free (electricity aside) | ~$10–20/month |
| **Reachable from** | Only devices you've added to your private Tailscale network | Any device, anywhere — sign-in screen decides access |
| **Needs to stay on** | Yes — it's your computer; if it's off, Outpost is off | No — Railway's servers run it |
| **Setup difficulty** | A bit more involved (install Docker, edit a config file) | Easier (click a button, fill in one field) |
| **Good for** | Technical-ish users, or anyone with a spare always-on PC | Anyone who wants "click and go" with zero hardware |

---

## Tutorial 1: Run it on your own computer (Tailscale)

This gets Outpost running on a computer you own, reachable from your phone or
any other device — but only devices you've personally signed into the same
private Tailscale network. There's no public login screen here; **the private
network itself is what keeps strangers out**, so don't skip the "keep this
private" note in Step 6.

### What you need first (Tailscale)

- **A computer that can stay on** while you're using Outpost — a desktop, an old
  laptop, a NUC, a home server. It doesn't need a fancy graphics card.
- **[Docker Desktop](https://www.docker.com/products/docker-desktop/)** installed
  and running on that computer (Windows/Mac). On Linux, install
  [Docker Engine](https://docs.docker.com/engine/install/) instead. Docker is
  the "shipping container" software mentioned above — Outpost runs inside it.
- **A free [Tailscale](https://tailscale.com) account.** Tailscale creates a
  private network ("tailnet") that only your devices can join.
- **A [GitHub account](https://github.com/signup)**, same as Tutorial 2 — this
  is where your design files get saved.

### Step 1 — Get the Outpost files onto your computer

Go to the [Outpost repository](https://github.com/nerd-sniped/Outpost), click
the green **"Code"** button, then **"Download ZIP."** Extract it somewhere you'll
remember, like your Desktop.

(If you're comfortable with git, `git clone` works too — same result.)

### Step 2 — Create a GitHub token

Outpost needs permission to save files to your GitHub account on your behalf.

1. Go to [github.com/settings/tokens](https://github.com/settings/tokens) →
   **Generate new token** → **Generate new token (classic)**.
2. Give it any name (e.g. "Outpost"), check the **`repo`** scope box, and click
   **Generate token** at the bottom.
3. **Copy the token immediately** — GitHub only shows it once. It'll look like
   `ghp_xxxxxxxxxxxxxxxxxxxx`.

### Step 3 — Fill in your configuration file

Inside the folder from Step 1, find `.env.example`. Make a copy of it in the
same folder and rename the copy to `.env` (just `.env`, nothing before the dot).
Open it in Notepad (Windows) or TextEdit (Mac) and fill in two lines:

- `GITPDM_TOKEN=` — paste the token from Step 2 right after the `=`.
- `TS_AUTHKEY=` — leave this **blank**. You'll get a one-time login link in
  Step 5 instead, which is simpler for a first run.

Everything else can stay as-is. Save the file.

### Step 4 — Start Outpost

Open a terminal in that folder (Windows: right-click inside the folder →
"Open in Terminal" or "Open PowerShell window here"; Mac: right-click the
folder → Services → "New Terminal at Folder"), and run:

```
docker compose -f docker-compose.yml -f compose.tailscale.yml up -d --build
```

The first run takes a few minutes — Docker is downloading and building the full
FreeCAD environment. Scrolling text is normal, not an error.

### Step 5 — Connect Outpost to your Tailscale network

Run:

```
docker compose logs -f tailscale
```

You'll see a one-time login URL (starts with `https://login.tailscale.com/...`).
Open it in your browser and sign in — this links the Outpost container to your
tailnet. You only need to do this once; press `Ctrl+C` to stop watching the logs
once you've signed in.

### Step 6 — Add your other devices to the tailnet

On your phone (or any other device you want to use Outpost from), install the
[Tailscale app](https://tailscale.com/download) and sign in with the **same
account** you used in Step 5. That device is now part of your private network.

**Keep this list small and trusted** — anyone on your tailnet can open Outpost
and get a full FreeCAD Python console with no further login. There's no
gatekeeper on this path; the network membership *is* the security boundary.

### Step 7 — Open Outpost

From any device on your tailnet, browse to:

```
https://freecad.<your-tailnet-name>.ts.net
```

Your tailnet name is visible in the [Tailscale admin console](https://login.tailscale.com/admin/machines)
next to the `freecad` machine, or in the URL Tailscale showed you after sign-in.
FreeCAD should load in a few seconds. If you set a `GIT_REMOTE_URL` in Step 3
(optional — not covered above), your repo is already cloned; otherwise use
GitPDM's panel inside the app to clone or create one.

### Tutorial 1 troubleshooting

Hit a snag? These cover just about everything that trips people up on this path:

- **`docker compose` command not found.** Docker Desktop isn't running, or isn't
  installed. Open Docker Desktop and wait for it to say "Running," then retry.
- **No login URL appeared in the logs.** Give it a few seconds and re-run the
  `docker compose logs -f tailscale` command from Step 5 — it only prints once,
  early in startup.
- **`https://freecad.<tailnet>.ts.net` doesn't load on my phone.** Confirm the
  Tailscale app is installed *and signed in* on that phone (open the app, check
  it says "Connected"), and that it's the same Tailscale account as Step 5.
- **I closed the terminal — did Outpost stop?** No. `up -d` runs it in the
  background permanently (until you restart the computer or run
  `docker compose down`). Closing the terminal window is fine.
- **I want to stop it.** Run `docker compose -f docker-compose.yml -f compose.tailscale.yml down`
  in the same folder.

---

## Tutorial 2: Deploy to the cloud (Railway)

This gets you a private web address that only you can open, with a full CAD
program running inside it — nothing to install on your own computer, and no
hardware of your own to keep running. Unlike Tutorial 1, there's a public login
screen (the gatekeeper) doing the work Tailscale's private network did above.

### What you need first (Railway)

- **A GitHub account.** This is where your design files get saved, and it's also
  how you'll prove it's *you* signing in. If you don't have one, create one free at
  [github.com/signup](https://github.com/signup) — just a username, email, and
  password.
- **A Railway account, signed up using that same GitHub account.** Railway is the
  hosting service that actually runs Outpost for you. Go to
  [railway.com](https://railway.com), click "Login," and choose "Login with GitHub."
  Railway will ask you to add a payment method before you can deploy anything —
  this is normal for hosting services and is covered in "About cost" below.
- **The Hobby plan, not just the free trial.** New Railway accounts start on a
  free trial that caps you at 1 GB of RAM — not enough for Outpost (FreeCAD plus
  the streaming/display stack needs more than that, and on a trial account the
  deployment will crash-loop shortly after starting). Upgrade to the $5/month
  Hobby plan before deploying: in the Railway dashboard, go to your account or
  workspace settings and look for "Upgrade" / "Billing." This only takes a minute
  and is a one-time setup step, not something you repeat per deploy.

That's it. You do not need to install anything on your own computer.

### Step 1 — Click the deploy button

From [the Outpost repository page](https://github.com/nerd-sniped/Outpost), click
the **"Deploy on Railway"** button near the top of the README. This opens Railway
and starts setting up your own private copy of Outpost.

### Step 2 — Fill in one field: your GitHub username

Railway will show a short setup form. There's only one thing you *must* fill in:

- **`ALLOWED_GITHUB_USER`** — type your GitHub username here (the same account you
  logged into Railway with). This is what makes the workspace yours and yours
  alone: anyone who tries to open your link and signs in with a *different* GitHub
  account gets turned away automatically.

Everything else on the form already has a working default — you can leave it as
is. Click **Deploy**.

### Step 3 — Wait for the first build (get a coffee)

The first deploy takes about **5 minutes**. Railway is downloading and setting up
a full copy of FreeCAD in the background — this is normal and only happens once.
You'll see a progress screen with scrolling log text; that's expected, not an
error. If it's still going after 10 minutes, see Troubleshooting below.

When it says your deployment is live, open **Settings → Networking** on your
service and make sure a public domain is generated (a `something.up.railway.app`
address). Click it, or copy it into your browser.

### Step 4 — Sign in

Your new Outpost address will show a short numeric/letter code and a link to
`github.com/login/device`. Click that link (opens in a new tab), and when it asks
for a code, type the one Outpost showed you. Approve the request.

Switch back to the Outpost tab — it should now load into a full desktop application
(FreeCAD) running inside your browser. This may take a few seconds the first time.

**If it rejects you:** you signed in with a different GitHub account than the one
you typed into `ALLOWED_GITHUB_USER` in Step 2. Reload the page and sign in with
the matching account instead.

### Step 5 — Using it

You're now looking at a real CAD program, just streamed to you instead of
installed. A few things work differently from a normal website:

- Everything you do — clicking, drawing, orbiting the 3D view — happens on
  Railway's computer, not yours. Your browser is just a window into it.
- Your work is **not automatically backed up to GitHub as you go** — you still
  need to save. There's a save panel in the app (look for the GitPDM panel/icon)
  with buttons roughly like "Save," "Commit," and "Push." Think of it as: *Save*
  keeps your current changes in the workspace, *Commit + Push* backs them up to
  your own GitHub account so they're safe even if this workspace ever gets deleted.
- If the connection drops or the server restarts unexpectedly, Outpost
  automatically backs up any unsaved work to a recovery spot on your GitHub account
  first — so a dropped connection is a lot less scary than it sounds. Still, save
  and push often, the same way you'd save a document you care about.

### About cost

Railway bills for the time your workspace is running, plus a small amount for data
transferred while you're using it. As of this writing that's estimated in the
**$10–20/month range for regular personal use**, but that estimate is still being
verified against real-world usage — treat it as a rough starting expectation, not a
guarantee.

Two things worth doing on Railway's dashboard early on:
- Check **Usage/Billing** after your first session to see real numbers for your
  own usage pattern.
- If Railway offers a spending limit or budget alert in your account settings,
  turn it on — it's the easiest way to avoid a surprise.

You may want to manually stop the deployment from Railway's dashboard when you're
not using it for a while, so nothing runs (or bills) while you're away.

### Tutorial 2 troubleshooting

Hit a snag? These cover just about everything that trips people up on this path:

- **Stuck on "Building" for more than ~10 minutes.** Click into the deployment's
  logs in Railway's dashboard — if it's still scrolling text, it's still working.
  If it shows a red "failed" state, something's wrong with the deploy itself, not
  something you did in Step 2 — worth checking the repository's issues page.
- **The page never loads / times out.** Double check Step 3 finished (the
  dashboard shows the deployment as "Active," not "Building" or "Crashed"), and
  that you're using the exact `.up.railway.app` address from Settings → Networking.
- **"Access denied" after signing in with GitHub.** You signed in as the wrong
  account — see Step 4.
- **The page is stuck on "Starting…" and never shows a sign-in code, even after
  waiting several minutes.** This usually means you're still on Railway's free
  trial rather than the Hobby plan — the trial's 1 GB RAM limit isn't enough for
  Outpost, so the app repeatedly crashes and restarts in the background (you
  won't see an error, just the same "Starting…" screen looping). Upgrade to the
  Hobby plan in Railway's dashboard (Billing → Upgrade), then redeploy.
- **You lost a device you were signed into.** See `SECURITY.md` in the repository
  for the two-step "panic procedure" (rotate your session secret, revoke the
  GitHub authorization). It takes about a minute and immediately locks out any
  device using your old session.

---

## Technical reference (for Docker-comfortable users)

Everything below is the fast path — raw commands, no hand-holding. If you followed
Tutorial 1 or 2 above, you already have all of this running; this section is for
people who'd rather skip the walkthrough, plus the auth/config details that don't
fit either tutorial.

### Bare localhost (no Tailscale, no gatekeeper)

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

This mode has no auth boundary of its own — see "How auth actually works" below
before publishing port 3000 anywhere but `localhost`.

### Tailscale overlay, raw commands

Same result as Tutorial 1, without the walkthrough:

```bash
# set GITPDM_TOKEN + TS_AUTHKEY in .env (or leave TS_AUTHKEY blank for interactive login)
docker compose -f docker-compose.yml -f compose.tailscale.yml up -d --build
docker compose logs -f tailscale   # first run without an authkey: follow the login URL
```

Then browse to `https://freecad.<your-tailnet>.ts.net` from any device on the tailnet
(phone on cellular included). The sidecar owns the network namespace, so Selkies never
binds a host port — `tailscale serve` terminates a real cert on :443 and proxies to it
(WebSocket upgrade and all). Tailnet-only: no Funnel, nothing on the public internet.
Config lives in `compose.tailscale.yml` + `tailscale/serve.json`; rationale in
`docs/DECISIONS.md` D5.

### Gatekeeper, standalone (local test rig for rung 2)

The gatekeeper is a small Go shim in front of Selkies: GitHub device-flow login,
identity-pinned to one account, and a proxy that only lets an authenticated session
through. It's what makes a public URL (Railway, Tutorial 2) safe — and it also runs
fine standalone, as a local stand-in for that:

```bash
# GitHub OAuth App (device flow enabled, no client secret needed):
# https://github.com/settings/developers -> New OAuth App -> Enable Device Flow
cp .env.example .env   # set GITHUB_CLIENT_ID, ALLOWED_GITHUB_USER, SESSION_SECRET
docker compose -f docker-compose.yml -f compose.gatekeeper.yml up -d --build
```

Open `http://localhost:8081` — you'll see a device code and a link to
`github.com/login/device`. Enter the code there; if the account matches
`ALLOWED_GITHUB_USER`, the page reloads into FreeCAD, already GitPDM-authenticated
with no second prompt. Anyone else who completes device flow with a different account
is rejected outright — no cookie, no token written anywhere. Only the gatekeeper's
port is published; Selkies and `/healthz` are not reachable except through it.

Session cookies default to 24–48 h and are self-contained (encrypted, not stored
server-side) — see `docs/DECISIONS.md` D6. If a logged-in device is lost or stolen,
follow the panic procedure in `SECURITY.md` immediately.

### How auth actually works (read before exposing this to a network)

Worth two minutes before you point this at anything but your own tailnet. Outpost
requests `repo` scope because **the same token is the door key and the git
credential**. A valid session includes FreeCAD's Python console — i.e. arbitrary code
execution as the container user, including reading that token. What that means in
practice:

**Is Railway actually more secure than self-hosting?** Not exactly — it moves the
blast radius rather than shrinking it. Both paths share the same worst case above: a
compromised session is a full compromise of the box and the repo, either way. Railway
isolates that blast radius *from you* — if the container gets popped, the attacker
lands in a disposable cloud sandbox with no path to your other devices or home
network. Tailscale-only self-hosting trades that for a smaller attack surface in the
first place: nothing is reachable from the public internet at all, so there's nothing
for a stranger to even attempt against. Neither is strictly safer; they just fail
differently.

- On `localhost`/Tailscale (rung 1) the network *is* the auth boundary — do not publish
  port 3000 to the internet without the gatekeeper.
- On a public deployment (rung 2), the gatekeeper is the auth boundary — it is the only
  thing that should ever be publicly reachable; Selkies must not be independently
  published (Railway's own routing enforces this; `compose.gatekeeper.yml` enforces it
  locally today).
- Never branch-protect `gitpdm/recovery` or `gitpdm/presence` on your forge — GitPDM
  force-resets those refs and protection breaks its pruning.
- If a device with a live session is lost or stolen, see `SECURITY.md`'s panic
  procedure: rotate `SESSION_SECRET` (kills all sessions instantly) and revoke the
  GitHub OAuth App's authorization (kills the token itself).

### Configuration reference

Nothing hidden — all via `.env` (see `.env.example`). The load-bearing values:
`GITPDM_PROVIDER`, `GITPDM_TOKEN`, `GIT_REMOTE_URL`, `PUID`/`PGID` for rung 1;
`GITHUB_CLIENT_ID`, `ALLOWED_GITHUB_USER`, `SESSION_SECRET` for the gatekeeper (rung 2
/ local rung-2-style testing). Provider support day one is **GitHub** or **generic**
(any git remote via PAT-in-URL / ambient SSH) on rung 1 — the gatekeeper's device
flow is GitHub-only by design; other named hosts are on the roadmap.
