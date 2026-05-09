---
title: "Decision Record: Firebase + React + Capacitor platform pivot"
date: "2026-05-05"
author: "planner"
status: "approved"
related_plan: "plans/260505-1407-firebase-platform-pivot/plan.md"
supersedes_plans:
  - plans/260505-0947-dleague-pvp-game/plan.md (phases 2–5)
  - plans/260505-1319-mysql-heatwave-integration/plan.md
---

# Decision Record: Platform Pivot

## Context

Phase 1 (foundation) shipped: Go workspace, WS scaffolding, protobuf wire, `/health`, `/ws` ping-pong. Phase 2 (Ebitengine game core) and parallel MySQL HeatWave integration both unstarted code beyond scaffolding. Two research reports landed same day (`260505-1407`):
1. Web engine survey → recommends React/TS + Capacitor over Ebitengine for -dle genre
2. Firebase free-tier feasibility → confirms viable at ≤100 DAU, exit plan well-defined

Decision window pre-Phase-2 commit: pivot now or pay 4× rework cost later.

## Decision

**Pivot to:**
- Frontend: React 18 + TypeScript + Vite + Capacitor wrap (drops Ebitengine WASM)
- Backend store: Firebase Firestore + Firebase Auth (drops MySQL HeatWave AT TESTING SCALE)
- Auth: Firebase ID tokens verified per WS upgrade (drops sessions table)
- Server: Go stays as referee + Admin SDK writer
- Wire: single-WebSocket protobuf, with new AUTH_HELLO first frame
- Free-tier locked, exit triggers documented

## Drivers

| Driver | Weight |
|--------|--------|
| Ship speed (web MVP) | High — engine survey: 4–6w with React vs 8–12w Ebitengine |
| Zero DB ops at testing scale | High — Firestore eliminates schema migrations + connection pooling concerns |
| WCAG 2.1 AA accessibility | Med — semantic HTML free; canvas a11y debt avoided |
| Free-tier hard limit | Cert — testing phase, $0 spend tolerance |
| Anonymous play (guest mode) | Med — Firebase Auth has it built in |
| Stability of stack | Med — React + Firebase both mature 10+ years |

## Alternatives considered

| Alternative | Rejected because |
|-------------|------------------|
| Stay Ebitengine + MySQL | Slower web ship, a11y debt, DB ops burden, no anonymous-account upgrade pattern |
| Flutter + Flame + MySQL | Dart ramp-up + canvas a11y still problematic |
| Phaser + MySQL | Better than Ebitengine but unnecessary game-engine overhead for a UI puzzle |
| React + MySQL (no Firebase) | Auth + sessions still need building; testing-scale DB ops still felt |
| React + Supabase | Firebase has better free tier for anonymous + social auth in our region |

## Trade-offs accepted

| Trade-off | Mitigation |
|-----------|-----------|
| Firebase ecosystem lock-in | Server `store` package soft-deprecated, kept as escape hatch; Firestore export tooling documented |
| Daily quota cliffs (50k reads, 20k writes) | Monitor at 70% threshold; aggregation in Go server, not per-user reads |
| ID-token verify latency on every WS reconnect (~50–200ms cold) | Cert cache warm in Admin SDK after first call; in-process across reconnects |
| Anonymous → permanent UID stability | Use `linkWithCredential`; UID stays same; documented in phase-4 |
| Capacitor mobile path (vs gomobile) | Standard pattern; Android first; iOS deferred (needs macOS) |
| Cloud Functions unavailable on Spark | Server-side aggregation in Go (already running) |

## Exit plan

Migrate back to MySQL HeatWave when ANY:
- Firestore writes >70% of 20k/day for 3 consecutive days (~400 DAU)
- Firestore reads >70% of 50k/day for 3 consecutive days
- Firestore storage >800 MiB
- Firebase Auth MAU >40k/month

Migration cost (per Firebase research report): ~1d ETL + 2–3h downtime. MySQL `store` package retained warm.

## Status

Approved 2026-05-05. Implementation tracked in `plans/260505-1407-firebase-platform-pivot/plan.md`.
