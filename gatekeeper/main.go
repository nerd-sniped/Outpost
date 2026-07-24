// Outpost gatekeeper (Phase 3) — the door key for public (rung 2) deployments.
//
// A small, streaming-agnostic auth shim in front of Selkies: GitHub device flow,
// identity-pinned to ALLOWED_GITHUB_USER, a self-contained encrypted session cookie
// (no server-side session store), and a generic HTTP/WebSocket reverse proxy. It also
// hands the same GitHub token to GitPDM via a tmpfs file — one auth is both the door
// key and the git credential. See docs/OUTPOST_PROJECT_SCOPE.md §2 ("Door key") and
// docs/DECISIONS.md D6 for the design rationale (why the cookie is encrypted, not just
// signed, and why this binary is always-registered but AUTH_MODE-gated at runtime).
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	selkiesUpstream = "http://127.0.0.1:3000"
	healthzUpstream = "http://127.0.0.1:8080"
	cookieName      = "outpost_session"
)

var (
	githubClientID  string
	allowedUser     string
	tokenFile       string
	sessionLifetime time.Duration
	aesKey          [32]byte

	selkiesProxy *httputil.ReverseProxy
	healthzProxy *httputil.ReverseProxy

	httpClient = &http.Client{Timeout: 10 * time.Second}
)

func main() {
	githubClientID = requireEnv("GITHUB_CLIENT_ID")
	allowedUser = requireEnv("ALLOWED_GITHUB_USER")
	aesKey = sha256.Sum256([]byte(requireEnv("SESSION_SECRET")))

	tokenFile = os.Getenv("GITPDM_TOKEN_FILE")
	if tokenFile == "" {
		tokenFile = "/run/outpost/token"
	}

	sessionLifetime = 36 * time.Hour
	if h := os.Getenv("SESSION_LIFETIME_HOURS"); h != "" {
		if n, err := strconv.Atoi(h); err == nil && n > 0 {
			sessionLifetime = time.Duration(n) * time.Hour
		}
	}

	selkiesProxy = mustProxy(selkiesUpstream)
	healthzProxy = mustProxy(healthzUpstream)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("gatekeeper: listening on :%s (allowed_user=%s)", port, allowedUser)
	if err := http.ListenAndServe(":"+port, http.HandlerFunc(router)); err != nil {
		log.Fatalf("gatekeeper: server exited: %v", err)
	}
}

func requireEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("gatekeeper: required env var %s is not set", name)
	}
	return v
}

func mustProxy(rawURL string) *httputil.ReverseProxy {
	u, err := url.Parse(rawURL)
	if err != nil {
		log.Fatalf("gatekeeper: bad upstream url %q: %v", rawURL, err)
	}
	return httputil.NewSingleHostReverseProxy(u)
}

// --- Routing -----------------------------------------------------------------

// router is deliberately generic HTTP/WebSocket only — it knows nothing about
// Selkies beyond "an upstream to proxy bytes to." httputil.ReverseProxy has
// transparently supported WebSocket upgrades since Go 1.12; no custom hijack code
// is needed or written here.
func router(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/healthz", "/authz":
		// Unauthenticated by design: worst-case leak is a GitHub login string,
		// already public as ALLOWED_GITHUB_USER — never the token. Lets a rung-2
		// platform's healthcheck hit this without a cookie. See DECISIONS.md D6.
		healthzProxy.ServeHTTP(w, r)
		return
	case "/gatekeeper/poll":
		pollHandler(w, r)
		return
	}

	if p := validSession(r); p != nil {
		ensureTokenFile(p.Token)
		configureGitIdentity(p.Login)
		configureGitCredentials(p.Login, p.Token)
		selkiesProxy.ServeHTTP(w, r)
		return
	}
	// No valid session on ANY other path (including "/") -> the code-prompt page,
	// never a bare 404/redirect. This is what makes "unauthenticated visitor sees
	// only a code prompt" true everywhere, not just at the root.
	gateHandler(w, r)
}

// --- Session cookie: AES-256-GCM encrypted, not merely signed ---------------
//
// Carries {login, token, exp} so a request with a valid cookie can self-heal the
// tmpfs GITPDM_TOKEN_FILE after a container restart/sleep-wake (tmpfs is wiped on
// every stop/start) without a second device-flow round trip. This is a superset of
// "signed" (integrity + confidentiality) and costs no extra dependency (stdlib
// crypto/aes + crypto/cipher). Rotating SESSION_SECRET invalidates every
// outstanding cookie for free: verification derives the key from the *current* env
// value on every request, and an old ciphertext fails GCM's auth-tag check against
// a new key. See docs/DECISIONS.md D6.
type sessionPayload struct {
	Login string `json:"login"`
	Token string `json:"token"`
	Exp   int64  `json:"exp"`
}

