---
phase: 10
title: "Deploy + polish (Fly.io, hardening, docs sweep)"
status: pending
priority: P1
effort: 1w
dependencies: [9]
---

# Phase 10 — Deploy + polish

## Context Links
- `plans/reports/security-review-260508-2300-phase1-foundation.md` (M1 CI SHA pinning)
- `plans/reports/researcher-260508-2300-mongodb-atlas-go.md` §2 (Atlas IP allowlist Q5)
- `plans/reports/researcher-260508-2300-firebase-admin-go-auth.md` §7 (revocation), §10 (custom claims deferred)
- `docs/code-standards.md`, `docs/system-architecture.md`, `docs/deployment-guide.md`, `docs/development-roadmap.md`, `docs/codebase-summary.md`
- `.github/workflows/ci.yml` (action SHA pinning)

## Overview
Production-ready hardening: Fly.io deploy with secrets, Atlas IP allowlist decision, CI action SHA pinning (security review M1), final docs sweep filling all skeletons created in Phase 01, custom-claims for admin role (firebase report §7), token revocation check on auth-sensitive ops, drop residual transitional cruft.

## Key Insights
- **Atlas allowlist (mongo Q5):** Fly.io outbound IPs are a fixed pool but not stable per-machine. Decide: `0.0.0.0/0` for MVP (acceptable since Mongo user/password gates access). Revisit if/when moving to M10 + VPC peering.
- **Fly.io secrets:** `MONGO_URI`, Firebase service-account JSON (uploaded as base64-encoded secret + decoded at boot), `DLEAGUE_WS_ORIGINS`, `FIREBASE_PROJECT_ID`. NEVER commit.
- **CI action pinning (security M1):** every `uses: foo/bar@v1` → `uses: foo/bar@<full-sha>  # v1.x.y`. Use Dependabot `update-type: digest` for ongoing freshness.
- **Custom claims (firebase Q3 deferred):** Phase 10 adds `admin` claim only. Set via Admin SDK; surface via `Conn.isAdmin` for moderator endpoints (none yet — flag is plumbing for v2).
- **Revocation:** wrap critical ops (user delete, ban) with `VerifyIDTokenAndCheckRevoked`. Routine WS messages keep `VerifyIDToken` (cost trade-off).
- **WASM cruft:** code-standards.md still references "WASM bundle target" if Phase 06 missed a spot — verify here.
- **Mobile prep:** explicitly cancelled (was Phase 6 of old plan). v2.

## Requirements
**Functional:**
- `fly.toml` with: app name, primary region, [build] (Dockerfile), [services] (8080 internal, 80/443 external w/ TLS), `[[mounts]]` none (stateless), [env] for non-secret config.
- `Dockerfile` multi-stage: `golang:1.26` build → `node:20` for `web/` build → `gcr.io/distroless/static-debian12` runtime.
- `make deploy` target: `fly deploy --remote-only`.
- Fly secrets script `scripts/set-fly-secrets.sh`: documents commands to set MONGO_URI, FIREBASE_PROJECT_ID, FIREBASE_SERVICE_ACCOUNT_B64, DLEAGUE_WS_ORIGINS.
- Atlas: production user (read+write `dleague` DB only); `0.0.0.0/0` allowlist documented as accepted-risk in deployment-guide.md.
- CI actions all pinned to full SHAs.
- Dependabot config (`.github/dependabot.yml`) for action SHA bumps + go modules + npm.
- `Conn.isAdmin bool` populated from token claim; helper `requireAdmin(c)` for any future admin handler (no admin handler yet — but plumbing).
- `auth/firebase.go` exposes `SetAdminClaim(ctx, uid)` for one-off Admin SDK call (used via short admin script).
- Critical ops use `VerifyIDTokenAndCheckRevoked` — currently only `SetAdminClaim` itself.
- Final `docs/` content (no TODO markers remaining):
  - `docs/system-architecture.md` (full diagram + flows + index list + auth flow + sync PvP).
  - `docs/codebase-summary.md` (current file tree + module roles + build commands).
  - `docs/deployment-guide.md` (Fly.io step-by-step, secrets, Atlas setup, Firebase project setup).
  - `docs/development-roadmap.md` (10 phases with completion status; v2 backlog).
  - `docs/project-overview-pdr.md` (product goal, MVP scope, success metrics).
  - `docs/project-changelog.md` (entries for each commit/phase).
  - `docs/design-guidelines.md` (UI palette, animations, accessibility).
