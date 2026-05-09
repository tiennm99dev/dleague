---
phase: 1
title: "Archive superseded plans + docs bootstrap"
status: completed
priority: P1
effort: 0.5w
dependencies: []
completed_on: 2026-05-09
---

# Phase 01 — Archive superseded plans + docs bootstrap

## Context Links
- `plans/reports/docs-audit-260508-2300-architecture-coherence.md` (drift, missing docs, archive)
- Old plans: `plans/260505-0947-dleague-pvp-game/`, `plans/260505-1319-mysql-heatwave-integration/`, `plans/260505-1407-firebase-platform-pivot/`
- `README.md` line 40 (stale "Postgres") + lines 36-67 (stale stack/repo-layout/phases)
- `docs/code-standards.md` line 33 (Postgres repos), Ebitengine refs

## Overview
Resolve plan-conflict drift identified in docs-audit. Move three superseded plan dirs to `plans/archive/` preserving their frontmatter + cancellation reason. Update `README.md` + `docs/code-standards.md` to reflect new stack. Create skeletons for the seven required `docs/` files (CLAUDE.md mandate). No code touched.

## Key Insights
- docs-audit: only `code-standards.md` exists; 7 required docs missing. Pivot makes archival required, not optional.
- README "Postgres" already stale even pre-pivot (commit be9e68b locked MySQL); pivot now locks Mongo. Single source of truth must be README + new plan.md.
- Cancellation reason for the three archived plans must be preserved so future readers don't re-resurrect them.

## Requirements
**Functional:**
- Three plan dirs moved to `plans/archive/<original-name>/` intact.
- `plans/archive/README.md` exists, lists each archived plan with reason + supersede pointer.
- README.md reflects: SvelteKit+Phaser client, Mongo Atlas, Firebase Auth, Fly.io.
- `docs/code-standards.md` Postgres → Mongo idioms; Ebitengine → SvelteKit+Phaser; WASM bundle → JS bundle target.
- 6 new docs files created (skeletons with TODO sections).

**Non-functional:**
- All edits markdown-only; no source code changed.
- Each new `docs/*.md` <150 LOC skeleton.
- Existing frontmatter on archived plan.md files unchanged (only `status: cancelled` + `cancellation_reason` added if absent).

## Architecture
Pure documentation move + content updates. No data flow.

```
plans/
├── archive/
│   ├── README.md               # NEW — index of archived
│   ├── 260505-0947-dleague-pvp-game/  # MOVED
│   ├── 260505-1319-mysql-heatwave-integration/  # MOVED
│   └── 260505-1407-firebase-platform-pivot/  # MOVED
└── 260508-2300-svelte-phaser-firebase-mongo-pivot/  # active

docs/
├── code-standards.md           # MODIFIED
├── codebase-summary.md         # NEW skeleton
├── design-guidelines.md        # NEW skeleton
├── deployment-guide.md         # NEW skeleton
├── development-roadmap.md      # NEW skeleton
├── project-changelog.md        # NEW skeleton
├── project-overview-pdr.md     # NEW skeleton
└── system-architecture.md      # NEW skeleton
```

## Related Code Files
**Create:**
- `plans/archive/README.md`
- `docs/codebase-summary.md`
- `docs/design-guidelines.md`
- `docs/deployment-guide.md`
- `docs/development-roadmap.md`
- `docs/project-changelog.md`
- `docs/project-overview-pdr.md`
- `docs/system-architecture.md`

**Modify:**
- `README.md` (lines 16, 36-67: status, stack table, repo layout, phase table)
- `docs/code-standards.md` (line 33: store/ comment; Ebiten→Svelte refs)

**Move:**
- `plans/260505-0947-dleague-pvp-game/` → `plans/archive/260505-0947-dleague-pvp-game/`
- `plans/260505-1319-mysql-heatwave-integration/` → `plans/archive/260505-1319-mysql-heatwave-integration/`
- `plans/260505-1407-firebase-platform-pivot/` → `plans/archive/260505-1407-firebase-platform-pivot/`

**Delete:** none.

## Implementation Steps
1. `git mv` three plan dirs into `plans/archive/`.
2. For each archived `plan.md`, add to frontmatter (preserving rest): `status: cancelled`, `cancelled_on: 2026-05-08`, `cancellation_reason: "Superseded by 260508-2300-svelte-phaser-firebase-mongo-pivot/plan.md (stack pivot)"`.
3. Write `plans/archive/README.md` listing each archived plan + 1-line "why archived".
4. Edit `README.md`:
   - Line 16 (Status): "Phase 1 complete; pivoting to Svelte+Phaser+Mongo+Firebase. See active plan."
   - Lines 38-41 (Stack): replace 4 bullets with new stack table.
   - Lines 45-54 (Repo layout): `client/` → `web/` (SvelteKit); drop `wasm_exec.js`/Ebitengine refs; add `firebase.json`.
   - Lines 58-67 (Plan table): replace with 10-row pivot phase table; link `plans/260508-2300-svelte-phaser-firebase-mongo-pivot/plan.md`.
5. Edit `docs/code-standards.md`:
   - Line 33 `# Postgres repos (Phase 3)` → `# Mongo repos (per-collection, Phase 04)`.
   - Replace Ebitengine references with `SvelteKit + Phaser` conventions.
   - Drop "WASM bundle <10MB" target; replace with "JS bundle <400 KB gzipped".
   - Add Mongo conventions section: `_id` strategy (Firebase UID for users, ObjectId for matches/attempts), `bson:` tags, `schema_version` field.
6. Create skeleton for each new `docs/*.md` with sections: title, purpose, TODO blocks per major section. Each phase completing later will fill blocks.
7. `docs/development-roadmap.md` skeleton points to active `plans/260508-2300-.../plan.md` Phases table.
8. `docs/project-changelog.md` skeleton seeded with: 2026-05-08 — pivot decision recorded; Phase 1 foundation completed (cite commit 9937c7d); pivot plan adopted.
9. Verify all internal links resolve (relative path checks).

## Todo List
- [x] Move 3 plan dirs to `plans/archive/`
- [x] Add cancellation frontmatter to each archived `plan.md`
- [x] Write `plans/archive/README.md`
- [x] Update `README.md` status / stack / layout / plan table
- [x] Update `docs/code-standards.md` (Postgres + Ebiten → Mongo + Svelte)
- [x] Create 7 `docs/*.md` skeletons
- [x] Verify links resolve
- [ ] Commit message: `docs: archive superseded plans; bootstrap docs/ for pivot` (pending user approval)

## Success Criteria
- `ls plans/archive/` shows 3 dirs + `README.md`.
- `grep -r Postgres docs/ README.md` returns no hits.
- `grep -r Ebitengine docs/ README.md` returns no hits.
- All 8 files in `docs/` exist; `code-standards.md` is the only one with substantive content; others are skeletons.
- README phase table links resolve to `plans/260508-2300-.../*.md`.

## Risk Assessment
| Risk                                              | Likelihood | Impact | Mitigation                                                       |
|---------------------------------------------------|------------|--------|------------------------------------------------------------------|
| Stale link in archived plan.md to live phase file | High       | Low    | Don't fix; archives are frozen. Note in archive README.          |
| Future contributor confused by skeleton TODOs     | Medium     | Low    | Each skeleton names the phase that will fill it.                 |
| Git history loss on dir move                      | Low        | Low    | Use `git mv` not delete+add.                                     |

## Security Considerations
None. Markdown-only changes.

## Next Steps
- Phase 02 — Server hardening — depends on this phase only for clean docs baseline.
