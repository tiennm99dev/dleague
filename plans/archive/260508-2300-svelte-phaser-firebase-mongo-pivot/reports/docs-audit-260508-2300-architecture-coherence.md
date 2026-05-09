# Dleague Documentation Audit: Architecture Coherence
**Date:** 2026-05-08 | **Auditor:** docs-manager

---

## Executive Summary

**Three plan directories exist; two active strategies conflict fundamentally.** The codebase is **Phase 1 complete**, with Phase 2 (game core) ready. **Database decision is locked to MySQL HeatWave** (per commit be9e68b), but an **unapproved Firebase pivot plan exists** (created 2026-05-05 14:15, uncommitted) that supersedes 4+ phase files and contradicts the committed strategy.

**Docs gaps are severe:** only `code-standards.md` exists. Missing: `codebase-summary.md`, `system-architecture.md`, `project-overview-pdr.md`, `deployment-guide.md`, `design-guidelines.md`, `development-roadmap.md`, `project-changelog.md`.

**Phase 2 readiness:** phase file is detailed but only for game core (Ebitengine/Wordle). No ambiguity on game logic; unclear which backend strategy (MySQL or Firebase) applies.

---

## 1. Plan Directory & Strategy Alignment

### Committed Strategy (be9e68b — Main Branch)
- **Primary:** `260505-0947-dleague-pvp-game/plan.md` + 6 phase files (phase-01 completed; 02–06 pending)
- **Locked decision:** Phase 3 will use OCI MySQL HeatWave Always-Free (research chain confirmed in 5 reports)
- **Database plan:** `260505-1319-mysql-heatwave-integration/` (6 phases A–F, scaffolds Go store package + MySQL schema)
- **Status:** All committed; MySQL choice is binding for Phase 3

### Uncommitted Pivot Plan (not on main, created 14:15 2026-05-05)
- **Path:** `260505-1407-firebase-platform-pivot/plan.md` + 9 phase files
- **Scope:** Re-architect entire project — drop Ebitengine WASM client, adopt React/TS + Capacitor; drop MySQL, adopt Firebase (Auth + Firestore + optional RTDB)
- **Supersedes:** Explicitly marks phases 02–05 of main plan + full MySQL HeatWave plan as superseded
- **Status:** Not committed; marked `pending`; no decision record in main branch
- **Effort claim:** 6–8 weeks (vs 9.5w for main plan)

### Concrete Conflicts
| Aspect | Main Plan (Committed) | Firebase Pivot |
|--------|---|---|
| **Client** | Ebitengine (Go → WASM) | React/TS + Capacitor |
| **Game logic** | `shared/game/Game` (Go pluggable interface) | TS implementation in React layer |
| **Backend store** | OCI MySQL HeatWave (OLTP) | Firestore + optional RTDB |
| **Server role** | Full game, auth, match logic | Minimal referee + Admin SDK writer |
| **WebSocket** | Single WS endpoint, all messages | Kept for sync PvP; Firebase Auth sync |
| **Phase 2** | Game core (Ebitengine) | Game engine rewrite (TS) |
| **Phase 3** | Auth + MySQL store | Firebase Auth + Firestore setup |

**Verdict:** These plans cannot coexist. Committing Firebase requires rejecting MySQL plan + rewriting phases 01–02 outputs.

---

## 2. Docs Gaps vs CLAUDE.md Required Structure

### Required by `./CLAUDE.md` (documentation-management.md)
1. **project-overview-pdr.md** — Project overview + PDR (Product Development Requirements)
2. **code-standards.md** — Code standards (EXISTS; 150 LOC; Go-focused)
3. **codebase-summary.md** — Codebase structure summary (MISSING)
4. **design-guidelines.md** — Design system + guidelines (MISSING)
5. **deployment-guide.md** — Deployment instructions (MISSING)
6. **system-architecture.md** — Architecture + system design (MISSING)
7. **development-roadmap.md** — Project phases + milestones (MISSING; plan.md exists but is not in docs/)
8. **project-changelog.md** — Significant changes + features (MISSING)

