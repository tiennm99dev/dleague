---
phase: 6
title: "Polish + deploy + mobile prep"
status: pending
priority: P2
effort: 1.5w
dependencies: [5]
---

# Phase 6: Polish + deploy + mobile prep

## Overview

Production-ready: deploy backend + WASM to public web, observability, onboarding polish, mobile build target compiles via gomobile (no app store yet — defer to v2). Last MVP phase before launch.

## Requirements

- **Functional:**
  - Public domain wired (dleague.gg or fallback) with HTTPS
  - WASM bundle served gzip+brotli with proper cache headers
  - Onboarding tutorial: first-time user sees brief explainer overlay (skippable)
  - Result share images: server generates OG-image PNG of match summary for social sharing
  - Error reporting: Sentry or self-hosted GlitchTip for client + server errors
  - Basic analytics: page views + key events (game_start, game_won, match_created, queue_joined)
  - Privacy + terms pages
  - Mobile target: `gomobile bind` produces .aar (Android) and .xcframework (iOS) — verified compile only, no app
- **Non-functional:**
  - Time-to-first-game <3s on 4G (measured)
  - WASM bundle <10MB gzipped
  - 99% uptime target (single region acceptable for MVP)
  - Zero-downtime deploy via Fly.io rolling restart

## Architecture

**Deploy topology:**

```
Cloudflare DNS → Fly.io (api.dleague.gg)
                      ├── Go binary (chi + WS hub)
                      ├── Embedded static assets (web/, dist/wasm/)
                      └── Postgres (Fly managed PG, single region)

Cloudflare Pages (dleague.gg)  ← optional split: static front, api on subdomain
```

**Decision:** monolithic deploy first (Go binary serves both static + API). Split static→Cloudflare only if WASM bandwidth costs spike.

**Mobile prep:**
- Refactor `client/internal/scene` to be platform-agnostic (already mostly is via Ebitengine)
- Stub mobile entry: `client/cmd/mobile/main.go` exposes `RunGame()` for gomobile
- Verify `gomobile bind` compiles cleanly; defer device testing + UX redesign to post-launch

## Related Code Files

**Create:**
- `client/cmd/mobile/main.go` (gomobile entry)
- `server/internal/static/embed.go` (`go:embed dist/wasm/* web/*`)
- `server/internal/observability/{sentry.go, metrics.go}`
- `server/internal/http/share_image_handler.go` (OG-image PNG generator using `disintegration/imaging`)
- `client/internal/scene/onboarding.go`
- `web/privacy.html`, `web/terms.html`
- `fly.toml`
- `.github/workflows/deploy.yml`
- `Dockerfile` (multi-stage: build Go → distroless runtime)
- `docs/deployment.md`

**Modify:**
- `Makefile` (add `deploy`, `build-mobile-android`, `build-mobile-ios`)
- `server/cmd/api/main.go` (mount embedded static, init Sentry, graceful shutdown)
- `client/internal/scene/title.go` (gate first-time visit through onboarding)

## Implementation Steps

1. Multi-stage Dockerfile: stage 1 builds WASM + binary, stage 2 distroless with binary + static
2. `fly.toml` config: Postgres attached, autoscale off (single instance MVP), HTTPS forced
3. Embed `dist/wasm/main.wasm` + `web/*` into binary via `go:embed`
4. Server route: `GET /` → embedded HTML; `GET /static/*` → embedded WASM/CSS with cache headers
5. Cache: `main.wasm` immutable (filename hashed at build); `index.html` no-cache
6. Sentry init for both client (via JS bridge) and server (Go SDK)
7. Lightweight analytics: emit events to `analytics_events` table (or Plausible self-hosted later)
8. Onboarding scene: 3-screen explainer, "skip" + "got it" buttons, persist `onboarded` flag in cookie
9. OG-image: server endpoint `/og/match/{id}.png` renders match summary (winner, attempts, hints) as 1200×630 PNG
10. Add `<meta property="og:image">` injection per match URL for share unfurls
11. Privacy + terms pages (template starter, customize for jurisdiction)
12. `gomobile bind -target=android,ios` → verify build, store .aar/.xcframework as CI artifact (no upload yet)
13. Deploy workflow: GHA on push to main → build → fly deploy → smoke test
14. Add Lighthouse CI step to track perf budget
15. Document deploy in `docs/deployment.md`

## Todo List

- [ ] Multi-stage Dockerfile
- [ ] `fly.toml` + Postgres attached
- [ ] Static asset embedding + cache headers
- [ ] Sentry / error reporting
- [ ] Analytics event pipeline
- [ ] Onboarding scene
- [ ] OG-image share endpoint
- [ ] Privacy + terms pages
- [ ] gomobile compile target
- [ ] Deploy GHA workflow
- [ ] Lighthouse CI
- [ ] `docs/deployment.md`

## Success Criteria

- [ ] `git push main` → deploys to dleague.gg within 5min
- [ ] Lighthouse: Performance >85, Accessibility >90 (HTML overlay path), Best Practices >90
- [ ] WASM bundle <10MB gzipped (verified via CI)
- [ ] Twitter/Discord match-share unfurls show OG image
- [ ] gomobile compiles without errors for both Android + iOS targets
- [ ] Sentry captures injected test error from staging
- [ ] First-time visitor completes onboarding + first game in <60s

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| Bundle size bloat from gomobile + animations | Tree-shake unused Ebitengine subpackages; measure in CI |
| Fly.io single region latency for global users | Acceptable at MVP; document; multi-region post-launch |
| OG-image generation slow → 5xx on Twitter unfurl | Cache rendered PNG by match-id; CDN at Cloudflare |
| Sentry quota burn from WASM panics | Sample at 10%, sample errors at 100% |
| Mobile UX completely different from web | Defer mobile launch — gomobile here just proves portability |

## Security Considerations

- HTTPS-only (HSTS preload eligible after stable)
- CSP header tightened: `default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; img-src 'self' https://cdn.dleague.gg`
- Privacy policy lists analytics + error reporting + cookie usage
- No PII in OG images (display name only)
- Rate-limit OG endpoint to prevent abuse

## Next Steps (post-launch v2)

- Multi-region (Fly.io regions + Postgres replica)
- Redis matchmaking queue
- ELO/MMR ranked ladder
- Second -dle game type (music or geography)
- Mobile app store releases
- Localization (Vietnamese first, given user's MathMax audience)
- Cosmetics + monetization (themes, avatars)
