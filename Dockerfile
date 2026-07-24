# syntax=docker/dockerfile:1
#
# Outpost — a browser-based workstation for FreeCAD, git-native via GitPDM.
# One image, two deployment targets (rung 1 self-host, rung 2 Railway).
#
# We build on linuxserver's Selkies baseimage (display stack + single-app launch +
# RESTART_APP crash-relaunch) and layer in a *pinned* FreeCAD AppImage so the image
# only ever ships a FreeCAD we control. See docs/DECISIONS.md (D1–D3).

# ARGs used in a FROM must be declared before the *first* FROM to stay visible to
# every stage — redeclaring after a stage starts scopes them to that stage only.
ARG GO_VERSION=1.23
ARG SELKIES_BASE=lscr.io/linuxserver/baseimage-selkies:ubuntunoble

# --- Gatekeeper (Phase 3) builder: compiles to a static binary; only the binary
#     (not the Go toolchain) crosses into the final image below. ---
FROM golang:${GO_VERSION}-alpine AS gatekeeper-build
WORKDIR /src
COPY gatekeeper/ .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gatekeeper .

FROM ${SELKIES_BASE}

# --- Pins (all four are the knobs you bump; keep them in sync with docs/) ---
ARG FREECAD_VERSION=1.1.1
ARG FREECAD_SHA256=e2006138400b2fa85fa2e160e872d00767eb32964e85075830f7e198a3a876e1
ARG FREECAD_ARCH=x86_64
ARG GITPDM_VERSION=v0.6.3
ARG HISTORY_WB_VERSION=v0.1.0

LABEL org.opencontainers.image.title="Outpost" \
      org.opencontainers.image.description="Browser-based FreeCAD workstation, git-native via GitPDM" \
      org.opencontainers.image.source="https://github.com/nerd-sniped/Outpost"

# Build/runtime deps beyond the base: git+ca-certs for clone-on-boot, python3 for the
# headless auth.check probe and /healthz, mesa dri for llvmpipe software GL, curl to
# fetch the AppImage. FreeCAD's conda AppImage bundles its own Qt/Python — we don't
# apt-install those.
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      git ca-certificates curl python3 \
      libgl1-mesa-dri libglu1-mesa && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# --- FreeCAD AppImage: verify, extract (no FUSE), place at /opt/freecad ---
RUN cd /tmp && \
    AI="FreeCAD_${FREECAD_VERSION}-Linux-${FREECAD_ARCH}-py311.AppImage" && \
    curl -fsSL -o "$AI" \
      "https://github.com/FreeCAD/FreeCAD/releases/download/${FREECAD_VERSION}/${AI}" && \
    echo "${FREECAD_SHA256}  ${AI}" | sha256sum -c - && \
    chmod +x "$AI" && \
    ./"$AI" --appimage-extract && \
    mv squashfs-root /opt/freecad && \
    rm -f "$AI" && \
    # Sanity-check the extraction, but with HOME off /config: the base sets HOME=/config
    # even at build, and a bare `AppRun --version` would bake root-owned FreeCAD config/
    # cache dirs into /config that abc then can't write (GUI exits after splash).
    env HOME=/tmp/fcprobe /opt/freecad/AppRun --version || true; \
    rm -rf /tmp/fcprobe

# --- Addons baked image-internal (survives a /config volume mount); seeded into
#     FreeCAD's Mod/ at boot by custom-cont-init. HistoryWorkbench is LGPL-2.1:
#     runtime interop only, never vendored — pinned-tag clone at build satisfies that. ---
RUN mkdir -p /opt/outpost/addons && \
    git clone --depth 1 --branch "${GITPDM_VERSION}" \
      https://github.com/nerd-sniped/GitPDM.git /opt/outpost/addons/GitPDM && \
    git clone --depth 1 --branch "${HISTORY_WB_VERSION}" \
      https://github.com/eblanshey/HistoryWorkbench.git /opt/outpost/addons/HistoryWorkbench && \
    rm -rf /opt/outpost/addons/GitPDM/.git /opt/outpost/addons/HistoryWorkbench/.git

# Overlay: /defaults/autostart, /custom-cont-init.d/*, /custom-services.d/*, /opt/outpost/*
COPY root/ /

# Phase 3: the gatekeeper's static binary (see builder stage above). Lives in
# /opt/outpost alongside the other Outpost-owned binaries, not under root/ — it's
# build-time output, not overlay content copied as-is.
COPY --from=gatekeeper-build /out/gatekeeper /opt/outpost/gatekeeper

# Restore exec bits (lost when authored on Windows) and put the auth probe on PATH.
RUN chmod +x /defaults/autostart \
             /custom-cont-init.d/10-outpost-init.sh \
             /custom-services.d/healthz \
             /custom-services.d/gatekeeper \
             /opt/outpost/authcheck.sh \
             /opt/outpost/healthz.py \
             /opt/outpost/gatekeeper && \
    ln -sf /opt/outpost/authcheck.sh /usr/local/bin/outpost-authcheck

# Outpost defaults. Overridable per-deployment (.env / Railway template vars).
ENV TITLE="Outpost" \
    GITPDM_PROVIDER="github" \
    GITPDM_HOST="github.com" \
    OUTPOST_REPO_ROOT="/config/repo" \
    OUTPOST_ADDONS_DIR="/opt/outpost/addons" \
    GITPDM_TOKEN_FILE="/run/outpost/token" \
    RESTART_APP="true"

# 3000 HTTP / 3001 HTTPS (Selkies, from base) · 8080 Outpost /healthz ·
# 8081 gatekeeper (Phase 3, AUTH_MODE=gatekeeper only — idle otherwise)
EXPOSE 3000 3001 8080 8081
