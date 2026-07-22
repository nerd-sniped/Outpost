# Phase 2 — Rung 1 MVP Verification (results)

**Status:** 2.1 (Tailscale sidecar) **PASS**. 2.2 (field test) **PASS** — LAN, iPad
Safari, and **phone on cellular (Wi-Fi off)** all confirmed usable with good latency.
2.3 (fresh-machine test) **PASS** — brought up and reached working CAD on a second
machine. Only a clean per-session bandwidth number remains to log (non-blocking).

**Phase 2 exit gate: met.** This is the rung-1 personal MVP — tag the milestone.

First live bring-up on 2026-07-21 against a real tailnet, auth key path.

## 2.1 — Tailscale sidecar ✅

The overlay (`compose.tailscale.yml` + `tailscale/serve.json`) brought up clean on the
first run:

- Sidecar authenticated via `TS_AUTHKEY`, node registered as **`freecad`**, MagicDNS
  name `https://freecad.<your-tailnet>.ts.net` reachable with a real (Let's Encrypt)
  cert. `tailscale serve` created the proxy handler for `http://127.0.0.1:3000`
  (Selkies) — WebSocket stream included.
- **Zero host ports** (the phase gate): `docker compose ... ps` shows an empty `PORTS`
  column for both `outpost` and `outpost-tailscale`. Selkies binds only inside the
  shared netns; nothing is published on the host. ✅
- Userspace-networking warnings in the sidecar log (`TPM`, `tun dev stats`,
  `force-set UDP buffer size ... operation not permitted`) are expected and benign —
  the cost of running without `NET_ADMIN`/tun for host portability. No functional
  impact observed.

## 2.2 — Field test (partial)

Confirmed:

- **FreeCAD renders + is interactive** over the HTTPS URL on a laptop (`aeolian`,
  Windows) and on an **iPad Pro (Safari)**. Latency subjectively good — "usable for
  sure." This retires the plan's flagged risk that Safari media handling might be
  hostile to the stream.
- **Connection was direct, not relayed**, for both clients — `tailscale status`
  showed `active; direct 192.168.1.x` for laptop and iPad.

- **Phone on cellular, Wi-Fi off** → reloaded the URL and confirmed **still usable
  with good latency** over the off-LAN path (which routes through a DERP relay, per the
  sidecar's `home is now derp-1 (nyc)`). This is the leg that actually validates the
  remote/phone story — the plan's core "CAD from a phone on cellular" goal is met.

Remaining (minor, non-blocking):
- [ ] Upload bandwidth for a ~10-min session (feeds Phase 4 cost honesty). Cumulative
      `tx` in `tailscale status` is a rough proxy but not a clean per-session number.

## Observation — crash + FreeCAD auto-recovery

On first use a document crashed FreeCAD; the base image's `RESTART_APP` relaunched it
and FreeCAD's **local auto-recovery** offered to restore the document (recovered
successfully, session usable afterward). The FreeCAD log showed:

```
Reading failed from embedded file: WallTrace*.InternalShape.bin (0 bytes, 2 bytes compressed)
<ElementMap> ElementMap.cpp(476): No hasherRef
```

Reading:
- The model was a **FreeCAD sample that loaded automatically** — nothing was cloned.
  Most plausibly a resource/load spike (several things loading at once) under software
  rendering, not a reproducible fault; it did not recur.
- This is **FreeCAD's own transient auto-recovery**, not GitPDM's `gitpdm/recovery`
  branch — a different durability mechanism. The recovery snapshot preserved the
  document tree but not the geometry `.bin` blobs (0-byte `InternalShape`), so FreeCAD
  reported the empty reads and recomputes the shapes. Not an Outpost-side fault.
- `WallTrace` objects = a BIM/Arch (walls) model — the kind of heavy assembly where a
  software-GL (llvmpipe) crash would originate. Logged as a **Phase 1.5 data point**,
  not a bug to chase: one-off, recovered cleanly, did not reproduce.

## Observation — UI scale is device-dependent

On FreeCAD 1.1.1 the streamed UI reads well on a phone but looks oversized on a desktop
(and the reverse can happen depending on which device sizes the display first). Cause:
one server-side UI renders at a fixed `QT_SCALE_FACTOR`, and FreeCAD fixes its DPI at
launch — it doesn't re-evaluate when Selkies resizes the display for a different client.
The baked `QT_SCALE_FACTOR=0.9` (D6) is a legibility stopgap, tunable via `.env`. The
real fix — per-device / adaptive scaling — is logged as a **Phase 5 backlog item**, with
per-device legibility folded into the **4.4 touch pass**. Non-blocking for rung 1.

## 2.3 — Docs + fresh-machine test ✅

Brought up on a **second machine** following the documented setup and reached working
CAD in the browser. The README quickstart + Tailscale section held up on a machine that
wasn't the original dev box. No doc-blocking deviations reported.

Remaining (minor, non-blocking): log a clean per-session upload bandwidth number for
the Phase 4 cost model.