func gcmFor(key [32]byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encryptSession(p sessionPayload) (string, error) {
	gcm, err := gcmFor(aesKey)
	if err != nil {
		return "", err
	}
	plaintext, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func decryptSession(value string) (*sessionPayload, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	gcm, err := gcmFor(aesKey)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("gatekeeper: cookie too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	var p sessionPayload
	if err := json.Unmarshal(plaintext, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// validSession returns nil for any failure mode (missing cookie, tampered value,
// wrong SESSION_SECRET, expired) — all treated identically as "unauthenticated,"
// with no error detail leaked to the client.
func validSession(r *http.Request) *sessionPayload {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}
	p, err := decryptSession(c.Value)
	if err != nil {
		return nil
	}
	if p.Exp < time.Now().Unix() {
		return nil
	}
	return p
}

func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionLifetime.Seconds()),
		Secure:   isHTTPS(r),
	})
}

// --- Token handoff: write to tmpfs, self-heal on restart --------------------

// ensureTokenFile re-provisions GITPDM_TOKEN_FILE from an already-valid session's
// decrypted token when the tmpfs file is missing (e.g. after a container
// restart/sleep-wake, which wipes tmpfs). This is what makes "GitPDM already
// authenticated" survive sleep/wake, not just "the proxy still lets you in."
func ensureTokenFile(token string) {
	if _, err := os.Stat(tokenFile); err == nil {
		return
	}
	writeTokenFile(token)
}

// gitConfigPath is deliberately a fixed path, not HOME-derived: the gatekeeper runs
// as root, so `git config --global` (which resolves relative to *its own* HOME)
// would silently write to /root/.gitconfig, invisible to GitPDM/git operations that
// run as abc (HOME=/config). --file sidesteps that mismatch entirely. /config is a
// persistent volume (unlike the tmpfs token file), so this only needs to happen
// once — but it's cheap to re-check, and doing so on every proxied request also
// self-heals a git identity lost to a wiped/fresh /config volume, the same way
// ensureTokenFile self-heals the token after a container recreate.
const gitConfigPath = "/config/.gitconfig"

// configureGitIdentity sets user.name/user.email from the authenticated GitHub
// login if neither is already set — never overwrites a value the operator (or a
// previous run) already configured. Without this, GitPDM's first commit fails with
// "Git requires user.name and user.email" on every fresh deploy; nothing else in
// the stack sets it. login@users.noreply.github.com mirrors GitHub's own default
// commit-email convention and needs no extra token scope beyond what device flow
// already grants (the `repo` scope carries no email access).
func configureGitIdentity(login string) {
	if gitConfigHas("user.name") && gitConfigHas("user.email") {
		return
	}
	if err := gitConfigSet("user.name", login); err != nil {
		log.Printf("gatekeeper: failed to set git user.name: %v", err)
		return
	}
	if err := gitConfigSet("user.email", login+"@users.noreply.github.com"); err != nil {
		log.Printf("gatekeeper: failed to set git user.email: %v", err)
		return
	}
	if u, err := user.Lookup("abc"); err == nil {
		uid, uErr := strconv.Atoi(u.Uid)
		gid, gErr := strconv.Atoi(u.Gid)
		if uErr == nil && gErr == nil {
			_ = os.Chown(gitConfigPath, uid, gid)
		}
	}
	log.Printf("gatekeeper: configured git identity for %s", login)
}

func gitConfigHas(key string) bool {
	return exec.Command("git", "config", "--file", gitConfigPath, "--get", key).Run() == nil
}

func gitConfigSet(key, value string) error {
	return exec.Command("git", "config", "--file", gitConfigPath, key, value).Run()
}

// gitCredentialsPath, like gitConfigPath, is a fixed path rather than HOME-derived
// for the same root-vs-abc reason. Empirically necessary: GITPDM_TOKEN_FILE covers
// GitPDM's own headless credential resolver (auth.check, its background checkpoint
// pushes via GitClient), but GitPDM's UI-driven "Save Into Repo" push was observed
// shelling out to plain `git push` without consulting it at all — "could not read
// Username for 'https://github.com': No such device or address" (git falling back
// to an interactive prompt with no terminal attached). Configuring git's own
// credential store makes ANY plain git operation against github.com authenticate,
// regardless of which internal GitPDM code path invokes it — a supplement to the
// token file, not a replacement, and it closes the gap at the git layer instead of
// waiting on a GitPDM-side fix.
const gitCredentialsPath = "/config/.git-credentials"

