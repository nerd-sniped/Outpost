# Outpost — Phased Development & Test Plan

**Status:** Draft v1
**Companion docs:** `PROJECT_SCOPE.md` (what & why), `GITPDM_REQUIREMENTS.md` (addon requirements)
**Principle:** every increment ends in something *runnable and testable*, never in scaffolding. Each phase has an exit gate; a phase does not start until the prior gate passes. Kill-criteria are listed where a result could change the plan rather than just delay it.

Suggested cadence labels: **S** (an evening), **M** (a weekend), **L** (a week of evenings). These size effort, not calendar.

---

## Phase 0 — GitPDM dependency verification *(no longer a build phase)*

**Goal:** confirm GitPDM's headless surface is real and pinned before Outpost builds against it. As of GitPDM v0.6.3 the credential engine, checkpoint scheduler, SIGTERM hook, and provider abstraction are all **shipped and tested** — this phase is now verification, not development. (Storage-mode work is retired outright: GitPDM removed modes, delta is the only behavior.)
**Repo:** GitPDM (verify only); Outpost (record the pin).

### 0.1 Confirm the pinned tag is CI-green (S)
`gh run list --branch v0.6.3` (or the current latest) shows a passing run — lint, full suite, architecture guard, container-smoke import. Do **not** pin on tag existence alone (v0.4.0 predated its own CI).
**Record:** the exact tag in Outpost's Dockerfile as `ARG GITPDM_VERSION`.

### 0.2 Live auth smoke test (S) — *the one empirical gap*
With a real, scoped, throwaway PAT: `docker run -e GITPDM_TOKEN=<pat> -e GITPDM_PROVIDER=github <img> python -m freecad_gitpdm.auth.check` prints `OK — source=env provider=github …` and exits 0. This is the success path the audits couldn't close without a token — close it here.

### 0.3 Confirm the headless contract surface (S)
Sanity-check the four things Outpost hard-depends on, per GitPDM's deviations doc: env-var precedence (`GITPDM_TOKEN_FILE` > `GITPDM_TOKEN`); `core.checkpoint.register_sigterm_handler()` + `run_shutdown_checkpoint()` importable with FreeCAD absent; `GitClient` clone/commit/push callable; token value never appears in a captured process list or log.

**Phase 0 exit gate:** tag pinned + CI-green confirmed; `auth.check` returns OK against a real token in a keyring-less container. No GitPDM code written — if something's missing, it's a `GITPDM_AUDIT_FIXES.md`-style punch list back to the GitPDM repo, not work here.

**Known GitPDM-side follow-up (does not block Outpost Phase 1):** the `interactive_resolver` seam is unwired (deviation 2.9), so GitPDM's chain won't auto-launch device flow on a full miss. Rung 2 doesn't care (gatekeeper pre-populates the token). Rung 1's R2.4 first-run flow does — track it for Phase 2, not here.

---

## Phase 1 — The image

**Goal:** FreeCAD in a browser tab on `localhost`. The llvmpipe question gets answered here, deliberately early.
**Repo:** Outpost (new; sister to `nerd-sniped/GitPDM`).

### 1.1 Base image evaluation + pinned FreeCAD (S–M)
Start `FROM lscr.io/linuxserver/baseimage-selkies` (or extend `linuxserver/freecad`) — do **not** hand-build Xvfb/Openbox/Selkies; the base provides the display stack, single-app launch logic, and `RESTART_APP` crash-relaunch. Layer in the pinned FreeCAD AppImage (`ARG FREECAD_VERSION`, extracted) if extending the baseimage directly.
**Test:** browser → exposed port shows live FreeCAD; keyboard + all three mouse buttons work (middle-drag orbit is the CAD-critical one); kill the FreeCAD process inside the container → `RESTART_APP` relaunches it without a container restart (the crash-resilience test); note base-image conventions (ports 3000/3001, `PUID/PGID`, `shm_size`) that constrain gatekeeper design.
**Decision recorded at exit:** extend `linuxserver/freecad` vs. baseimage + own AppImage — driven by whether their FreeCAD version pinning satisfies the update requirement.

