---
phase: D
title: "Schema migration design (MySQL dialect)"
status: pending
priority: P1
effort: 0.5d
dependencies: [C]
---

# Phase D: Schema migration design (MySQL dialect)

## Context Links

- Parent Phase 3 (PG-flavored draft schema): [`plans/260505-0947-dleague-pvp-game/phase-03-backend-auth.md`](../260505-0947-dleague-pvp-game/phase-03-backend-auth.md)
- MySQL idioms research: [`reports/researcher-260505-1308-mysql-heatwave-deep-dive.md`](../reports/researcher-260505-1308-mysql-heatwave-deep-dive.md) — Q10
- Phase C migrator scaffolding: [`phase-c-go-integration-scaffolding.md`](phase-c-go-integration-scaffolding.md)

## Overview

Translate the Phase 3 schema (currently PostgreSQL-flavored) into MySQL 8 idiom and write `0001_init.sql` (the file Phase C scaffolded as empty). Concrete table content + index design, no app code.

## Requirements

**Functional**
- All Phase 3 tables created in `dleague` schema by `0001_init.sql`
- Idempotent — `CREATE TABLE IF NOT EXISTS` everywhere
- UUID primary keys stored as `BINARY(16)` (UUIDv7, app-generated)
- Email uniqueness enforced case-insensitively via functional unique index
- All text columns `utf8mb4` + `utf8mb4_0900_ai_ci`
- Game state stored as opaque `JSON`

**Non-functional**
- Migration applies cleanly to a fresh `mysql:8` Docker container
- Each `CREATE TABLE` <50 lines; readable
- Indexes designed for known query patterns from parent Phase 3–5 plans

## Architecture — table inventory

Derived from `phase-03-backend-auth.md` and `phase-04-async-pvp.md`:

```
users      ─┐
sessions   ─┴─ auth + identity
puzzles    ─── daily seed registry
matches    ─── async + sync match metadata
attempts   ─── per-player guess history per match
```

Leaderboards are a query against `attempts` — no materialized table for now (YAGNI, <100 users).

## Schema sketch (MySQL 8)

```sql
-- 0001_init.sql

-- users
CREATE TABLE IF NOT EXISTS users (
  id              BINARY(16) NOT NULL,
  email           VARCHAR(254) NOT NULL,
  password_hash   VARCHAR(120) NOT NULL,        -- bcrypt $2b$ tagged, 60 chars + headroom
  display_name    VARCHAR(64) NOT NULL,
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_email_lower ((LOWER(email)))
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- sessions
CREATE TABLE IF NOT EXISTS sessions (
  token       BINARY(32) NOT NULL,             -- random 256-bit
  user_id     BINARY(16) NOT NULL,
  expires_at  TIMESTAMP NOT NULL,
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (token),
  KEY idx_sessions_user (user_id),
  CONSTRAINT fk_sessions_user FOREIGN KEY (user_id)
    REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- puzzles
CREATE TABLE IF NOT EXISTS puzzles (
  puzzle_date   DATE NOT NULL,
  game_id       VARCHAR(32) NOT NULL,           -- e.g. "wordle"
  seed          BIGINT NOT NULL,
  answer_hash   BINARY(32) NOT NULL,            -- sha256 of canonical answer
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (puzzle_date, game_id)
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- matches
CREATE TABLE IF NOT EXISTS matches (
  id            BINARY(16) NOT NULL,
  kind          ENUM('async','sync','daily') NOT NULL,
  game_id       VARCHAR(32) NOT NULL,
  puzzle_date   DATE NOT NULL,
  creator_id    BINARY(16) NOT NULL,
  joiner_id     BINARY(16) NULL,                -- null until joined
  status        ENUM('open','in_progress','completed','expired') NOT NULL DEFAULT 'open',
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  completed_at  TIMESTAMP NULL,
  PRIMARY KEY (id),
  KEY idx_matches_open (status, kind, created_at),
  KEY idx_matches_creator (creator_id),
  KEY idx_matches_joiner (joiner_id),
  CONSTRAINT fk_matches_creator FOREIGN KEY (creator_id) REFERENCES users(id),
  CONSTRAINT fk_matches_joiner  FOREIGN KEY (joiner_id)  REFERENCES users(id),
  CONSTRAINT fk_matches_puzzle  FOREIGN KEY (puzzle_date, game_id)
    REFERENCES puzzles(puzzle_date, game_id)
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- attempts
CREATE TABLE IF NOT EXISTS attempts (
  id            BINARY(16) NOT NULL,
  match_id      BINARY(16) NOT NULL,
  user_id       BINARY(16) NOT NULL,
  attempts_used SMALLINT UNSIGNED NOT NULL,     -- guesses 1..N
  duration_ms   INT UNSIGNED NOT NULL,
  won           TINYINT(1) NOT NULL,            -- 0|1
  state         JSON NOT NULL,                  -- opaque per shared/game.State
  finished_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_attempt_per_user_match (match_id, user_id),
  KEY idx_attempts_user (user_id),
  KEY idx_attempts_won_user (user_id, won),    -- leaderboard hint
  CONSTRAINT fk_attempts_match FOREIGN KEY (match_id) REFERENCES matches(id) ON DELETE CASCADE,
  CONSTRAINT fk_attempts_user  FOREIGN KEY (user_id)  REFERENCES users(id)
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
```