func configureGitCredentials(login, token string) {
	line := fmt.Sprintf("https://%s:%s@github.com\n", url.QueryEscape(login), url.QueryEscape(token))
	if existing, err := os.ReadFile(gitCredentialsPath); err != nil || string(existing) != line {
		if err := os.WriteFile(gitCredentialsPath, []byte(line), 0600); err != nil {
			log.Printf("gatekeeper: write git credentials failed: %v", err)
			return
		}
		if u, err := user.Lookup("abc"); err == nil {
			uid, uErr := strconv.Atoi(u.Uid)
			gid, gErr := strconv.Atoi(u.Gid)
			if uErr == nil && gErr == nil {
				_ = os.Chown(gitCredentialsPath, uid, gid)
			}
		}
	}
	if !gitConfigHas("credential.helper") {
		if err := gitConfigSet("credential.helper", "store"); err != nil {
			log.Printf("gatekeeper: failed to set credential.helper: %v", err)
		}
	}
}

func writeTokenFile(token string) {
	if err := os.MkdirAll(filepath.Dir(tokenFile), 0700); err != nil {
		log.Printf("gatekeeper: mkdir token dir failed: %v", err)
	}
	if err := os.WriteFile(tokenFile, []byte(token), 0600); err != nil {
		log.Printf("gatekeeper: write token file failed: %v", err)
		return
	}
	// Best-effort: the process may already run as abc (chown-to-self is a no-op)
	// or as root (chown succeeds); never hard-fail either way. Resolved by name,
	// never a hardcoded uid, matching root/custom-cont-init.d/10-outpost-init.sh's
	// own stated principle (PUID only remaps abc's uid when explicitly set).
	if u, err := user.Lookup("abc"); err == nil {
		uid, uErr := strconv.Atoi(u.Uid)
		gid, gErr := strconv.Atoi(u.Gid)
		if uErr == nil && gErr == nil {
			_ = os.Chown(tokenFile, uid, gid)
		}
	}
}

// --- GitHub device flow -------------------------------------------------------

type deviceCodeResp struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type accessTokenResp struct {
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
}

type githubUser struct {
	Login string `json:"login"`
}

func requestDeviceCode() (*deviceCodeResp, error) {
	form := url.Values{"client_id": {githubClientID}, "scope": {"repo"}}
	req, err := http.NewRequest(http.MethodPost, "https://github.com/login/device/code", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var dc deviceCodeResp
	if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
		return nil, err
	}
	if dc.DeviceCode == "" {
		return nil, fmt.Errorf("device code request failed: status %d", resp.StatusCode)
	}
	return &dc, nil
}

func pollAccessToken(deviceCode string) (string, error) {
	form := url.Values{
		"client_id":   {githubClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	req, err := http.NewRequest(http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var at accessTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&at); err != nil {
		return "", err
	}
	if at.Error != "" {
		return "", errors.New(at.Error)
	}
	if at.AccessToken == "" {
		return "", errors.New("empty access token")
	}
	return at.AccessToken, nil
}

func fetchGithubLogin(token string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github user lookup: status %d", resp.StatusCode)
	}
	var u githubUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return "", err
	}
	return u.Login, nil
}

// --- Flow state: one in-flight device-flow handshake at a time --------------
//
// Single-tenant by design (ALLOWED_GITHUB_USER names exactly one person), so a
// single current flow (not a map keyed by client-visible IDs) is simpler and
// sufficient. Distinct from the session cookie: this is a short-lived handshake
// that does not need to survive a restart.
type flow struct {
	status    string // starting | pending | authorized | mismatch | denied | expired | error
	userCode  string
	verifyURI string
	cookieVal string
	login     string
}

var (
	currentFlowMu sync.Mutex
	currentFlow   *flow
)

func getOrStartFlow() *flow {
	currentFlowMu.Lock()
	defer currentFlowMu.Unlock()
	if currentFlow != nil && (currentFlow.status == "pending" || currentFlow.status == "starting") {
		return currentFlow
	}
	f := &flow{status: "starting"}
	currentFlow = f
	go runDeviceFlow(f)
	return f
}

func setFlow(f *flow, mutate func(*flow)) {
	currentFlowMu.Lock()
	mutate(f)
	currentFlowMu.Unlock()
}

