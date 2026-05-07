---
phase: 7
title: "Cleanup + docs + supersession"
status: completed
priority: P2
effort: "1d"
dependencies: [5, 6]
---

# Phase 7: Cleanup + docs + supersession

## Context links

- Old impls to delete: `server/internal/store/{couchbase,redis}/`.
- Docker services to drop: `docker-compose.yml` (couchbase, redis services).
- Docs to refresh: `docs/{system-architecture,codebase-summary,migration-readiness,deployment-guide,project-roadmap,project-overview-pdr}.md`.
- Predecessor plan to supersede: `plans/260505-1604-firebase-couchbase-redis-pivot/plan.md`.

## Overview

After 24+ hours of healthy Atlas-backed prod traffic, delete the old Couchbase + Redis Go packages, drop their docker services, refresh every doc that mentions the old stack, archive the predecessor plan, and update the project roadmap. This is the **point of no return** — only run after Phase 6's cutover is fully validated.

## Requirements

**Functional:**
- Zero remaining Go imports of `github.com/couchbase/gocb/v2` or `github.com/redis/go-redis/v9`.
- Zero remaining Docker services for couchbase / redis.
- Zero remaining doc references to "Couchbase" or "Redis" as the *active* backend (historical references in archived plans/journals are fine).
- A grep-isolation CI check that fails if any non-mongodb file imports the Mongo driver — adapted from the Couchbase/Redis grep tests in `migration-readiness.md`.

**Non-functional:**
- Final repo state: `internal/store/{store.go, errors.go, mongodb/, memstore/}` — that's it.

## Architecture

```
Before Phase 7:
  internal/store/
    ├── store.go
    ├── errors.go
    ├── couchbase/   ← delete
    ├── redis/       ← delete
    ├── memstore/
    └── mongodb/

After Phase 7:
  internal/store/
    ├── store.go
    ├── errors.go
    ├── memstore/
    └── mongodb/
```

## Related code files

- **Delete:** `server/internal/store/couchbase/` (entire directory).
- **Delete:** `server/internal/store/redis/` (entire directory).
- **Modify:** `docker-compose.yml` — remove `couchbase` and `redis` services + their volumes + their depends_on references.
- **Modify:** `.env.example` — remove `COUCHBASE_*` and `REDIS_*` vars; keep `MONGODB_URI`.
- **Modify:** `server/internal/config/config.go` — drop Couchbase + Redis fields.
- **Modify:** `Makefile` — drop targets that reference couchbase/redis (e.g. `make couchbase-shell` if any exists).
- **Modify:** `go.mod` — `go mod tidy` should drop `github.com/couchbase/gocb/v2` and `github.com/redis/go-redis/v9`.
- **Rewrite:** `docs/system-architecture.md` — diagram replaces couchbase + redis boxes with one Atlas box; update "Migration seam" section.
- **Rewrite:** `docs/codebase-summary.md` — `internal/store/` listing.
- **Rewrite:** `docs/migration-readiness.md` — Atlas now active. Old recipe (Couchbase + Redis → Atlas) becomes "this is how we got here". Add new outbound recipe (Atlas → anywhere).
- **Update:** `docs/deployment-guide.md` — Coolify env config no longer lists couchbase/redis; lists `MONGODB_URI`. Drop ARM64 Couchbase notes.
- **Update:** `docs/project-roadmap.md` — flip phases of `260505-1604` plan; add this plan with phase status; update "Recently shipped".
- **Update:** `docs/project-overview-pdr.md` — stack section: "Primary store: MongoDB Atlas".
- **Update:** `README.md` — Stack table.
- **Update:** `plans/260505-1604-firebase-couchbase-redis-pivot/plan.md` — frontmatter `superseded_by: 260507-1648-mongodb-atlas-only-migration/plan.md` (post-deploy-target supersession).

## Implementation steps

1. **Confirm health.** Atlas-backed prod has been live ≥24h. No P1/P2 incidents. `/health` returns 200. Manual sign-in works.
2. **Rewire `cmd/dleague-export/main.go` to Mongo** (the step deferred from Phase 5). Either:
   - (a) Replace the Couchbase constructor with `mongodb.New(...)` and let it call `store.Export(ctx, w)` on the Mongo store. JSONL output shape stays identical because `Export` is a method on the `Store` interface, not impl-specific.
   - (b) Retire `cmd/dleague-export` entirely in favor of `mongodump --uri "$MONGODB_URI" --db dleague --out ./snapshot/`. Native Mongo tool, BSON output, no JSONL transformer needed for outbound migration.
   Recommend **(b)** — KISS, native tooling. Document the choice in `migration-readiness.md`.
