# Phase 9: Supersede parent phases + cleanup + docs

## Context Links
- Parent: `plans/260505-0947-dleague-pvp-game/plan.md` (phase 2/3/4/5 to mark superseded)
- MySQL plan to supersede: `plans/260505-1319-mysql-heatwave-integration/plan.md` + 6 phase files
- README: `/config/workspace/tiennm99/dleague/README.md`
- Docs: `docs/code-standards.md` (only existing doc); `docs/system-architecture.md` (to create)
- Locked: keep `server/internal/store/` package compiled but soft-deprecated until firestore proven; delete after free-tier exit verified safe

## Overview
- **Priority:** P2 (cleanup; non-blocking but required for clarity)
- **Status:** pending
- **Effort:** 1d
- Mark all superseded plan phases. Update README + docs. Decide MySQL `store` fate. Sync TS + Go Firestore type mirrors. Strip dead WASM Make targets. Add free-tier monitoring runbook.

## Key Insights
- Soft-deprecate vs hard-delete tradeoff for `server/internal/store/`:
  - **Keep (soft-deprecate):** ~400 LOC of compiled Go; zero runtime cost when `STORE_BACKEND=firestore`; preserves the migration-back escape hatch the free-tier exit plan relies on
  - **Delete:** removes maintenance burden, removes mysql driver from dep tree (~5 MB)
  - **Decision:** KEEP for now; delete only after 30 consecutive days of stable firestore production AND zero quota near-misses. Document trigger in this phase.
- Parent plan supersession: don't delete the parent plan files; just add a YAML frontmatter `status: superseded` + a top-of-file pointer to this plan
- TS↔Go type drift is the biggest long-term hazard; document a "review checklist" both sides update in same PR

## Requirements

### Functional
1. Add `status: superseded` + `superseded_by` frontmatter to:
   - `plans/260505-0947-dleague-pvp-game/phase-02-game-core-pluggable.md`
   - `plans/260505-0947-dleague-pvp-game/phase-03-backend-auth.md`
   - `plans/260505-0947-dleague-pvp-game/phase-04-async-pvp.md`
   - `plans/260505-0947-dleague-pvp-game/phase-05-sync-pvp-websocket.md`
   - `plans/260505-1319-mysql-heatwave-integration/plan.md`
   - All 6 phase files in mysql-heatwave plan
2. Update parent `plans/260505-0947-dleague-pvp-game/plan.md` phase table — mark phases 2/3/4/5 superseded; phases 1 + 6 remain valid (foundation done; polish/deploy redirected to phase-8 here)
3. Update `README.md`:
   - Stack section: replace "Ebitengine WASM" with "React 18 + TypeScript + Vite + Capacitor"
   - Stack section: replace "Postgres" with "Firebase Firestore (testing) / MySQL HeatWave (fallback)"
   - Quickstart section: replace `make dev` flow with new web + server dual-process flow
   - Repo layout: update to reflect deleted `client/`, refactored `web/`
   - Phase table: link to new plan + describe new phase numbering
4. Create `docs/system-architecture.md`:
   - Component diagram (mermaid): React+Capacitor → WSS → Go server → Firebase (Auth + Firestore)
   - Sequence diagrams: auth handshake, async match, sync match
   - Data model: Firestore collections (from phase-3)
   - Free-tier exit plan summary
5. Update `docs/code-standards.md`:
   - Add TS section: kebab-case files, `.tsx` for React, strict TS, file size limit, no `any`
   - Add Firestore section: doc shape changes require BOTH TS + Go mirror updates in same PR
   - Add proto section: regenerated `.pb.go` and `.ts` committed
6. MySQL `store` decision documented; placeholder `// Deprecated:` comment added to `server/internal/store/store.go` package doc
7. Strip `Makefile` of dead WASM targets (already partially done in phase-4; verify)
8. Decision-record report at `plans/reports/decision-record-260505-1407-platform-pivot.md`

### Non-functional
- README updated under 100 LOC
- system-architecture.md under 250 LOC

## Architecture (cleanup map)

