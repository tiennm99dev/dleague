---
phase: 4
title: "Async PvP"
status: pending
priority: P1
effort: 1.5w
dependencies: [3]
---

# Phase 4: Async PvP

## Overview

Ship the first PvP loop: daily puzzle leaderboard + challenge-friend-via-link. No real-time required — players play same puzzle independently, results compared. Validates the core competitive feel before building WebSocket sync layer.

## Requirements

- **Functional:**
  - Daily puzzle: every user plays the same Wordle answer; leaderboard ranks by (won, attempts ASC, duration ASC)
  - Challenge link: user finishes a game → gets share URL → friend opens → plays same seed → result compared
  - Match record stores both players' attempt sequences + winner
  - User profile shows: win rate, daily streak, total played, recent matches
  - Anonymous "ghost" play allowed (no account = play but no leaderboard entry)
  - Anti-cheat: friend cannot submit until they've actually played (server tracks session timing)
- **Non-functional:**
  - Leaderboard query <100ms at 10k entries (proper indexing)
  - Challenge URLs unguessable (UUID v4 tokens)
  - Idempotent submit — same match can't be double-counted

## Architecture

**Daily leaderboard:**
- Materialize daily_leaderboards (game_id, user_id, attempts, duration_ms, rank) via trigger or scheduled job
- API: `GET /api/v1/leaderboard/daily?date=YYYY-MM-DD` paginated, top 100 + user's rank

**Challenge flow:**
1. Player A finishes game → POST `/api/v1/matches` with their attempts → server creates `matches` row, returns `share_token`
2. Share URL: `https://dleague.gg/m/{share_token}`
3. Player B opens link → frontend fetches match metadata (seed, A's stats hidden until B plays)
4. Player B plays same seed → POST `/api/v1/matches/{token}/join` with attempts
5. Server determines winner: tie-break by attempts ASC, then duration ASC; persist + notify A via stored "pending result" (Phase 6 polish: email/push)

**Schema additions:**

```sql
matches (
  id uuid pk,
  share_token uuid unique,
  game_type text,
  seed bigint,
  challenger_id uuid fk users,
  challenger_attempts jsonb,
  challenger_won bool,
  challenger_duration_ms int,
  challengee_id uuid fk users null,
  challengee_attempts jsonb null,
  challengee_won bool null,
  challengee_duration_ms int null,
  winner_id uuid null,
  status text,                  -- pending | completed | expired
  created_at, completed_at, expires_at
)

daily_leaderboards (
  game_id uuid fk games,
  user_id uuid fk users,
  attempts smallint,
  duration_ms int,
  won bool,
  rank int,
  pk(game_id, user_id)
)
```

## Related Code Files

**Create:**
- `server/internal/store/pg/matches.go`
- `server/internal/store/pg/leaderboard.go`
- `server/internal/store/migrations/0004_matches.sql`, `0005_leaderboards.sql`
- `server/internal/http/match_handlers.go` (create, get, join, mine)
- `server/internal/http/leaderboard_handlers.go`
- `server/internal/leaderboard/refresh.go` (scheduled rebuild)
- `shared/dto/match.go`, `shared/dto/leaderboard.go`
- `client/internal/scene/leaderboard.go`
- `client/internal/scene/challenge.go` (share flow + accept flow)
- `client/internal/ui/share_button.go` (clipboard via `syscall/js`)

**Modify:**
- `client/internal/scene/results.go` (add "Challenge friend" button)
- `client/internal/scene/title.go` (add "Daily" + "Leaderboard" entries)
- `server/internal/http/router.go` (mount /matches, /leaderboard)

## Implementation Steps

1. Migration: `matches` + `daily_leaderboards` tables + indexes
2. `MatchRepo`: create, get-by-token, join (atomic with row lock), list-for-user
3. POST `/matches` — validates challenger's attempts via shared logic, creates match with `status=pending`
4. GET `/matches/{token}` — returns seed only; hides A's stats until B has joined
5. POST `/matches/{token}/join` — atomic update with `SELECT FOR UPDATE`, decides winner, sets `completed`
6. Leaderboard: nightly cron + on-submit incremental update for daily puzzle
7. Client: results scene "Challenge a friend" → calls API → copies link to clipboard
8. Client: `/m/{token}` route → loads match → plays → submits join
9. Profile scene: list user's matches with W/L badges
10. Add expiry: matches expire after 7d if not joined; cron sweep
11. Tests: concurrent-join race condition, expired match rejection, double-join idempotency

## Todo List

- [ ] Matches + leaderboards migrations
- [ ] MatchRepo with atomic join
- [ ] Create/get/join/list match endpoints
- [ ] Daily leaderboard query + caching
- [ ] Results scene "Challenge friend" button
- [ ] Match acceptance scene at `/m/{token}`
- [ ] Profile scene with match history
- [ ] Match expiry cron
- [ ] Race-condition tests
- [ ] Anti-cheat: minimum duration check on join

## Success Criteria

- [ ] User A challenges B via link → B plays → both see correct winner
- [ ] Daily leaderboard shows correct ranking (won + attempts + duration)
- [ ] Concurrent join attempts: only one succeeds, other gets clear error
- [ ] Match link stays valid for 7d, then expires gracefully
- [ ] Anonymous user can play daily but does not appear on leaderboard
- [ ] Profile loads match history <500ms for 100 matches

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| Race condition on simultaneous join | `SELECT FOR UPDATE` + transaction; tested explicitly |
| Leaderboard rebuild slow at scale | Incremental update on submit; nightly full rebuild as fallback |
| Friend sees A's exact attempts before playing → cheats | Endpoint hides A's data until B's submit lands |
| Stale share link spam | Expire after 7d + rate-limit match creation per user |
| Daily puzzle TZ confusion | UTC midnight, show countdown in user's local TZ |

## Security Considerations

- Share tokens: UUID v4, unguessable, single-use after both sides played (still viewable but not joinable)
- Anti-cheat heuristic: reject `duration_ms < 500` (impossible-fast) or `> 86400000` (>24h, replay attack)
- Profile is public (read-only) — sanitize display name; rate-limit profile-fetch
