---
phase: F
title: "Documentation + cleanup"
status: pending
priority: P2
effort: 0.5d
dependencies: [C, D, E]
---

# Phase F: Documentation + cleanup

## Context Links

- `docs/code-standards.md` — already exists from Phase 1 sign-off
- `plans/260505-0947-dleague-pvp-game/phase-03-backend-auth.md` — currently PG-flavored, needs MySQL alignment
- `docker-compose.yml` — currently provisions `postgres:16`, now obsolete
- `README.md` — Quickstart mentions Postgres

## Overview

Bring all project artifacts in line with the MySQL HeatWave decision. Update standards, replace PG references with MySQL, swap or remove the dev Postgres container, and author the restore runbook declared in Phase E.

## Requirements

**Functional**
- `docs/code-standards.md` adds a "Database conventions" section (UUIDv7 + `BINARY(16)`, functional unique indexes, JSON column patterns, `database/sql` pool defaults, error handling)
- `phase-03-backend-auth.md` schema fragments updated to MySQL flavor; cross-link to this plan added at the top
- `docker-compose.yml`: replace `postgres:16` service with **either** an optional `mysql:8` dev container **or** remove the DB service entirely (decision below)
- `README.md` Quickstart and Stack section reference MySQL, not Postgres
- `docs/runbooks/restore-mysql.md` authored — concrete steps to restore from weekly Object Storage dump

**Non-functional**
- All edits keep file LOC budgets (per `ck:docs.maxLoc=800`)
- No code changes — markdown + a tiny compose tweak only

## Architecture

```
docs/
├── code-standards.md           ← amend
├── runbooks/
│   └── restore-mysql.md        ← NEW
plans/260505-0947-dleague-pvp-game/
├── phase-03-backend-auth.md    ← amend (top reference + schema flavor)
docker-compose.yml              ← amend (remove or swap postgres service)
README.md                       ← amend
```

## Compose decision

**Two viable options for the local dev DB:**

| Option | Pros | Cons |
|--------|------|------|
| **A) Replace `postgres:16` with `mysql:8`** | One-command local dev DB; familiar pattern | Most devs already have MySQL via Docker Desktop / Lima; creates duplicate |
| **B) Remove DB from compose entirely** | Compose stays focused on app services; devs use their own MySQL | Adds setup friction for new contributors |

**Recommended:** Option A. Keep barrier-to-entry low. Document an opt-out flag for devs who want to point at OCI directly.

## Related Code Files

**Modify (markdown / yaml only):**
- `docs/code-standards.md` (add MySQL section)
- `plans/260505-0947-dleague-pvp-game/phase-03-backend-auth.md` (schema flavor + cross-link)
- `docker-compose.yml` (swap service)
- `README.md` (Quickstart wording)

**Create:**
- `docs/runbooks/restore-mysql.md`

**Delete:** none

## Implementation Steps

1. **`docs/code-standards.md`** — append a "Database conventions" subsection covering:
   - DSN format
   - Pool defaults (`MaxOpenConns=25`, `MaxIdleConns=5`, `ConnMaxLifetime=30m`)
   - UUIDv7 generated app-side via `github.com/google/uuid` v1.6+, stored as `BINARY(16)`
   - Email uniqueness via `UNIQUE KEY ((LOWER(email)))` functional index
   - `JSON` column for opaque blobs; never query inside in Phase 3 scope
   - `INSERT ... ON DUPLICATE KEY UPDATE` for upserts
   - `SELECT FOR UPDATE` semantics in InnoDB REPEATABLE READ
   - Error handling: distinguish `mysql.MySQLError.Number == 1062` (dup key), `1213` (deadlock retry), `1205` (lock wait timeout retry); use `errors.As`
   - Time zone: store UTC, use `parseTime=true&loc=UTC` DSN flags
   - Charset: `utf8mb4` + `utf8mb4_0900_ai_ci`