### Files to modify
- `README.md`
- `plans/260505-0947-dleague-pvp-game/plan.md`
- `plans/260505-0947-dleague-pvp-game/phase-02-game-core-pluggable.md` (frontmatter only)
- `plans/260505-0947-dleague-pvp-game/phase-03-backend-auth.md` (frontmatter only)
- `plans/260505-0947-dleague-pvp-game/phase-04-async-pvp.md` (frontmatter only)
- `plans/260505-0947-dleague-pvp-game/phase-05-sync-pvp-websocket.md` (frontmatter only)
- `plans/260505-1319-mysql-heatwave-integration/plan.md` (frontmatter)
- `plans/260505-1319-mysql-heatwave-integration/phase-{a,b,c,d,e,f}-*.md` (frontmatter)
- `server/internal/store/store.go` (Deprecated: comment)
- `server/internal/store/users.go` (same)
- `server/internal/store/migrate.go` (same)
- `Makefile` (verify WASM targets gone)
- `docker-compose.yml` (delete or empty if no services remain)
- `.gitignore` (verify covers `web/dist/`, `web/node_modules/`, `secrets/*.json`)

### Files to create
- `docs/system-architecture.md`
- `plans/reports/decision-record-260505-1407-platform-pivot.md`

### Files to delete (deferred decisions)
- `server/internal/store/` (DEFER per soft-deprecate decision)
- `client/` (already deleted in phase-4)
- `docker-compose.yml` (DELETE if no services; or keep as stub)

## Implementation Steps

### Frontmatter supersession
1. For each superseded phase file, prepend or amend frontmatter:
   ```yaml
   ---
   ...existing fields...
   status: superseded
   superseded_by: plans/260505-1407-firebase-platform-pivot/plan.md
   superseded_on: 2026-05-05
   ---
   ```
2. At top of each file body (right after frontmatter), add admonition block:
   ```
   > **SUPERSEDED** by [plans/260505-1407-firebase-platform-pivot/plan.md](../260505-1407-firebase-platform-pivot/plan.md). Original content preserved below for context.
   ```
3. For MySQL plan + 6 phase files: same treatment

### Parent plan table update
1. Edit `plans/260505-0947-dleague-pvp-game/plan.md` phase table:
   - Phase 2 status → `superseded` + new phase ref
   - Phase 3 status → `superseded` + new phase refs (2, 3)
   - Phase 4 status → `superseded` + new phase ref (6)
   - Phase 5 status → `superseded` + new phase ref (7)
   - Phase 6 status → keep but re-scope to "covered by phase-8 of new plan"
2. Add note section: "PIVOT NOTICE — see plans/260505-1407-firebase-platform-pivot/"

### README rewrite (sections)
1. Stack section:
```
## Stack
- **Client:** React 18 + TypeScript + Vite, Capacitor for mobile wrap
- **Backend:** Go (`chi` HTTP + `nhooyr.io/websocket`); Firebase Admin SDK for Auth verify + Firestore writes
- **Auth:** Firebase Auth (Google + Email/Pass + Anonymous); ID tokens verified per WS upgrade
- **Database:** Firebase Firestore (Spark free tier); MySQL HeatWave kept as fallback (soft-deprecated `server/internal/store/`)
- **Hosting:** Firebase Hosting (web), Coolify on OCI ARM (Go server), Capacitor Android (mobile, Phase 8)
- **Wire:** Single WebSocket carrying binary protobuf envelopes; first frame = AUTH_HELLO
```
2. Quickstart:
```bash
make tools         # protoc + buf + protobuf-ts
make proto-gen     # Go + TS protobuf code

# install web deps
cd web && npm install && cd ..

# run server + vite dev (parallel)
make dev
```
3. Repo layout:
```
dleague/
├── server/        # Go HTTP API + WS hub + Firestore Admin SDK
├── shared/        # Generated Go protobuf
├── proto/         # .proto sources
├── web/           # React + TS + Vite + Capacitor
├── docs/          # design + deploy docs
├── plans/         # implementation plans
├── secrets/       # gitignored — service-account JSON for local dev
├── firestore.rules
├── firestore.indexes.json
└── firebase.json
```
4. Phase table → swap to new plan link

### docs/system-architecture.md
Sections:
- Overview (1 paragraph)
- Component diagram (mermaid)
- Auth handshake sequence (mermaid)
- Async match sequence (mermaid)
- Sync match sequence (mermaid)
- Firestore data model (table from phase-3)
- Free-tier limits + exit triggers (table from plan.md)
- Trust model (server-mediated writes)
- Security rules summary

