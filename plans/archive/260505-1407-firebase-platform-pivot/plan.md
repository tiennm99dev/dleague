---
title: "Firebase + React/TS + Capacitor platform pivot"
description: "Drop Ebitengine WASM client and MySQL HeatWave backend store. Adopt React/TS + Capacitor web-first client and Firebase (Auth + Firestore + optional RTDB) as the backend store. Go server stays as referee/admin-SDK writer. Free tier only, testing scale."
status: superseded
superseded_by: ../260505-1604-firebase-couchbase-redis-pivot/plan.md
priority: P1
effort: 6-8w
branch: main
tags: [firebase, firestore, react, typescript, capacitor, websocket, pivot, free-tier, superseded]
created: 2026-05-05
parent_plan: 260505-0947-dleague-pvp-game/plan.md
supersedes:
  - 260505-0947-dleague-pvp-game/phase-02-game-core-pluggable.md
  - 260505-0947-dleague-pvp-game/phase-03-backend-auth.md
  - 260505-0947-dleague-pvp-game/phase-04-async-pvp.md
  - 260505-0947-dleague-pvp-game/phase-05-sync-pvp-websocket.md
  - 260505-1319-mysql-heatwave-integration/plan.md
research:
  - reports/researcher-260505-1407-engine-survey-web-first.md
  - reports/researcher-260505-1407-firebase-as-backend-feasibility.md
  - reports/decision-record-260505-1407-platform-pivot.md
---

# Firebase platform pivot

> **Superseded by:** [`260505-1604-firebase-couchbase-redis-pivot/`](../../260505-1604-firebase-couchbase-redis-pivot/plan.md). The Firestore-as-primary-store path was discarded in favor of a self-hosted Couchbase 8.0 + Redis 8.4 data plane on the Coolify VM (Firebase Auth retained). Reasons: free-tier Firestore quotas + sync-PvP write fan-out costs, vendor lock-in, and `store.Store` migration seam easier to enforce on Go-owned drivers. Kept here as historical context.

## Goal

Re-baseline dleague onto a **web-first React/TS + Capacitor** client and **Firebase Auth + Firestore (+ optional Realtime DB)** backend store, while preserving the Go server as the gameplay referee. Eliminate self-hosted SQL operational burden at testing scale. Stay on free tier with explicit migration-back triggers.

## What stays / what goes

| Component | Verdict | Notes |
|-----------|---------|-------|
| Go workspace, `server/cmd/api`, chi router | KEEP | server stays referee + Admin-SDK writer |
| `server/internal/ws/` (nhooyr WebSocket, hub, conn) | KEEP | sync transport for live PvP |
| `proto/dleague/v1/`, `shared/pb/` | KEEP + EXTEND | add `AUTH_HELLO`, game messages |
| `server/internal/store/` (MySQL Store) | SOFT-DEPRECATE | code stays compiled, gated by `STORE_BACKEND` env; no runtime cost when `firestore` selected. Delete in phase-9 once Firestore proven |
| `client/` (Ebitengine Go→WASM) | DELETE | replaced by Vite-built React app |
| `web/index.html`, `web/wasm_exec.js`, `web/main.wasm`, `web/styles.css` | DELETE | replaced by Vite `dist/` |
| MySQL HeatWave provisioning (plan `260505-1319`) | SUPERSEDE | mark superseded; OCI VM still hosts Coolify backend, no DB attached |

## Phases

| # | Phase | File | Status | Effort |
|---|-------|------|--------|--------|
| 1 | Firebase project provisioning (manual) | [phase-1-firebase-project-provisioning.md](phase-1-firebase-project-provisioning.md) | pending | 0.5d |
| 2 | Server: Firebase Admin SDK + auth gate | [phase-2-server-firebase-admin-integration.md](phase-2-server-firebase-admin-integration.md) | pending | 4d |
| 3 | Firestore data model + security rules | [phase-3-firestore-data-model-and-rules.md](phase-3-firestore-data-model-and-rules.md) | pending | 2d |
| 4 | React + Capacitor client bootstrap | [phase-4-react-capacitor-client-bootstrap.md](phase-4-react-capacitor-client-bootstrap.md) | pending | 5d |
| 5 | Pluggable game engine (TS) | [phase-5-pluggable-game-engine-web.md](phase-5-pluggable-game-engine-web.md) | pending | 4d |
| 6 | Async PvP on Firestore | [phase-6-async-pvp-on-firestore.md](phase-6-async-pvp-on-firestore.md) | pending | 4d |
| 7 | Sync PvP transport decision (WS over RTDB) | [phase-7-sync-pvp-firebase-or-ws.md](phase-7-sync-pvp-firebase-or-ws.md) | pending | 4d |
| 8 | Deploy: Coolify backend + Firebase Hosting | [phase-8-deploy-coolify-firebase-hosting.md](phase-8-deploy-coolify-firebase-hosting.md) | pending | 2d |
| 9 | Supersede + cleanup parent plans, README, docs | [phase-9-supersede-parent-phases-and-cleanup.md](phase-9-supersede-parent-phases-and-cleanup.md) | pending | 1d |

