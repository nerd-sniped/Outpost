#!/usr/bin/env python3
"""Outpost health endpoint (Phase 1.4). Tiny stdlib HTTP server on :8080.

  GET /healthz  -> 200 always-when-up (liveness). JSON body includes auth status.
  GET /authz    -> 200 if GitPDM credentials resolve (runs auth.check), else 503.

Kept deliberately minimal — this is a boot canary and a compose/Railway healthcheck
target, not an API. The gatekeeper (Phase 3) sits in front for public deployments;
this endpoint stays behind it.
"""
import json
import os
import subprocess
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ADDONS = os.environ.get("OUTPOST_ADDONS_DIR", "/opt/outpost/addons")
PORT = int(os.environ.get("OUTPOST_HEALTHZ_PORT", "8080"))


def _auth_ok():
    """Run GitPDM's auth.check; return (ok, one_line_summary). Never raises."""
    env = dict(os.environ, PYTHONPATH=os.path.join(ADDONS, "GitPDM"))
    try:
        p = subprocess.run(
            ["python3", "-m", "freecad_gitpdm.auth.check"],
            env=env, capture_output=True, text=True, timeout=15,
        )
        summary = (p.stdout or p.stderr or "").strip().splitlines()
        return p.returncode == 0, (summary[-1] if summary else "")
    except Exception as e:
        return False, f"auth.check error: {e}"


class Handler(BaseHTTPRequestHandler):
    def _send(self, code, payload):
        body = json.dumps(payload).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path.rstrip("/") in ("/healthz", ""):
            ok, summary = _auth_ok()
            self._send(200, {"status": "up", "auth": "ok" if ok else "absent", "detail": summary})
        elif self.path.rstrip("/") == "/authz":
            ok, summary = _auth_ok()
            self._send(200 if ok else 503, {"auth": "ok" if ok else "failed", "detail": summary})
        else:
            self._send(404, {"error": "not found"})

    def log_message(self, *_):  # silence per-request stderr noise
        pass


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", PORT), Handler).serve_forever()