### Current State
- **In docs/:** Only `code-standards.md` (150 LOC, excellent quality)
- **In plans/:** 6 research reports + 3 plan overviews + 18 phase files (scattered across 3 dirs)
- **README.md:** Good project intro; outdated DB ref (says "Postgres" in stack, contradicts commit be9e68b which locked MySQL)

### Impact
- **Onboarding:** New contributor reads README (sees Postgres), not warned about MySQL decision
- **Architecture visibility:** No single source of truth for system design, deployment, or feature roadmap
- **Maintenance:** Phase updates not reflected in docs/ (phase files scattered in plans/, not versioned in docs/)

---

## 3. Documented Architecture Drift (Concrete Examples)

### README.md (lines 36–41)
```
## Stack
- Client: [Ebitengine]...
- Backend: Go (chi HTTP + nhooyr.io/websocket...)
- DB: Postgres           ← STALE
- Deploy: Fly.io / Railway for backend...
```

**Drift:** Line 40 says "Postgres". Commit be9e68b (2026-05-05 13:41) locked MySQL HeatWave Always-Free.

**Fix:** Update to "MySQL (OCI HeatWave Always-Free)" + cross-reference `plans/260505-1319-mysql-heatwave-integration/plan.md`.

### code-standards.md (lines 32–34)
```
├── server/internal/
    ├── store/      # Postgres repos (Phase 3)
```

**Drift:** References "Postgres repos" but Phase 3 will use MySQL idioms (no `citext`, `uuid` → `BINARY(16)`, `JSONB` → `JSON`).

**Fix:** Change to "MySQL repos" or "RDBMS repos (MySQL flavor per Phase 3-D schema)".

### phase-03-backend-auth.md (lines 11–12, 39–44)
```
> DB binding: see [phase-03 integration](../260505-1319-mysql-heatwave-integration/plan.md)
> DB committed: OCI MySQL HeatWave Always-Free (not PostgreSQL)...

users (id uuid pk, email citext unique...)
```

**Drift:** Header warns schema is "still PG-flavored as originally drafted". Schema uses `uuid`, `citext`, `JSONB` which don't exist in MySQL.

**Status:** Acknowledged but not yet fixed. Phase D of MySQL integration plan will translate.

---

## 4. Phase 2 Readiness Assessment

### File: `plans/260505-0947-dleague-pvp-game/phase-02-game-core-pluggable.md` (135 LOC)

**Strengths:**
- Clear architecture: `Game` interface (`Init`, `HandleKey`, `Tick`, `Render`, `State()`, `IsTerminal()`, `Result()`)
- Detailed Requirements section (functional + non-functional)
- Specific file list to create/modify (12 files named explicitly)
- 98-line implementation step-by-step (11 numbered steps, granular)
- Test coverage target (>80% for `wordle/`)
- Risk mitigation table (6 risks, 6 mitigations)

**Weaknesses:**
- **Backend assumption:** Phase 2 is offline-only; no mention of how game state syncs with server Phase 3 (async/sync PvP)
- **Data serialization:** Line 24 says "Game state serializable to JSON (for later async PvP sync)" but no schema shown
- **Server validation:** Mentions server will re-validate in Phase 3 but doesn't specify wire format or API contract
- **Ebitengine knowledge:** Assumes reader knows Ebiten pattern; two files are "ports" of Ebiten examples with copyright headers

**Readiness verdict:** **Actionable for a planner agent.** Enough detail to delegate implementation. Backend handoff (Phase 3) needs explicit JSON schema and WS message types to avoid rework.

### Missing phase-02 Detail for Backend Integration
- No JSON schema for `GameState` / `Result` (will be needed by Phase 3 WS messages)
- No mention of how `shared/game/` logic will be invoked server-side during validation
- No test boundary: will Phase 2 tests include server-side validation replay?

---

## 5. Single-Source-of-Truth Recommendations