- `docs/code-standards.md` cleaned: no `WASM`, no `Postgres`, no `MySQL`, no `Ebitengine`.
- README.md final pass with deploy badge + production URL.

**Non-functional:**
- Production boot succeeds in <5s.
- Container image <100 MB (distroless static).
- TLS A grade on SSL Labs.
- All 8 docs files <250 LOC each.

## Architecture
```
Fly.io app "dleague-prod" (region: iad)
  └─ Container (distroless static)
       ├─ /server binary (Go 1.26 static build)
       ├─ /web/dist/ (SvelteKit static)
       └─ env from fly secrets
              MONGO_URI=mongodb+srv://prod_user:****@cluster.mongodb.net/...
              FIREBASE_PROJECT_ID=dleague-prod
              FIREBASE_SERVICE_ACCOUNT_B64=...
              DLEAGUE_WS_ORIGINS=https://dleague.gg
              DLEAGUE_MAX_CONNS=2000
              DLEAGUE_TRUSTED_PROXIES=fly-ips
              ENV=production

External:
  Cloudflare → Fly edge proxy → :8080 internal
  /ws upgraded; /health for fly-checks; / served from web/dist
  Atlas M0 (region: us-east-1, paired with iad): 0.0.0.0/0 allowlist
  Firebase Auth (cloud, no region): GOOGLE_APPLICATION_CREDENTIALS via secret
```

## Related Code Files
**Create:**
- `Dockerfile`
- `fly.toml`
- `scripts/set-fly-secrets.sh`
- `scripts/seed-wordlists.sh` (one-shot: upload wordlist to prod Mongo)
- `scripts/promote-admin.sh` (one-shot: SetAdminClaim by UID)
- `.github/dependabot.yml`
- `cmd/admin/main.go` (small CLI: `promote-admin <uid>`, `revoke-token <uid>` — uses Admin SDK)

**Modify:**
- `.github/workflows/ci.yml` — pin all actions to SHAs; add SHA-bump dependabot
- `server/internal/auth/firebase.go` — add `SetAdminClaim`, `VerifyIDTokenAndCheckRevoked` wrapper
- `server/internal/ws/conn.go` — populate `Conn.isAdmin` from `token.Claims["admin"] == true`
- `server/internal/config/config.go` — add `Env string` ("development"/"production") for boot-time assertions
- `server/cmd/server/main.go` — boot-time: in production, assert WS origins non-empty; warn if MONGO_URI starts with `mongodb://` (not `mongodb+srv://`)
- `Makefile` — `deploy`, `seed-wordlists-prod` targets
- `README.md` — production URL, deploy badge
- `docs/code-standards.md` — final cleanup (no WASM/Postgres/MySQL/Ebitengine references); add TS conventions section
- `docs/system-architecture.md` — fill all sections from skeleton
- `docs/codebase-summary.md` — fill from current state
- `docs/deployment-guide.md` — fill: Fly.io, Atlas, Firebase setup
- `docs/development-roadmap.md` — fill: 10 phases status + v2 backlog
- `docs/project-overview-pdr.md` — fill from plan.md goal/scope
- `docs/project-changelog.md` — entries per phase commit
- `docs/design-guidelines.md` — fill: Wordle palette, accessibility, animations
- `.gitignore` — append fly state, secrets, etc.

**Delete:**
- Any leftover `WASM` references in `docs/code-standards.md`
- `web/wasm_exec.js` if any residual
- Mobile-prep stubs if anywhere referenced

