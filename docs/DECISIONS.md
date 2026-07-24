# Outpost — Decision Log

Running record of load-bearing choices, newest first. Each entry: what, why, and
what would reverse it (mirrors the scope doc's escape-hatch philosophy).

---

## D12 — Self-service in-session shutdown control (follow-up to D11)

**Decision:** the gatekeeper now exposes an authenticated `POST /gatekeeper/shutdown`
route that calls Railway's public API (`deploymentStop`) directly, and injects a
small floating button + a `Ctrl+Alt+End` shortcut into the proxied Selkies page so a
logged-in user can trigger it from inside the browser session — no terminal, no
Railway dashboard. Entirely disabled (not just hidden) unless `RAILWAY_API_TOKEN` and
the auto-injected `RAILWAY_DEPLOYMENT_ID` are both present, so it's a no-op on rung 1
(local compose) or any deploy that hasn't opted in.

**Why this exists:** D11 established that Railway's automatic `sleepApplication`
doesn't trigger for this deployment, and that Railway's API has no way to manually
enter that state with auto-wake preserved — `deploymentStop` is the closest lever,
but it's a full stop (no auto-wake on the next visit, matching the accepted manual
`railway down` workaround already in `docs/PHASE4_DEPLOY.md`). The explicit design
goal, stated directly by the project owner: this needs to be usable by someone **not
comfortable with a terminal, ideally not even that comfortable on a computer** — which
ruled out "just document `railway down`" as sufficient and ruled out a
keyboard-shortcut-only control (undiscoverable without being told). A visible button
is the primary mechanism; the shortcut is a bonus for people who prefer it, not a
replacement for it.

**Why a project-scoped token, not an account token:** checked directly (Railway's
docs/support KB, not guessed) — project tokens exist, scoped to one environment,
authenticated via a `Project-Access-Token` header (not `Authorization: Bearer`, which
is account/workspace tokens only). A leaked project token can only stop deployments in
this one project; an account token could act on the operator's entire Railway account.
Given the gatekeeper is a publicly-reachable service (however authenticated), minimizing
blast radius on whatever secret it holds is the obvious call.

**`RAILWAY_DEPLOYMENT_DRAINING_SECONDS` broke container boot outright — reverted,
replaced with a design that doesn't need it.** First attempt: this Railway variable
(SIGTERM-to-SIGKILL buffer on a deployment stop, defaults to `0`) seemed like the
obvious fix for giving Phase 1.4's checkpoint hook (SIGTERM → save dirty document →
push to `gitpdm/recovery`, tested to complete in under 10s) time to run before
Railway kills the container. Set to `30` (Dockerfile default + explicit Railway
variable) and deployed — **the deploy failed outright**, crash-looping on
`s6-overlay-suexec: fatal: can only run as pid 1`. This base image's init system
(`s6-overlay`) hard-requires being PID 1; setting this specific variable appears to
make Railway wrap the container's entrypoint in its own process (to manage the drain
timing itself), which breaks that requirement. Confirmed by reverting alone (both the
Dockerfile `ENV` line and the Railway variable) and redeploying — boot restored
immediately, isolating this one variable as the cause, not the new Go code shipped in
the same deploy.