### A. Immediately (Phase 2 blocker)
1. **Resolve plan conflict:** Choose MySQL or Firebase. Cannot proceed with both.
   - If MySQL → Delete Firebase plan dir + document decision in README
   - If Firebase → Mark MySQL plan as superseded, update phase-01 onwards accordingly, rewrite phase-02 for React/TS

2. **Update README.md**
   - Fix DB: "Postgres" → "MySQL (OCI HeatWave Always-Free)" + link to `plans/260505-1319-mysql-heatwave-integration/`
   - Add decision timeline: "Phase 1 complete (2026-05-05). Phase 3 data layer: MySQL via OCI Always-Free (researched in be9e68b). See research reports in `plans/reports/`."

3. **Update code-standards.md**
   - Change "Postgres repos" → "MySQL repos (Phase 3-D schema, see `plans/260505-1319-mysql-heatwave-integration/phase-d-schema-migration-mysql.md`)"

### B. Before Phase 3 Implementation (Medium priority)
4. **Create `docs/codebase-summary.md`**
   - Source: `repomix-output.xml` structure
   - Sections: Project structure, module layout, key files, build commands, testing, deployment checklist
   - Link to phase files for detailed implementation plans
   - ~300 LOC

5. **Create `docs/system-architecture.md`**
   - Go monorepo layout (client/, server/, shared/)
   - Component diagram: WASM client ↔ WebSocket hub ↔ MySQL store
   - Message flow: ClientWS → Router → Handler → Game/Store
   - Data layer: OCI MySQL HeatWave (post-Phase 3), ERD diagram for users/games/attempts/matches
   - Protobuf wire format (binary + debug JSON)
   - ~400 LOC

6. **Create `docs/project-overview-pdr.md`**
   - Product goal (PvP -dle games, Dleague brand)
   - MVP scope (Wordle-style, sync + async PvP, leaderboard)
   - Constraints (YAGNI, KISS, <10MB WASM, 200 LOC/file)
   - Phases + timeline
   - Success metrics (user can play solo, challenge friend, race live opponent)
   - Cross-reference: `plans/260505-0947-dleague-pvp-game/plan.md` for full detail

7. **Create `docs/development-roadmap.md`**
   - Lift phase overview from `plan.md` into docs/
   - Add completion status, blockers, recent commits
   - Link each phase to its plan file
   - Update weekly as work progresses
   - ~150 LOC

### C. Ongoing (Continuous)
8. **Create `docs/project-changelog.md`**
   - Add entry per git commit (starting with Phase 1 complete)
   - Format: date | commit | phase | description | blockers
   - Template for Phase 2–6 as they complete

9. **Maintain cross-references:**
   - All docs link to plans/ when detailed implementation is needed
   - All plans link back to docs/ for context (e.g., phase-03 links to system-architecture.md for DB schema rationale)
   - Code files link to both (e.g., `shared/game/game.go` has comment: "See docs/system-architecture.md → Game Architecture section")

---

## 6. Specific Recommendations by File

### README.md
- **Line 41:** "DB: Postgres" → "DB: MySQL 8.x (OCI HeatWave Always-Free)"
- **Line 41:** Add footnote: "DB decided in Phase 1; see `plans/260505-1319-mysql-heatwave-integration/plan.md`"
- **Add section** before "## Quickstart": "Plan Status — Phase 1 complete (foundation + protobuf wire). Phase 2 pending (game core, Ebitengine Wordle). See `plans/260505-0947-dleague-pvp-game/plan.md` for roadmap."

### code-standards.md (line 33)
- Change `├── store/      # Postgres repos (Phase 3)` → `├── store/      # MySQL repos (Phase 3-D schema MySQL dialect)`