func runDeviceFlow(f *flow) {
	dc, err := requestDeviceCode()
	if err != nil {
		log.Printf("gatekeeper: device code request failed: %v", err)
		setFlow(f, func(f *flow) { f.status = "error" })
		return
	}
	setFlow(f, func(f *flow) {
		f.userCode = dc.UserCode
		f.verifyURI = dc.VerificationURI
		f.status = "pending"
	})

	interval := time.Duration(dc.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(interval)
		token, err := pollAccessToken(dc.DeviceCode)
		if err != nil {
			switch err.Error() {
			case "authorization_pending":
				continue
			case "slow_down":
				interval += 5 * time.Second
				continue
			case "expired_token":
				setFlow(f, func(f *flow) { f.status = "expired" })
				return
			case "access_denied":
				setFlow(f, func(f *flow) { f.status = "denied" })
				return
			default:
				log.Printf("gatekeeper: token poll error: %v", err)
				setFlow(f, func(f *flow) { f.status = "error" })
				return
			}
		}

		login, err := fetchGithubLogin(token)
		if err != nil {
			log.Printf("gatekeeper: github user lookup failed: %v", err)
			setFlow(f, func(f *flow) { f.status = "error" })
			return
		}
		if !strings.EqualFold(login, allowedUser) {
			// The identity-pinning test: device flow succeeded, but this is not
			// the allowed account. Discard the token — never written, never
			// logged. Only the (non-secret) login is logged.
			log.Printf("gatekeeper: rejected github login %q (not %q)", login, allowedUser)
			setFlow(f, func(f *flow) { f.status = "mismatch" })
			return
		}

		writeTokenFile(token)
		configureGitIdentity(login)
		configureGitCredentials(login, token)
		cookieVal, err := encryptSession(sessionPayload{
			Login: login,
			Token: token,
			Exp:   time.Now().Add(sessionLifetime).Unix(),
		})
		if err != nil {
			log.Printf("gatekeeper: session cookie encryption failed: %v", err)
			setFlow(f, func(f *flow) { f.status = "error" })
			return
		}
		setFlow(f, func(f *flow) {
			f.login = login
			f.cookieVal = cookieVal
			f.status = "authorized"
		})
		return
	}
	setFlow(f, func(f *flow) { f.status = "expired" })
}

// --- HTTP handlers: code-prompt page + poll ----------------------------------

const pageHTML = `<!DOCTYPE html>
<!-- outpost-gatekeeper -->
<html><head><meta charset="utf-8"><title>Outpost &mdash; sign in</title>
<style>
body{font-family:system-ui,sans-serif;background:#111;color:#eee;display:flex;
  align-items:center;justify-content:center;height:100vh;margin:0}
.card{text-align:center;max-width:28rem;padding:2rem}
code{font-size:1.5rem;letter-spacing:.15em;background:#222;padding:.3em .6em;border-radius:.3em}
a{color:#6cf}
</style></head><body><div class="card">
<h1>Outpost</h1>
<div id="status">%s</div>
</div>
<script>
async function poll(){
  try{
    const r = await fetch('/gatekeeper/poll');
    const j = await r.json();
    if(j.status === 'authorized'){ location.reload(); return; }
    if(j.status === 'pending' || j.status === 'starting'){ setTimeout(poll, 2000); return; }
    setTimeout(function(){ location.reload(); }, 5000);
  }catch(e){ setTimeout(poll, 3000); }
}
poll();
</script></body></html>`

func statusHTML(f *flow) string {
	if f == nil {
		return "<p>Starting&hellip;</p>"
	}
	switch f.status {
	case "pending":
		return fmt.Sprintf(
			`<p>Enter this code at <a href="%s" target="_blank" rel="noopener">%s</a>:</p><p><code>%s</code></p>`,
			html.EscapeString(f.verifyURI), html.EscapeString(f.verifyURI), html.EscapeString(f.userCode),
		)
	case "mismatch":
		return "<p>Access denied &mdash; that GitHub account is not authorized. Reload to try again.</p>"
	case "denied":
		return "<p>Authorization denied. Reload to try again.</p>"
	case "expired":
		return "<p>Code expired. Reload to try again.</p>"
	case "error":
		return "<p>Something went wrong starting sign-in. Reload to try again.</p>"
	default:
		return "<p>Starting&hellip;</p>"
	}
}

func gateHandler(w http.ResponseWriter, r *http.Request) {
	f := getOrStartFlow()
	currentFlowMu.Lock()
	body := fmt.Sprintf(pageHTML, statusHTML(f))
	currentFlowMu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, body)
}

func pollHandler(w http.ResponseWriter, r *http.Request) {
	currentFlowMu.Lock()
	f := currentFlow
	status, cookieVal, login := "", "", ""
	if f != nil {
		status, cookieVal, login = f.status, f.cookieVal, f.login
	}
	currentFlowMu.Unlock()

	if status == "authorized" && cookieVal != "" {
		setSessionCookie(w, r, cookieVal)
		log.Printf("gatekeeper: session established for %s", login)
	}
	if status == "" {
		status = "starting"
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]string{"status": status})
}
