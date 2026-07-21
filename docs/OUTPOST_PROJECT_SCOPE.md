# Outpost — Scope & Tech Stack

**Status:** Draft v1
**Name:** Outpost. A browser-based FreeCAD workstation, reachable from anywhere. "FreeCAD" appears in descriptive copy, not the wordmark (trademark + naming-collision reasons — see naming notes below).
**Strategy:** Rung 1 (self-host, free, any hardware) + Rung 2 (Railway template, paid convenience, 25% usage kickback to the project). Rung 3 (SaaS) explicitly out of scope.
**Sister project:** `nerd-sniped/GitPDM` (data layer, the engine Outpost runs on). This repo contains zero FreeCAD logic.

---

## 1. The idea, evaluated

### The pitch in one sentence

Outpost is a stateless, disposable FreeCAD workstation reachable from any browser, where all durable state lives in a git host — so the server can die, the client can die, and the work survives both.

### Why this holds together

**The architecture is self-reinforcing.** Every major decision feeds the others:

- GitPDM keeps state in git → the server is disposable → scale-to-zero is safe → costs collapse → the Railway template is viable → the kickback funds the project → which funds GitPDM. Nothing here is load-bearing on a vendor: the same Dockerfile runs on a home box, Railway, Fly, or a bare VPS.
- Device flow means **one pre-registered OAuth app serves every instance ever deployed**, with no callback URLs and no hosted auth broker. This is the single property that makes a template-distributed product possible without the project taking custody of anyone's credentials.
- **The client is stateless too — with per-rung honesty about the session.** The phone is only a screen; work never lives on it. On **rung 1** (always-on box), a dead client is trivial: the FreeCAD process is still running server-side with the document open — reconnect from any device and resume. On **rung 2**, Railway's serverless sleep *stops the process* after ~10 idle minutes, so the live session does not survive an abandoned connection. The durability story there is layered instead: continuous checkpointing (GitPDM R2.5) pushes work on idle, and the SIGTERM handler saves + pushes on platform stop. Net guarantee, both rungs: **lose at most ~a minute of work, from any failure of any device on either end.** Never claim "the session survives" for rung 2.

### The honest risk register, ranked

1. **FreeCAD on a touchscreen** — the biggest unknown, bigger than streaming. Selkies handles touch input translation, but FreeCAD's UI assumes a mouse: dense toolbars, small hit targets, hover states, right-click context menus, middle-drag orbit. An iPad with a keyboard and mouse/trackpad (or stylus) will be genuinely good. Bare-finger phone CAD will be "check a dimension, tweak a value, commit" — not full modelling. **Scope the phone story as field access, not field authoring**, and it's honest; promise more and it disappoints.
2. **llvmpipe performance** — still unmeasured (carried open question). No GPU means software rasterisation. Determines whether the Railway tier is pleasant or painful on real assemblies. Test before publishing the template.
3. **Egress economics** — video streaming inverts normal cloud costs. ~15 Mbps ≈ 6.75 GB/hr. On Railway this bills as network egress on top of the ~$5/mo Hobby minimum. Estimate honestly in the template description: **~$10–20/mo for regular use**, not "$5."
4. **Railway platform risk** — kickback terms, serverless behaviour, and pricing can change. Mitigation is already built in: the Dockerfile is portable, and rung 1 is the escape hatch. Never build anything Railway-specific outside the template manifest.
5. **Audience overlap** — people who want browser CAD *and* chose FreeCAD is a niche within a niche (Onshape exists and is browser-native). The mitigation is that rung 2 costs the project nothing to offer. Ondsel's 2024 shutdown is the cautionary tale for building a *company* here; it is not an argument against publishing a template.

### Positioning: browser-FreeCAD is a commodity — the workflow is the product

Two adjacent projects must be acknowledged up front, because the first question on any launch thread will be "how is this different?":

