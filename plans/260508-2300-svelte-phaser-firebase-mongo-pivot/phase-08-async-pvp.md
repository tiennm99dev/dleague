---
phase: 8
title: "Async PvP (challenge link + daily leaderboard)"
status: pending
priority: P1
effort: 1w
dependencies: [7]
---

# Phase 08 — Async PvP

## Context Links
- `plans/archive/260505-0947-dleague-pvp-game/phase-04-async-pvp.md` (mine for spec; rebase to single-WS + Mongo)
- `plans/reports/researcher-260508-2300-mongodb-atlas-go.md` §3, §5 (matches/attempts schemas + transactions)
- `server/internal/game/wordle/daily.go` (Phase 07)
- `server/internal/store/{matches.go,attempts.go,leaderboards.go}` (skeletons from Phase 04)
- `proto/dleague/v1/envelope.proto` + `wordle.proto` (extend with challenge/leaderboard messages)

## Overview
Ship the first PvP loop entirely over the existing single-WS architecture (no new HTTP routes). User finishes a daily round → emits `MESSAGE_TYPE_CHALLENGE_CREATE` → server creates `matches` doc + returns shareable URL. Friend opens URL → client emits `CHALLENGE_JOIN{token}` → server returns same seed → friend plays → server records attempt + decides winner via Mongo transaction. Daily leaderboard refreshed via in-process scheduled job.

## Key Insights
- **Single WS, no REST:** all flows go through Envelope dispatch — keeps wire surface tight (one auth gate, one error path).
- **Atomic join:** Mongo transaction on `matches` + `attempts` + `users` (winner stats) — research §5 confirms M0 supports transactions.
- **Daily leaderboard:** pre-computed in `leaderboards` collection (research §3 + §6). Refresh frequency: 5-min in-process tick during peak hours. Mongo Change Streams deferred to v2.
- **Async vs sync attempts (mongo unresolved Q3):** SAME `attempts` collection, `mode: "async"` field. Index on `match_id` already covers both.
- **Anonymous play:** anonymous users CAN create challenges (have UID via Firebase) but their attempts are excluded from leaderboards (filter `is_anonymous != true`).
- **Anti-cheat minimum:** reject attempts with `time_ms < 500` or `> 86400000`.
- **Challenge URL:** `https://dleague.gg/m/<share_token>` — token is UUID v4 stored on match doc; server-side route `/m/<token>` falls back to `index.html` per Phase 06 SPA fallback; client reads token from path on mount.

## Requirements
**Functional:**
- New proto messages in `proto/dleague/v1/match.proto`:
  - `ChallengeCreate{string game_id; int64 seed_override (optional)}` → `ChallengeCreateAck{string match_id; string share_token; int64 seed}`
  - `ChallengeJoin{string share_token}` → `ChallengeJoinAck{string match_id; int64 seed; string game_id}`
  - `AttemptSubmit{string match_id; repeated string guesses; int32 time_ms; bool won}` → `AttemptSubmitAck{string winner_uid; string status}` (status = `pending`|`completed`)
  - `LeaderboardQuery{string game_id; string period}` → `LeaderboardSnapshot{repeated LeaderboardEntry rankings}` with `LeaderboardEntry{string uid; string display_name; int32 attempts; int32 time_ms; int32 rank}`.
- `MatchRepo` (fill from skeleton): `Create`, `GetByShareToken`, `JoinAsChallengee`, `RecordAttempt`, `Complete` — all use `mongo.SessionContext` for transaction-safe variants where needed.
- `AttemptRepo`: `Insert`, `ListByMatch`.
- `LeaderboardRepo`: `Refresh(ctx, gameID, period)` — aggregation pipeline producing `rankings[]` sorted by (won DESC, attempts ASC, time_ms ASC); upsert into `leaderboards` doc.
- WS handlers `server/internal/ws/handlers/match.go`: `handleChallengeCreate`, `handleChallengeJoin`, `handleAttemptSubmit`, `handleLeaderboardQuery`.
- `Background tick`: `server/internal/scheduler/tick.go` — every 5 min, refresh leaderboards for current daily wordle.
- Anonymous attempts excluded from leaderboards (filter at refresh-aggregation stage).
- Client routes:
  - `/play` (Phase 07) — after game ends, "Challenge friend" button.
  - `/m/<share_token>` — challenge accept page; reads token from URL, sends `ChallengeJoin`, plays game, on finish `AttemptSubmit`.
  - `/leaderboard` — pulls daily leaderboard via `LeaderboardQuery`.
- Match expiry: 7 days. Background tick also cleans up expired matches (drop `attempts` rows belonging to them).

**Non-functional:**
- Concurrent join attempts: only one challengee accepted (transaction enforces uniqueness via update with filter `challengee_uid: null`).
- Leaderboard query <100ms at 10K entries (index on `_id` of leaderboards doc).
- Each handler file <200 LOC.
- Match transaction p95 <50ms.

