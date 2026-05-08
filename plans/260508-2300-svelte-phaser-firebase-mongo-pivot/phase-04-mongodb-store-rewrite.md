---
phase: 4
title: "MongoDB store rewrite (drop MySQL)"
status: pending
priority: P1
effort: 1w
dependencies: [3]
---

# Phase 04 — MongoDB store rewrite

## Context Links
- `plans/reports/researcher-260508-2300-mongodb-atlas-go.md` (driver, schema, indexes, transactions)
- `plans/reports/code-review-260508-2300-phase1-foundation.md` (L4: docker-compose dead Postgres)
- `plans/reports/security-review-260508-2300-phase1-foundation.md` (H1, L5: docker-compose mismatch)
- Existing files to delete: `server/internal/store/{store.go,migrate.go,users.go}`, `server/internal/store/migrations/`
- `docker-compose.yml` (Postgres → Mongo)

## Overview
Rip out the Phase 1 MySQL/Postgres-shaped store. Introduce `mongo-driver/v2`-backed `Client` + per-collection repositories. Schema-version-per-doc lazy migration; collections + indexes provisioned on startup. Replaces `store.New()` API but preserves `Ping()` for `/health`.

## Key Insights
- `mongo-driver/v2` (v2.6.0+) is the recommended driver for new projects (mongo report §1).
- M0 free tier: 512 MB, 100 ops/sec, 500 conns; supports transactions on its 3-node replica set (mongo report §2, §5).
- TLS required by Atlas — driver handles transparently with `mongodb+srv://...`.
- One `*mongo.Client` per process; share across handlers. Pool defaults adequate (min 10, max 100, 30s idle).
- **Lazy migration over bulk** — schema_version field per doc; transform on read if old (mongo report §6 Option A).
- Repos for: `users` (firebase UID `_id`), `games` (registry), `matches`, `attempts`, `daily_puzzles`, `leaderboards`. Phase 04 ships skeletons + indexes; only `UserRepo` + `GameRepo` get methods used in Phase 05/07. Match/Attempt/Leaderboard methods land in Phase 08/09.
- Docker-compose already wrong (Postgres image, MySQL driver) — replace with `mongo:7` + `mongo-express`.

## Requirements
**Functional:**
- `server/internal/store/mongo.go`: `Client` constructor, `Connect(uri)`, `Ping(ctx)`, `Disconnect(ctx)`, `Database()` accessor.
- `server/internal/store/users.go`: `UserRepo` with `UpsertByUID(ctx, uid, profile)`, `GetByUID(ctx, uid)`. Doc keyed by Firebase UID as `_id`.
- Skeleton repo files: `matches.go` (`MatchRepo`), `attempts.go` (`AttemptRepo`), `daily_puzzles.go` (`DailyPuzzleRepo`), `leaderboards.go` (`LeaderboardRepo`), `games.go` (`GameRepo`). Each file has constructor + `// TODO: phase-NN` comments naming the phase that fills the repo.
- `server/internal/store/indexes.go`: `EnsureIndexes(ctx, db)` creates the index list from research report §4 in one call.
- `server/internal/store/models.go`: `User`, `Match`, `Attempt`, `DailyPuzzle`, `Leaderboard` structs with `bson:` tags + `SchemaVersion int` field on each.
- Boot flow: `Connect → Ping → EnsureIndexes` (in `main.go`).
- `/health` calls `Client.Ping(ctx)` instead of MySQL ping.
- `MONGO_URI` env var replaces `DATABASE_URL`. `config.go` updated.
- docker-compose: `mongo:7` + `mongo-express` per research report §7. Credentials via `.env` (referenced; never committed). `.env.example` provided.

**Non-functional:**
- Each store file <200 LOC.
- `EnsureIndexes` idempotent (Mongo `CreateIndexes` is naturally idempotent for same-keyed indexes).
- Boot fails fast if Mongo unreachable (15s ping timeout).
- No TLS bypass; Atlas string carries `?ssl=true` implicitly.