- **`linuxserver/freecad`** — a maintained, free, Selkies-based FreeCAD-in-a-browser image. Anyone can run it today. This project *builds on it* (see tech stack) rather than competing with it.
- **SealSkin** (Selkies project / LinuxServer) — a self-hosted VDI portal: API control plane + Caddy proxy, browser-extension entry point, passwordless RSA-keypair auth, launching many app containers via Docker on an always-on host. Excellent for the "one server, many apps, collaboration invites" persona — and GitPDM works fine inside its FreeCAD sessions (worth a docs sentence; it's a distribution channel, not a threat). What it structurally is not: stateless, scale-to-zero, PaaS-deployable, or reachable from an arbitrary borrowed browser in 20 seconds — its per-device extension + key-config model is the opposite of the field scenario.

**Therefore the pitch is never "FreeCAD in a browser."** It is: *identity-gated, git-native CAD where auth, versioning, and durability are one system — deployable anywhere, killable everywhere, loses at most a minute of work.* The README and template description lead with the workflow, not the streaming.

**Security note (the door key opens more than a door):** FreeCAD's Python console means a valid session is arbitrary code execution as the container user — including reading the git token. A stolen logged-in device is total compromise of the repos and the box. Mitigations shipped, not bolted on: session cookies default to 24–48 h; docs include a **panic procedure** (rotate `SESSION_SECRET` to kill all sessions + revoke the GitHub authorization); and the docs state plainly that the door-key auth requests `repo` scope because it doubles as the git credential.

### Verdict

Proceed. The two-rung strategy has near-zero downside: rung 1 is valuable to you personally regardless of adoption, rung 2 is free to offer and pays for itself if anyone uses it. The kill-criteria for rung 2 are cheap to evaluate: if llvmpipe is unusable on representative models, or first-month template deployments show bills wildly above estimate, pull the template and rung 1 loses nothing.

### 1a. Escape hatches (vendor risk map)

The architecture's portability is deliberate, not incidental — Dockerfile-as-product, device flow, statelessness, and provider abstraction were each chosen partly because they keep every vendor swappable. This table names the exit per layer and what would trigger taking it. **It is a map, not a work plan**: no preemptive abstractions (no `StreamingProvider` interface, no parallel deploy targets). Code stays concrete until an exit is actually taken.

| Layer | Current bet | Fallback | Switch cost | Risk level | Switch trigger |
|---|---|---|---|---|---|
| Streaming | Selkies | **KasmVNC** | Days — deepest coupling in the stack | Low (active project) | Project abandonment, or a Selkies regression blocking a FreeCAD version bump |
| Rung-2 platform | Railway | **Fly.io** | ~1 day | Moderate | Kickback/serverless/pricing terms change materially; Phase 4.3-style cost validation fails on re-check |
| Mesh access | Tailscale | **Headscale** (self-hosted control plane, same clients) | Hours | Low | Free-tier terms change; user demands full self-hosting |
| Forge | GitHub (rung 2) + generic (rung 1) | Named providers via generic path; first-class later | Config value, no rearchitecture | Solved | Demand for a named non-GitHub host through the gatekeeper |
| FreeCAD packaging | AppImage | conda/pixi | Hours | Low | AppImage distribution deprecated |

Standing guardrails that keep the exits open:

- **Gatekeeper stays streaming-agnostic.** It proxies generic HTTP/WebSocket and knows nothing Selkies-specific. This single boundary is what makes the streaming swap a Dockerfile edit rather than a rewrite — protect it in review.
- **Nothing Railway-specific outside `railway.json`** (restated from §2). The Phase 5 Fly port exists to *prove* this exit works rather than assume it — one planned spike, not a second maintained template.
- **Headscale gets one sentence in the rung-1 docs**, no support commitment. The container doesn't care which control plane the Tailscale sidecar talks to, so the fully-vendor-free persona is served at zero cost.

---

### Naming

**Outpost.** Chosen over compound options like "FreeCAD Forge" or "CADForge" for two concrete reasons: "FreeCAD" is a trademark of the FPA, so baking it into a product wordmark implies an affiliation that doesn't exist; and "___CAD"/"CAD___" is one of the most saturated naming conventions in the industry (ForgeCAD already exists as a shipping CAD product — a direct collision one word-swap away from CADForge). Outpost cleared a collision check against the CAD, PDM, and git-tooling space; the one adjacent hit worth knowing about is AWS Outposts (hybrid cloud infrastructure — different category, same general "deploy anywhere" theme), which affects SEO distinctiveness more than it poses any real conflict. "FreeCAD" stays in descriptive copy ("Outpost — a browser-based workstation for FreeCAD") rather than the name itself. Verify org/domain handle availability before publishing the repo publicly; `outpost` alone is likely taken given how reused the word is — a qualified handle (`outpost-cad`, `freecad-outpost`) is the fallback.

---

## 2. Concrete tech stack

One image, two deployment targets. Every component below is pinned by version in the Dockerfile.

### The image

| Layer | Choice | Why / notes |
|---|---|---|
| Base | **`lscr.io/linuxserver/baseimage-selkies`** (or extend `linuxserver/freecad` with a pinned FreeCAD) | Maintained upstream base that already provides the virtual display, a bare WM, Selkies, single-app launch logic, and `RESTART_APP` crash-relaunch. Do **not** hand-build the Xvfb/Openbox/Selkies stack — that maintenance burden belongs upstream. LinuxServer rebased their whole catalog onto Selkies; this is their mainline |
| CAD | **FreeCAD AppImage**, `--appimage-extract` at build | Pinned via `ARG FREECAD_VERSION`. Extract, don't FUSE-mount — FUSE in containers is privilege pain |
| PDM | **GitPDM `v0.6.3`**, pinned release tag | Installed into `Mod/`; requires GitPDM R3.1 (tagged releases). Re-verify `gh run list --branch v0.6.3` is green before build; if a later tag ships, re-verify and bump the pin the same way — don't pin on tag existence alone (confirmed failure mode: v0.4.0 was tagged before its own CI workflow existed) |
| Visual diff | **HistoryWorkbench**, pinned (`ARG HISTORY_WB_VERSION`) | Installed alongside GitPDM as a separate addon (LGPL-2.1 — runtime interop only, never vendored). The image ships only version pairs that passed the GitPDM↔HW CI pair test (GitPDM R5.5c). Makes visual 3D diff a launch feature of the deployment at near-zero cost |
| Streaming | **Selkies** (provided by the base image; WebSocket transport default) | Browser-native, touch input support, H.264/VP8 software encode ~150% CPU at 1080p, no TURN server needed on WebSocket mode |
| Door key | **`gatekeeper`** — a small (~200-line) auth shim, Python/FastAPI or Go | Sits in front of Selkies. Runs GitHub device flow, compares the authenticated login to `ALLOWED_GITHUB_USER`, sets a signed session cookie (**default 24–48 h**, not longer — see security note below), proxies to Selkies. The same token is written to `GITPDM_TOKEN_FILE` on tmpfs — **one auth = door key + git credential**. Must also set `GITPDM_PROVIDER` to match (the token is used for both host-API calls *and* git-over-HTTPS auth; they must agree). Stays streaming-agnostic: generic HTTP/WebSocket proxy only |
| Boot / shutdown | `entrypoint.sh` + base-image init hooks | Shallow-clone the repo (`--depth 20` — GitPDM's own `DEFAULT_SHALLOW_CLONE_DEPTH`) if `GIT_REMOTE_URL` set; health endpoint on `/healthz`. **SIGTERM handler**: GitPDM ships `core.checkpoint.register_sigterm_handler()` and `run_shutdown_checkpoint()` specifically for "a headless deployment's own process supervisor wires this in" — Outpost calls these rather than reimplementing save-on-terminate. This is what makes scale-to-zero safe for unsaved work |
| State | **No volume by default** | Clone-on-boot; durable state is the git host. A volume is an opt-in optimisation, never a requirement |

### Self-host target (rung 1)

```
docker-compose.yml
├── outpost            (the image above)
└── tailscale          (sidecar; TS_AUTHKEY or interactive login)
```

- Access via `https://freecad.<tailnet>.ts.net` — Tailscale serves TLS, ACLs are the auth layer, **gatekeeper is disabled** (`AUTH_MODE=tailscale`), zero open ports.
- Config: `.env` with ~5 documented values (`FREECAD_VERSION`, `GITPDM_PROVIDER`, `GIT_REMOTE_URL`, `GITPDM_TOKEN`, `TS_AUTHKEY`). This persona edits `.env`; that is fine. (`GITPDM_PROVIDER`/`GITPDM_TOKEN` are GitPDM's actual env contract — see integration section.)
- Forge support (day one): **GitHub, or any git remote via the generic provider** (`GITPDM_PROVIDER=generic` + `GITPDM_TOKEN`/PAT-in-URL or ambient SSH). This covers the sovereign persona's self-hosted Gitea/Forgejo/bare-SSH without surfacing those as *named* providers. The four other named providers GitPDM supports (GitLab, Bitbucket, Gitea, SourceHut) are reachable via the generic path today and get first-class Outpost surfacing later — see provider scoping note below.
- Optional compose overlay `gpu.yml`: NVIDIA runtime + NVENC encode for Selkies. A spare desktop with a cheap GPU outperforms any affordable cloud tier.

### Railway target (rung 2)

```
railway template
└── outpost            (same image; AUTH_MODE=gatekeeper)
```

- Railway terminates TLS and provides the public hostname — no Tailscale, no Caddy.
- **Serverless mode on**: no requests for 10 min → sleeps, no compute charges → wakes on next request. The live video stream keeps it awake during use naturally.
- Template variables the deployer fills at click-time: `ALLOWED_GITHUB_USER` (required), `GIT_REMOTE_URL` (optional — can also be set later from inside FreeCAD). Nothing else.
- Forge support (day one): **GitHub only.** Device flow is GitHub-shaped and GitHub is the sole provider with device flow in GitPDM today — scoping the gatekeeper to GitHub keeps it the ~200-line shim it's meant to be. Other providers through the *door* would mean a second auth path (PAT-paste, a worse UX); deferred by design. Self-hosters who want another forge use rung 1's generic path.
- `SESSION_SECRET` auto-generated by Railway's template variable functions, never typed by a human.

### What is deliberately absent

No Kubernetes, no TURN server, no reverse-proxy config for users to write, no database, no volume by default, no auth broker hosted by the project, no Railway-specific code outside `railway.json`.

### GitPDM integration contract (pinned to v0.6.3)

Outpost depends on GitPDM's **headless surface**, which is FreeCAD-agnostic by construction and unit-tested with FreeCAD absent. The reference is GitPDM's own `GITPDM_ARCHITECTURE_AND_DEVIATIONS.md`; the load-bearing facts for Outpost:

**Credential handoff is entirely environment variables — no keyring, no file editing:**
- `GITPDM_TOKEN_FILE` (highest precedence) → gatekeeper writes the token here on tmpfs.
- `GITPDM_PROVIDER` → must match the token's host (`github`|`gitlab`|`gitea`|`bitbucket`|`sourcehut`|`generic`); used for *both* host-API and git-over-HTTPS auth, so a mismatch breaks pushes silently.
- `GITPDM_HOST` → host for the identity-check API (defaults `github.com`).
- `GITPDM_ALLOW_FILE_TOKENS=1` → only needed for the on-disk file store; the tmpfs `GITPDM_TOKEN_FILE` path does **not** require it. Set it only if Outpost ever persists a credential to `~/.config/GitPDM/`.
- Token values are injected into git by name only (`_headless_credential_args()` references `$GITPDM_TOKEN`/`$GITPDM_TOKEN_FILE`) — they never appear on a command line or in a process listing. Outpost's own logging must uphold the same invariant.

**Auth verification is a real CLI Outpost can call:** `python -m freecad_gitpdm.auth.check` resolves the chain, hits the host identity API, prints `OK — source=… provider=… host=… login=…` (exit 0) or `FAILED — …` (exit 1). This is the container health/auth probe — use it directly rather than building one.

**The chain does not prompt when headless.** `resolve_credential()` fully implements only file→env→keyring; the `interactive_resolver` seam exists but is unwired (GitPDM deviation 2.9). With no credential present it returns `None`/exit 1 — never hangs, never prompts. This is the desired container behavior, but it means **Outpost's rung-1 first-run flow (R2.4) cannot rely on GitPDM's chain auto-launching device flow** — gatekeeper (rung 2) sidesteps this by pre-populating the token file; rung 1's panel-driven first-run needs Outpost (or a future GitPDM wiring) to trigger auth explicitly.

**Checkpointing is already built and default-on — Outpost consumes, doesn't build it:**
- `core.checkpoint` is FreeCAD-agnostic with injected `is_busy`/`save_if_dirty` callables. Outpost wires `register_sigterm_handler()` + `run_shutdown_checkpoint()` for clean checkpoint-on-terminate (the intended headless hook).
- Checkpoints commit to a shadow branch `refs/heads/gitpdm/recovery` (raw plumbing, never touches HEAD/working tree), **auto-push default-on everywhere** (deviation 2.5). Outpost must *tolerate frequent background pushes* — this is real egress on rung 2 and factors into the cost model. It can be pinned off via `checkpoint_auto_push_override=False`, but that's a FreeCAD-settings write (not headless-reachable), so in practice Outpost accepts the pushes.
- **The recovery branch is internal plumbing:** it churns, force-resets after every real commit, and must **never be branch-protected** on the git host or GitPDM's pruning fails. If Outpost's docs tell users to protect branches, exclude `gitpdm/recovery` and `gitpdm/presence`.

**Storage mode is gone — delta is the only behavior (deviation 2.2).** There is no LFS, no storage-mode choice, no `git lfs install` anywhere. Outpost must **never write `storageMode` into `.freecad-pdm/config.json`**. If Outpost ingests a pre-existing repo carrying a legacy `"storageMode": "lfs"`, GitPDM shows a one-time notice but does not auto-migrate — don't assume its absence. (This retires the old R1.1–R1.3 storage-coupling work entirely; see build-phases note.)

**No hard file locking exists (deviation 2.1/2.6).** `gitpdm/presence` is an advisory cross-user "who else has this open" signal; `session_lock.py` is an advisory same-container-two-tabs guard. Neither enforces exclusivity. If Outpost ever needs true single-writer mutual exclusion (it doesn't, at user scale 1), it builds that itself against gatekeeper — GitPDM won't stop two writers racing.

**Version truth is the git tag, not `__version__` (deviation 2.12).** Pin Outpost's image to a tag whose CI is confirmed green (`gh run list --branch <tag>`); don't read `freecad_gitpdm.__version__`, which can drift between releases.

**Install path caveat (deviation 2.13):** installing GitPDM via FreeCAD's Addon Manager does *not* pull `secretstorage`/`keyring` (Linux/macOS) — but Outpost uses the env-var credential path, which needs neither. If the image installs via `pip install` into FreeCAD's Python (recommended), the deps resolve normally anyway.

### Provider scope: GitHub + generic first, named providers later

GitPDM already implements five named providers (GitHub, GitLab, Bitbucket, Gitea/Forgejo, SourceHut) plus a fully-functional `generic` remote. Outpost deliberately does **not** surface all five at launch — not to save provider-implementation work (there is none; GitPDM did it), but to keep *Outpost's own* auth and test surface small.

**Day one:**
- **Rung 2 (gatekeeper):** GitHub only. One device-flow auth path keeps the door key a thin shim; a second provider through the door means PAT-paste or a parallel auth flow.
- **Rung 1 (self-host):** GitHub **or** `generic` — any git remote via PAT-in-URL or ambient SSH. This is plain `git` against any URL (which clone-on-boot already does), *not* a sixth integration to build. It preserves the sovereign persona (self-hosted Gitea/Forgejo/bare-SSH) at zero cost.

**Deferred to a post-launch gatekeeper phase:** first-class surfacing of GitLab / Bitbucket / Gitea / SourceHut in Outpost's UI and door key. They are reachable via the generic path today; promoting them is a gatekeeper feature, never an image rearchitecture. Two of them (SourceHut, Bitbucket) are flagged by GitPDM as live-unverified — GitHub-first means Outpost isn't the one discovering those breaks.

**Honest framing for docs:** "Outpost's one-click path supports GitHub. Self-hosting? Any git remote works via the generic provider. Other named hosts are on the roadmap." Accurate, not overclaiming, and it keeps the portability promise intact.

---

## 3. User flows (target state)

### Rung 2 — "personal account somewhere" (5 steps, 2 services)

1. Create GitHub account (likely exists) — make an empty repo for CAD files, or skip and do it later from GitPDM.
2. Create Railway account (~2 min, card on file).
3. Click **Deploy** on the template → enter your GitHub username → deploy (~3–4 min build).
4. Open `https://<app>.up.railway.app` → gatekeeper shows a device code → authorise on github.com/login/device from any device.
5. FreeCAD appears in the browser. GitPDM is already authenticated (same token). Clone or create your repo from the GitPDM panel. Work. Save. Commit. Push.

Later, from the field: open the URL on iPad/phone → cookie still valid (or re-auth in ~20 s) → machine wakes in seconds → your session or a fresh clone is there.

Phone explodes → open a laptop → `git clone` (or just open the same URL). Files present within seconds of the last push; the live session itself is also still running server-side.

### Rung 1 — self-host (~15 min, technical)

1. Box with Docker (home desktop, NUC, VPS).
2. `git clone <sister-repo> && cp .env.example .env` → set ~5 values.
3. `docker compose up -d` → click the Tailscale auth URL in the logs once.
4. `https://freecad.<tailnet>.ts.net` from any enrolled device. Done; steady state is electricity.

---

## 4. Build phases

**Phase 0 — GitPDM gates** *(nothing ships before this)*
GitPDM v0.6.3+ headless credentials (env-var chain, confirmed working) + CI-green tag to pin against. The old R1.1–R1.3 storage-mode coupling is **retired** — GitPDM removed storage modes entirely (delta-only), so there is no coupling to build. Checkpointing (former R2.5) is **already shipped and default-on** in GitPDM; Outpost consumes `core.checkpoint`'s SIGTERM hook rather than building it. Exit: GitPDM pushes from a container with no keyring, no SSH key (verified — `auth.check` returns OK against a real token).

**Phase 1 — The image**
Dockerfile, entrypoint, clone-on-boot, health endpoint. Exit: FreeCAD in a browser on `localhost`, commit + push round-trip. **Run the llvmpipe benchmark here** on a representative assembly (~200 parts): if orbit/zoom is not usable at 1080p on ~4 vCPU, the GPU overlay moves from optional to documented-requirement and the Railway template description changes accordingly.

**Phase 2 — Rung 1 complete**
Tailscale sidecar, compose, `.env.example`, docs. Exit: reachable from a phone on cellular. **This is the personal MVP** — everything after is distribution.

**Phase 3 — Gatekeeper**
Device-flow door key, session cookie, token handoff to GitPDM, identity pinning to `ALLOWED_GITHUB_USER`. Exit: unauthenticated visitor sees only a code prompt; wrong GitHub account is rejected; authorised user lands in FreeCAD with GitPDM working, zero additional auth.

**Phase 4 — Rung 2 publish**
`railway.json`, serverless config, template listing with **honest cost estimate ($10–20/mo typical use)**, touch-input pass (Selkies gestures verified on iPad Safari + Android Chrome; FreeCAD toolbar-scale preference documented). Exit: a stranger deploys from the directory and reaches working CAD without contacting you.

**Phase 5 — Field polish** *(demand-driven, not speculative)*
GPU overlay docs, wake-on-LAN guide, iPad input guide (mouse/stylus recommended), FreeCAD touch-profile preset, template README iteration from real user bills.

---

## 5. Open questions (carried + new)

1. **llvmpipe benchmark** — Phase 1 gate. Decides the Railway story. Must be measured **through the browser with the stream active**: render and software encode contend for the same vCPUs, so FreeCAD-only FPS overstates reality.
2. **iPad Safari: decode first, touch second** — a 10-minute Safari *decode* smoke test moves up to Phase 2's field test (does the stream even play cleanly?); the full touch pass (orbit, pan, zoom, right-click emulation) stays in Phase 4. Safari media quirks are the least transferable claim in the plan — SealSkin's own iOS limitations are a warning sign. If Safari is hostile, document "use a mouse / use Chrome" honestly.
3. **Cold-start time on Railway** — image is large (FreeCAD + GStreamer, likely 2–3 GB). Wake-from-sleep should be seconds; *cold deploy* will be minutes. Measure; set expectations in the template.
4. **Kickback mechanics** — verify current Railway template kickback terms at publish time; treat as bonus, not plan.
5. **Session cookie lifetime vs. Railway sleep** — cookie must outlive sleep cycles so wake-ups don't demand re-auth, but the stolen-device blast radius (see security note) caps it. Target: 24–48 h signed cookie; revocation = rotate `SESSION_SECRET`. A daily 20-second re-auth is the accepted price.