## Architecture
```
challenger flow:
  /play wins → sendRequest(CHALLENGE_CREATE, {game_id:"wordle"})
       ↳ server: matches.InsertOne {challenger_uid, share_token, seed=today.seed, mode:"async"}
       ↳ ack {match_id, share_token, seed}
  client copies link "/m/<share_token>" to clipboard

challengee flow:
  visit /m/<token>
  on mount: sendRequest(CHALLENGE_JOIN, {share_token})
       ↳ server: matches.FindOne(share_token) → ack {match_id, seed, game_id}
  /play with seed override
  on finish: sendRequest(ATTEMPT_SUBMIT, {match_id, guesses, time_ms, won})
       ↳ server: WithTransaction:
            ├─ attempts.InsertOne
            ├─ matches.UpdateOne (set challengee_uid + completed_at)
            ├─ if both attempts present → compute winner (won DESC, attempts ASC, time ASC)
            ├─ matches.UpdateOne (set winner_uid, state="complete")
            └─ users.UpdateOne (winner.stats.wins++, loser.stats.losses++) — skip if anonymous
       ↳ ack {winner_uid, status:"completed"}
  client navigates to results

leaderboard:
  /leaderboard mount → sendRequest(LEADERBOARD_QUERY, {game_id, period:"daily"})
       ↳ server: leaderboards.FindOne({_id: "wordle_daily_<today>"})
       ↳ ack {rankings:[...]}

scheduled tick (every 5 min):
  for each (game, period) in [("wordle","daily")]:
    aggregate attempts → top 100 → upsert into leaderboards
```

## Related Code Files
**Create (Go):**
- `proto/dleague/v1/match.proto`
- `server/internal/ws/handlers/match.go`
- `server/internal/ws/handlers/leaderboard.go`
- `server/internal/scheduler/tick.go`
- `server/internal/scheduler/tick_test.go`
- `server/internal/store/matches_test.go`
- `server/internal/store/attempts_test.go`
- `server/internal/store/leaderboards_test.go`

**Create (Svelte):**
- `web/src/routes/m/[token]/+page.svelte` (challenge accept)
- `web/src/routes/m/[token]/+page.ts` (load token from params)
- `web/src/routes/leaderboard/+page.svelte`
- `web/src/lib/components/share-button.svelte` (clipboard)
- `web/src/lib/components/leaderboard-table.svelte`
- `web/src/lib/components/results-screen.svelte`

**Modify:**
- `server/internal/store/matches.go` — fill in methods
- `server/internal/store/attempts.go` — fill in
- `server/internal/store/leaderboards.go` — fill in `Refresh`
- `server/internal/ws/hub.go` — register handlers
- `server/cmd/server/main.go` — start scheduler tick goroutine; clean shutdown
- `proto/dleague/v1/envelope.proto` — new MessageType enum values
- `web/src/routes/play/+page.svelte` — add "Challenge friend" button on win
- `web/src/lib/ws.ts` — likely no changes (generic enough)
- `docs/system-architecture.md` — fill async-PvP flow section

**Delete:** none.

## Implementation Steps
1. **Proto:** new `match.proto` with 8 messages + 4 envelope enum values: `CHALLENGE_CREATE=9`, `CHALLENGE_CREATE_ACK=10`, `CHALLENGE_JOIN=11`, `CHALLENGE_JOIN_ACK=12`, `ATTEMPT_SUBMIT=13`, `ATTEMPT_SUBMIT_ACK=14`, `LEADERBOARD_QUERY=15`, `LEADERBOARD_SNAPSHOT=16`. Run `make proto-gen`.
2. **MatchRepo fill:**
   - `Create(ctx, m Match) (string matchID, string shareToken, error)` — InsertOne with `share_token = uuid.NewString()`, `state="pending"`, `mode="async"`, `expires_at=now+7d`.
   - `GetByShareToken(ctx, token)`.
   - `JoinAsChallengee(ctx, token, uid)` — `FindOneAndUpdate({share_token, challengee_uid: nil}, $set: {challengee_uid: uid, joined_at: now})` returns updated doc; if no match → "already joined or not found" error.
3. **AttemptRepo fill:** `Insert(ctx, a Attempt)`, `ListByMatch(ctx, matchID)`.
4. **LeaderboardRepo fill:** `Refresh(ctx, gameID, period)` aggregation:
   ```
   match: {game_id, mode: "async", state: "complete", solo: false (or daily-flag)}
   join attempts -> users (lookup display_name, is_anonymous)
   filter out is_anonymous: true
   sort: won DESC, attempts ASC, time_ms ASC
   limit 100
   project: {uid, display_name, attempts, time_ms}
   ```
   Upsert into `leaderboards.{_id: "<game>_<period>_<date>"}`.
5. **Match handler:** `handleAttemptSubmit` opens session.WithTransaction:
   - InsertOne attempt
   - UpdateOne match (set challengee_uid if not set + state if both attempts present)
   - If both present → compute winner (helper) → UpdateOne users