2. **`docker-compose.yml`** — Option A swap:
   ```yaml
   services:
     mysql:
       image: mysql:8
       container_name: dleague-mysql
       environment:
         MYSQL_ROOT_PASSWORD: dleague
         MYSQL_DATABASE: dleague
         MYSQL_USER: dleague_app
         MYSQL_PASSWORD: dleague
       ports:
         - "3306:3306"
       volumes:
         - dleague-mysql:/var/lib/mysql
       healthcheck:
         test: ["CMD", "mysqladmin", "ping", "-h", "localhost", "-u", "root", "-pdleague"]
         interval: 5s
         timeout: 3s
         retries: 5
   volumes:
     dleague-mysql:
   ```
   Drop the `postgres` service + volume.
3. **`README.md`** — replace "Postgres" with "MySQL HeatWave" in Stack section; update Quickstart command from `make db-up` (postgres) to whatever the new compose service is. Note OCI HeatWave for production.
4. **`plans/260505-0947-dleague-pvp-game/phase-03-backend-auth.md`** — amend at top:
   ```markdown
   > **DB binding:** see `plans/260505-1319-mysql-heatwave-integration/plan.md` for the data layer plan. Schema below has been translated from PostgreSQL to MySQL idiom — `citext` → `LOWER(email)` unique index, `uuid` → `BINARY(16)`, `JSONB` → `JSON`.
   ```
   Then walk the schema section and replace PG-flavored fragments with MySQL flavor (matching Phase D output).
5. **`docs/runbooks/restore-mysql.md`** — author concrete restore steps:
   - Prerequisites (OCI CLI access, target MySQL endpoint)
   - List available backups via `oci os object list -bn dleague-backups --prefix weekly/`
   - Download chosen dump
   - Restore to non-prod target first (drill or recovery DB system)
   - Verify integrity (table counts, latest `created_at`)
   - Cut over (only if recovery, not drill)
   - Rollback story (keep prior dump until verified)
6. **Lint pass:** all changed markdown files render clean (no broken cross-links).
7. **Pre-commit check:** `git status` shows only the expected files modified.

## Todo List

- [ ] Append "Database conventions" section to `docs/code-standards.md`
- [ ] Swap `postgres:16` → `mysql:8` in `docker-compose.yml` (Option A)
- [ ] Update `README.md` Stack + Quickstart sections
- [ ] Add cross-link + MySQL-flavor schema to `phase-03-backend-auth.md`
- [ ] Author `docs/runbooks/restore-mysql.md`
- [ ] Verify all modified markdown files render correctly (no broken anchors)
- [ ] Confirm no orphaned references to "postgres" remain in repo (`grep -ri postgres .`)

## Success Criteria

- [ ] `docs/code-standards.md` has a Database section keyed to MySQL idioms
- [ ] `docker-compose.yml` no longer mentions Postgres
- [ ] `README.md` Stack table shows MySQL HeatWave
- [ ] `phase-03-backend-auth.md` schema is MySQL-flavored and links to this plan
- [ ] `docs/runbooks/restore-mysql.md` exists with executable steps
- [ ] `grep -ri postgres .` returns only intentional references (e.g. historical research reports)

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| Stale "postgres" mentions in older plan files mislead future readers | Low | Final grep step + add note where historical context is retained |
| `docs/runbooks/restore-mysql.md` written without testing | Med | Tied to Phase B/E restore drill; the runbook describes what was actually done |
| Phase 3 author later reverts schema changes assuming PG | Med | Cross-link from `phase-03-backend-auth.md` top is the durable signal |
| `README` change conflicts with parallel branch updates | Low | Phase F runs late; consolidate edits in a single commit |

## Security Considerations

- Restore runbook documents how to reach production data — restrict who can read `docs/runbooks/` if repo is ever opened up
- Compose passwords (`dleague` / `dleague`) are dev-only; do NOT mirror to any deployed env
- `phase-03` document edits do not introduce new secrets

## Next Steps

After Phase F success criteria:
- This plan is complete; mark `plan.md` status `completed`
- Parent Phase 3 of `260505-0947-dleague-pvp-game` is unblocked
- Run `/ck:journal` to capture decisions and lessons