## Implementation Steps
1. **Dockerfile multi-stage:**
   - Stage 1 `golang:1.26-alpine`: `go build -ldflags='-s -w' -o /out/server ./server/cmd/server`.
   - Stage 2 `node:20-alpine`: `cd web && npm ci && npm run build`.
   - Stage 3 `gcr.io/distroless/static-debian12`: copy `/out/server` + `/web/dist/`. CMD `["/server"]`.
2. **fly.toml:** `app = "dleague-prod"`, region `iad`, internal_port 8080, services [http]/[tls], force_https=true, min_machines_running=1.
3. **Fly secrets:** `scripts/set-fly-secrets.sh` (commented commands, not executed by Make):
   ```
   fly secrets set MONGO_URI="..."
   fly secrets set FIREBASE_PROJECT_ID="dleague-prod"
   fly secrets set FIREBASE_SERVICE_ACCOUNT_B64="$(base64 -w0 service-account.json)"
   fly secrets set DLEAGUE_WS_ORIGINS="https://dleague.gg"
   ```
4. **Boot-time secret decode:** main.go: if `FIREBASE_SERVICE_ACCOUNT_B64` set, decode → write to `/tmp/sa.json` → set `GOOGLE_APPLICATION_CREDENTIALS`.
5. **Atlas allowlist decision:** add `0.0.0.0/0` in Atlas console; document in deployment-guide.md as accepted-risk (auth gates access; M10 + VPC for v2).
6. **Atlas prod user:** create user `dleague_prod` with role `readWrite@dleague`. Encode password URL-safe in MONGO_URI.
7. **CI SHA pinning:** for each `uses: x/y@v?` in `ci.yml`, replace with `uses: x/y@<sha>  # v?.?.?`. Get SHAs via `gh api repos/x/y/commits/v? --jq .sha`.
8. **Dependabot config:** `.github/dependabot.yml` schedules digest updates for actions, weekly for go-mod and npm.
9. **Custom-claims:**
   - `auth/firebase.go`: `SetAdminClaim(ctx, uid)` calls `authClient.SetCustomUserClaims(ctx, uid, map[string]interface{}{"admin": true})`.
   - `Conn.isAdmin` populated from `token.Claims["admin"]` cast to bool.
   - `cmd/admin/main.go`: small CLI parsing `--uid` and `--action` (promote-admin / revoke-token / verify-token-revoked).