### 1.2 Stream quality baseline (S)
**Test:** measure input-to-photon latency roughly with a screen recording of click → response (<100 ms on LAN is the bar); confirm WebSocket transport and software encode settings; record CPU consumed by the encoder at 1080p while orbiting (feeds 1.5's interpretation).

### 1.3 GitPDM + HistoryWorkbench inside (S)
Pinned GitPDM release in `Mod/`; pinned HistoryWorkbench alongside it (`HISTORY_WB_VERSION`); `GITPDM_TOKEN` passed through.
**Test:** full round-trip *through the browser*: open panel → clone → edit part → save → commit → push → verify on the forge. Then commit a second dimension change and open the visual diff via GitPDM's adapter — the comparison renders in the streamed session. (If GitPDM G8 isn't merged yet, HW still ships in the image and is verified to launch standalone; the adapter test activates when G8 lands.)

### 1.4 Entrypoint & lifecycle (M)
Clone-on-boot (`--depth 20`, GitPDM's `DEFAULT_SHALLOW_CLONE_DEPTH`), `/healthz`, and the **SIGTERM checkpoint handler**: wire GitPDM's shipped `core.checkpoint.register_sigterm_handler()` + `run_shutdown_checkpoint()` (the intended headless supervisor hook) so platform stop/sleep triggers a save + recovery-branch push before exit. Outpost provides the `save_if_dirty` callable (FreeCAD document save) and calls the hook — it does not reimplement checkpoint logic. This is what makes Railway's serverless sleep safe for unsaved work.
**Test:** `docker stop` completes <10 s **and** a dirty document's work appears on the `gitpdm/recovery` branch afterwards (kill-mid-edit test); boot with `GIT_REMOTE_URL` set lands in FreeCAD with repo already present; boot *without* it lands in the first-run panel flow. Note: `gitpdm/recovery` auto-pushes by default (GitPDM deviation 2.5) — confirm Outpost tolerates the frequent background pushes and never branch-protects that ref.

### 1.5 🔑 llvmpipe benchmark (S–M) — *the plan's main measurement*
Representative models: one ~50-part and one ~200-part assembly. On ~4 vCPU: record FPS during orbit/zoom/pan at 1080p, plus TechDraw page regen time. **Methodology requirement: measure perceived FPS in the browser with the stream active, not FreeCAD-side render FPS** — rendering and Selkies' software encode contend for the same vCPUs, and FreeCAD-only numbers overstate reality by whatever the encoder steals (which 1.2's CPU measurement quantifies).
**Kill/branch criteria:**
- Orbit ≥ 20 fps on the 200-part assembly → Railway story stands as written.
- 10–20 fps → Railway template ships with "small/medium assemblies" framing.
- < 10 fps → GPU becomes a documented requirement; Railway rung is re-scoped to light use and the template description says so. **The project does not die in any branch — only the marketing changes.**

**Phase 1 exit gate:** browser round-trip test green; benchmark numbers recorded in the repo.

---

## Phase 2 — Rung 1 complete (personal MVP)

**Goal:** you, personally, doing CAD from a phone on cellular. Everything after this phase is distribution, not capability.

### 2.1 Tailscale sidecar (S)
Compose service, `TS_AUTHKEY` or interactive URL from logs, MagicDNS + Tailscale-served HTTPS, `AUTH_MODE=tailscale` (gatekeeper absent).
**Test:** `https://freecad.<tailnet>.ts.net` from a second machine on the tailnet; confirm **zero listening ports** on the host's public interface (`ss -tlnp` / router check).

### 2.2 Field test (S)
Phone on cellular (Wi-Fi off), tablet if available.
**Test:** connect, orbit a model, make an edit, commit, push. Note subjective latency and any CGNAT/DERP relay (Tailscale status shows relay vs. direct). Record upload bandwidth consumed for a 10-min session — this number feeds Phase 4's cost honesty.
**Includes the iPad Safari decode smoke test** (promoted from Phase 4): 10 minutes on Safari specifically — does the stream decode and play cleanly at all? Safari's media handling is the least transferable claim in the plan; if it's hostile, that news must arrive now, not at template publish. (The full *touch* pass stays in 4.4.)

### 2.3 Docs + fresh-machine test (M)
`.env.example` (~5 values, each commented), README quickstart.
**Test:** the real one — a fresh VM or friend's machine, following *only the README*, reaches working CAD. Every deviation found is a doc bug; fix and re-run until clean.

