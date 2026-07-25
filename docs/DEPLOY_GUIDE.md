# Deploying Outpost — a guide for non-technical users

This guide assumes you've never used Docker, Git, or a cloud hosting dashboard
before. It walks through getting your own private FreeCAD workspace running in a
browser tab, step by step, in plain language. If you get stuck, the troubleshooting
section at the bottom covers the most common surprises.

You'll end up with a private web address that only you can open, with a full CAD
program running inside it — nothing to install on your own computer.

## What you need first

- **A GitHub account.** This is where your design files get saved, and it's also
  how you'll prove it's *you* signing in. If you don't have one, create one free at
  [github.com/signup](https://github.com/signup) — just a username, email, and
  password.
- **A Railway account, signed up using that same GitHub account.** Railway is the
  hosting service that actually runs Outpost for you. Go to
  [railway.com](https://railway.com), click "Login," and choose "Login with GitHub."
  Railway will ask you to add a payment method before you can deploy anything —
  this is normal for hosting services and is covered in "About cost" below.

That's it. You do not need to install anything on your own computer.

## Step 1 — Click the deploy button

From [the Outpost repository page](https://github.com/nerd-sniped/Outpost), click
the **"Deploy on Railway"** button near the top of the README. This opens Railway
and starts setting up your own private copy of Outpost.

## Step 2 — Fill in one field: your GitHub username

Railway will show a short setup form. There's only one thing you *must* fill in:

- **`ALLOWED_GITHUB_USER`** — type your GitHub username here (the same account you
  logged into Railway with). This is what makes the workspace yours and yours
  alone: anyone who tries to open your link and signs in with a *different* GitHub
  account gets turned away automatically.

Everything else on the form already has a working default — you can leave it as
is. Click **Deploy**.

## Step 3 — Wait for the first build (get a coffee)

The first deploy takes about **5 minutes**. Railway is downloading and setting up
a full copy of FreeCAD in the background — this is normal and only happens once.
You'll see a progress screen with scrolling log text; that's expected, not an
error. If it's still going after 10 minutes, see Troubleshooting below.

When it says your deployment is live, open **Settings → Networking** on your
service and make sure a public domain is generated (a `something.up.railway.app`
address). Click it, or copy it into your browser.

## Step 4 — Sign in

Your new Outpost address will show a short numeric/letter code and a link to
`github.com/login/device`. Click that link (opens in a new tab), and when it asks
for a code, type the one Outpost showed you. Approve the request.

Switch back to the Outpost tab — it should now load into a full desktop application
(FreeCAD) running inside your browser. This may take a few seconds the first time.

**If it rejects you:** you signed in with a different GitHub account than the one
you typed into `ALLOWED_GITHUB_USER` in Step 2. Reload the page and sign in with
the matching account instead.

## Step 5 — Using it

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

## About cost

Railway bills for the time your workspace is running, plus a small amount for data
transferred while you're using it. As of this writing that's estimated in the
**$10–20/month range for regular personal use**, but that estimate is still being
verified against real usage (see the project's `docs/OUTPOST_DEV_PLAN.md`, Phase
6.1) — treat it as a rough starting expectation, not a guarantee.

Two things worth doing on Railway's dashboard early on:
- Check **Usage/Billing** after your first session to see real numbers for your
  own usage pattern.
- If Railway offers a spending limit or budget alert in your account settings,
  turn it on — it's the easiest way to avoid a surprise.

There's also a shutdown button (a small power icon in the corner of the FreeCAD
window) if you set the optional `RAILWAY_API_TOKEN` variable — it stops the server
completely when you're done, so nothing runs (or bills) while you're not using it.
This is optional and skippable for a first deploy; the guide above works without
it, just note you may want to manually stop the deployment from Railway's dashboard
when you're not using it for a while.

## Troubleshooting

- **Stuck on "Building" for more than ~10 minutes.** Click into the deployment's
  logs in Railway's dashboard — if it's still scrolling text, it's still working.
  If it shows a red "failed" state, something's wrong with the deploy itself, not
  something you did in Step 2 — worth checking the repository's issues page.
- **The page never loads / times out.** Double check Step 3 finished (the
  dashboard shows the deployment as "Active," not "Building" or "Crashed"), and
  that you're using the exact `.up.railway.app` address from Settings → Networking.
- **"Access denied" after signing in with GitHub.** You signed in as the wrong
  account — see Step 4.
- **You lost a device you were signed into.** See `SECURITY.md` in the repository
  for the two-step "panic procedure" (rotate your session secret, revoke the
  GitHub authorization). It takes about a minute and immediately locks out any
  device using your old session.