**Redesigned to control the drain ourselves instead of relying on Railway's
mechanism.** Since the gatekeeper is the one *initiating* the stop (unlike Railway's
own automatic sleep-detection, whose timing isn't ours to control), it doesn't need
Railway's outer buffer at all — `shutdownHandler` now signals FreeCAD directly
(`pkill -TERM -f /opt/freecad/usr/bin/freecad`, hitting the exact same SIGTERM
handler Phase 1.4 already wired and tested) and sleeps `checkpointDrainDelay` (12s —
10s measured need + margin) *before* ever calling `deploymentStop`. By the time
Railway is asked to stop anything, there's nothing left to save, so however abruptly
Railway's own default (`0`s buffer) tears the container down afterward no longer
matters. This sidesteps the PID-1 conflict entirely rather than working around it.

**Implementation notes:**
- No idle-timer, no automatic triggering anywhere in this — a human clicks the button
  or presses the shortcut. The Phase 5 backlog watchdog/proxy service (self-managed,
  automatic) is a separate, much larger undertaking, tracked in
  `docs/OUTPOST_DEV_PLAN.md`, not this.
- The button is injected by rewriting Selkies' proxied HTML response
  (`selkiesProxy.ModifyResponse`), not by patching Selkies' own frontend files —
  keeps this entirely inside `gatekeeper/main.go`, unaffected by base-image bumps.
  Only responses with a `text/html` Content-Type are touched; JS/CSS/WebSocket
  traffic passes through completely untouched.
- The proxy's outbound `Accept-Encoding` header is stripped when this feature is
  active, so the injected response body is never gzip-compressed — sidesteps writing
  a gzip decode/re-encode path in the response modifier for a few KB of HTML.
- The confirm step is a plain `window.confirm()` — deliberately not a custom modal;
  native, reliable, needs no CSS, matches this project's preference for the simplest
  thing that actually works over a bespoke UI for a single yes/no gate.
- The injected page's "shutting down" message is shown optimistically, right after
  confirm — not after the `/gatekeeper/shutdown` fetch resolves. The handler
  deliberately blocks for ~12s before the container can disappear, and the response
  may never cleanly arrive once it does; waiting on it client-side isn't reliable.

**Two more live-test-only bugs found and fixed getting the button to actually
appear:**
1. **Selkies wipes its own DOM after load.** A plain one-time HTML injection was
   confirmed present in the raw response (verified directly with a minted test
   session cookie + curl, bypassing the browser entirely to isolate server-side from
   client-side) but never visible in the browser — Selkies is a single-page app that
   manages its own DOM post-load and was clearing it. Fixed by making the injection
   self-healing: `ensureButton()` runs once immediately and again every 1.5s
   (`setInterval`), re-creating the button via `appendChild` if Selkies has removed
   it. The JS execution context (the timer, the listeners) survives a DOM wipe even
   though the `<script>` tag's own node doesn't.
2. **Browser caching via conditional GETs, immune to an ordinary hard refresh.**
   nginx serves Selkies' page shell as a static file with an `ETag`/`Last-Modified`
   that never changes across Outpost's own deploys (baked into the base image). A
   browser that already had any cached copy could get `304 Not Modified` back
   indefinitely and keep reusing a stale body — a valid, expected response to a
   revalidation request, which a hard refresh does not bypass (304 is genuinely
   correct HTTP behavior, not a caching bug on nginx's part). Fixed by stripping
   `If-None-Match`/`If-Modified-Since` on the outbound proxied request (forces a
   real `200` + body every time) and stripping `ETag`/`Last-Modified` + setting
   `Cache-Control: no-store` on the way back (stops the browser from caching this
   response going forward either). Confirmed fixed the same way — curl with a
   deliberately stale `If-None-Match` header, verifying a fresh `200` came back
   regardless.

**Final scope decision: ships as an owner-only convenience, not a public restart-safe
control — the wake-function idea was considered and explicitly rejected, not just
deferred.** Testing surfaced the actual gap starkly: `deploymentStop` leaves no way
back in except the Railway dashboard or CLI (confirmed live — the URL 502s
afterward, not a friendly page), which defeats the stated design goal of this whole
feature (usable by someone not comfortable with a terminal). The fix on the table was
a second always-on service acting as a front door: check Outpost's status, proxy
through if running, show a "wake it up" button if not. **Rejected after weighing it
against the actual dollar stakes**, not built reflexively: D11 already established
real, measured idle cost (~$0.40/day, ~$12–13/month worst-case bursty personal use)
is small; a permanent proxy hop in front of a WebSocket-heavy, latency-sensitive
streaming app (D10 already fights for every millisecond) is a real, ongoing cost with
no cap, paid on every request forever. The dollar cost of doing nothing is bounded
and small; the latency cost of "fixing" it is unbounded and paid by every future
session. Given that comparison, leaving Outpost running continuously and treating the
shutdown button as an optional, owner-only escape hatch (confirm dialog now says so
explicitly) is the better trade, not a compromise settled for.

**Reverses if:** Railway ships a documented, supported way to manually enter the real
sleep state with auto-wake preserved (removes the need for `deploymentStop` and this
whole button), or a future situation changes the cost/latency trade-off enough to
revisit the wake-function idea (e.g. genuinely idle for weeks at a time, where the
latency cost of a front door would be paid rarely rather than on every session).

---

## D11 — Phase 4.2: `sleepApplication` enabled but not actually sleeping — root cause not isolated, accepted with a manual workaround

**Status: root cause unresolved; workaround decided and in place.** Kept as one entry
(rather than splitting the investigation from the resolution) so the reasoning stays
attached to the evidence that produced it.

**What was tried:** enabled Railway's Serverless (`deploy.sleepApplication: true`,
confirmed live in the service config), redeployed, left the service with no client
connected for ~18 minutes.

**What was found:** a client reconnect after the gap *looked* like a wake (Selkies
logged a fresh `data_websocket` handshake), but pulling actual `CPU_USAGE` /
`MEMORY_USAGE_GB` metrics for that exact window showed continuous, unbroken usage —
no drop to zero anywhere. The container was never actually stopped. What looked like
a wake was Selkies pausing/resuming its own capture pipeline on client
disconnect/reconnect — an application-level behavior, unrelated to the container's
actual running state. No container-boot signature (`gatekeeper: listening on`,
Xvfb/openbox startup) reappeared at reconnect, which a genuine restart would produce
(compare Phase 4.1's redeploy logs, D9's writeup, where that signature is clearly
visible).

**GitPDM's checkpoint scheduler ruled out, two ways.** First by reading
`core/checkpoint.py` directly: `should_checkpoint()` returns `False` immediately
`if not state.dirty` — the max-interval backstop only fires while there are unsaved
edits sitting dirty, it does not push on a bare timer regardless of document state, so
a clean/saved document (or no document at all) should never trigger it. Confirmed
empirically too: redeployed clean, logged in via the still-valid session cookie (a
second live confirmation of D7's self-heal, now surviving a redeploy on top of the
already-open session it was provisioned under), deliberately never opened or cloned
any repo at all, then left it idle ~16.5 minutes. **Still no sleep** — `CPU_USAGE`
stayed at a steady ~0.045–0.047 vCPU baseline (never zero), `MEMORY_USAGE_GB` steady
at ~1.07 GB, for the entire window. With zero GitPDM state in play, it cannot be the
cause.

**Railway's own repeated healthcheck also ruled out.** Pulled `http`-type logs
(Railway's edge proxy log, distinct from container `deploy` logs) for the exact same
idle window: only 3 requests total, all within the first two minutes (the initial page
load, one manual verification `curl`, and the Selkies WebSocket closing ~2 min after
the tab closed) — nothing repeating for the remaining ~14 minutes. So Railway is not
silently re-hitting `/healthz` in the background to keep the container "active."

**Where this leaves it:** the two most obvious, checkable-from-here theories are both
eliminated by direct evidence, not guesswork. What's left is either a low-level daemon
inside the container (NTP time sync, D-Bus, PulseAudio) emitting periodic outbound
packets that wouldn't appear in HTTP proxy logs, or a Railway platform-side gap in how
`sleepApplication` interacts with this container shape. Distinguishing those needs
packet-level egress visibility from inside the container, which isn't available
through the tooling used for this investigation (Railway MCP tools, CLI, deploy/http
log streams). Guessing further past this point would be exactly the kind of
premature, unverified fix this log tries to avoid.

**SSH inspection attempted, inconclusive.** Railway CLI SSH access works (`railway ssh`
— required registering a key via `railway ssh keys add` and pre-accepting
`ssh.railway.com`'s host key via `ssh-keyscan`, since non-interactive sessions can't
accept it themselves). `ps aux` showed nothing unexpected: standard s6-supervised
services (Xvfb, PulseAudio, nginx, dbus, cron, the gatekeeper/healthz binaries,
FreeCAD), no rogue process. `/etc/crontab` + `/etc/cron.d` showed only standard
hourly/daily/weekly/monthly housekeeping (apt-compat, dpkg, man-db, e2scrub_all) —
nothing sub-hourly that would explain continuous activity. Checked `/proc/net/dev`
twice a few minutes apart hoping to catch live byte counters incrementing (or not);
got byte-identical readings both times, suggestive of zero traffic — but then
`/proc/uptime` reported **63 days** against a container that had booted 24 minutes
earlier, revealing `/proc` inside this SSH session isn't reliably scoped to the
container in Railway's sandboxed environment. That undermines trusting the
`/proc/net/dev` reading as authoritative, even though it looked clean — not treating
it as confirmed evidence either way.

**Next step (investigation):** file with Railway support, leading with the
trustworthy evidence (GitPDM ruled out via source + a real bisection test against
actual compute metrics; Railway's own healthcheck ruled out via real HTTP proxy logs;
nothing abnormal in the process list) and noting that container-internal `/proc`
inspection wasn't reliable enough in this environment to go further from the inside —
"`sleepApplication` enabled and confirmed live in config, but never triggers even
with zero application-level activity" is a more useful report than a bare "sleep
doesn't work," and shifts the remaining diagnosis to whoever has visibility into the
platform's own inactivity detection.

**Checked whether we could work around it ourselves — no cheap path exists.** Asked
directly (via Railway's own support knowledge base, not guessed): is there a public
API to manually put a service into the sleeping state on demand, preserving
auto-wake, so the gatekeeper could trigger it itself the moment it sees zero active
clients? **No.** The only deployment-control mutations Railway's public API exposes
are `deploymentStop`, `deploymentRestart`, `deploymentRedeploy`, `deploymentRollback`.
`deploymentStop` is the closest, but it's permanent — it does not preserve
auto-wake-on-next-request, which is fused to Railway's own internal inactivity
detector (the one that isn't firing). Replicating real auto-wake ourselves would mean
building a second, always-on watchdog/proxy service — a real feature, not a quick
fix (tracked as a Phase 5 backlog item in `docs/OUTPOST_DEV_PLAN.md`, not built now).

**Decision: accept manual stop/start as the interim workaround, backed by a real cost
analysis rather than assumption.** Using this deployment's own measured idle rate
(~0.045 vCPU, ~1.1 GB memory) against the account's actual billing rates ($0.000463 /
vCPU / min, $0.000231 / GB / min):

| Scenario | Cost |
|---|---|
| Idle 24/7, never stopped | ~$0.40/day → **~$12/month** |
| Forgot to stop it overnight (8 hrs) | **~$0.13** |
| 3 active hrs/day + rest idle, sleep still broken | ~$0.43/day → **~$12.80/month** |
| Same 3 hrs/day, if sleep worked | **~$2.40/month** |

(Active-use CPU is estimated from Phase 1.5's benchmark — ~0.39 vCPU windowed
average during orbit/edit on comparable hardware, not a direct measurement of this
exact deployment; the idle numbers are real, measured here.)

The stakes are real (~$10/month of the broken-sleep total is pure idle waste worth
eventually fixing) but small enough that neither a support ticket's uncertain timeline
nor an unbuilt watchdog service should block using the deployment now. `railway down
-y` (equivalently, the dashboard's stop control) reliably stops billing —
confirmed live: `latestDeployment` goes to `null`, the domain 404s, cost stops
accruing. Bringing it back is a `redeploy` against the most recent (even `REMOVED`)
deployment, which reuses the build snapshot rather than rebuilding from scratch —
confirmed fast in practice, no multi-minute cold build. Full manual procedure:
`docs/PHASE4_DEPLOY.md` §4.2.

**Noted, not acted on:** once this ships as a public template (Phase 4.5), Railway's
kickback program pays the template creator a percentage of the *deploying user's
usage spend* — meaning an idle instance nobody remembers to stop technically earns
more kickback, not less. This is a real, acknowledged incentive tension, not a
reason to skip building the Phase 5 watchdog — the honest fix (auto-sleep working
correctly) benefits the deploying stranger even though it's not in the template
author's narrow financial interest.

**Reverses if:** Railway support identifies and fixes the root cause (upgrade this
back to "confirmed working," drop the manual-stop workaround), or the Phase 5
watchdog gets built (upgrade to "self-managed, no longer dependent on Railway's
detector").

---

## D10 — Selkies audio + gamepad streams default off (Phase 4.1, live latency observation; `NO_GAMEPAD` required, not just `SELKIES_GAMEPAD_ENABLED`)

**Decision:** `SELKIES_AUDIO_ENABLED=false`, `SELKIES_GAMEPAD_ENABLED=false`, and
`NO_GAMEPAD=true` are now Outpost's own defaults (`Dockerfile` `ENV`, overridable via
`.env`/Railway template vars like everything else). All three are the base image's own
documented env vars (`linuxserver/docker-baseimage-selkies`), not new Outpost plumbing.

**Why:** during the Phase 4.1 live Railway test, round-trip latency measured ~109ms;
manually disabling audio and gamepad streaming in the Selkies sidebar UI visibly
brought it down. Neither stream has any use for a CAD workflow (no in-app sound, no
gamepad input), so there's no tradeoff to weigh. Not deeply benchmarked (no formal
before/after numbers, just the operator's live observation), but free enough that
formal measurement isn't worth gating on.

**Why `NO_GAMEPAD` too, found on the very next redeploy:** shipping just
`SELKIES_AUDIO_ENABLED=false` + `SELKIES_GAMEPAD_ENABLED=false` and redeploying, audio
was confirmed actually disabled server-side (deploy logs: `Server-to-client audio is
disabled by server settings. Not starting pipeline.` — the sidebar toggle icon still
*looks* on, a client-side cosmetic default, but no pipeline runs, so the latency win is
real), but the operator still visually saw gamepad as "on" too. Added `NO_GAMEPAD=true`
expecting it to gate a device-node/interposer creation step; redeployed and checked the
app's own parsed-settings log line to verify directly rather than guess again:
`gamepad_enabled: (False, False)` and `ui_sidebar_show_gamepads: (False, False)` are
both correctly applied — the gamepad section is now fully absent from the sidebar, not
just toggled off. Deploy logs *still* show `Initializing 4 persistent gamepad
instances...` at boot every time, but this turns out to be unconditional internal
socket-listener setup this Selkies release always performs regardless of
`gamepad_enabled` — confirmed by the settings dump showing the config correctly false
right alongside it. Idle listeners with nothing plugged in cost nothing per-frame, so
this line is noise, not evidence the feature is still active.

**Final status: accepted as a known limitation, not fully resolved.** Despite the
server confirming `gamepad_enabled: False` and `ui_sidebar_show_gamepads: False` in
its own parsed config, the operator still visually saw the gamepad section in the
browser after a hard reload — a client-side discrepancy (likely a cached static
frontend bundle or a client-side default that doesn't consult this particular setting)
that wasn't worth chasing further. **Accepted workaround: instruct users to manually
disable gamepad in the Selkies sidebar if latency matters to them** — audio's
functional fix (confirmed, pipeline genuinely never starts) is the real win here;
gamepad's is cosmetic-only at this point. Worth revisiting if a future Selkies base
image version changes this behavior, but not gating Phase 4.1 on it.

**Reverses if:** a future workflow actually wants audio (e.g., screen-recording demos
with narration) or gamepad input (unlikely for FreeCAD) — set the corresponding env
var(s) back per-deployment; no code change needed either way.

---

## D9 — Outpost's internal `/healthz` port moved from :8080 to :8090; gatekeeper refuses to start on a `PORT`/healthz collision (Phase 4.1, found on first live Railway deploy)

**Decision:** `root/opt/outpost/healthz.py` and `gatekeeper/main.go` both now read the
internal healthz target from a single env var, `OUTPOST_HEALTHZ_PORT`, defaulting to
**8090** (was a hardcoded `8080` literal in each file, independently). The gatekeeper
also now hard-fails at startup (`log.Fatalf`, not a silent bind failure) if its own
`PORT` ever equals `OUTPOST_HEALTHZ_PORT`.

**What happened:** the first real `railway up` deploy (Phase 4.1) built and reported
`SUCCESS`, but `/healthz` reliably 502'd and the public root page loaded only
intermittently. Runtime logs showed the gatekeeper logging `listening on :8080` on a
~5s repeating cycle — a crash loop, not a hung process. Cause: Railway injected
`PORT=8080` for this service, which collided with the *old* hardcoded
`healthzUpstream = "http://127.0.0.1:8080"` in `gatekeeper/main.go` — the exact same
port healthz.py binds by default. Whichever process won the bind race that instant
served the request; the loser's `http.ListenAndServe` errored and s6 restarted it,
over and over. This is exactly the class of thing D8 flagged as "verify live, don't
guess" — except it hit the gatekeeper's own listener, not the `shm_size` risk D8 was
actually watching for.

**Why 8090 and not just "un-hardcode it":** the two ports were hardcoded
*independently* in two files (`healthz.py`'s `OUTPOST_HEALTHZ_PORT` default and
`main.go`'s `healthzUpstream` const) with no shared source of truth — they only
happened to agree because nobody had changed one without the other yet. Moving both
to read the same env var (baked into the `Dockerfile` as `OUTPOST_HEALTHZ_PORT=8090`)
makes them structurally unable to drift apart again, which is the actual bug class,
not just today's specific collision with `8080`.

**Residual, unfixed risk:** Selkies' own port (`:3000`, fixed by the linuxserver base
image, not something this Dockerfile controls) could theoretically collide with a
future Railway-injected `PORT` the same way `8080` just did. No preemptive fix is
applied — per this project's own practice, that's a real problem only if it happens,
and the gatekeeper's own `PORT`-vs-`OUTPOST_HEALTHZ_PORT` fatal check at least turns
any *future* internal-port collision into a clean crash-with-log instead of a silent
flaky 502, which is what made this one slow to diagnose.

**Reverses if:** Railway (or another rung-2 host) starts guaranteeing `PORT` values
outside some documented range, making the collision class provably impossible rather
than just less likely at 8090.

---

## D8 — `railway.json`: Dockerfile builder, healthcheck through the gatekeeper's own `/healthz` proxy, `shm_size` left unresolved (Phase 4.1 prep)

**Decision:** `railway.json` (repo root) sets `build.builder: DOCKERFILE` (pointing at
the existing `Dockerfile`, no `startCommand` override — the base image's own s6 init
stays the entrypoint) and `deploy.healthcheckPath: /healthz`. No other deploy-time Compose
setting (`shm_size`, `tmpfs`, `restart`) has a `railway.json` equivalent, and none is
added here as a workaround — see `docs/PHASE4_DEPLOY.md` §5.

**Why `/healthz` and not a Railway-specific probe:** the gatekeeper already proxies
`/healthz` unauthenticated to Outpost's own health service on `:8090` (D6, port changed
by D9) specifically
so a rung-2 platform's healthcheck can reach it without a cookie. Railway's PORT-routed
healthcheck hits exactly this path through the gatekeeper — same code path Phase 3
already verified locally via `compose.gatekeeper.yml`, not a new one built for Railway.

**Why `shm_size` is flagged, not solved:** `docker-compose.yml` sets `shm_size: "1gb"`
for Selkies' GL/browser pipeline — a Compose-only setting with no field in Railway's
`railway.json` schema and no CLI equivalent found. Railway containers get Docker's
64 MB default `/dev/shm`. Whether this actually degrades Selkies under Railway is
**unmeasured** — no privileged remount workaround is applied speculatively, since
Railway containers are unlikely to grant the capability for one anyway and a workaround
for an unconfirmed problem is exactly the kind of premature fix this project's decision
log tries to avoid. Phase 4.1's live test is the actual verification; if it surfaces a
real problem, the fix (lower resolution ceiling, bitrate cap, or an accepted
degradation noted in the template description) gets recorded here as a follow-up
decision, not guessed at now.

**Reverses if:** Railway ships a `railway.json` field for shared-memory size, making
this moot; or the 4.1 test finds no observable degradation, in which case this note is
downgraded from "risk" to "checked, non-issue" rather than reversed.

## D7 — Gatekeeper also provisions git identity + a git credential store, not just `GITPDM_TOKEN_FILE` (Phase 3, found during manual verification)

**Decision:** on every successful login (and self-heal), the gatekeeper additionally
(1) sets `user.name`/`user.email` in `/config/.gitconfig` from the authenticated GitHub
login, and (2) writes a standard git credential-store entry
(`/config/.git-credentials` + `credential.helper=store`) alongside the existing
`GITPDM_TOKEN_FILE` handoff. See `gatekeeper/main.go`'s `configureGitIdentity` and
`configureGitCredentials`.

**Why:** discovered during the real end-to-end verification pass
(`docs/PHASE3_VERIFICATION.md`), not anticipated by design. Two real gaps surfaced:

1. **No git identity anywhere.** GitPDM's first commit failed outright — "Git requires
   user.name and user.email" — because nothing in the stack ever configured one.
   Blocks every fresh deploy identically, not just this test session.
2. **`GITPDM_TOKEN_FILE` doesn't cover every git invocation.** GitPDM's headless
   resolver (`auth.check`, its background `gitpdm/recovery` checkpoint pushes via
   `GitClient`) correctly reads `GITPDM_TOKEN_FILE` — but its UI-driven "Save Into
   Repo" push was observed shelling out to a plain `git push` that never consults it,
   failing with `could not read Username for 'https://github.com': No such device or
   address` (git falling back to an interactive prompt with no terminal attached). This
   is a GitPDM-side inconsistency between its own code paths, not something fixable
   from Outpost's Dockerfile — but a standard git credential store closes the gap at
   the git layer, transparently, regardless of which internal path GitPDM uses.

**Why `--file <path>` and not `--global`:** the gatekeeper runs as root; `git config
--global` resolves relative to *its own* `HOME` (root's), which would silently write to
`/root/.gitconfig` — invisible to GitPDM/git operations that actually run as `abc`
(`HOME=/config`). Caught by testing, not by inspection — the first live attempt showed
credentials being written somewhere FreeCAD's git operations never looked. `--file
/config/.gitconfig` sidesteps the mismatch entirely by not depending on `HOME`
resolution at all.

**Why this doesn't change the threat model:** the raw GitHub token now also lives in
plaintext at `/config/.git-credentials` (`0600`, `abc`-owned), not only inside the
encrypted session cookie and the tmpfs token file. `/config` is a persistent volume
locally (see the `docker restart`/recreate notes in `PHASE3_VERIFICATION.md`) but is
expected to be ephemeral-per-deploy on Railway (Phase 4, matching "no volume by
default"), so this doesn't introduce durable server-side custody beyond what already
exists — the scope doc already treats a valid session as full compromise regardless
(FreeCAD's Python console reads any of these paths anyway).

**Reverses if:** GitPDM ships a fix that routes its manual push action through the same
credential resolver as its headless/checkpoint paths — at that point the git
credential-store half of this decision becomes redundant (identity configuration would
still be needed regardless, since GitPDM has no identity-setting logic of its own).

**Validation note:** the first pass at this fix was live-patched into a running
container (binary hot-swapped via `docker cp`, not a rebuilt image) to unblock a test
session without interrupting it. That patch was lost on the next container *recreate*
(reverted to the stale pre-fix image — anonymous volumes survive a recreate, but the
image-baked binary doesn't), and the recreate's apparent success was a false positive:
the already-provisioned `/config/.gitconfig`/`.git-credentials` files simply survived
on the untouched volume, masking that the running binary could no longer have created
them. Caught by deliberately deleting both files and forcing a clean re-provision
against a properly rebuilt image — confirmed working for real
(`gatekeeper: configured git identity for nerd-sniped` logged from a genuinely clean
state). Lesson: a hot-swap is fine for live debugging, but treat it as unverified until
the actual image is rebuilt and re-tested — don't let a volume's leftover state stand
in for proof the binary itself does the work.

## D6 — Gatekeeper: Go stdlib, always-registered/runtime-gated service, AEAD-encrypted session cookie (Phase 3)

**Decision:** the gatekeeper (`gatekeeper/main.go`) is a single-file Go program, stdlib
only — no third-party dependencies, built in a `golang:alpine` Docker stage and copied
into the final image as a static binary (`/opt/outpost/gatekeeper`); the Go toolchain
itself never reaches the final image. It's launched by a new
`root/custom-services.d/gatekeeper` s6 script that is **always registered** in the image
but idles (`exec sleep infinity`) unless `AUTH_MODE=gatekeeper` — a runtime branch, not a
build-time one.

**Why Go, not Python/FastAPI:** `net/http/httputil.ReverseProxy` has transparently
proxied WebSocket upgrades since Go 1.12 — exactly what Selkies needs — with zero custom
hijack/splice code. Python's stdlib has no equivalent, and reaching for FastAPI would add
a pip-install dependency chain (fastapi, uvicorn, plus a WS-capable HTTP client) purely
for this one shim. Go compiles to one static binary with no runtime deps at all.

**Why runtime-gated, not a build-time branch:** the project is explicitly "one image, two
deployment targets" — branching the *build* would mean two images or baking `AUTH_MODE`
in at build time, breaking the `.env`/Railway-template-variable model both rungs already
use. The idle cost of one `sleep infinity` process is nil, and it binds no port, so rung 1
(`AUTH_MODE=tailscale`/unset) has zero exposure from its mere presence — verified by
inspection: `compose.tailscale.yml`'s `tailscale serve` config targets `127.0.0.1:3000`
directly and never routes through this process.

**Why the session cookie is AES-256-GCM *encrypted*, not merely HMAC-*signed* (as the dev
plan's wording suggested):** the dev plan says "signed session cookie," but a
signature-only cookie can't survive `GITPDM_TOKEN_FILE` living on tmpfs — tmpfs is wiped
on every container stop/start, and Railway's serverless sleep/wake (Phase 4) is exactly
that. Without the token recoverable from the cookie, wake-from-sleep would silently break
"GitPDM already authenticated, no second prompt," one of Phase 3's own tests. Encrypting
the cookie (key = `sha256(SESSION_SECRET)`, payload `{login, token, exp}`) is a superset
of "signed" — GCM gives both integrity and confidentiality — and costs no extra
dependency (`crypto/aes`/`crypto/cipher` are stdlib). Every request with a valid cookie
does a cheap `os.Stat` on the tmpfs path and self-heals it from the decrypted payload if
missing, so sleep/wake is transparent rather than requiring a second device-flow round
trip. This doesn't change the threat model: the scope doc already treats a valid session
as full compromise (FreeCAD's Python console reads the token regardless), and `HttpOnly`
keeps the cookie off-limits to page JS/XSS either way. Rotating `SESSION_SECRET` still
invalidates every session for free — verification derives the AES key from the *current*
env value on every request, so an old ciphertext fails GCM's auth-tag check against a new
key; there is no server-side session table to separately purge.
**Reverses if:** confidentiality-in-cookie is judged an unacceptable risk surface on
review — fallback is plain HMAC, accepting that sleep/wake then requires one re-auth to
repopulate the token file.

**Why `/healthz` and `/authz` are proxied through *unauthenticated*:** worst case they
leak is a GitHub login string (the last line of `auth.check`'s output) — already public
as the operator's own `ALLOWED_GITHUB_USER` value, never the token. A public deploy's own
platform healthcheck needs to reach these without a cookie. This is a narrow, explicit
exception (two fixed paths, proxied unmodified to `:8090`, nothing else) — not a general
hole in "everything requires a session."

## D5 — Tailscale reachability via a shared-netns sidecar + `serve`, userspace mode (Phase 2.1)

**Decision:** ship `compose.tailscale.yml` as an overlay on the base compose. A
`tailscale/tailscale` sidecar owns the network namespace; `outpost` joins it with
`network_mode: service:tailscale`. `tailscale serve` (config in `tailscale/serve.json`)
terminates HTTPS on :443 with a real cert and reverse-proxies to Selkies on
`127.0.0.1:3000`. Tailnet-only — **no Funnel**. Default to **userspace** networking
(`TS_USERSPACE=true`). `AUTH_MODE=tailscale` is set on the outpost service.

**Why shared netns (not a bridge-network proxy):** `tailscale serve` proxies to
localhost within its own netns; sharing the netns is the reliable, documented pattern
and means Selkies binds **no host port at all** — satisfying the phase gate's "zero
listening ports on the public interface" directly, not just by binding loopback.

**Why `ports: !reset []` on outpost:** publishing ports is incompatible with
`network_mode: service:`, and Compose's default merge *appends* the base file's
`127.0.0.1:3000/8080` list rather than replacing it. `!reset []` (Compose ≥ 2.24)
clears it so the overlay contributes zero published ports. The base file stays a
clean localhost-only Phase 1 run on its own.

**Why userspace mode:** portable across hosts without `/dev/net/tun` (Docker Desktop
included, which is the dev box) and needs no `NET_ADMIN`. `serve` works in userspace.
**Reverses to kernel mode** for a Linux host wanting throughput: set
`TS_USERSPACE=false` and add `cap_add: [NET_ADMIN]` + `devices: [/dev/net/tun:/dev/net/tun]`
to the tailscale service.

**Why no Funnel:** rung 1's boundary is the tailnet itself; a full FreeCAD Python
console behind the URL must not face the public internet until the gatekeeper (Phase 3)
pins identity. `AUTH_MODE=tailscale` records that boundary; nothing consumes it yet.

**Validation status:** `docker compose ... config` renders the merged file correctly
(netns shared, ports dropped, cert-domain templating intact). The live tailnet round
trip (2.2 field test — phone on cellular, iPad Safari decode, `ss -tlnp` zero-ports
check) is manual and still pending a real auth key.

## D4 — Startup wiring via a FreeCAD addon, not a command-line script (Phase 1.1)

**Decision:** launch FreeCAD **bare** (`exec /opt/freecad/AppRun`, no arguments) and
put startup wiring (the SIGTERM checkpoint hook) in a FreeCAD addon,
`Mod/OutpostBoot/InitGui.py`.

**Why:** discovered during first live bring-up — passing a `.py` on FreeCAD's command
line runs it in **console mode and then exits**. Under the base's `RESTART_APP`
watchdog that becomes an invisible relaunch loop into an immediate exit: a *blank*
Selkies stream. An addon's `InitGui.py` loads at GUI init, on the main thread (where
`register_sigterm_handler` must run), without exiting. This is also how GitPDM itself
hooks into FreeCAD.

Two adjacent facts nailed down at the same time: the base's real display is
**`DISPLAY=:1`** (Xvfb :1), already exported into the container environment; and the
base copies `/defaults/autostart` → `$HOME/.config/openbox/autostart` and runs it via
`sh` (the shebang is cosmetic there).

## D3 — Base image: `baseimage-selkies` + own pinned FreeCAD AppImage (Phase 1.1)

**Decision:** `FROM lscr.io/linuxserver/baseimage-selkies` and layer in a pinned
FreeCAD AppImage, rather than extending `lscr.io/linuxserver/freecad`.

**Why:** the whole project ethos is honest pinning to *tested pairs* (GitPDM ↔
HistoryWorkbench, R5.5c). `linuxserver/freecad` pins FreeCAD on *their* cadence; we
need `ARG FREECAD_VERSION` under our control so the image only ever ships a FreeCAD
we've paired-tested. The baseimage still provides the display stack, single-app
launch, and `RESTART_APP` crash-relaunch — we do **not** hand-build Xvfb/Openbox/
Selkies.

**Reverses if:** maintaining the AppImage extraction becomes a burden and
`linuxserver/freecad`'s version cadence ends up matching our tested pairs anyway —
then extending their image drops the extraction step. Low likelihood.

## D2 — FreeCAD pinned to 1.0.2 (not 1.0.1, not latest 1.1.x)

**Decision:** `ARG FREECAD_VERSION=1.0.2`, x86_64 conda AppImage, SHA256
`e00be00ad9fdb12b05c5002bfd1aa2ea8126f2c1d4e2fb603eb7423b72904f61`.

**Why 1.0.x not 1.1.x:** GitPDM's `package.xml` declares `<freecadmin>1.0</freecadmin>`
— 1.0 is its stated baseline. FreeCAD 1.1.x is stable and available, but there is **no
GitPDM↔HW pair test yet** (the HW G8 adapter is still a planned spike), so chasing
1.1.x buys risk with no verification behind it.

**Why .2 not .1:** the 1.0.1 conda **py311** AppImage self-reports a mismatch at
startup — *"FreeCAD version (1.0.1) must be at least 1.0.2 to work with Python 3.11 and
above."* 1.0.2 is the patch that clears it, still within the 1.0 baseline. Caught
during the first live browser bring-up (see D4).

**Bump path:** when the GitPDM↔HistoryWorkbench pair test lands (GitPDM G8 / R5.5c),
re-run the benchmark script on a 1.1.x candidate and bump the ARG + SHA256 if green.
CalVer means this is routine, not exceptional.

## D1 — Addons baked image-internal, symlinked into `Mod/` at boot

**Decision:** clone GitPDM (`v0.6.3`, MIT) and HistoryWorkbench (`v0.1.0`, LGPL-2.1)
into `/opt/outpost/addons/` at build; a `custom-cont-init.d` script symlinks them
into `$HOME/.local/share/FreeCAD/Mod/` at boot.

**Why:** linuxserver's home is `/config`, the conventional volume mount point. Baking
addons *directly* into `/config/...` would be wiped by a user-mounted `/config`
volume. Baking them image-internal at `/opt/outpost/addons` and seeding the Mod dir
at boot survives the volume case (scope: "volume is opt-in, never required").
HistoryWorkbench is LGPL-2.1 → **runtime interop only, never vendored** into our
source tree; cloning a pinned tag at build satisfies this.
