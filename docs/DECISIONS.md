# Outpost — Decision Log

Running record of load-bearing choices, newest first. Each entry: what, why, and
what would reverse it (mirrors the scope doc's escape-hatch philosophy).

---

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
exception (two fixed paths, proxied unmodified to `:8080`, nothing else) — not a general
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