### phase-03-backend-auth.md (new section at top)
- Add explicit note:
  ```
  ## Database Binding
  This phase assumes MySQL 8.x via OCI HeatWave Always-Free (decided Phase 1, schema translation in `260505-1319/phase-d-schema-migration-mysql.md`).
  Update schema below to MySQL idiom:
  - `uuid` → `BINARY(16)` with `UUID_TO_BIN()` helpers
  - `citext` → `VARCHAR(255) COLLATE utf8mb4_general_ci` + unique index on LOWER(email)
  - `JSONB` → `JSON`
  ```

---

## Unresolved Questions

1. **Firebase pivot decision:** Is `260505-1407-firebase-platform-pivot/` an approved alternative or an exploratory dead-end? If approved, who owns the decision? Need decision-record commit on main.

2. **Phase 2 JSON schema:** What is the exact shape of `GameState` and `Result` as serialized to JSON for async PvP? Will it live in `shared/game/types.go` or a separate `.proto` message?

3. **MySQL schema timeline:** Phase D is penciled for 0.5d effort but involves translating 8 SQL statements from PG to MySQL. Is this built into Phase 3 implementation or done as separate MR before Phase 3 starts?

4. **Deployment target:** README says "Fly.io / Railway for backend; static WASM on Cloudflare Pages or same backend". Which one? Fly.io or Railway? Cloudflare or same backend? Needs explicit decision + ops runbook.

5. **Mobile stub (Phase 6):** gomobile bindings are "prepped" but what does "prepped" mean? Just a directory structure or actual Xcode/Android configs? Blocks iOS/Android CI/CD planning.

6. **Firebase free-tier exit plan:** Plan `260505-1407` outlines triggers (70% quota utilization). But if MySQL plan is primary, is Firebase only "if-we-pivot"? Or is it a parallel test track?

---

## Summary Table: Documentation Status

| File | Exists | Quality | Stale | Action |
|------|--------|---------|-------|--------|
| README.md | ✓ | Good | **DB ref (Postgres)** | Update line 40-41 |
| code-standards.md | ✓ | Excellent | Minor (Postgres repo mention) | Update line 33 |
| codebase-summary.md | ✗ | — | — | **CREATE** from repomix |
| system-architecture.md | ✗ | — | — | **CREATE** (ERD, message flow) |
| project-overview-pdr.md | ✗ | — | — | **CREATE** (lift from plan.md) |
| development-roadmap.md | ✗ | — | — | **CREATE** (phases, status, ETA) |
| deployment-guide.md | ✗ | — | — | **CREATE** (Fly.io or Railway runbook) |
| design-guidelines.md | ✗ | — | — | **CREATE** (Wordle UI, animations) |
| project-changelog.md | ✗ | — | — | **CREATE** (git log → structured entries) |

---

## Architecture Coherence Score

| Dimension | Score | Notes |
|-----------|-------|-------|
| **Plan alignment** | 40% | Two conflicting strategies; MySQL is committed, Firebase is speculative |
| **Docs-to-code sync** | 35% | README outdated; phases exist but scattered; no codebase-summary or architecture |
| **Phase 2 clarity** | 85% | Detailed game core spec; backend integration schema missing |
| **Decision documentation** | 50% | MySQL decision in commit message + plan.md; no decision-record or approval flow doc |
| **Onboarding readiness** | 25% | New contributor has code-standards but no project overview, architecture, or roadmap |

**Overall: 47%** (Needs docs consolidation + plan conflict resolution before Phase 2 implementation.)

---

**Status:** DONE_WITH_CONCERNS

**Summary:** Audit complete. Three critical blockers identified: (1) Firebase vs MySQL plan conflict unresolved, (2) README database reference stale, (3) 7 required docs missing. Phase 2 is implementable but backend handoff (Phase 3) needs JSON schema + explicit MySQL binding. Recommend resolving plan conflict and updating README before delegating Phase 2 to implementation.

**Concerns:** Firebase pivot plan is detailed and well-researched but uncommitted; unclear if it's a serious alternative or exploratory spike. If exploratory, recommend moving to separate branch or marking explicitly as "research-only / not planned". If serious candidate, need decision-record commit and leadership buy-in before proceeding.