## Architecture
```
main.go boot
  ├─ store.Connect(MONGO_URI) → *Client (pool warm)
  ├─ Client.Ping(ctx, 15s) → fail-fast on err
  ├─ store.EnsureIndexes(ctx, db) → idempotent
  ├─ users := store.NewUserRepo(db)
  ├─ matches := store.NewMatchRepo(db)   // skeleton
  ├─ attempts := store.NewAttemptRepo(db) // skeleton
  └─ ... wired into hub for later phases

dispatch path
  Hub → handler → repo.Method(ctx, ...) → bson decode → return
```

Collection layout (mongo report §3):
- `users` `_id`=firebase_uid, embedded `stats`
- `games` `_id`=game_id ("wordle")
- `matches` ObjectId, `players[]`, `mode`, `state`, `seed`, timestamps
- `attempts` ObjectId, `match_id`, `player_uid`, `attempts[]`, `time_ms`
- `daily_puzzles` `_id`=YYYY-MM-DD, `seed`, `solution_hash`
- `leaderboards` `_id`=`{game}_{period}_{period_end}`, `rankings[]`

Indexes — see research report §4 (8 indexes total).

## Related Code Files
**Create:**
- `server/internal/store/mongo.go`
- `server/internal/store/indexes.go`
- `server/internal/store/models.go`
- `server/internal/store/users.go` (rewritten)
- `server/internal/store/games.go`
- `server/internal/store/matches.go`
- `server/internal/store/attempts.go`
- `server/internal/store/daily_puzzles.go`
- `server/internal/store/leaderboards.go`
- `server/internal/store/mongo_test.go` — `EnsureIndexes` smoke test (gated by `MONGO_TEST_URI` env)
- `server/internal/store/users_test.go` — `UpsertByUID` round-trip
- `.env.example` — `MONGO_URI=mongodb://admin:admin@localhost:27017/?authSource=admin`

**Modify:**
- `server/cmd/server/main.go` — boot calls (Connect, Ping, EnsureIndexes)
- `server/internal/config/config.go` — `MongoURI string` replaces `DatabaseURL`
- `server/internal/http/health.go` — call `Client.Ping` not SQL ping
- `server/go.mod` — add `go.mongodb.org/mongo-driver/v2`; drop `github.com/go-sql-driver/mysql` and `filippo.io/edwards25519`
- `docker-compose.yml` — replace Postgres service with `mongo:7` + `mongo-express`
- `Makefile` — `compose-up` / `compose-down` targets unchanged but pointing at new compose
- `docs/system-architecture.md` — fill collections + indexes section
- `docs/code-standards.md` — already updated in Phase 01; verify no `Postgres`/`MySQL` ref remains.

**Delete:**
- `server/internal/store/store.go` (MySQL driver wrapper)
- `server/internal/store/migrate.go`
- `server/internal/store/store_test.go` (MySQL DSN gated)
- `server/internal/store/migrations/` (entire dir)
- `server/internal/store/users.go` ORIGINAL (replaced by new file of same name)

## Implementation Steps
1. `cd server && go get go.mongodb.org/mongo-driver/v2/mongo go.mongodb.org/mongo-driver/v2/mongo/options go.mongodb.org/mongo-driver/v2/bson`.
2. Delete old store files (5 files + `migrations/` dir). `go.mod` cleanup pending step 6.
3. Write `mongo.go`: `Client` struct wrapping `*mongo.Client`; `Connect(ctx, uri)` calling `mongo.Connect(options.Client().ApplyURI(uri).SetMaxPoolSize(100).SetServerSelectionTimeout(5s).SetConnectTimeout(10s))`; `Ping(ctx)`; `Disconnect(ctx)`; `Database(name string) *mongo.Database`.
4. Write `models.go`: 6 structs with `bson:` tags + `SchemaVersion int \`bson:"schema_version"\``.
5. Write `indexes.go`: `EnsureIndexes(ctx, db)` creates the 8 indexes from research §4 across 5 collections via `Indexes().CreateMany`.
6. Write `users.go`: `UserRepo{coll *mongo.Collection}`; `UpsertByUID(ctx, uid, p Profile)` uses `FindOneAndUpdate(upsert=true)`; `GetByUID(ctx, uid)`. Returns `(nil, nil)` on `ErrNoDocuments`.
7. Write skeleton repos: `games.go`, `matches.go`, `attempts.go`, `daily_puzzles.go`, `leaderboards.go`. Each: constructor + 1 `Get` method + `// TODO(phase-NN): implement Y/Z`.
8. Update `config.go`: `MongoURI string` from `MONGO_URI`. Drop `DatabaseURL`.
9. Update `main.go`: replace MySQL bootstrapping (lines 25-40 area) with Mongo bootstrap. Use `bootCtx, cancel := context.WithTimeout(ctx, 15*time.Second)` for Connect+Ping+EnsureIndexes.
10. Update `health.go`: `c.store.Ping(ctx)` calls Mongo `Client.Ping`.
11. Update `docker-compose.yml`: copy from research §7 (mongo:7 service + mongo-express, healthcheck, volumes). Replace credentials with `${MONGO_USERNAME}`/`${MONGO_PASSWORD}` from `.env`.
12. Add `.env.example` listing `MONGO_URI`, `MONGO_USERNAME`, `MONGO_PASSWORD`, `DLEAGUE_MAX_CONNS`, `DLEAGUE_WS_ORIGINS`, etc.
13. `go mod tidy` — confirm MySQL driver gone.
14. Tests:
    - `mongo_test.go`: `MONGO_TEST_URI` gate (skips if absent). Connects, pings, creates indexes; checks count.
    - `users_test.go`: same gate. UpsertByUID + GetByUID round-trip.