3. **Add the grep-isolation CI check** for the Mongo driver:
   ```sh
   # add to a Makefile target or CI job:
   ! grep -r '"go.mongodb.org/mongo-driver/v2"' server/ \
       | grep -v 'internal/store/mongodb'
   ```
4. **Delete `internal/store/couchbase/`** and `internal/store/redis/`:
   ```sh
   git rm -r server/internal/store/couchbase server/internal/store/redis
   ```
5. **Run `go mod tidy`** — `gocb` and `go-redis` should disappear from `go.mod`/`go.sum`.
6. **Edit `docker-compose.yml`** — strip out the `couchbase` + `redis` services, their volumes, and any `depends_on` entries. The compose file should now define only the Go server (and possibly a dev memstore for local-only flow, optional).
7. **Edit `.env.example`** — remove `COUCHBASE_*` and `REDIS_*` lines. Keep `MONGODB_URI=mongodb+srv://<user>:<pass>@<cluster>/dleague?...` placeholder.
8. **Edit `server/internal/config/config.go`** — drop unused fields. Run `go vet` to surface dead config readers elsewhere.
9. **Update `Makefile`** — drop now-obsolete targets.
10. **Rewrite docs** in this order (each one read first, then edited; each <800 lines per project rule):
    - `system-architecture.md` (most diagrams; the centerpiece)
    - `codebase-summary.md` (matches the new dir tree)
    - `migration-readiness.md` (history + new outbound recipe + grep tests for `go.mongodb.org/mongo-driver/v2`)
    - `deployment-guide.md` (Coolify env; document `0.0.0.0/0` allowlist beta-only trade-off; flag "static-IP NAT or VPC peering required for non-beta launch")
    - `project-overview-pdr.md` (one-liner update)
    - `README.md` (Stack section)
    - `project-roadmap.md` (final pass after everything else)
11. **Mark predecessor plan superseded.** Edit `plans/260505-1604-firebase-couchbase-redis-pivot/plan.md` frontmatter — add `superseded_by: 260507-1648-mongodb-atlas-only-migration/plan.md`. Add a short note at the top of `plan.md` body: "Stack pivoted to Atlas-only on 2026-05-XX — see superseding plan."
12. **Update `set-active-plan`** — point to this plan's directory.
13. **Run `/ck:journal`** to write a brief technical journal entry covering the consolidation decision and what we learned.

## Todo list

- [ ] 24h+ healthy Atlas prod confirmed
- [ ] `cmd/dleague-export` rewired to Mongo (or retired in favor of `mongodump`)
- [ ] Grep-isolation CI check added
- [ ] `internal/store/couchbase/` deleted
- [ ] `internal/store/redis/` deleted
- [ ] `go mod tidy` removes gocb + go-redis
- [ ] `docker-compose.yml` cleaned
- [ ] `.env.example` cleaned
- [ ] `config.go` cleaned
- [ ] `Makefile` cleaned
- [ ] All 6 docs files refreshed
- [ ] `README.md` Stack section updated
- [ ] Predecessor plan marked superseded
- [ ] Active plan pointer updated
- [ ] Journal entry written
- [ ] `go test ./...` + integration tests green post-cleanup
- [ ] `golangci-lint run ./...` clean

## Success criteria

- `git grep -E 'gocb|go-redis|couchbase|redis:8' server/ docs/` returns only historical mentions in archived plans/journals.
- The grep-isolation CI check is wired and green.
- A new contributor following `README.md` quickstart can stand up the project against Atlas in <10 minutes.
- `docs/system-architecture.md` diagram has one data-plane box (Atlas), not two.

## Risk assessment

- **Premature deletion (Phase 7 before Phase 6 stabilizes).** Mitigation: 24h soak gate; no rollback to old containers if Phase 7 ships.
- **Doc drift.** Mitigation: a single PR for the cleanup; reviewer checklist mirrors the todo list above.
- **Forgotten import.** Mitigation: the new grep-isolation CI check + `go mod tidy` are deterministic.
- **Beta tester churn during downtime.** Mitigation: cutover happened in Phase 6; Phase 7 is purely doc/code housekeeping with no service impact.

## Security considerations

- Final secrets sweep: confirm no `MONGODB_URI` (with password) in any committed file. `.env.example` placeholder only.
- Old Coolify secrets for `COUCHBASE_*` / `REDIS_*` should be removed from Coolify dashboard after deletion (not a code task, but document in the deployment-guide).

## Next steps

After Phase 7, the migration is complete. Post-beta milestones (`docs/project-roadmap.md`) inherit the new state — the next migration question is "do we ever need to leave Atlas?" rather than "when do we leave Couchbase?". The seam is preserved (`store.Store` is unchanged; new impl could plug in alongside `mongodb/` and `memstore/`).