10. **Revocation wrapper:** `VerifyIDTokenAndCheckRevoked` only used by `cmd/admin` (via the CLI). Document for future moderator-action handlers.
11. **Final docs sweep:**
    - System-architecture: insert ASCII diagrams for boot flow, WS dispatch, async flow, sync flow, Mongo collections + indexes.
    - Codebase-summary: walk current tree, list modules, build commands, common operations.
    - Deployment-guide: prerequisites (fly CLI, atlas account, firebase project), step-by-step deploy, rollback procedure (fly releases rollback), monitoring.
    - Development-roadmap: 10 phases with status + dates + v2 backlog (mobile, custom Phaser build, Redis pub/sub, change streams, full-text search, tournament brackets, spectator).
    - Project-overview-pdr: product goal, MVP scope, success metrics, constraints.
    - Project-changelog: chronological entries from git log.
    - Design-guidelines: Wordle palette (#6aaa64 green, #c9b458 yellow, #787c7e gray), animation timing, mobile breakpoints, prefers-reduced-motion fallback, accessibility (ARIA on board, contrast ratios).
12. **Code-standards cleanup:** grep -i "wasm\|postgres\|mysql\|ebitengine" docs/ → must return 0 hits.
13. **README.md final:** production URL, Fly.io deploy badge, brief "How it works" with link to system-architecture.md.
14. **Smoke deploy:** `make deploy` to a `dleague-staging` app first; verify `/health`, sign-in, daily play, challenge link, quick match. Then promote to `dleague-prod`.
15. **Atlas safety check:** verify Atlas backups (M0 has none; document) and migration to M10 trigger criteria (storage >70% of 512 MB, or sustained ops >50/sec).

## Todo List
- [ ] Dockerfile multi-stage
- [ ] fly.toml
- [ ] Fly secrets script
- [ ] Boot-time secret decode
- [ ] Atlas prod user + 0.0.0.0/0
- [ ] CI action SHA pinning
- [ ] Dependabot config
- [ ] SetAdminClaim + Conn.isAdmin
- [ ] cmd/admin CLI
- [ ] VerifyIDTokenAndCheckRevoked wrapper
- [ ] Fill docs/system-architecture.md
- [ ] Fill docs/codebase-summary.md
- [ ] Fill docs/deployment-guide.md
- [ ] Fill docs/development-roadmap.md
- [ ] Fill docs/project-overview-pdr.md
- [ ] Fill docs/project-changelog.md
- [ ] Fill docs/design-guidelines.md
- [ ] Cleanup WASM/Postgres/MySQL/Ebitengine refs
- [ ] README.md final pass
- [ ] Staging smoke deploy
- [ ] Production deploy
- [ ] Atlas migration triggers documented

## Success Criteria
- [ ] `make deploy` succeeds; `/health` returns 200 from prod URL
- [ ] Sign-in (3 providers) works on prod
- [ ] Daily play, challenge link, quick match all work end-to-end on prod
- [ ] `grep -i "wasm\|postgres\|mysql\|ebitengine" docs/` → 0 hits
- [ ] All `uses:` in `ci.yml` pinned to full SHAs
- [ ] Dependabot opens 1+ PR within a week (sanity check)
- [ ] `cmd/admin promote-admin --uid <test>` sets admin claim; subsequent token has `admin: true`
- [ ] Container image <100 MB
- [ ] SSL Labs grade A on production domain
- [ ] All 8 docs/*.md files have no `TODO` placeholders
- [ ] Production boot <5s

## Risk Assessment
| Risk                                                   | Likelihood | Impact | Mitigation                                                       |
|--------------------------------------------------------|------------|--------|------------------------------------------------------------------|
| Atlas M0 connection-cap saturation in production       | Medium     | High   | Monitor; pre-plan M10 upgrade path. Set `DLEAGUE_MAX_CONNS` ≤ 400. |
| Service-account JSON leaked in image                   | Low        | Critical | Never `COPY` JSON; only via Fly secret env var.                |
| Atlas auto-pause after 30d idle in staging              | High       | Low    | Synthetic ping cron from CI nightly.                              |
| Production WS origin misconfigured → CSWSH              | Low        | High   | Boot-time assertion `len(WSOrigins) > 0 in production` (Phase 02). |
| CI dependabot churn / spurious failures                 | Medium     | Low    | Auto-merge digest-only PRs after CI green.                       |
| Forgotten WASM ref in docs                              | Medium     | Low    | grep CI gate added to lint.                                      |
| Custom-claims plumbing breaks if claim absent           | Low        | Low    | Defensive cast `claim, _ := token.Claims["admin"].(bool)`.       |

## Security Considerations
- TLS-only enforced by Fly.io and `force_https=true`.
- Service-account JSON never on disk in image; injected only via env at boot.
- Prod Atlas user is least-privilege (read+write `dleague` DB only; no admin/cluster perms).
- `0.0.0.0/0` allowlist accepted-risk; auth gate is the actual security boundary. Documented.
- `VerifyIDTokenAndCheckRevoked` exists for moderator/admin actions; not on hot path.
- Dependabot keeps actions + go modules patched.
- Fly.io issuer-trust for HTTP `X-Forwarded-For` (`DLEAGUE_TRUSTED_PROXIES` set to Fly's published IP ranges).
- CSP from Phase 02 (no `wasm-unsafe-eval` after Phase 06) maintained.

## Next Steps
- v2 backlog: native mobile (Capacitor or gomobile), Redis pub/sub matchmaking, Mongo Change Streams for live leaderboard, full-text search, tournament brackets, spectator mode, custom Phaser build, M10 + VPC peering.