This is the **proposed** content for `0001_init.sql`. Implementation phase confirms it against parent phase-03 final auth flow and adjusts column widths if needed.

## Related Code Files

**Modify (write content):**
- `server/internal/store/migrations/0001_init.sql` — populate with the SQL above

**No other code changes** in this phase. The migrator from Phase C runs this file.

## Implementation Steps

1. Open `server/internal/store/migrations/0001_init.sql` (created empty in Phase C).
2. Paste the schema sketch above; confirm idempotency (`CREATE TABLE IF NOT EXISTS` everywhere).
3. Test against a local Docker `mysql:8`:
   ```
   docker run --rm -d --name dleague-mysql-test \
     -e MYSQL_ROOT_PASSWORD=root -e MYSQL_DATABASE=dleague \
     -p 3306:3306 mysql:8
   # wait for healthy, then:
   mysql -h 127.0.0.1 -uroot -proot dleague < server/internal/store/migrations/0001_init.sql
   mysql -h 127.0.0.1 -uroot -proot dleague < server/internal/store/migrations/0001_init.sql  # second run, must succeed (idempotent)
   ```
4. Run server's migrator (Phase C `Migrate` function) against the same container; confirm `_migrations` table records `0001`.
5. Spot-check schema:
   ```sql
   SHOW INDEXES FROM users;          -- expect uniq_email_lower as functional index
   SHOW CREATE TABLE matches \G     -- expect FK constraints
   ```
6. Run on the live OCI MySQL HeatWave instance (via `dleague_app` user — confirm CREATE/ALTER/INDEX grants from Phase A are sufficient).

## Todo List

- [ ] Write `0001_init.sql` with the 5 tables above
- [ ] Verify idempotency by running migration twice locally
- [ ] Verify functional unique index on `LOWER(email)` works as expected
- [ ] Test against live MySQL HeatWave with `dleague_app` user
- [ ] Document any deviations from the sketch in this file's "Notes" section

## Success Criteria

- [ ] `0001_init.sql` applies cleanly to fresh `mysql:8` Docker
- [ ] Re-running the file produces no errors (idempotent)
- [ ] All FKs created with explicit names (matches sketch)
- [ ] `INSERT INTO users (email) VALUES ('foo@x.com')` then `... ('FOO@X.COM')` fails with duplicate-key error (case-insensitive uniqueness verified)
- [ ] Migrator records `0001` in `_migrations` after first apply, no-op after that

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| `dleague_app` lacks `REFERENCES` privilege → FK creation fails | Med | Phase A grants list explicitly includes `REFERENCES`; verify before running migration |
| Functional index on `LOWER(email)` not supported on the running MySQL minor version | Low | Requires MySQL 8.0.13+; HeatWave Always-Free is on 8.4 LTS — far beyond cutoff |
| `SQLSTATE 42000` due to reserved word collision | Low | All table/column names checked against MySQL 8 reserved word list |
| Future Phase E mysqldump misses functional index defs | Low | `mysqldump --routines --triggers` includes index definitions in `CREATE TABLE` output by default |

## Security Considerations

- Foreign keys enforce referential integrity at DB level — defense in depth for delete cascades
- `password_hash` column sized for bcrypt headroom; **DO NOT** widen to fit unhashed values; force hashing in app layer
- `BINARY(32)` session token is opaque — store base64'd in cookies, decode to bytes in lookup
- `JSON` state column should NOT contain raw user input that hasn't passed through the game-engine validator (defense against JSON injection of pathological structures)

## Notes

(populate during implementation)

- Deviations from sketch:
- Performance observations:
- Pending items for parent Phase 3 author:

## Next Steps

After Phase D success:
- Parent Phase 3 implementation can begin against the locked schema
- If schema needs evolve, add `0002_*.sql` etc. — never edit `0001_init.sql` post-deploy
