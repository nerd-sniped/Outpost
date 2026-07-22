# Phase 2 — Rung 1 MVP Verification (results)

**Status:** 2.1 (Tailscale sidecar) **PASS**. 2.2 (field test) **PASS** — LAN, iPad
Safari, and **phone on cellular (Wi-Fi off)** all confirmed usable with good latency;
only a clean per-session bandwidth number remains to log. 2.3 (fresh-machine test) not
started.

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
- This is **FreeCAD's own transient auto-recovery**, not GitPDM's `gitpdm/recovery`
  branch — a different durability mechanism. The recovery snapshot preserved the
  document tree but not the geometry `.bin` blobs (0-byte `InternalShape`), so FreeCAD
  reported the empty reads and recomputes the shapes. Not an Outpost-side fault.
- `WallTrace` objects = a BIM/Arch (walls) model. The crash is a likely **Phase 1.5
  (llvmpipe) stability data point**: software-rendering a heavy BIM assembly is exactly
  where a software-GL crash would originate.
- Follow-up: identify the model (FreeCAD sample vs. cloned repo) and whether re-opening
  reproduces the crash. If it reproduces, it belongs in the 1.5 benchmark notes.

## 2.3 — Docs + fresh-machine test

Not started. The README quickstart + Tailscale section exist; the real test (a fresh
machine reaching working CAD from the README alone) is pending.
