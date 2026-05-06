# Phase 12 Supersession & Cleanup — Legacy Ebitengine Axe, License Fit Confirmed

**Date**: 2026-05-06 09:33  
**Severity**: N/A (Milestone)  
**Component**: Codebase hygiene — deprecated client removal, plan archival, license validation  
**Status**: Completed (deployment guide + final PDR review pending external input)

## What Happened

Phase 12 closed out architectural cleanup: deleted the dormant Ebitengine WASM client (`client/cmd/`, `client/internal/`, Go module files), linked two superseded pivot plans with "Superseded by" banners in frontmatter, refreshed three docs to strike "Phase 12 decision pending" wording (README, codebase-summary, project-roadmap), and executed a full license-fit review for Couchbase CE + future rewards. Codebase reduced from 3 active Go modules to 2; `client/` now holds only Svelte 5 + Phaser 4 + Capacitor web stack.

## The Brutal Truth

The deletion felt overdue and clean in concept. In execution, it exposed a gotcha: the `go.work` monorepo manifest still listed `./client` despite module deletion, which broke the entire workspace build until removed. This is the kind of "I was confident I cleared this" mistake that burns 10 minutes of trust. The lesson stung because it's exactly the kind of coupling you try to avoid in modular architecture — one stale reference breaks the graph.

The license review was thorough (80-line research report plus 5-option fallback analysis) for what amounts to "current posture is safe, future rewards trigger a decision gate." Overkill? Possibly. But the researcher subagent's care about BSL 1.1 vs. Community Agreement distinctions, and the explicit reward-shape table, gave me clear confidence that we're compliant *now* and have a cheap escape route (Capella 1-day cutover) if needed later. Better paranoid-then-relieved than surprised.

Docs refresh was mechanical and uncovered no new risks. All three (README, codebase-summary, project-roadmap) had stale "Phase 12 TBD" placeholders that needed strikethrough or removal.

## Technical Details

**go.work breakage:**
```
[go.work before cleanup]
go 1.26
use (
    ./client      // ← still listed, but ./client/go.mod + ./client/go.sum deleted
    ./server
    ./shared
)

[go.work after cleanup]
go 1.26
use (
    ./server      // ← ./client removed
    ./shared
)
```

Build failed with "module ./client not found" until this was corrected. **Lesson:** when deleting modules, grep for module name in `go.work`, `go.mod` (in `replace` directives), and makefile/CI configs. Don't assume git rm alone is sufficient.

**License-fit conclusion (from researcher report):**

| Reward Type | Compliant on CE? | Action |
|---|---|---|
| Cosmetic tokens / in-game perks | ✓ Yes | Proceed without review |
| Free credits (redeemable post-beta for store value) | ⚠ Maybe | Consult Couchbase before launch |
| Cash / monetized referrals / subscriptions | ✗ No | Migrate to Capella or pursue commercial license |

**Current state:** unpaid public beta (no monetization) = **fully compliant on CE**. Trigger for migration: any reward converting beta participation to external monetary value.

## What We Tried

1. **Keep Ebitengine client as deprecated artifact** — rejected. The module was dead weight (no active developer on Ebitengine, web stack proven and shipping), and leaving it would create false signal that a WASM option still exists. Deletion is cleaner than a "DO NOT USE" flag in a file nobody reads.

2. **License migration to MongoDB or PostgreSQL upfront** — rejected. Current unpaid beta doesn't justify the 2–5 day gocb→mongo-driver or SQL schema redesign. Capella (1-day cutover, same SDK) is available if monetization forces a move.

3. **Leave license review to Phase 13+** — rejected. BSL 1.1 language was vague enough that shipping without clarity felt reckless. The research report's distinction between "CE binaries" (restricted) and "CE source forks" (more restricted) flipped my confidence from "probably OK" to "definitely OK for current posture, clear gate for future."

## Root Cause Analysis

The `go.work` miss came from parallel deletion of three separate files/patterns (`client/cmd/`, `client/internal/`, `client/go.*`) and assuming a top-level cleanup script would catch all references. It didn't — because I didn't write one. **Root cause:** manual cleanup of a monorepo without a checklist is error-prone. **Fix:** shell script with grep + validation.

The license ambiguity reflected real tension in Couchbase's 2024 BSL 1.1 shift: the docs say "non-commercial-only" but bury the "except self-hosted internal use" carve-out three levels deep in legal pages. A thorough research report wasn't overkill — it was the cost of actual confidence. The researcher subagent's deep dive into BSL Additional Use Grants and the concrete reward-shape decision table (cosmetics vs. cash) was the difference between "I think we're OK" and "I know why, and I know when to pivot."

## Lessons Learned

1. **Monorepo surgery requires full validation.** `go.work` is a single point of truth for the module graph; stale entries break everything silently. Add to pre-commit: `go mod tidy && go work tidy && go build ./...`. Commit only after full build succeeds.

2. **License reviews compound value with specificity.** Generic "BSL is OK" is useless. The reward-shape table (cosmetics safe, cash triggers migration) is actionable and lets the team ship without guilt. Invest in specificity early; it saves midnight panics later.

3. **Dead code removal saves future cognition.** The Ebitengine client was a false option. Every developer reading the code would wonder "are we maintaining two clients?" Deletion answered that question permanently.

4. **Capella fallback is your friend.** The research report's ranking of five migration options (Capella, MongoDB, PostgreSQL, ScyllaDB, FerretDB) with cost/risk/legality per option gives the team a cheap escape hatch. If rewards go live and CE gets uncomfortable, Capella is 1 day away — not 2 weeks.

## Next Steps

**Blocking final Phase 12 closure:**
- **Deployment guide finalization** — walkthrough of Coolify live-VM launch (explicitly deferred to post-Phase-11 per user scope). Phase 11 documented the conceptual architecture; Phase 12 doesn't add technical content.
- **Final PDR / architecture docs user review** — open question whether stakeholders want one more pass on `docs/project-overview-pdr.md` and system-architecture before beta launch. Not a technical blocker; capture for user.

**Before any monetized reward goes live:**
1. Confirm exact reward shape (cosmetics, credits, cash, subscriptions) — cross-check against license table in `docs/migration-readiness.md` (updated 2026-05-06).
2. If reward shape triggers ⚠ or ✗: either (a) consult Couchbase sales for CE→Enterprise quote, or (b) run `dleague-export`, migrate to Capella (1 day) or FerretDB (1 day, Apache 2.0 bulletproof).

**Archive sweep validation:**
- Confirmed two superseded plans (`archive/260505-1407-firebase-platform-pivot/`, `archive/260505-1319-mysql-heatwave-integration/`) now have `superseded_by: 260505-1604-firebase-couchbase-redis-pivot` frontmatter + "Superseded by" banner at doc top. Both human-readable and machine-indexable.
