# Phase 8: Deploy — Coolify backend + Firebase Hosting

## Context Links
- Coolify VM: existing OCI ARM Always-Free instance (per parent MySQL plan; reused, MySQL not provisioned)
- Firebase Hosting Spark: 10 GB storage, 360 MB/day egress (per Firebase research report — verify current limits at deploy time)
- Phase-2 server expects `FIREBASE_*` env vars
- Phase-4 web build outputs `web/dist/`
- Locked: root → Hosting; `api.dleague.tld` → Coolify; mobile = Capacitor stub only

## Overview
- **Priority:** P1 (no traffic without deploy)
- **Status:** pending
- **Effort:** 2d
- Deploy Go server to Coolify (Docker image), deploy React app to Firebase Hosting (Vite build), configure DNS so `dleague.tld` → Hosting and `api.dleague.tld` → Coolify (with WSS proxy). Smoke-test full E2E flow. Set up free-tier monitoring + budget alerts.

## Key Insights
- Firebase Hosting + Coolify on different hostnames means client opens WS to a DIFFERENT origin than the page; Hosting CSP + CORS irrelevant for WS (browser allows cross-origin WS), BUT server must whitelist Origin header for upgrade — already supported via `DLEAGUE_WS_ORIGINS` env (per `server/internal/config/config.go:40`)
- TLS for `api.dleague.tld`: Coolify auto-provisions Let's Encrypt cert if Coolify's reverse proxy (Traefik) is in front. Verify Coolify reverse-proxy is enabled
- Firebase Hosting auto-provisions TLS for custom domain after DNS verification (TXT record + A/AAAA records)
- Firestore + Auth quotas reset at midnight Pacific (UTC-8/UTC-7); document this in monitoring
- Capacitor Android: APK build needs Android Studio + Android SDK; not blocking web launch — defer iOS entirely; Android optional at this phase
- No CI/CD this phase: manual deploys via `firebase deploy` + Coolify push-to-deploy. CI scripts deferred to v2

## Requirements

### Functional
1. Go server deploys to Coolify VM via Dockerfile; reachable at `https://api.dleague.tld`
2. React app deploys to Firebase Hosting; reachable at `https://dleague.tld` and `https://www.dleague.tld`
3. Client connects from `dleague.tld` → `wss://api.dleague.tld/ws` (cross-origin WS allowed by server origin whitelist)
4. `/health` endpoint at `https://api.dleague.tld/health` returns 200 + Firestore reachable
5. Firebase Hosting serves SPA fallback (`/m/{token}` rewrites to `/index.html`)
6. Service-account JSON env-injected via Coolify env settings (NOT in image)
7. Free-tier budget alerts active in Cloud Console

### Non-functional
- Cold start <2s for Go server (Docker image <50 MB compressed)
- React bundle gzipped <500 KB delivered from Firebase CDN
- Initial page load Lighthouse Performance >85
- Reasonable uptime; no SLA promised at MVP

## Architecture

