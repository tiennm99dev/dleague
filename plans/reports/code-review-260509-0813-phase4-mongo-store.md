# Phase 04 — MongoDB Store Rewrite: Adversarial Code Review

**Reviewer:** code-reviewer (adversarial pre-commit)
**Date:** 2026-05-09
**Scope:** untracked + modified files since `f5b9c9d` covering the MongoDB store rewrite.

## Summary

Phase 04 ships a clean, minimal `mongo-driver/v2` store layer + skeletons. Build is green (`go build ./...`), `go vet` clean, gated tests skip cleanly without `MONGO_TEST_URI`. API usage matches mongo-driver v2.6.0 (verified against module cache). Wire/ws/* package untouched — no Phase 02 regression.

Two correctness-relevant findings (both low severity, both around test ergonomics, neither blocking the commit):
1. `Client.Database()` hardcodes `"dleague"`, so the `MONGO_TEST_URI` path-segment (`/dleague_test`) is ignored — CI integration tests actually mutate the `dleague` database.
2. `repomix-output.xml` (518 KB) is untracked at repo root and not gitignored; would be picked up by an unguarded `git add .`.

Everything else is on-spec.

## Files Reviewed

- `server/internal/store/{mongo,models,indexes,users,games,matches,attempts,daily_puzzles,leaderboards,mongo_test,users_test}.go`
- `server/cmd/api/main.go`
- `server/internal/config/config.go`
- `server/internal/http/{health,router}.go`
- `server/go.mod`
- `docker-compose.yml`, `Makefile`, `.github/workflows/ci.yml`, `.env.example`
- `docs/system-architecture.md`

## Findings

### Critical
_None._

### High
_None._

### Medium

- **M1 — `MONGO_TEST_URI` database segment ignored**
  - `server/internal/store/mongo.go:84` always returns `c.inner.Database("dleague")`.
  - `mongo_test.go:13`, `.env.example:13`, `ci.yml:74` all advertise `dleague_test` via the URI path. The driver does not auto-select the URI-path database for `client.Database(name)` — the hardcoded `defaultDB` wins.
  - Effect: integration tests on CI write to a database named `dleague` (the production-name), not `dleague_test`. In CI this is harmless (ephemeral container), but the docs/comments mislead future humans and any local dev pointing `MONGO_TEST_URI` at a shared cluster would silently scribble into the prod-named DB.
  - Fix (next phase, not blocking): either (a) accept a database name in `Connect`/expose `Database(name string)`, or (b) drop the `/dleague_test` segment from URIs and rename to clarify.

- **M2 — Boot context is shared between Connect+Ping+EnsureIndexes AND held alive past the boot phase**
  - `server/cmd/api/main.go:28-29` creates a 15-second `bootCtx` and `defer cancelBoot()`.
  - `bootCtx` is consumed by `Connect`, `Ping`, `EnsureIndexes` — fine. But the `defer cancelBoot()` only fires at process exit, so the boot context lingers (cancellation doesn't matter post-`EnsureIndexes`, just messy). Not a correctness issue; the timeout already elapsed before the server starts serving.
  - Also: the 15s budget covers all three operations together. If Atlas takes 10s to connect + 4s to ping, `EnsureIndexes` gets 1s. Fine for warm clusters; could trip on first cold deploys with cold Atlas indexing.
  - Not blocking; revisit if you ever see boot-timeout errors against M0 cold-start.

### Low

- **L1 — `repomix-output.xml` untracked at repo root, not in `.gitignore`**
  - 518 KB file. Trivially included by `git add .`. Add a `repomix-output.*` pattern to `.gitignore`.

- **L2 — `EnsureIndexes` log message hardcodes "5 collections"**
  - `indexes.go:102` — if a future phase adds a `sessions` collection's indexes the count drifts; consider `len(specs)`. Cosmetic.

- **L3 — `UpsertByUID` swallows `ErrNoDocuments` from `FindOneAndUpdate`**
  - `users.go:68-70`. With `SetUpsert(true)` + `SetReturnDocument(After)`, the v2 driver should never return `ErrNoDocuments` on success path (insert produces a doc, update returns the updated doc). Swallowing is defensive, not wrong. Minor: `SetReturnDocument(After)` is also unused — `UpsertByUID` does not consume `res.Decode(...)`. Drop the option for cleanliness, or wire a return value.

- **L4 — `users.go` upsert doesn't normalize `display_name`**
  - The `users.display_name` index is unique. If two callers race-upsert profiles with the same display name on different UIDs, the second `FindOneAndUpdate` errors with a duplicate-key error and the user-visible message is `store: upsert user "uid": ...`. Acceptable for now; later phases need a sentinel `ErrDisplayNameTaken` and a retry/rename flow. Document the constraint near `UserProfile`.

- **L5 — `Client.Database()` exposes raw `*mongo.Database`**
  - Repos receive `*mongo.Database` directly, breaking the encapsulation that `Client` was supposed to provide. This is fine for YAGNI/MVP, but it leaks the driver type into every repo signature, making mock-via-interface harder if you ever want unit tests without a live Mongo. Note for later, not a fix.

- **L6 — `Client.Inner()` exposes raw `*mongo.Client`**
  - `mongo.go:88` adds `Inner()` for "transaction helpers" that don't exist yet. YAGNI — remove until phase-09 actually starts a session.

- **L7 — `TestUserRepo_EmptyUID_Rejected` gates on `MONGO_TEST_URI`**
  - `users_test.go:106-126` requires Mongo even though the tested code path returns `ErrEmptyUID` before any DB call. Hoist this test out of the gated block so it runs in offline CI / dev as a pure unit test of the boundary check.

- **L8 — Mongo-Express in dev exposes admin auth via env vars without HTTP basic auth**
  - `docker-compose.yml:21-32` uses `ME_CONFIG_MONGODB_ADMINUSERNAME` for upstream auth but does NOT set `ME_CONFIG_BASICAUTH_USERNAME`/`_PASSWORD`. Newer mongo-express images refuse to boot without basic-auth set; older images expose the UI on `:8081` with no login. Phase spec said "no auth in dev — confirm or flag." Flagging: confirm image tag works; if it fails to start, you'll need to add `ME_CONFIG_BASICAUTH_USERNAME=admin` + `ME_CONFIG_BASICAUTH_PASSWORD=admin` and `ME_CONFIG_BASICAUTH_ENABLED=true`, or pin a known-working tag (e.g. `mongo-express:1.0.2`).

- **L9 — `server/go.mod` go directive is `go 1.26`**
  - `go.mod:3`. CI uses `go-version: "1.26"` — match. Note: research report §1 says driver requires Go 1.19+; you're well above. Just confirming no skew.

### Nit

- **N1 — `models.go` capitalizes `_id` for ObjectID-typed structs but uses `,omitempty`**
  - `Match.ID`, `Attempt.ID` use `bson:"_id,omitempty"` — correct pattern so `InsertOne` lets the driver auto-generate ObjectID when the field is zero.

- **N2 — `Match.EndedAt *time.Time` is the only pointer time field**
  - Other "nullable" times (`User.LastLogin`) are zero-valued `time.Time`. Inconsistent — `EndedAt` rightly pointers because "match in progress" must distinguish from epoch-zero. `User.LastLogin` could likewise be a pointer; but since it's set on every `UpsertByUID`, zero-value reads are unreachable. Fine.

- **N3 — `attempts.go`, `matches.go`, `daily_puzzles.go`, `leaderboards.go` skeletons store `coll` but never use it**
  - golangci-lint `unused` won't complain because `coll` is exported via the struct field, and the constructor consumes `db`. Fine.

- **N4 — `mongo_test.go` `cursor.All(ctx, &docs)` decodes into `[]interface{}`**
  - Works, but `[]bson.M` would be more idiomatic. Cosmetic.

- **N5 — `.env.example` does not document `DLEAGUE_ADDR` or `DLEAGUE_WEB`**
  - Both have defaults, so omission is intentional. Add a comment at top: "All other vars use sensible defaults — see config.go."

## Checklist Verification

| Item | Status | Notes |
|---|---|---|
| `mongo.go` Connect/Ping/Disconnect/Database semantics | OK | v2 API: `mongo.Connect(opts...)` no ctx, `Ping(ctx, *ReadPref)` nil OK |
| Pool sizing matches research §1 (max 100, min 10, idle 30s, connect 10s, sel 5s) | OK | `mongo.go:21-33` |
| TLS for `mongodb+srv://` | OK | implicit; no explicit `?ssl=true` injection |
| Disconnect ordering (separate timeout, defer in main) | OK | `main.go:35-41` uses 5s `context.Background()` budget |
| Models have `bson:` tags + `SchemaVersion` | OK | all 6 structs |
| `_id` types per schema | OK | string for users/games/daily_puzzles/leaderboards; ObjectID for matches/attempts |
| 8 indexes from research §4 via CreateMany | OK | 1 users + 3 matches + 2 attempts + 1 daily_puzzles + 1 leaderboards = 8 |
| ESR ordering (state ASC, created_at DESC) | OK | `indexes.go:46-49` |
| `EnsureIndexes` idempotent | OK | tested twice in `TestEnsureIndexes` |
| Unique `users.display_name` | OK | flagged race risk in L4 |
| `UpsertByUID` semantics: `$setOnInsert` for created_at + stats | OK | `users.go:55-63` |
| `ErrEmptyUID` boundary | OK | both methods |
| `GetByUID` returns `(nil, nil)` on `ErrNoDocuments` | OK | `users.go:85-87` |
| Skeleton repos: constructor + 1 method + TODO | Mostly | matches/attempts/daily_puzzles/leaderboards skeletons have **no `Get` method** — only TODOs. Spec says "constructor + 1 `Get` method." Only `games.go` has a method. Doesn't break anything since they're not called yet, but doesn't match phase spec literally. |
| TODO phase numbers | OK | matches references phase-08/09; daily_puzzles references phase-07; leaderboards phase-08 |
| No import cycles | OK | `go build ./...` clean |
| Tests gated by `MONGO_TEST_URI` with `t.Skip` | OK | all 5 test funcs |
| UpsertByUID round-trip + empty-UID rejection covered | OK | `users_test.go` |
| Idempotency test runs `EnsureIndexes` twice | OK | `mongo_test.go:59-64` |
| Index count check ≥8 | OK | exact `== 8` check |
| `main.go` ordering: Connect → Ping → EnsureIndexes | OK | `main.go:31-51` |
| 15s boot budget | OK | `main.go:28` |
| Disconnect own timeout | OK | 5s, `main.go:36-37` |
| Repos constructed but unused (`_ =`) | OK | `main.go:54-59` |
| Origin assertion + Hub.MaxConns wiring | OK | `main.go:63-69` |
| `health.go` calls `Client.Ping(ctx)` | OK | `health.go:31` |
| `health.go` nil-Client returns "ok" | OK | `health.go:22-26` |
| `DatabaseURL` → `MongoURI`, `ErrMissingDatabaseURL` → `ErrMissingMongoURI` | OK | `config.go:31, 61, 81` |
| `MONGO_URI` documented in `.env.example` | OK | line 1-5 |
| `docker-compose.yml`: `mongo:7` + `mongo-express` + healthcheck + volume | OK | flagged mongo-express auth in L8 |
| Credentials from `${MONGO_USERNAME}`/`${MONGO_PASSWORD}` | OK | not hardcoded |
| CI Mongo service container + `MONGO_TEST_URI` | OK | unauthed `mongodb://mongo:27017/dleague_test` matches unauthed service container (no `MONGO_INITDB_ROOT_USERNAME` set) — L1/M1 caveat about db name |
| Phase 02 carryover: ws/* untouched | OK | `git diff -- server/internal/ws/` empty |
| File LOC < 200 each | OK | max is `users_test.go` at 126; `models.go` 108; `indexes.go` 104 |
| `go vet ./internal/store/...` | clean | |
| `go build ./...` | clean | |
| `go test -count=1 ./internal/store/...` | passes (skipped without env) | |

## Unresolved Questions

1. **M1 fix scope** — should `Connect` accept a database name parameter, or should we drop the path segment from `MONGO_TEST_URI` and document that all envs use database `dleague`? Affects whether CI tests pollute the same DB as a colocated dev mongo.
2. **mongo-express tag** — does the unspecified `mongo-express` tag (latest) require basic-auth env vars to start? Quick `make compose-up` smoke test would resolve.
3. **`Client.Inner()` necessity** — leave for phase-09 or remove until needed?
4. **Display-name uniqueness UX** — when collision is hit, what surfaces to the client? Phase 05 will hit this; flag for the auth-flow design.
5. **`repomix-output.xml`** — likely a forgotten artefact from an analysis run; confirm it should be `.gitignore`'d (and possibly removed locally).

---

**Status:** DONE_WITH_CONCERNS

**Concerns (none correctness-blocking):**
- M1 hardcoded `defaultDB="dleague"` makes the `dleague_test` URI path a lie. Tests in CI write to a DB named `dleague`, not `dleague_test`.
- L1 `repomix-output.xml` untracked + ungitignored at repo root.
- L8 mongo-express likely needs basic-auth env vars on newer image tags; phase did not pin a tag.

Recommend committing as-is and addressing M1 + L1 + L8 in a follow-up before Phase 05 ships, since Phase 05 (Firebase Auth) will exercise `UpsertByUID` against the same misnamed test DB.