**Total active effort:** ~26 days (~5–6 weeks solo).

## Key decisions (locked)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Sign-in methods | Google + Email/Pass + Anonymous | All three. Anonymous unlocks "play as guest, upgrade later" via `linkWithCredential` |
| User PK | Firebase UID | Drops `users.id` BINARY(16); accepts deeper Firebase lock-in |
| Session model | ID token verified per WS upgrade (every reconnect) | TTL=1h, server revalidates. NO sessions table |
| Server creds | `FIREBASE_CREDENTIALS_JSON` env (Coolify-injected) | Single env var, no file mount |
| Frontend engine | React 18 + TS + Vite + Capacitor | Per engine-survey research: web-native, WCAG 2.1 AA, smallest bundle |
| Backend store | Firestore (canonical) + RTDB (optional, sync only) | Per Firebase feasibility research |
| Cloud Functions | NOT USED | Spark plan no longer includes Functions; Go server does aggregation |
| Trust model | Server-mediated writes via Admin SDK; client direct writes only for ephemeral presence | Security rules deny client writes on `/users/*`, `/matches/*`, `/attempts/*` |
| Wire change | First WS frame = `MESSAGE_TYPE_AUTH_HELLO` envelope | Server rejects all subsequent frames until ID token verified |
| Sync PvP transport | Go WebSocket (server-authoritative); persist final attempt to Firestore | Avoids Firestore listener cost; RTDB only if WS proves inadequate |

## Free-tier exit plan

**Trigger migration-back when ANY of:**
- Firestore writes >70% of 20k/day (~14k writes/day) for 3 consecutive days → ~400 DAU per research
- Firestore reads >70% of 50k/day (~35k reads/day) for 3 consecutive days
- Firestore storage >800 MiB (80% of 1 GiB)
- RTDB downloads >7 GB/month (70% of 10 GB)
- Auth MAU >40k/month (80% of 50k)

**Migration target:** Resurrect MySQL HeatWave plan `260505-1319-mysql-heatwave-integration/` (kept warm via soft-deprecated `server/internal/store/` package). Effort estimate per Firebase research: ~1 day ETL + 2–3h downtime.

**Monitoring:** Phase 8 includes Firebase console budget alert at $1 (catches accidental Blaze auto-upgrade). Daily quota dashboards bookmarked.

## Open risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| ID token verify on every reconnect adds ~50–200ms latency on cold path | Low | Cache decoded token in-memory keyed by token-hash with TTL=token expiry |
| Anonymous → permanent account upgrade collision (UID changes) | Med | Phase 4 documents `linkWithCredential` flow; UID stays same after link |
| Firestore composite indexes count toward project limits (200 max) | Low | Phase 3 enumerates required composites; <10 expected |
| Free-tier daily reset at midnight Pacific creates EU/Asia evening cliff | Low | Document timezone in phase-8; surface quota in `/health` |
| Coolify reverse-proxy + Firebase Hosting CORS for `/ws` | Med | Phase 8 specifies: Hosting serves React; client opens WS to `wss://api.dleague.tld/ws` direct (separate hostname) |
| Capacitor iOS build requires macOS + Xcode | Low | Defer iOS to post-web; Android first via Capacitor + Android Studio (Linux OK) |
| Realtime DB connection cap 100 simultaneous | Low | Sync PvP uses Go WS, not RTDB. RTDB only used if WS migration ever needed |

## Dependencies

- Firebase project (created in phase-1)
- Service-account JSON (env-injected as `FIREBASE_CREDENTIALS_JSON`)
- Coolify VM keeps running (existing OCI ARM Always-Free VM)
- Domain: root `dleague.tld` → Firebase Hosting; `api.dleague.tld` → Coolify backend
- Wordlist (open-source Wordle answer list, 2315 words) — phase-5

## Success criteria

- [ ] User signs in via Google, email/pass, OR anonymous; same UID stable across reconnects
- [ ] Anonymous user upgrades to permanent (Google or email) without losing match history
- [ ] WS connection rejects frames until `AUTH_HELLO` ID token verified
- [ ] Daily Wordle puzzle loads from Firestore, server validates each guess
- [ ] Async match: creator → joiner → result comparison; both attempts in Firestore
- [ ] Sync match: 1v1 race over WS, winner persisted to Firestore on completion
- [ ] React app deployed to Firebase Hosting; Go server on Coolify; both reachable
- [ ] Firestore Security Rules enforce server-mediated-writes-only on `/matches`, `/attempts`, `/users`
- [ ] `/health` reports Firestore reachability + (optional) MySQL fallback status
- [ ] Phase-1/3-5 of parent plan marked superseded; README stack section rewritten
- [ ] Free-tier quota alerts configured in Firebase console
