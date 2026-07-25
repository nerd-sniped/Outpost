# Railway template listing — copy + config

Draft content for the actual Railway Template (dashboard: your project → "..." →
**Create Template**, or Account → Templates → New Template). Railway has no
repo-native file for this — it's configured in their UI when you create the
template from this deployed project. This doc is what to paste in, and what to set
each variable's generator/description to.

## Listing copy

**Name:** Outpost — FreeCAD in your browser

**Category:** Developer Tools / Productivity (whatever Railway's taxonomy offers
closest to "self-hosted app")

Paste the block below as-is into Railway's template description field — it
matches Railway's own template-listing format.

```markdown
# Deploy and Host Outpost on Railway

Outpost is a private, browser-based FreeCAD workstation — no local installation
required. Sign in with your own GitHub account and design inside a full CAD
session streamed straight to your browser tab. Your work saves directly to a
git repository you own, so your files stay usable outside the browser too.

Think of it as the free, open-source answer to Onshape: the same "CAD lives in
the cloud, reach it from anywhere" convenience, minus the subscription lock-in —
your designs live in a git repo you control, not a vendor's database.

## About Hosting Outpost

Hosting Outpost means running one Railway service: a small Go authentication
shim (the gatekeeper) sitting in front of a containerized FreeCAD desktop,
streamed over WebSocket. The gatekeeper handles GitHub device-flow sign-in,
pins access to a single GitHub account you specify, and hands your session's
token straight to the CAD environment's git integration — one login doubles as
your git credential. The first deploy takes a few minutes, since Railway is
building and setting up a full FreeCAD environment from scratch; every deploy
after that is fast. Because your files live in a git repo, not on the server,
the running instance itself is fully disposable.

## Common Use Cases

- A personal CAD workstation reachable from any device — laptop, tablet, even a
  phone — without installing FreeCAD locally
- Hobbyists and makers who want real version history on their designs without
  learning git directly
- A disposable, always-current CAD environment for occasional projects, so
  you're not maintaining a permanent workstation just for the times you need it

## Dependencies for Outpost Hosting

- A GitHub account — used both to sign in and as the home for your saved design
  files
- A Railway account with a payment method on file (Hobby plan covers this
  comfortably)

### Deployment Dependencies

- [Outpost repository](https://github.com/nerd-sniped/Outpost) — source,
  Dockerfile, and full documentation
- [GitPDM](https://github.com/nerd-sniped/GitPDM) — the git-native version
  control layer Outpost is built on
- [FreeCAD](https://www.freecad.org/) — the CAD application itself
- [Deploy guide](https://github.com/nerd-sniped/Outpost/blob/main/docs/DEPLOY_GUIDE.md)
  — a non-technical, step-by-step walkthrough

### Implementation Details

The only field you need to fill in below is `ALLOWED_GITHUB_USER` — your GitHub
username. This is the identity check that keeps your workspace private:
anyone who signs in with a different GitHub account is turned away
automatically, no matter who else has the deployment URL.

## Why Deploy Outpost on Railway?

Because you shouldn't need to own a server, tune a reverse proxy, or babysit
uptime just to get real CAD software running somewhere you can reach it. Click
deploy, sign in with GitHub, and you're modeling — Railway handles the
infrastructure so the only thing you have to think about is your design.
```

## Variable configuration (set when defining the template)

**Correction (2026-07):** Railway's template composer has no "optional/advanced —
hide this field" toggle. Every variable attached to the service in the template
editor shows up as a fillable field on the deploy page, full stop — "optional"
in Railway's own convention just means "give it a description," not "hide it."
So getting a variable off the deploy page means **deleting it from the
template's Variables tab entirely**, not marking it optional. The table below
reflects that.

| Variable | What to do in the template's Variables tab | Why |
|---|---|---|
| `AUTH_MODE` | Keep, fixed value `gatekeeper` | Not a deployer-facing choice — the template only makes sense in gatekeeper mode. |
| `ALLOWED_GITHUB_USER` | Keep, no default, description "Your GitHub username. Only this account will be able to open your workspace." | **The one field a deployer should see and must fill in.** |
| `SESSION_SECRET` | Keep, value set to the generator `${{secret(32)}}` | Auto-generated per deploy — deployer never sees or types it. |
| `GITHUB_CLIENT_ID` | **Delete from the template.** | No longer needed as a variable at all — `gatekeeper/main.go` now defaults it to the shared "Outpost" OAuth App when unset. Leaving it in the template just shows a scary-looking field with nothing to do. |
| `RAILWAY_API_TOKEN` | **Delete from the template.** | Advanced/rare (in-session shutdown button). Anyone who wants it can add it manually as a custom variable on their deployed service afterward — documented in `docs/DEPLOY_GUIDE.md`. Not worth a confusing field on every deploy. |
| `GIT_REMOTE_URL` | **Delete from the template.** | Fully redundant with GitPDM's own first-run panel (clone/create from inside the app after sign-in) — its only value is saving one in-app click for someone who already has a specific repo picked out. Not worth a second visible field for that; an advanced user can still add it manually as a custom variable post-deploy if they want the pre-clone shortcut. |

Net result: the deploy page should show exactly **one** field — `ALLOWED_GITHUB_USER`. `AUTH_MODE` and `SESSION_SECRET` are still present as real variables, just fixed/generated so nothing is asked of the deployer.

**Tried and ruled out: auto-populating `ALLOWED_GITHUB_USER` via
`${{RAILWAY_GIT_REPO_OWNER}}`.** Since Railway forks the template's source repo
into the deployer's own GitHub account before building, the theory was that
this system variable would reflect their username for free. Live-tested on a
second account (2026-07-25): it doesn't resolve — the literal string
`${{RAILWAY_GIT_REPO_OWNER}}` landed in the container's env unexpanded, and the
gatekeeper crash-looped on `required env var ALLOWED_GITHUB_USER is not set`
(healthcheck never came up). Reference-variable syntax (`${{...}}`) only
resolves variables you've explicitly defined (same-service or
`${{ServiceName.VAR}}` cross-service) — it doesn't reach into Railway's
injected system variables like `RAILWAY_GIT_REPO_OWNER`. Reverted to a blank
manual-entry field; don't retry this approach without a different mechanism.

Also set in the template's service config:
- **Auto-generate a public domain** for the service (so the deployer never has to
  run `railway domain` or find Settings → Networking themselves).
- **Healthcheck path** `/healthz` — already in `railway.json`, should carry
  through automatically since the template deploys from this repo's Dockerfile.

## After creating the template

1. ~~Copy the resulting template URL and swap it into the README badge.~~ Done —
   live at <https://railway.com/deploy/outpost-1?referralCode=D4kUtS&utm_medium=integration&utm_source=template&utm_campaign=generic>.
2. Do a clean stranger-style test pass before wide publish (Phase 6.3): a fresh
   Railway account (or a friend's), deploying from the template listing alone, no
   out-of-band help — same bar as Phase 2.3's fresh-machine test. This is also the
   point where the repo needs to be public (see prior discussion) — a stranger's
   Railway account can't fork a private repo to deploy from it.
