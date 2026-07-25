# Railway template listing — copy + config

Draft content for the actual Railway Template (dashboard: your project → "..." →
**Create Template**, or Account → Templates → New Template). Railway has no
repo-native file for this — it's configured in their UI when you create the
template from this deployed project. This doc is what to paste in, and what to set
each variable's generator/description to.

## Listing copy

**Name:** Outpost — FreeCAD in your browser

**Short description (one line, shown in search/cards):**
> Private, browser-based FreeCAD with your work auto-saved to your own GitHub.

**Long description (template detail page):**
> Outpost gives you a full FreeCAD workstation running in your browser — nothing
> to install. Sign in with your own GitHub account (only you can get in), design,
> and your work saves straight to a repo in your own GitHub — so it's never
> trapped on someone else's server. Built on GitPDM for git-native version
> control, so files stay usable outside the browser too, on any machine with
> FreeCAD installed.
>
> **First deploy takes about 5 minutes** (downloading and setting up FreeCAD) —
> that's normal, not a stall. After filling in your GitHub username below, just
> click Deploy and wait.
>
> **New to this?** Follow `docs/DEPLOY_GUIDE.md` in the repository for a
> step-by-step, no-jargon walkthrough.
>
> **Cost:** rough personal-use estimate is $10–20/month (compute + a small amount
> of data transfer) — check Railway's Usage tab after your first session to see
> your own real number, and set a spending limit if your account offers one.

**Category:** Developer Tools / Productivity (whatever Railway's taxonomy offers
closest to "self-hosted app")

## Variable configuration (set when defining the template)

| Variable | Type / generator | Required | Notes for the field's description text |
|---|---|---|---|
| `AUTH_MODE` | fixed value `gatekeeper` | yes (locked, not shown to deployer) | Not a deployer-facing choice — the template only makes sense in gatekeeper mode. |
| `ALLOWED_GITHUB_USER` | text input, no default | **yes — the only field a deployer must fill in** | "Your GitHub username. Only this account will be able to open your workspace." |
| `SESSION_SECRET` | Railway generator `${{secret(32)}}` | auto | Mark as a generated/secret field — deployer never sees or types it. |
| `GITHUB_CLIENT_ID` | leave unset (code default applies) | no — advanced/collapsed | "Optional — leave blank to use the shared Outpost sign-in app. Only set this if you're using your own GitHub OAuth App." |
| `GIT_REMOTE_URL` | text input, blank default | no — advanced/collapsed | "Optional. Leave blank to create/clone a repo from inside the app after you sign in." |
| `RAILWAY_API_TOKEN` | text input, blank default | no — advanced/collapsed | "Optional. Adds an in-session shutdown button. Needs a project-scoped Railway token — skip this for a first deploy." |

Also set in the template's service config:
- **Auto-generate a public domain** for the service (so the deployer never has to
  run `railway domain` or find Settings → Networking themselves).
- **Healthcheck path** `/healthz` — already in `railway.json`, should carry
  through automatically since the template deploys from this repo's Dockerfile.

## After creating the template

1. Copy the resulting template URL (`https://railway.com/template/<id>`).
2. Give it to me (or paste it directly) so the README badge's placeholder
   (`REPLACE_ME_WITH_TEMPLATE_ID`) and `docs/DEPLOY_GUIDE.md`'s Step 1 link can be
   swapped to the real one.
3. Do a clean stranger-style test pass before wide publish (Phase 6.3): a fresh
   Railway account (or a friend's), deploying from the template listing alone, no
   out-of-band help — same bar as Phase 2.3's fresh-machine test.
