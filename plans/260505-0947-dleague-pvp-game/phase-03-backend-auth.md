---
phase: 3
title: "Backend + auth"
status: pending
priority: P1
effort: 1.5w
dependencies: [1, 2]
---

# Phase 3: Backend + auth

## Overview

Stand up Go API: user accounts, session-cookie auth, Postgres schema for users/games/attempts. Server can validate any client-submitted Wordle attempt independently (anti-cheat foundation). No PvP yet — this phase just enables identity + persistence.

## Requirements

- **Functional:**
  - Email + password registration with bcrypt hash
  - Login → secure HttpOnly session cookie (30d)
  - `/me` returns current user
  - Logout invalidates session
  - Rate-limit auth endpoints (5 req/min/IP)
  - `/api/v1/games/wordle/daily` returns today's seed (server-authoritative)
  - `/api/v1/games/wordle/submit` accepts attempt result, server re-runs game logic from `shared/game/wordle` to verify
  - All API errors follow consistent JSON shape `{error: {code, message}}`
- **Non-functional:**
  - Postgres migrations via `goose` or `golang-migrate`
  - Repo pattern: `store.UserRepo`, `store.AttemptRepo` interfaces, `pgx` impl
  - Each handler/repo file <200 LOC
  - 100% of mutation endpoints CSRF-protected (double-submit cookie)

## Architecture

**Schema (initial):**

```sql
users (id uuid pk, email citext unique, password_hash text, display_name text, created_at)
sessions (token text pk, user_id uuid fk, expires_at, created_at)
games (id uuid pk, type text, seed bigint, daily_date date, created_at, unique(type, daily_date))
attempts (id uuid pk, user_id uuid fk, game_id uuid fk, attempts jsonb, won bool, duration_ms int, created_at)
```

**Anti-cheat strategy:**
- Client submits full attempt sequence: `[{guess: "CRANE", at: 1234}, ...]`
- Server re-runs `shared/game/wordle` with daily seed
- If submitted hints don't match server-computed → reject + flag user
- Only after validation: insert into `attempts`

**Module layout:**

```
server/internal/
├── http/
│   ├── router.go            # chi setup, middleware chain
│   ├── auth_handlers.go     # /register, /login, /logout, /me
│   ├── game_handlers.go     # /games/wordle/daily, /submit
│   ├── middleware/
│   │   ├── session.go
│   │   ├── ratelimit.go
│   │   └── csrf.go
│   └── errors.go            # JSON error helper
├── store/
│   ├── store.go             # interfaces
│   ├── pg/                  # pgx impl
│   │   ├── users.go
│   │   ├── sessions.go
│   │   └── attempts.go
│   └── migrations/          # goose .sql files
└── auth/
    ├── password.go          # bcrypt
    └── session.go           # token gen, cookie helpers
```

## Related Code Files

**Create:**
- `server/internal/http/router.go`, `auth_handlers.go`, `game_handlers.go`, `errors.go`
- `server/internal/http/middleware/{session.go, ratelimit.go, csrf.go}`
- `server/internal/store/store.go` (interfaces)
- `server/internal/store/pg/{users.go, sessions.go, attempts.go, conn.go}`
- `server/internal/store/migrations/0001_init.sql` ... `0003_attempts.sql`
- `server/internal/auth/{password.go, session.go}`
- `server/internal/config/config.go` (env via `kelseyhightower/envconfig`)
- `shared/dto/auth.go`, `shared/dto/game.go`
- `server/cmd/api/main.go` (rewire from /health stub)
- `server/test/integration/auth_test.go`

**Modify:**
- `docker-compose.yml` (add app envvar example)
- `Makefile` (`db-migrate`, `db-reset`)

## Implementation Steps

1. Add Postgres migrations: users, sessions, games, attempts
2. `pgx` connection pool with sane defaults; config via `DATABASE_URL`
3. `auth.password` — bcrypt cost 12
4. Session: 32-byte random token → bcrypt store → set as HttpOnly+Secure+SameSite=Lax cookie
5. Auth handlers: register/login/logout/me with strict input validation
6. Rate-limit middleware (in-memory token bucket per IP for MVP, Redis later)
7. CSRF: double-submit cookie pattern (since we're using cookie auth)
8. Daily-puzzle handler: lookup or insert game row for today's UTC date+wordle
9. Submit handler: server re-runs `shared/game/wordle` with same seed, validates client's attempts match server hints exactly
10. Integration tests: spin Postgres testcontainer, exercise auth flow + submit happy/cheating paths
11. Wire client: replace local-only state with `/api/v1/games/wordle/submit` call

## Todo List

- [ ] Postgres migrations (users/sessions/games/attempts)
- [ ] `pgx` pool + repo impls
- [ ] bcrypt password hashing
- [ ] Session cookie auth
- [ ] Register/login/logout/me handlers
- [ ] Rate-limit middleware
- [ ] CSRF double-submit
- [ ] Daily seed endpoint
- [ ] Submit endpoint with server re-validation
- [ ] Integration tests (testcontainers)
- [ ] Client integration: submit attempts to API

## Success Criteria

- [ ] Register → login → submit attempt → see in `/me` history
- [ ] Tampered submit (wrong hints) returns 400, no row inserted
- [ ] Session cookie survives page reload, expires after 30d
- [ ] Rate-limit blocks brute-force login attempts
- [ ] Integration tests cover happy + cheating + auth-fail paths
- [ ] No N+1 queries in /me, /history endpoints

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| Bcrypt cost too high → slow login | Cost 12 baseline; benchmark on target host; bump if <100ms |
| Session table grows unbounded | Cron deletes expired sessions daily; or Postgres `partition` later |
| CSRF token leakage via XSS | HttpOnly cookies + strict CSP header in HTML shell |
| Server-side game logic divergence from client | `shared/game/wordle` is single source — no duplicate impl |
| Email collision attack on registration | Treat email as `citext`; case-insensitive unique constraint |

## Security Considerations

- Passwords: bcrypt cost 12, never logged
- Session tokens: 32-byte CSPRNG, stored hashed in DB
- Rate limit register/login (5/min/IP) + global (1000/min)
- CSP: `default-src 'self'; script-src 'self' 'wasm-unsafe-eval'`
- Reject any submit with future timestamp or duration <500ms (cheat detection signals)
