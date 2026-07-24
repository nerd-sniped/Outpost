# Security

Outpost's door key (the Phase 3 gatekeeper) requests `repo` scope because the same
GitHub token is both the login and the git credential — see `README.md`'s "How auth
works" section for the full rationale. This file is the re-runnable adversarial checklist
(re-run manually before any release that touches the gatekeeper, per
`docs/OUTPOST_DEV_PLAN.md`'s "Test infrastructure" section) and the panic procedure.

## Threat model, stated plainly

A valid gatekeeper session includes FreeCAD's Python console — i.e. arbitrary code
execution as the container user, including reading the token file. **A stolen, logged-in
device is a total compromise of the repo(s) and the box.** Nothing in this design tries
to reduce that blast radius further; the mitigations below are about (1) keeping
strangers from ever getting a valid session, and (2) capping how long a stolen session
stays valid.

## Adversarial checklist (re-run before any gatekeeper-touching release)

See `docs/PHASE3_VERIFICATION.md` for the full step-by-step with checkboxes; the short
form:

- [ ] No cookie → code-prompt page on every path; Selkies never reachable directly.
- [ ] A GitHub account that is not `ALLOWED_GITHUB_USER` completes device flow →
      rejected — no cookie, no token written, no trace of the token in logs.
- [ ] A tampered or expired `outpost_session` cookie → rejected, no error detail leaked.
- [ ] The real token never appears in `docker inspect` env output or in any log line
      (grep the full log for the literal token string).
- [ ] Rotating `SESSION_SECRET` invalidates every outstanding session immediately.

## Panic procedure (device stolen while logged in)

Two steps, both required — they close different holes:

1. **Rotate `SESSION_SECRET`.** Locally: edit `.env`, `docker compose up -d`. Railway
   (Phase 4): regenerate the template variable and redeploy. This alone invalidates
   every outstanding session cookie instantly — the stolen device's cookie stops
   decrypting under the new key on its very next request. **This step does not revoke
   the GitHub token itself** — only the proxy's willingness to accept a session.
2. **Revoke the GitHub OAuth App's authorization** at
   <https://github.com/settings/applications> (find the app, click Revoke). This is what
   actually kills the token that's sitting in `GITPDM_TOKEN_FILE` and possibly still
   cached in the stolen device's browser cookie — without this step, a copy of the raw
   token taken before rotation (e.g. from a proxy log the attacker already had, or from
   memory-scraping the device) would remain a working git credential indefinitely, since
   this design does not otherwise time-limit the GitHub token itself.

Do both, in either order, as soon as a device is known or suspected stolen while logged
in. Step 1 is instant and requires no GitHub access; do it first if you're not near a
computer with GitHub access yet, then do step 2 as soon as you are.
