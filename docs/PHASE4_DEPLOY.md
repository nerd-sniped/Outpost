# Phase 4.1 — Manual Railway Deploy (runbook)

**Goal (per `docs/OUTPOST_DEV_PLAN.md` 4.1):** same image, `AUTH_MODE=gatekeeper`, deployed
to Railway via the CLI first (before any template listing exists). Full flow from
`docs/OUTPOST_PROJECT_SCOPE.md` §3 works end-to-end on the public URL.

This is CLI-first, not the template flow — `railway.json` (repo root) is the build/deploy
config Railway reads; there is no Railway-native file for *template* click-to-deploy
variables, that's configured in the dashboard later, in 4.5.

---

## 0. Prerequisites

- A Railway account with billing attached (Hobby plan is enough to start; egress is
  metered regardless — see 4.3).
- Railway CLI: `npm i -g @railway/cli` (or the installer at
  <https://docs.railway.com/guides/cli>). Not installed in this dev environment —
  run these steps from a shell where it is.
- The same GitHub OAuth App from Phase 3 (device flow enabled) works as-is — device
  flow ignores the callback URL, so no new app or reconfiguration is needed. Reuse the
  `GITHUB_CLIENT_ID` from your Phase 3 testing.

## 1. Create the Railway project and deploy

```bash
railway login
railway init          # creates a new Railway project, run from the repo root
railway up            # builds Dockerfile, deploys
```

`railway up` builds from the repo's `Dockerfile` per `railway.json`
(`build.builder: DOCKERFILE`). First build will be slow — the image bundles FreeCAD +
the Selkies base (dev plan open question 3: expect **minutes**, not seconds; this
differs from the wake-from-sleep case in 4.2, which should be fast).

## 2. Set environment variables

Railway env vars replace `.env` here — nothing in `.env`/`.env.example` is read on
Railway. Set each with `railway variables set KEY=value`, or via the dashboard:

| Variable | Value | Notes |
|---|---|---|
| `AUTH_MODE` | `gatekeeper` | **Required.** Without this, `custom-services.d/gatekeeper` no-ops (`sleep infinity`) and nothing listens on Railway's injected `PORT` — the deploy will fail its healthcheck. |
| `GITHUB_CLIENT_ID` | (from Phase 3's OAuth App) | Device flow, no secret needed. |
| `ALLOWED_GITHUB_USER` | your GitHub login | Case-insensitive match, per `gatekeeper/main.go`. |
| `SESSION_SECRET` | `openssl rand -hex 32` | Generate fresh — don't reuse the Phase 3 local value. |
| `GIT_REMOTE_URL` | optional | Omit to land in GitPDM's first-run panel instead. |
| `RAILWAY_API_TOKEN` | optional — a **project-scoped** Railway token | Enables the in-session shutdown control (D12): a floating button + `Ctrl+Alt+End` inside the browser session that stops the deployment on demand, working around D11's broken auto-sleep. Create at Project Settings → Tokens, scoped to this project/environment — **not** an account token (blast radius). Omit to leave the control absent entirely (not just hidden). |

**Do not set `RAILWAY_DEPLOYMENT_DRAINING_SECONDS`** — tried as the obvious fix for
giving the checkpoint hook time before shutdown, but it breaks container boot outright
on this base image (`s6-overlay-suexec: fatal: can only run as pid 1` — see D12 for
the full story). The shutdown control handles its own drain timing instead
(`checkpointDrainDelay` in `gatekeeper/main.go`), so this variable is never needed.

Do **not** set `GITPDM_TOKEN` — the gatekeeper populates `GITPDM_TOKEN_FILE` at
runtime after a successful login; a stray `GITPDM_TOKEN` env value would be a leftover
credential sitting in plain env on a platform where that's unnecessary exposure (see
`compose.gatekeeper.yml`'s comment on the same point for the local stand-in).

`PORT` is injected by Railway automatically — the gatekeeper already reads it
(`gatekeeper/main.go`, falls back to 8081 only when unset, which never happens on
Railway). Nothing to set.

## 3. Generate a public domain

```bash
railway domain
```

Railway routes only to the port your process listens on (`PORT`) — since the
gatekeeper is the only thing binding that port, Selkies (`:3000`) and the raw
`/healthz` service (`:8080`) are structurally unreachable from outside the container,
the same property `compose.gatekeeper.yml` proves locally. Nothing extra to configure
for this — flagged here only so it's verified, not assumed, during the test below.

## 4. Test — full flow (mirrors `docs/PHASE3_VERIFICATION.md`, now on a public URL)

- [x] Open the Railway URL → code-prompt page (gatekeeper's device-flow start), not a
      raw Selkies stream or an error. **Verified** at
      `https://outpost-production-eb59.up.railway.app/` — served the gatekeeper's
      sign-in HTML (`<!-- outpost-gatekeeper -->`), not Selkies or a Railway error page.
- [x] Complete device flow as `ALLOWED_GITHUB_USER` → lands in FreeCAD. **Verified.**
      GitPDM's panel shows "not connected" — this is the same known, benign UI quirk
      documented in `docs/PHASE3_VERIFICATION.md` (§3.2, finding 1): the panel's status
      *label* only reflects GitPDM's own keyring-backed token store, never
      `GITPDM_TOKEN_FILE`. The *headless* resolver (what `auth.check` and actual git
      operations use) is unaffected. Don't click GitPDM's own "Connect GitHub" button —
      it's solving a problem that's already solved and fails harmlessly (no OS keyring
      in the container). What actually matters is the functional test below.
- [x] Clone or create a repo from the GitPDM panel. **Verified** — created a new repo
      from the first-run panel (`GIT_REMOTE_URL` was deliberately left unset for this
      pass).
- [x] Edit a part → save → commit → push → verify the commit on the real remote.
      **Verified** — made a design, committed, and pushed successfully, despite the
      panel's toolbar still showing "not connected" (the known cosmetic issue above —
      confirmed benign a second time, now on Railway rather than just local Phase 3).
- [x] Confirm direct access to Selkies/healthz ports is not possible from outside.
      Only one Railway domain routes anywhere; there's no separate address for `:3000`
      to even attempt against, consistent with "no such address exists" rather than a
      live reachability test — matches what this checkbox expected to find.

## 5. Known open risks for this pass — not pre-solved, verify live

- **`shm_size` has no Railway equivalent — checked live, non-issue.** `docker-compose.yml`
  sets `shm_size: "1gb"` for Selkies' GL/browser pipeline (Compose-only setting);
  Railway gets Docker's small default `/dev/shm` (64 MB) instead, with no equivalent
  field to set it higher. During the real editing session (new repo, design, commit,
  push), the stream stayed usable throughout — no crashes, no Chromium/GL failures. It
  felt "slightly less snappy," attributed to real network latency (first true
  over-the-network test) rather than a shared-memory ceiling. Per D8's own reversal
  clause: downgraded from "risk" to "checked, non-issue."
- **Cold build/deploy time — measured: ~4m 22s** (deployment `createdAt` 18:45:06Z →
  `SUCCESS` at 18:49:28Z, first `railway up` from a cold cache). A same-image redeploy
  after the D9 fix (build cache warm) reached `SUCCESS` well under a minute. Feeds the
  4.2 sleep/wake comparison as the "cold deploy" baseline to compare wake-from-sleep
  against.
- **Found and fixed live, not pre-solved: gatekeeper/healthz internal port collision.**
  First deploy built and reported `SUCCESS`, but `/healthz` 502'd and `/` loaded only
  intermittently. Runtime logs showed the gatekeeper crash-looping (`listening on
  :8080` repeating every ~5s) — Railway had injected `PORT=8080`, which collided with
  the then-hardcoded internal healthz target (also `8080`). Fixed by moving both
  `healthz.py` and `gatekeeper/main.go` onto a shared `OUTPOST_HEALTHZ_PORT` env var
  (new default `8090`) plus a fatal startup check if `PORT` ever equals it again. Full
  writeup: `docs/DECISIONS.md` D9. Redeployed and reverified: `/` and `/healthz` both
  stable `200` across 5 consecutive requests, single (non-repeating) "listening on"
  log line.

- **Selkies audio/gamepad tuning (D10):** disabled by default for latency. Audio's fix
  is confirmed functional (server never starts the pipeline). Gamepad's is
  server-confirmed correct in config but still visually shows enabled client-side after
  a hard reload — accepted as a known cosmetic limitation, not gating this phase.
  Documented workaround: tell users to toggle it off manually in the sidebar if it
  matters to them.

**4.1 exit criteria: met.** Every checklist item above is checked on a real public
Railway URL: gatekeeper sign-in page, device-flow login, GitPDM clone/edit/commit/push
round-trip, Selkies/healthz unreachable outside the gatekeeper, `shm_size` a
non-issue, and the one real bug the live test surfaced (D9's port collision) found and
fixed.

---

# Phase 4.2 — Sleep/wake behavior (runbook)

**Goal (per `docs/OUTPOST_DEV_PLAN.md` 4.2):** Railway's serverless mode on. Idle 15
min → confirm sleep (no compute billing); revisit → measure wake time (target:
seconds); cookie survives the sleep (wake ≠ re-auth); an active stream does *not*
sleep mid-session.

**Note on the actual threshold:** Railway's own docs put inactivity detection at
**10 minutes** of no outbound traffic, not the dev plan's 15 — using the real number
for this pass.

Enabled via the Railway MCP `railway-agent` (`deploy.sleepApplication: true`) — no
CLI flag for this exists; the dashboard equivalent is Service Settings → Deploy →
Serverless → "Enable Serverless". Required a redeploy to take effect (staged config,
not live until the next deployment).

## Checklist

- [x] Session cookie survives a container restart. **Verified** — confirmed twice:
      once across an ordinary redeploy (tmpfs `GITPDM_TOKEN_FILE` wiped, landed straight
      back in FreeCAD authenticated, no re-prompt — D7's `ensureTokenFile` self-heal
      working as designed), and implicitly again below.
- [ ] Idle 10+ min with no requests → confirm the service actually sleeps.
      **FAILED — real finding, not yet resolved.** Enabled `sleepApplication: true`
      (confirmed live in service config), left the tab closed ~18 minutes. Reconnecting
      afterward looked like a wake at first glance (Selkies logged a fresh client
      handshake), but pulling actual compute metrics for that exact window
      (`CPU_USAGE`/`MEMORY_USAGE_GB`, 1-min resolution) shows **continuous, unbroken
      usage the entire time** — no drop to zero, no restart spike. The container never
      actually slept; what looked like a "wake" was Selkies' own
      client-disconnect/reconnect pipeline pause-and-resume (`data_websocket`
      teardown/rebuild), a purely application-level behavior that happens on every
      browser tab close/reopen regardless of Railway's serverless feature. No
      `gatekeeper: listening on` line or Xvfb/openbox startup banner reappeared at
      reconnect — the signature a genuine container restart would leave (compare
      against the real boot sequences captured during 4.1's redeploys).
- [ ] Revisit the URL after a *confirmed* sleep → measure wake time. **Blocked** on the
      above — can't measure a wake that isn't happening.
- [ ] Stream actively for ~20 min → confirm no mid-session sleep/interruption.
      **Not run** — no point testing "does it avoid sleeping while active" when it
      isn't sleeping while idle either.

**Bisection run: GitPDM ruled out.** Fresh redeploy, logged in via the still-valid
session cookie (self-heal working again, D7), deliberately never opened or cloned any
repo. Left idle ~16.5 minutes — still no sleep (`CPU_USAGE` steady ~0.045–0.047 vCPU,
`MEMORY_USAGE_GB` steady ~1.07 GB, no zero anywhere). Also checked Railway's `http`
edge logs for the same window: only 3 requests, all in the first 2 minutes — Railway
isn't silently re-pinging `/healthz` either. Both leading theories eliminated by
direct evidence. Full writeup: `docs/DECISIONS.md` D11.

**SSH inspection attempted, inconclusive.** Process list showed nothing abnormal;
cron only has standard hourly/daily housekeeping. `/proc/net/dev` byte counters looked
flat across a multi-minute gap (suggestive of zero traffic), but `/proc/uptime`
reported 63 days against a 24-minute-old container — `/proc` isn't reliably
container-scoped in this sandboxed SSH environment, so the network-counter reading
isn't trustworthy evidence either way. Full writeup: `docs/DECISIONS.md` D11.

**4.2 exit criteria: not met on Railway's own terms** (automatic sleep/wake never
confirmed working) **— accepted with a manual workaround instead.** Root cause not
isolated (see `docs/DECISIONS.md` D11 for the full investigation and cost analysis);
Railway's API has no way to manually trigger the sleep state with auto-wake preserved,
so a real fix means building a watchdog/proxy service (tracked as Phase 5 backlog, not
built now — the cost of not having it is small, ~$12–13/month worst case for bursty
personal use, per D11).

## Manual stop/start procedure (interim workaround)

Since Railway's serverless mode doesn't trigger for this deployment, stop it yourself
between sessions instead of relying on automatic sleep:

**Stop (confirmed, live-tested):**
```bash
railway down -y
```
Confirms via `latestDeployment: null` in `railway status --json`; the public domain
404s immediately after. No compute billed while stopped.

**Resume (confirmed, live-tested — ~58 seconds total, no rebuild):**
```bash
railway redeploy   # or: railway up, if the CLI's redeploy doesn't resolve a target
```
Reuses the most recent build snapshot (even from a `REMOVED` deployment) rather than
rebuilding the Dockerfile from scratch — measured at ~1s to `BUILDING`, ~29s to
`DEPLOYING`, ~58s to `SUCCESS` in practice. Your session cookie survives this (D7's
self-heal) — no re-login needed if you were already authenticated.

**Dashboard equivalent:** the same stop/redeploy actions are available from the
service's Deployments tab (3-dot menu) if the CLI isn't handy.

Filing the `sleepApplication` gap with Railway support (station.railway.com) is a
separate, low-effort action worth doing regardless — see D11 for the evidence to
include — but isn't a blocker for using the deployment in the meantime.