6. **Anonymous filter:** lookup user `is_anonymous` flag inside transaction; skip stats increment for anonymous.
7. **Scheduler:** `scheduler/tick.go` — `Run(ctx, refreshInterval, repos)` runs `LeaderboardRepo.Refresh("wordle","daily")` every 5min until ctx cancelled. main.go calls in goroutine; shuts down cleanly.
8. **Match expiry sweep:** within scheduler tick (15min interval), `matches.DeleteMany({state:"pending", expires_at < now})`. Cascade delete dangling attempts is overkill at MVP — accept dangling rows.
9. **Anti-cheat:** in `handleAttemptSubmit`, reject `time_ms < 500 || time_ms > 86400000` with `MESSAGE_TYPE_ERROR{422}`.
10. **Idempotency:** if same user submits twice for same match, return existing ack (lookup by `match_id + player_uid` in attempts before insert).
11. **Client `/m/[token]/+page.svelte`:** load uses `+page.ts` to extract `params.token`. On mount: `await ws.sendRequest(CHALLENGE_JOIN, {share_token: token})`. On success: navigate `/play?match=<id>&seed=<seed>`. On error (already joined / not found): show clear message.
12. **Client `/play` extensions:**
   - Read `?match` and `?seed` query params; if present, run challenge mode (uses provided seed instead of today's daily).
   - On terminal state: post `ATTEMPT_SUBMIT`. Show "Challenge friend" button if no match query (means user played daily solo).
13. **Client `/leaderboard/+page.svelte`:** subscribe to LEADERBOARD_QUERY result; render table with rank, display_name, attempts, time. Refresh on focus.
14. **Tests:**
    - `matches_test.go`: race condition for join — two concurrent JoinAsChallengee calls; only one wins.
    - `attempts_test.go`: insert + list round-trip.
    - `leaderboards_test.go`: aggregation correctness with anonymous filter.
    - `scheduler/tick_test.go`: tick runs, calls refresh.
    - End-to-end: two test users, A creates challenge, B joins, both submit, server returns correct winner.
15. **Manual smoke:** A signs in → plays daily → wins → "Challenge friend" → copy link → open in incognito → sign in as B → play same seed → both see correct winner. /leaderboard shows both entries (ordered correctly).

## Todo List
- [ ] Proto: match.proto + envelope enum values
- [ ] MatchRepo fill (Create, GetByShareToken, JoinAsChallengee)
- [ ] AttemptRepo fill (Insert, ListByMatch)
- [ ] LeaderboardRepo.Refresh aggregation
- [ ] handleChallengeCreate
- [ ] handleChallengeJoin
- [ ] handleAttemptSubmit (transaction)
- [ ] handleLeaderboardQuery
- [ ] Anonymous filter in leaderboard
- [ ] Anti-cheat time bounds
- [ ] Idempotency on duplicate submit
- [ ] Scheduler tick goroutine
- [ ] Match expiry sweep
- [ ] /m/[token] route
- [ ] /play challenge mode (seed override)
- [ ] /leaderboard route
- [ ] Share button (clipboard)
- [ ] Race-condition test
- [ ] E2E test
- [ ] Manual smoke

## Success Criteria
- [ ] A challenges B → B joins → both play → correct winner persisted
- [ ] Concurrent joins: only one succeeds; second gets clear error
- [ ] Anonymous user attempt does NOT appear on leaderboard
- [ ] Leaderboard refresh runs every 5 min in background
- [ ] Expired match deleted by sweep within 15 min of expiry
- [ ] Same user double-submit → second is idempotent (returns existing result)
- [ ] /leaderboard renders top 100 < 100ms after WS reply

## Risk Assessment
| Risk                                                   | Likelihood | Impact | Mitigation                                                       |
|--------------------------------------------------------|------------|--------|------------------------------------------------------------------|
| Mongo transaction abort under contention               | Medium     | Medium | `WithTransaction` retries automatically; document max-retry log. |
| Leaderboard aggregation pipeline > M0 32MB sort limit  | Low (MVP)  | High   | Limit to top 100 + index-driven sort; revisit at 100K matches.   |
| Anonymous user manipulates display_name → leak         | Medium     | Low    | Anonymous users have no display_name field; filter excludes anyway. |
| Share token guessable                                   | Low        | High   | UUID v4 (122 bits entropy).                                      |
| Scheduler goroutine survives shutdown                   | Low        | Low    | Tied to ctx; `main.go` cancels on SIGTERM.                       |
| Concurrent submit creates duplicate attempt rows        | Medium     | Medium | Check for existing `{match_id, player_uid}` inside transaction.  |

## Security Considerations
- Anti-cheat: reject impossible-fast (`<500ms`) and replay-attack (`>24h`) submissions.
- Idempotent submit prevents leaderboard pollution via repeated submit.
- Anonymous users gated from leaderboard via `is_anonymous` flag, not display_name absence (defensive).
- Share tokens single-use for joining (after challengee_uid set, second join rejected).
- Match doc reveals challenger's attempts only after challengee submits (enforce in `handleChallengeJoin` response shape — return only `seed` + `game_id`, not attempts).
- Rate-limit `CHALLENGE_CREATE` per user per minute (defer to Phase 09 with general rate-limit).

## Next Steps
- Phase 09 — Sync PvP — depends on match infrastructure shipped here.
