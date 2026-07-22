# Outpost — Decision Log

Running record of load-bearing choices, newest first. Each entry: what, why, and
what would reverse it (mirrors the scope doc's escape-hatch philosophy).

---

## D7 — GitPDM bumped v0.6.3 → v0.6.4 (Open/Clone first-run flow fix)

**Decision:** `ARG GITPDM_VERSION=v0.6.4`. The tested pair for FreeCAD 1.1.1 is now
**GitPDM v0.6.4 + HistoryWorkbench v0.1.0**.

**Why:** v0.6.3's panel **Open/Clone repo** flow raised
`'GitPDMDockWidget' object has no attribute '_on_github_connect_clicked'` — a refactor
leftover (the GitHub connect handler had moved to `ConnectionsDialog`, but the call site
in `panel.py._on_open_clone_repo_clicked` wasn't re-pointed). Found during the FreeCAD
1.1.1 bring-up ([[D6]]); the bug was version-independent (present on v0.6.3 and main, not
a 1.1.1 issue). Reported upstream with a fix spec; **fixed in GitPDM v0.6.4** — the call
site now routes to `self._connections_dialog.request_github_connect`.

**Reverses if:** a v0.6.4 regression surfaces — but v0.6.3's Open/Clone flow is broken,
so prefer rolling *forward* to a later fix over pinning back.

## D6 — FreeCAD bumped to 1.1.1 (supersedes D2's 1.0.2 pin)

**Decision:** `ARG FREECAD_VERSION=1.1.1`, x86_64 AppImage, SHA256
`e2006138400b2fa85fa2e160e872d00767eb32964e85075830f7e198a3a876e1` (from FreeCAD's
published `...-SHA256.txt` release asset).

**Why now:** D2 pinned 1.0.2 and named the exact release condition — *"when the
GitPDM↔HistoryWorkbench pair test lands, re-run the benchmark and bump if green."* The
GitPDM maintainer (this project's owner) has **verified GitPDM + HistoryWorkbench
working on 1.1.1** in the GitPDM repo, which is precisely the pair-test gate D2 waited
on. With the pair validated by the party who owns it, staying on 1.0.2 no longer buys
safety — it just ships an older FreeCAD.

**Naming change caught in the bump:** the 1.1.x AppImage dropped the `conda-` segment —
`FreeCAD_1.1.1-Linux-x86_64-py311.AppImage` (1.0.2 was `...-conda-Linux-...`). The
Dockerfile's filename template was updated to match. Still py311, so the py311 self-check
that forced 1.0.2 over 1.0.1 (see D2) is a non-issue at 1.1.1.

**Reverses if:** a 1.1.1 regression surfaces in real use — pin back to 1.0.2 (restore
the `conda-` filename segment and the D2 SHA256). The benchmark (`scripts/benchmark/`)
should be re-run on 1.1.1 to refresh the numbers recorded there against 1.0.2.

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

## D2 — FreeCAD pinned to 1.0.2 (not 1.0.1, not latest 1.1.x) — *superseded by [[D6]] (now 1.1.1)*

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

**Decision:** clone GitPDM (`v0.6.4`, MIT) and HistoryWorkbench (`v0.1.0`, LGPL-2.1)
into `/opt/outpost/addons/` at build; a `custom-cont-init.d` script symlinks them
into `$HOME/.local/share/FreeCAD/Mod/` at boot.

**Why:** linuxserver's home is `/config`, the conventional volume mount point. Baking
addons *directly* into `/config/...` would be wiped by a user-mounted `/config`
volume. Baking them image-internal at `/opt/outpost/addons` and seeding the Mod dir
at boot survives the volume case (scope: "volume is opt-in, never required").
HistoryWorkbench is LGPL-2.1 → **runtime interop only, never vendored** into our
source tree; cloning a pinned tag at build satisfies this.