**Phase 2 exit gate:** fresh-machine test passes without out-of-band help. *(Tag this milestone — it's the version you use forever even if rung 2 never ships.)*

---

## Phase 3 — Gatekeeper (the door key)

**Goal:** public-URL deployments are safe. This phase is security-critical; its tests are adversarial, not happy-path.

### 3.1 Standalone shim (M)
~200-line FastAPI/Go service: GitHub device flow, verify authenticated login == `ALLOWED_GITHUB_USER`, signed session cookie (**24–48 h default** — a valid session includes FreeCAD's Python console, i.e. code execution and token access, so the stolen-device blast radius caps cookie life), reverse-proxy to Selkies. Must stay streaming-agnostic: generic HTTP/WebSocket only, nothing Selkies-specific.
**Test (adversarial):**
- No cookie → only the code-prompt page is reachable; Selkies port unreachable directly from outside the container network.
- **Wrong GitHub account completes device flow → rejected** (the identity-pinning test; this is the whole point of the shim).
- Tampered/expired cookie → rejected.
- WebSocket upgrade proxied correctly (Selkies is useless without it — test early, this is the classic proxy footgun).

### 3.2 Token handoff (S)
Gatekeeper writes the token to tmpfs; GitPDM reads via `GITPDM_TOKEN_FILE`. Gatekeeper **must also set `GITPDM_PROVIDER`** to match the token's host — GitPDM uses it for both host-API and git-over-HTTPS auth, so a mismatch breaks pushes silently.
**Test:** after browser auth, GitPDM panel is *already* authenticated — no second prompt. Token absent from `docker inspect` env output and from any log line (grep the full log for the token string).

### 3.3 Session lifecycle (S)
**Test:** cookie survives container sleep/restart (signed with `SESSION_SECRET`, not stored server-side); rotating `SESSION_SECRET` invalidates all sessions (the revocation story); re-auth after expiry lands back in the running session, not a dead end.
Also ship the documented **panic procedure** (stolen logged-in device): rotate `SESSION_SECRET` + revoke the GitHub authorization; walk through it once for real as the test.

**Phase 3 exit gate:** all adversarial tests green; a second person *tries* to get in with their own GitHub account and fails.

---

## Phase 4 — Rung 2 publish (Railway)

**Goal:** the deployment mechanics work end-to-end on a real public Railway URL —
the CLI-first deploy itself, and living with (or working around) Railway's own
platform behavior around idle cost. Validating that a *stranger* can do this from a
template listing, and that the published cost claims hold up, moved to Phase 6 —
those need calendar time and other people, not something a single deploy session
proves.

### 4.1 Manual Railway deploy (S)
Same image, `AUTH_MODE=gatekeeper`, via CLI first.
**Test:** full flow from `PROJECT_SCOPE.md` §3 works end-to-end on the public URL.
**Status: exit gate met.** See `docs/PHASE4_DEPLOY.md` §4 and `docs/DECISIONS.md`
D9/D10 for the two real bugs the live test surfaced and fixed along the way.

### 4.2 Sleep/wake behaviour (S)
Serverless mode on.
**Test:** idle 15 min → confirm sleep (dashboard shows no compute billing); revisit URL → measure wake time (target: seconds, record actual); **cookie survives the sleep** so wake ≠ re-auth; active video stream does *not* trigger sleep mid-session (stream for 20 min, confirm no interruption).
**Status: root cause not resolved, accepted with a workaround instead of blocking
here indefinitely.** Railway's `sleepApplication` never triggers for this
deployment despite confirmed-zero application-level activity (`docs/DECISIONS.md`
D11) — filed with Railway support, not guessed at further. A self-managed
watchdog/proxy fix was considered and explicitly rejected (D12): the ongoing
latency cost on every request isn't worth it to avoid a small, bounded idle cost.
Outpost now runs continuously; an owner-only manual shutdown button exists as an
escape hatch (D12), not a public control.

**Phase 4 exit gate: met**, on the terms above — deployment mechanics work, and the
one open platform issue (sleep/wake) has a deliberate, documented resolution rather
than a silent gap.

---

## Phase 5 — Demand-driven polish (no exit gate; backlog, not commitment)

GPU compose overlay (NVENC) + docs · Wake-on-LAN guide · FreeCAD touch-profile preset (large toolbars) · recovery-restore integration test (kill container mid-session → GitPDM offers restore from `gitpdm/recovery` on next boot — exercises GitPDM's already-shipped checkpoint restore, not new code) · Fly.io port of the gatekeeper flow (same image, proves portability claim) · GitLab.com device-flow provider (exercises token refresh in production; note GitLab is PAT-only in GitPDM today per the capability matrix).

**Considered and explicitly rejected, not deferred: a self-managed sleep/wake
watchdog** (a second always-on service fronting Outpost's domain — proxy through if
running, show a "wake it up" page if not, working around `sleepApplication` never
triggering per `docs/DECISIONS.md` D11). Built the shutdown half first (D12: an
in-session button + `Ctrl+Alt+End` calling `deploymentStop`), which surfaced the real
shape of the trade-off: the watchdog would add a permanent proxy hop to every request
of a WebSocket-heavy, latency-sensitive streaming app (D10 already fights for every
millisecond there) to save a bounded, already-small idle cost (~$0.40/day, ~$12–13/mo
worst case, D11). Decided the ongoing latency cost isn't worth paying to avoid a
small, capped dollar cost — Outpost runs continuously now; the shutdown button stays
as an owner-only manual escape hatch (its confirm dialog says restarting needs
dashboard/CLI access), not a public-facing control. Revisit only if the usage pattern
changes to genuinely-idle-for-weeks, where a rarely-paid proxy hop would be a better
trade than it is for routine daily use.

---

## Phase 6 — Rung 2 validation & rollout (post-deployment)

**Goal:** a stranger deploys from the template directory and reaches working CAD
without contacting you. Split out from Phase 4 because these three all need
something a single deploy session can't produce — calendar time, other people's
devices, or an actual stranger — not because they're less load-bearing than 4.1/4.2.

### 6.1 💰 Cost validation (M) — *the honesty gate*
One week of realistic personal use on Railway (~5–10 hrs).
**Test:** actual bill vs. the $10–20/mo estimate. Egress is the number to watch (compare against 2.2's bandwidth measurement). If reality is >1.5× estimate, either tune Selkies (bitrate cap, adaptive framerate, resolution ceiling) or **change the template description** — the estimate must match reality before strangers see it. Note the now-continuous (no sleep/wake) deploy pattern (Phase 4.2) when interpreting this number — it's a real always-on cost, not a sleep-optimized one.

### 6.2 Touch pass (M)
iPad Safari + Android Chrome, finger and stylus/mouse where available.
**Test:** the four CAD-critical gestures — orbit, pan, zoom, right-click emulation — verified per platform; GitPDM commit flow completable by finger (R2.6); findings written into an honest "input devices" doc section (expected outcome: *iPad + mouse = good; bare finger = field access, not authoring*).

### 6.3 Template publish + stranger test (S)
`railway.json`, template listing with the *validated* cost estimate and benchmark-informed assembly-size guidance.
**Test:** someone who is not you, from the directory listing alone, deploys and pushes a commit. Their confusion points are the final doc bugs.

**Phase 6 exit gate:** stranger test passes; published cost/perf claims match measured reality.
**Kill-criterion (cheap, pre-agreed):** if llvmpipe framing + real costs make the template a bad product, don't publish — rung 1 is untouched and the gatekeeper still serves any future host.

---

## Test infrastructure (built once, in Phase 0–1)

- **CI:** image builds on every push; a container-boot smoke test ("FreeCAD launches under the base image and stays up 60 s") plus the `auth.check` probe (Phase 0.2) as the cheapest canaries; the GitPDM↔HistoryWorkbench pair test (GitPDM R5.5c) runs on any bump of either pinned addon version — the image only ever ships tested pairs. Full browser round-trips stay manual — Selkies E2E automation is not worth its maintenance cost at this scale.
- **The benchmark script** (0.3 pack-size + 1.5 llvmpipe) lives in the repo, re-runnable on every FreeCAD version bump — CalVer means version bumps are now routine, and each one should re-verify both numbers.
- **A `SECURITY.md` checklist** from Phase 3's adversarial tests, re-run manually before any release that touches gatekeeper.

## Dependency spine (what blocks what)

```
0 (verify pin) ──► 1.1 ──► 1.3 ──► 1.4 ──► 2.x (rung 1 MVP)
                                  └──► 3.x ──► 4.x (rung 2 deploy) ──► 6.x (validation & rollout)
1.5 gates only *messaging*, never build order
G8 (HW adapter, GitPDM-side) ──► 1.3's visual-diff test
Phase 5 is backlog, not on this critical path at all
```

The critical path to your personal MVP is **Phase 0 verify → 1.1–1.4 → 2.1–2.3**. Because Phase 0 collapsed from a build phase into a verification gate (GitPDM shipped the credential engine, checkpointing, and SIGTERM hook already), this is now roughly **three to five focused weekends** — the credential/checkpoint work that was the largest chunk of the original estimate is done. Everything else is distribution.
