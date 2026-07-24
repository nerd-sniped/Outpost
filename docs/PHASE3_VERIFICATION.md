# Phase 3 — Gatekeeper Verification (runbook + results)

**Status: every checkbox below is checked. Phase 3 exit gate: met** (2026-07-24), via a
full manual run against a real GitHub OAuth App and two real accounts.

Three real bugs surfaced during this pass:

- Two were **Outpost-side** and are fixed and re-verified against a properly rebuilt
  image (not just a live patch — see the validation note in `docs/DECISIONS.md` D7):
  missing git identity, and `GITPDM_TOKEN_FILE` not covering every GitPDM code path
  (fixed via a supplementary git credential store).
- One is **GitPDM-side**, out of scope for this repo to fix directly: the Open/Clone
  repo-picker wizard crashes outright. Root-caused and filed with a suggested fix —
  [nerd-sniped/GitPDM#8](https://github.com/nerd-sniped/GitPDM/issues/8).

Everything else worked as designed on the first real try: the identity-pinning
rejection, tampered/expired-cookie rejections, no-cookie lockout, restart/recreate
token self-heal, `SESSION_SECRET` rotation, and the two-step panic procedure
(empirically confirmed the two steps are independent — see that section below).

Commands below assume Git Bash / a POSIX shell (what this session used). If you're in
PowerShell, `docker`/`docker compose`/`curl` invocations are identical; only inline
env-var syntax (`VAR=x cmd`) and `openssl rand` differ — noted inline where it matters.

To save typing, define a shortcut for the two-file compose invocation:

```bash
alias dcgk='docker compose -f docker-compose.yml -f compose.gatekeeper.yml'
```

```powershell
function dcgk { docker compose -f docker-compose.yml -f compose.gatekeeper.yml @args }
```

---

## 0. Setup (once)

1. **Create a GitHub OAuth App** at <https://github.com/settings/developers> → **New
   OAuth App**:
   - Application name: anything, e.g. `Outpost Gatekeeper (dev)`.
   - Homepage URL / Authorization callback URL: anything plausible — device flow doesn't
     use either (e.g. `https://github.com/nerd-sniped/Outpost` for both).
   - Check **Enable Device Flow**.
   - Register, then copy the **Client ID** shown on the app's page. No client secret is
     needed — leave it alone.

2. `cp .env.example .env` and fill in:
   - `GITHUB_CLIENT_ID` — from step 1.
   - `ALLOWED_GITHUB_USER` — your GitHub login (case-insensitive match).
   - `SESSION_SECRET` — `openssl rand -hex 32` (bash) or, in PowerShell:
     `-join ((1..32) | ForEach-Object { '{0:x2}' -f (Get-Random -Max 256) })`.
   - Leave `PORT=8081`.

3. Bring it up and watch the gatekeeper's own log lines:

   ```bash
   dcgk up -d --build
   dcgk logs -f outpost | grep --line-buffered "gatekeeper:"
   ```

4. A **second GitHub account** (alt account, or a friend's) is required for 3.1's
   identity-pinning test — that's the one test nothing else can substitute for.

---

## 3.1 — Standalone shim (adversarial)

### No cookie → only the code-prompt page is reachable

```bash
# Any path, not just "/" — all must return the code-prompt page, never Selkies.
curl -s http://localhost:8081/                | grep -q outpost-gatekeeper && echo "OK: /"
curl -s http://localhost:8081/some/weird/path | grep -q outpost-gatekeeper && echo "OK: /some/weird/path"

# Selkies itself must be unreachable — compose.gatekeeper.yml only publishes 8081.
curl -s --max-time 3 http://localhost:3000 && echo "FAIL: Selkies reachable" \
  || echo "OK: Selkies unreachable"
```

- [x] All paths return the code-prompt page (grep for `outpost-gatekeeper`, the marker
      HTML comment). Verified 2026-07-24.
- [x] `localhost:3000` fails to connect (connection refused/timeout), not just a 404.
      Verified 2026-07-24.

### Wrong GitHub account completes device flow → rejected (the identity-pinning test)

1. Open `http://localhost:8081` in a **private/incognito window** (clean cookie jar).
2. Follow the code shown to `github.com/login/device`, but authorize with the **second**
   GitHub account — not `ALLOWED_GITHUB_USER`.
3. The page should land on "Access denied — that GitHub account is not authorized."

```bash
# The rejection should be logged with the wrong login, never a token:
dcgk logs outpost | grep "rejected github login"

# The token file must NOT exist (assuming no prior successful login this session):
dcgk exec outpost sh -c 'test -f /run/outpost/token && echo EXISTS || echo ABSENT'
```

- [x] Browser shows the rejection message, not an error page or Selkies. Verified
      2026-07-24 (second account: Factorem-io) — the frontend auto-reloads ~5s after a
      rejection and starts a fresh device-flow round automatically; this can look like
      it's "stuck on Starting…" if you're not watching closely, but it isn't — check
      `/gatekeeper/poll` or the logs if in doubt.
- [x] Log shows `rejected github login "<their-login>"` — never a token value. Verified:
      `rejected github login "Factorem-io" (not "nerd-sniped")`, twice.
- [x] `/run/outpost/token` untouched by the rejected attempt — still the real user's
      token (confirmed same byte size before/after; `outpost-authcheck` still resolved
      `login=nerd-sniped`, not the rejected account).
- [x] No session ever granted to the rejected account (implied by the above — the only
      cookie ever issued in this container's lifetime was for `nerd-sniped`).

### Tampered cookie → rejected

```bash
curl -s -o /tmp/body.html -w "%{http_code}\n" http://localhost:8081/ \
  -H "Cookie: outpost_session=not-a-real-cookie-value"
grep -q outpost-gatekeeper /tmp/body.html && echo "OK: tampered cookie -> code prompt"
```

- [x] Returns the code-prompt page (200 + marker present), not a 500 or leaked error
      detail. Verified 2026-07-24.

### Expired cookie → rejected

A cookie's expiry is a plain Unix-timestamp check inside the encrypted payload, so the
only way to see it fire live (short of waiting out the real ~36h lifetime) is to mint an
already-expired cookie with the same `SESSION_SECRET`. Save this as `forge_cookie.go`
next to nothing in particular (a scratch file — it's a test tool, not part of the
shipped image) and run it once:

```go
// forge_cookie.go — throwaway test utility (not part of the shipped gatekeeper).
// Mints an already-expired outpost_session value using the same AES-256-GCM scheme
// as gatekeeper/main.go, so the expiry branch can be exercised without waiting.
// Usage: go run forge_cookie.go <SESSION_SECRET> <github-login>
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func main() {
	secret, login := os.Args[1], os.Args[2]
	key := sha256.Sum256([]byte(secret))
	block, _ := aes.NewCipher(key[:])
	gcm, _ := cipher.NewGCM(block)
	payload, _ := json.Marshal(map[string]any{
		"login": login,
		"token": "forged-expired-test-token",
		"exp":   time.Now().Add(-1 * time.Hour).Unix(), // already expired
	})
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	sealed := gcm.Seal(nonce, nonce, payload, nil)
	fmt.Println(base64.RawURLEncoding.EncodeToString(sealed))
}
```

```bash
COOKIE=$(go run forge_cookie.go "$SESSION_SECRET" "$ALLOWED_GITHUB_USER")
curl -s -o /tmp/body.html -w "%{http_code}\n" http://localhost:8081/ \
  -H "Cookie: outpost_session=$COOKIE"
grep -q outpost-gatekeeper /tmp/body.html && echo "OK: expired cookie -> code prompt"
```

- [x] Returns the code-prompt page even though the cookie decrypts *correctly* (proves
      the `exp` check, not just the HMAC/AEAD check, is doing work). Verified
      2026-07-24 against the live container (the real `SESSION_SECRET`, forged in a
      throwaway `golang:alpine` container, secret piped through a file and never
      printed to any terminal).

### WebSocket upgrade proxies correctly once authenticated

1. In a normal (non-incognito) browser tab, complete device flow as
   `ALLOWED_GITHUB_USER` at `http://localhost:8081`.
2. Confirm the page reloads into a live, interactive FreeCAD stream — move the mouse,
   orbit a model. A static/frozen page or a page that never gets past "Starting…" means
   the WS upgrade isn't reaching Selkies.

- [x] FreeCAD renders and responds to input through `:8081` (not just a page load).
      Verified repeatedly throughout this session — moved/edited parts, orbited, and
      worked across multiple reconnects (post-restart, post-recreate) with no WS-layer
      issues at any point.

---

## 3.2 — Token handoff

Run these right after the successful login from the WebSocket test above.

```bash
# GitPDM should already show authenticated inside FreeCAD's panel — no second prompt.
# (Check visually in the GitPDM panel inside the streamed session.)

# The real token must never be a bare env value:
docker inspect outpost --format '{{json .Config.Env}}' | tr ',' '\n' | grep -i token
# Expect only: "GITPDM_TOKEN_FILE=/run/outpost/token"  (a path, not a value)

# The real token must never appear in logs. GitHub tokens have recognizable prefixes
# (gho_ for OAuth app tokens, ghp_ for classic PATs) — grep for those, not the literal
# token (which you may not want to paste into a shell history anyway):
dcgk logs outpost | grep -Ei "gh[op]_[A-Za-z0-9]"
# Expect: no output.
```

- [x] GitPDM's *headless* credential resolver is authenticated with no second prompt
      (`auth.check` resolves via `GITPDM_TOKEN_FILE` immediately after device flow).
      **Two separate GitPDM-side UI issues found 2026-07-24, distinguished carefully
      since they look similar but aren't:**
      1. GitPDM's *own* "Connect GitHub" button (in its Connections dialog) triggers a
         real OAuth flow that completes but then fails to persist to an OS keyring
         (`Secret Service not available` — no `secretstorage`/D-Bus in a headless
         container). Expected/benign — doesn't affect the real credential, which is
         already resolved via `GITPDM_TOKEN_FILE`. Don't click it during testing; it's
         solving a problem that's already solved.
      2. The Open/Clone → repo-picker flow separately crashes outright —
         `'GitPDMDockWidget' object has no attribute '_on_github_connect_clicked'` —
         a real bug, not an environment limitation. Root-caused and filed with a
         suggested fix: **[nerd-sniped/GitPDM#8](https://github.com/nerd-sniped/GitPDM/issues/8)**.
         Workaround used during this test pass: skip the wizard, open the
         already-cloned repo directly (`GIT_REMOTE_URL` clone-on-boot already provides
         it at `/config/repo`).
- [x] Full log grep for a GitHub-token-shaped string (`gh[op]_...`, `github_pat_...`)
      returns nothing. Verified 2026-07-24.
- [x] `docker inspect` env shows only the *path*, never a token value. **Initially
      failed** (a leftover rung-1 `GITPDM_TOKEN` from an earlier `.env` was still
      present, since that container pre-dated the `compose.gatekeeper.yml` fix that
      blanks it) — **re-verified clean** after the later full rebuild+recreate:
      `docker inspect` now shows `GITPDM_TOKEN=` (empty) alongside
      `GITPDM_TOKEN_FILE=/run/outpost/token` (a path), never a real value.
- [x] **Pushes actually work**, not just `auth.check` passing. Verified 2026-07-24: a
      real commit (`95d6c20`) was pushed via GitPDM's "Save Into Repo" and confirmed
      matching `git ls-remote`'s HEAD on the real `Outpost-Test` remote. **Found and
      fixed a real bug to get here:** GitPDM's UI push action shells out to plain `git
      push` without consulting `GITPDM_TOKEN_FILE` at all (`could not read Username for
      'https://github.com'` — no credential, no terminal to prompt). Fixed by having
      the gatekeeper also configure a standard git credential store
      (`/config/.git-credentials` + `credential.helper=store`) alongside the token
      file — see `gatekeeper/main.go`'s `configureGitCredentials`. Also required
      manually setting `user.name`/`user.email` the first time (fixed permanently via
      `configureGitIdentity`, same file) — GitPDM's first commit otherwise fails with
      "Git requires user.name and user.email", and nothing else in the stack sets it.

---

## 3.3 — Session lifecycle

### Restart survives; token self-heals

```bash
dcgk restart outpost
```

- [x] Reload `http://localhost:8081` in the **same** browser tab (cookie untouched by
      the restart) → lands straight in FreeCAD, no new device-flow prompt. Verified
      2026-07-24 (a reload attempted *during* the restart window looked like a logout —
      that was just the request racing the container coming back up, not a real cookie
      failure; a clean reload well after the container reported healthy worked
      correctly). Landed on FreeCAD's own Document Recovery screen since `docker
      restart` kills FreeCAD non-gracefully — expected FreeCAD behavior (see Phase 2
      verification notes), unrelated to auth.
- [x] `dcgk exec outpost test -f /run/outpost/token && echo OK` — the tmpfs file, wiped
      by the restart, has reappeared without a second device-flow round trip. Verified
      2026-07-24.

### Rotating `SESSION_SECRET` invalidates every session

```bash
# Edit .env: change SESSION_SECRET to a new random value, then:
dcgk up -d   # recreates outpost with the new env
```

- [x] Reload the same browser tab → back to the code-prompt page (old cookie now fails
      to decrypt under the new key). Verified 2026-07-24 — note this was `up -d`
      (recreate), not `restart`: Compose's recreate copies the container's anonymous
      volumes over to the new container (unlike a full `down`+`up`), so `/config`
      (repo, gitconfig, credential store) survived intact even though the container
      itself was replaced.

### Re-auth after expiry/rotation resumes the same session, no lost work

- [x] After the rotation above, complete device flow again as `ALLOWED_GITHUB_USER` →
      confirm your *committed/saved work* is intact (`/config` survives) — not lost.
      Verified 2026-07-24. **Calibrated expectation:** a recreate (unlike a plain
      `restart`) always kills and relaunches the FreeCAD process itself, so "the same
      running session" doesn't literally hold across a recreate — undo history etc.
      resets. The actual guarantee this project makes is "lose at most ~a minute of
      work" via what's saved/committed to `/config` and git, not "the live process
      survives every disruption." Token/git-identity/git-credentials all
      re-provisioned automatically from the fresh login with zero manual steps —
      confirms the fixes found earlier this session are durable, not one-off patches.

### Panic procedure walkthrough (do this for real, once)

Follow `SECURITY.md` exactly:

1. Rotate `SESSION_SECRET` (as above) — confirm the old cookie is rejected.
2. Revoke the OAuth App's authorization at
   <https://github.com/settings/applications> → find the app → **Revoke**.

```bash
# Confirm the OLD token is now dead (401), proving step 2 actually killed the credential
# and not just the proxy's willingness to accept a session:
curl -s -o /dev/null -w "%{http_code}\n" https://api.github.com/user \
  -H "Authorization: Bearer <the-old-token-from-/run/outpost/token>"
# Expect: 401
```

- [x] Old cookie rejected after step 1 (rotation). Verified 2026-07-24, same rotation
      test as the session-lifecycle section above.
- [x] Old token returns 401 from GitHub's API after step 2 (revocation at
      github.com/settings/applications). Verified 2026-07-24 — `curl` with the old
      token against `api.github.com/user` returned `401` immediately after revoking.
      **Important nuance confirmed empirically:** the two steps are independent —
      revoking (step 2) alone does *not* end an already-established gatekeeper
      session/proxy access (the cookie still decrypts fine and isn't expired, so
      FreeCAD/Selkies stay reachable); it only kills git operations using that token.
      Rotating `SESSION_SECRET` (step 1) is what actually ends proxy access. Both
      steps are required for a real panic response — this is already how
      `SECURITY.md` documents it, and this test empirically confirms that framing is
      correct, not just theoretical.

---

**Phase 3 exit gate: met** (2026-07-24). Every box above checked; a second person
(GitHub account `Factorem-io`), using their own GitHub account and nothing else, failed
to get in — twice, cleanly, with no trace in the token file or logs.