15. CI: add `mongo:7` service container + `MONGO_TEST_URI` env. Verify green.
16. Manual: `make compose-up` → mongo-express at `:8081` lists empty `dleague` DB → boot server → DB lists 5 collections (lazy via first index ops) and 8 indexes.
17. Update `docs/system-architecture.md` collection diagram + index list.

## Todo List
- [ ] Add mongo-driver/v2; drop MySQL driver
- [ ] Delete old SQL store + migrations
- [ ] Write `mongo.go` Client wrapper
- [ ] Write `models.go` with `schema_version` per struct
- [ ] Write `indexes.go` with 8 indexes
- [ ] Write `users.go` (UserRepo: Upsert + Get)
- [ ] Write 4 skeleton repos with TODO markers
- [ ] Update `config.go` MONGO_URI
- [ ] Update `main.go` boot path
- [ ] Update `health.go` ping
- [ ] Replace docker-compose with mongo:7 + mongo-express
- [ ] `.env.example`
- [ ] Tests: mongo_test, users_test
- [ ] CI Mongo service container
- [ ] Verify `go mod tidy` clean
- [ ] Update system-architecture.md

## Success Criteria
- [ ] `make compose-up && make dev` boots; mongo-express shows `dleague` DB created
- [ ] `db.users.getIndexes()` returns ≥2 indexes (incl `display_name` unique)
- [ ] `/health` returns 200 when Mongo up, 503 when down
- [ ] `go mod why github.com/go-sql-driver/mysql` returns "package not in module"
- [ ] CI green with Mongo service container
- [ ] UpsertByUID then GetByUID returns same doc

## Risk Assessment
| Risk                                                   | Likelihood | Impact | Mitigation                                                       |
|--------------------------------------------------------|------------|--------|------------------------------------------------------------------|
| Atlas TLS handshake fails on bad URI                   | Low        | High   | Validate URI shape on boot; fail fast with clear error.          |
| Index creation slow on cold cluster                    | Low        | Low    | `EnsureIndexes` idempotent; runs once at boot.                   |
| `_id` collision (Firebase UID empty for some provider) | Low        | High   | Reject empty UID at `UpsertByUID` entry; defensive check.        |
| `mongo-driver/v2` API drift vs v1 examples online      | Medium     | Low    | Cross-reference research report; pin version in go.mod.          |
| Skeleton repos confuse later phases                    | Low        | Low    | Each has TODO comment naming the implementing phase.             |

## Security Considerations
- Atlas connection string contains password; must come from env (never committed).
- `.env` already in `.gitignore`; add `.env.example` with placeholder values.
- `mongo:7` local container uses dev credentials in `.env`; never reused in prod.
- No DDL-vs-DML user split needed (Mongo permissioning is collection-level; defer to deploy phase).
- Schema-version field defends against partial-deploy reads of old shape.

## Next Steps
- Phase 05 — Firebase Auth — depends on `UserRepo.UpsertByUID` shipped here.