### Hostnames
| Hostname | Target | TLS |
|----------|--------|-----|
| `dleague.tld` | Firebase Hosting | Auto (Firebase) |
| `www.dleague.tld` | Firebase Hosting (canonical redirect to apex) | Auto |
| `api.dleague.tld` | Coolify VM (Go server) | Auto (Let's Encrypt via Traefik) |

### Files to create
- `server/Dockerfile` (~30 LOC) — multi-stage: builder (golang:1.22-alpine) → runtime (alpine:3.19); embeds `web/dist/` is NOT needed because Firebase Hosting serves it; Go server only serves `/health` + `/ws`
- `server/.dockerignore` (~10 LOC)
- `web/firebase.json` already from phase-3; ensure `hosting.public = "dist"` and SPA rewrite:
  ```json
  "hosting": {
    "public": "dist",
    "ignore": ["firebase.json", "**/.*", "**/node_modules/**"],
    "rewrites": [{ "source": "**", "destination": "/index.html" }]
  }
  ```
- `web/.firebaserc` — links project ID
- `coolify.json` (or Coolify UI config) — env vars + build cmd
- `docs/deployment-guide.md` — step-by-step deploy doc (~150 LOC)
- `.github/workflows/deploy.yml` — DEFERRED (v2)

### Files to modify
- `server/internal/http/router.go` — `webRoot` becomes optional (when Firebase Hosting is canonical, server doesn't need to serve static); accept empty string and skip static handler if so. Behind env: `DLEAGUE_SERVE_STATIC=false` (default true for backward compat / dev)
- `server/internal/config/config.go` — add `ServeStatic bool`
- `server/cmd/api/main.go` — pass `ServeStatic` to router
- `web/.env.example` — add `VITE_WS_URL=wss://api.dleague.tld/ws` for prod build

### Server Dockerfile (sketch)
```dockerfile
# stage 1: build
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY shared/ ./shared/
COPY server/ ./server/
WORKDIR /app/server
RUN go build -trimpath -ldflags="-s -w" -o /out/dleague-server ./cmd/api

# stage 2: runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/dleague-server /usr/local/bin/dleague-server
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/dleague-server"]
```

### Coolify env vars
- `DLEAGUE_ADDR=:8080`
- `DLEAGUE_SERVE_STATIC=false`
- `DLEAGUE_WS_ORIGINS=dleague.tld,www.dleague.tld,localhost:5173`
- `STORE_BACKEND=firestore`
- `FIREBASE_PROJECT_ID=dleague-prod`
- `FIREBASE_CREDENTIALS_JSON=<paste full JSON>`

### DNS records (registrar UI)
- `A` `dleague.tld` → Firebase Hosting IP (provided by Firebase console after add-domain)
- `AAAA` `dleague.tld` → Firebase Hosting IPv6
- `A` `www` → same / CNAME
- `A` `api.dleague.tld` → Coolify VM public IP
- (TXT records for Firebase domain verification)

## Implementation Steps

### Server deploy
1. Create `server/Dockerfile` + `server/.dockerignore`
2. Build locally: `docker build -t dleague-server -f server/Dockerfile .` (build context = repo root for shared/)
3. Test image locally: `docker run --rm -e FIREBASE_CREDENTIALS_JSON=... -p 8080:8080 dleague-server`
4. Push to Coolify: connect git repo; Coolify builds on push (or Docker Hub registry if Coolify pulls from registry)
5. Set Coolify env vars (above list)
6. Set Coolify domain → `api.dleague.tld`; verify TLS auto-provisioned
7. Verify `/health` returns 200: `curl https://api.dleague.tld/health`

### Web deploy
1. Build: `cd web && VITE_WS_URL=wss://api.dleague.tld/ws npm run build`
2. Install Firebase CLI: `npm i -g firebase-tools`
3. Login: `firebase login`
4. Initialize hosting target: `firebase init hosting` (if not done in phase-1) → confirm `web/dist`
5. Deploy: `firebase deploy --only hosting` from repo root (where `firebase.json` lives)
6. Add custom domain in Firebase console → Hosting → Add custom domain → `dleague.tld` → follow DNS prompts
7. Wait for cert provisioning (5–15 min)
8. Smoke test: open `https://dleague.tld` → React app loads → sign-in works → WS connects to `wss://api.dleague.tld/ws` → AUTH_ACK received

### Mobile (optional)
1. `cd web && npx cap sync android`
2. Open Android Studio → import `web/android/`
3. Build → Generate Signed APK (debug keystore for testing)
4. Sideload to physical device; verify auth + WS work in WebView
5. iOS deferred (requires macOS)

### Monitoring
1. Cloud Console → Billing → set $1 budget alert (already done in phase-1; verify still active)
2. Firebase Console → Firestore → Usage tab → bookmark for daily checks
3. Add `/admin/quota` simple endpoint? OUT OF SCOPE this phase. Just bookmark Firebase console
4. Document in `docs/deployment-guide.md` how to inspect quota

## Todo List
- [ ] Create `server/Dockerfile` + `.dockerignore`
- [ ] Add `ServeStatic` to config + router
- [ ] Local docker build + run smoke test
- [ ] Push to Coolify; configure env + domain
- [ ] Verify `/health` returns 200 over HTTPS
- [ ] Configure `web/firebase.json` hosting + rewrite
- [ ] Build web with prod `VITE_WS_URL`
- [ ] `firebase deploy --only hosting`
- [ ] Configure custom domain in Firebase console + DNS records at registrar
- [ ] Wait for Firebase Hosting cert provisioning
- [ ] E2E smoke test: page load → sign-in → WS connect → play 1 daily puzzle
- [ ] (Optional) Capacitor Android APK build + sideload test
- [ ] Verify $1 Cloud Billing alert active
- [ ] Bookmark Firestore Usage tab + Auth quota dashboards
- [ ] Write `docs/deployment-guide.md`

## Success Criteria
- [ ] `https://dleague.tld` loads React app < 2s
- [ ] `https://api.dleague.tld/health` returns 200 + JSON `{ok: true, store: "firestore"}`
- [ ] Sign-in (anon, email, Google) works in production
- [ ] WS connects from production domain to `api.dleague.tld`
- [ ] Daily puzzle playable end-to-end on production
- [ ] Async match: create on phone, join on laptop, verify result
- [ ] Sync match: 2 devices race
- [ ] Firebase budget alert configured at $1
- [ ] Quota dashboard accessible
- [ ] `docs/deployment-guide.md` complete

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| WS cross-origin Origin header rejected by server | High first-deploy | Med | Pre-set `DLEAGUE_WS_ORIGINS=dleague.tld,www.dleague.tld`; verify by curl with Origin header before web E2E |
| Coolify Traefik doesn't auto-issue cert for `api.` subdomain | Med | Med | Test before web deploy; manual cert via certbot fallback |
| Firebase Hosting custom domain TLS provisioning takes >24h | Low | Low | Use temporary `.web.app` domain for first smoke test |
| Service-account JSON not loaded (env truncated by Coolify) | Med | High | Verify length + parse on boot via log line (mask all but project_id); fail-fast if invalid |
| Coolify free VM auto-suspends after idle | Low | Med | Check OCI Always-Free policy: ARM compute idle-reclaim mitigated by 5-min keepalive cron (carried over from MySQL plan Phase B) |
| Firebase Hosting daily egress 360 MB cap hit | Low | Low | At 100 DAU × 500 KB initial load = 50 MB/day; 14% of cap; fine |
| Firestore project ID in client bundle visible | Cert | Low | By design (public); not a secret |
| Capacitor Android APK requires JDK + SDK | High | Low | Optional this phase; defer if blocking |
| Forgot to update `VITE_WS_URL` for prod build → defaults to `/ws` (relative; broken on Hosting) | High | High | Build script logs final WS URL; CI guard later |
| MOBILE later: APK transitive npm deps version drift | N/A this phase | Low | Defer |

## Security Considerations
- HTTPS everywhere (no HTTP fallback); Hosting + Coolify both auto-TLS
- Service-account JSON env-injected; Coolify masks in UI; verify NOT logged on boot
- Firestore Rules deployed (phase-3); verify in console post-deploy that current rules match committed `firestore.rules`
- WS Origin whitelist locked to known prod domains; localhost dev only when explicitly added
- CSP headers: defer to v2 (Firebase Hosting allows custom headers via `firebase.json`); no XSS risk at MVP since we don't `dangerouslySetInnerHTML`
- Auth domain whitelist (Firebase console) includes ALL deployed origins
- Verify free-tier badge present in Firebase console after deploy (catch accidental Blaze)

## Next Steps
- **Unblocks:** phase-9 (cleanup + docs)
- **Phase-9:** README updated with deploy URLs + stack
- **Post-launch:** monitor quota daily for first 2 weeks; tune denormalization if reads spike

## Unresolved Questions
1. Should Coolify deploy be branch-triggered (push-to-deploy) or manual? Recommend manual at MVP; less surprise
2. Production WSS reverse-proxy: does Coolify Traefik upgrade WS by default? Verify before deploy; may need explicit `traefik.http.routers.dleague.entrypoints=websecure` config
3. CDN for `web/dist/`: Firebase Hosting has built-in CDN; no extra setup. Confirmed
4. Should we run `firebase deploy --only firestore:rules` from CI? Phase-9 v2 candidate
5. Mobile APK signing for distribution: out of scope at MVP (debug-signed only)