### Soft-deprecate `store` package
Edit `server/internal/store/store.go` package doc:
```go
// Package store is the dleague data layer over MySQL HeatWave.
//
// DEPRECATED: As of 2026-05-05, dleague uses Firebase Firestore by default
// (see server/internal/firestore/). This package is kept compiled and
// gated behind STORE_BACKEND=mysql for free-tier exit / fallback per
// plans/260505-1407-firebase-platform-pivot/plan.md. Do NOT add new
// features here. Will be deleted after 30 consecutive days of stable
// Firestore production.
package store
```

### Decision-record report
`plans/reports/decision-record-260505-1407-platform-pivot.md`:
- Decision: pivot to Firebase + React + Capacitor
- Date: 2026-05-05
- Decision-makers: solo dev
- Drivers: web-first ship speed (per engine survey), zero DB ops (per Firebase research), free-tier locked
- Alternatives considered: stay with Ebitengine + MySQL HeatWave; Flutter + Flame + MySQL; Phaser + MySQL
- Rejected because: WASM a11y debt, slower web ship, MySQL ops burden at testing scale
- Trade-offs accepted: Firebase lock-in, daily quota cliffs, ID-token re-verify latency
- Exit plan: documented in plan.md "Free-tier exit plan" section

## Todo List
- [ ] Add superseded frontmatter to 4 parent phase files (02–05)
- [ ] Add superseded frontmatter to MySQL plan + 6 phase files
- [ ] Update parent plan.md phase table
- [ ] Rewrite README stack + quickstart + repo layout + phase table
- [ ] Create docs/system-architecture.md (with mermaid diagrams)
- [ ] Update docs/code-standards.md (TS + Firestore + proto sections)
- [ ] Add Deprecated comment to server/internal/store/store.go pkg doc
- [ ] Verify Makefile WASM targets removed
- [ ] Delete or stub docker-compose.yml (no postgres needed)
- [ ] Verify .gitignore covers all new artifacts
- [ ] Create decision-record report
- [ ] Sync verify: TS types in web/src/types/firestore-docs.ts == Go types in server/internal/firestore/types.go
- [ ] Add "Type-mirror review" checklist item to docs/code-standards.md
- [ ] Final pass: grep for "Ebitengine", "WASM", "MySQL", "Postgres" in repo; update or comment as historical

## Success Criteria
- [ ] All superseded files clearly marked
- [ ] README reflects new stack accurately; no Ebitengine/WASM mentions in active sections
- [ ] docs/system-architecture.md exists, builds (mermaid renders), <250 LOC
- [ ] docs/code-standards.md covers TS conventions
- [ ] `server/internal/store/` package compiles, has Deprecated comment
- [ ] No dead Makefile targets
- [ ] Decision-record report committed
- [ ] TS + Go Firestore type mirrors are in sync (manual diff verification)

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| README confusion if stack section wrong | Med | Med | Cross-check against plan.md "What stays / what goes" table |
| Soft-deprecated `store` pkg rots over time | High | Low | Track 30-day-stable trigger; delete decisively at threshold |
| Mermaid diagrams render differently in viewers | Low | Low | Test in GitHub view + local markdown viewer |
| Type-mirror drift (TS vs Go) | High | Med | Code-standards explicit "same PR" rule; future codegen tool deferred |
| `docker-compose.yml` deletion breaks existing dev workflow | Low | Low | Check no Make target references it; if dev needs it, keep stub |
| Forgot phase to mark superseded | Med | Low | Grep pre-merge: `grep -rn "status: pending" plans/260505-0947-` |
| `client/` deletion regret | Low | Low | git history retains; can `git revert` if needed |

## Security Considerations
- Verify `.gitignore` blocks `secrets/`, `*.local`, `firebase-adminsdk*.json`, `web/.env.local`
- Verify no service-account JSON committed (run `git log --all -S 'service_account'` final check)
- Verify `docs/system-architecture.md` does NOT include any project IDs or secrets

## Next Steps
- After phase-9, plan complete; transition to operating phase
- Monitor free-tier quota daily for 2 weeks
- After 30 days stable: trigger MySQL `store` package deletion task

## Unresolved Questions
1. When exactly is "30 consecutive days stable"? Define metric: zero >70% quota days, zero auth errors, zero data inconsistency reports
2. Should `docker-compose.yml` keep an empty stub or delete? KISS says delete; keep IF Capacitor Android testing needs containerized dev DB later (unlikely)
3. Should we tag a release `v0.1.0-firebase-pivot` to mark the cutover commit? Recommend yes; helps git-bisect later
4. CI verification of TS/Go type mirror: defer to v2 (codegen from .proto into both TS and Go is the long-term solution)
