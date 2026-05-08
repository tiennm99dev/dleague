# Phase 6: Async PvP on Firestore

## Context Links
- Supersedes: `plans/260505-0947-dleague-pvp-game/phase-04-async-pvp.md`
- Phase-3 schema: `/matches/{matchId}` + `/matches/{matchId}/attempts/{uid}`
- Phase-5 game flow: GAME_START / GAME_GUESS / GAME_FEEDBACK / GAME_END
- Locked: server-mediated writes; client direct read of own match (rules enforce)

## Overview
- **Priority:** P1 (one of two PvP modes; engagement core)
- **Status:** pending
- **Effort:** 4d
- Async match = creator solves puzzle, shares link, joiner solves SAME puzzle later, results compared. No live opponent presence. Match lifecycle entirely server-orchestrated; clients only display state.

## Key Insights
- "Same puzzle" = same `(puzzle_date, game_id)` → both attempts use same daily answer → comparison is trivial (attempts_used + duration_ms)
- Share link format: `https://dleague.tld/m/{share_token}` — client extracts token, sends `MATCH_JOIN` over WS
- Match status transitions: `open` (creator created, no joiner) → `in_progress` (joiner accepted, may also encompass "creator done, joiner playing") → `completed` (both attempts done) → `expired` (>7d, no joiner)
- Tie-breaker rule: lower `attempts_used` wins; if tied, lower `duration_ms`; if tied, draw
- Daily mode = a degenerate async match: everyone "joins" the global daily puzzle; leaderboard is the comparison
- Client subscribes to `/matches/{matchId}` via Firebase JS SDK `onSnapshot` for live "joiner accepted" notification (cheap: 1 read on each delta)

## Requirements

### Functional

#### Wire format additions
- `MESSAGE_TYPE_MATCH_CREATE = 11` — request: `{game_id, kind: 'async'}` → server creates match doc, returns share token
- `MESSAGE_TYPE_MATCH_CREATED = 12` — response: `{match_id, share_token, share_url}`
- `MESSAGE_TYPE_MATCH_JOIN = 13` — request: `{share_token}` → server validates + binds joiner_uid
- `MESSAGE_TYPE_MATCH_JOINED = 14` — response: `{match_id, game_id, puzzle_date}` — client then sends GAME_START with this match_id
- `MESSAGE_TYPE_MATCH_RESULT = 15` — server pushes when BOTH attempts done: `{match_id, winner_uid, draw, creator: {attempts, duration}, joiner: {attempts, duration}}`
- `MESSAGE_TYPE_LEADERBOARD_QUERY = 16` — request: `{game_id, date}` → returns top 100
- `MESSAGE_TYPE_LEADERBOARD = 17` — response: leaderboard doc

#### Match lifecycle (server-driven)
1. Creator: WS → `MATCH_CREATE{game_id, kind: async}`
   - server: generate `match_id` (20-char base62 random)
   - server: generate `share_token` (8-char base62 random; collision-check)
   - server: write `/matches/{match_id}` with creator_uid, status='open', share_token
   - server → `MATCH_CREATED{match_id, share_token, share_url: "https://dleague.tld/m/{token}"}`
2. Creator plays → standard GAME_START / GAME_GUESS / GAME_FEEDBACK / GAME_END flow (phase-5)
3. Joiner opens share link in any device → React route `/m/{token}` → after auth, WS → `MATCH_JOIN{share_token}`
   - server: lookup match by share_token (composite query? OR write index)
   - server: validate match.status == 'open' AND match.creator_uid != joiner_uid
   - server: update match.joiner_uid + status='in_progress'
   - server → `MATCH_JOINED{match_id, game_id, puzzle_date}`
4. Joiner plays → standard game flow → GAME_END
5. Server detects both attempts complete:
   - on every `MarkComplete` (phase-5 attempts.go), check if other attempt exists
   - if yes: compute winner; write match.winner_uid + status='completed'; push `MATCH_RESULT` to BOTH conns (if connected) AND store result
6. Either user reconnects later → `/matches/{match_id}` doc readable; client renders past result from doc

#### Leaderboard (daily, simple)
- Server-side aggregation: every match completion in `daily` kind triggers `RecomputeLeaderboard(date, game_id)` (idempotent)
- Reads top-100 attempts via Firestore query `attempts where won=true order by attempts_used asc, duration_ms asc limit 100` (collection-group query)
- Writes single `/leaderboards/{date}_{gameId}` doc with `entries[]`
- Clients fetch single doc on home-screen mount (1 read)

### Non-functional
- Match creation < 200ms p95 (1 Firestore write)
- Daily leaderboard query: <500ms p95 at 100 daily attempts
- Server can recompute leaderboard 100x/day without exceeding write budget

## Architecture

### Files to create

#### Server
- `server/internal/matches/types.go` — domain types (~50 LOC)
- `server/internal/matches/create.go` — `CreateAsyncMatch(ctx, creatorUID, gameID)` (~80 LOC)
- `server/internal/matches/join.go` — `JoinByToken(ctx, joinerUID, token)` (~80 LOC)
- `server/internal/matches/result.go` — `ComputeAndPushResult(ctx, matchID)` (~120 LOC)
- `server/internal/matches/leaderboard.go` — `RecomputeLeaderboard(ctx, date, gameID)` (~100 LOC)
- `server/internal/matches/types_test.go`, `create_test.go`, `result_test.go`
- `server/internal/firestore/matches.go` — CRUD: `Get`, `SetCreator`, `SetJoiner`, `SetCompleted`, `FindByShareToken` (~120 LOC)
- `server/internal/firestore/leaderboards.go` — `WriteLeaderboard`, `GetLeaderboard` (~80 LOC)
- `server/internal/ws/handlers/match_create.go` (~60 LOC)
- `server/internal/ws/handlers/match_join.go` (~80 LOC)
- `server/internal/ws/handlers/leaderboard.go` (~50 LOC)

#### Client
- `web/src/components/lobby.tsx` — list "my matches" + "create match" button + "join via token" input (~150 LOC)
- `web/src/components/match-result.tsx` — winner banner, both players' stats (~80 LOC)
- `web/src/components/leaderboard.tsx` — top 100 table (~80 LOC)
- `web/src/components/share-link.tsx` — share token + copy button (~50 LOC)
- `web/src/hooks/use-matches.ts` — subscribes to `/users/{uid}.matches[]` or queries `/matches where (creator_uid==uid OR joiner_uid==uid)` (~80 LOC)
- `web/src/routes/match-route.tsx` — extracts share_token from URL, triggers MATCH_JOIN
- `web/src/ws/client.ts` — extend with typed match helpers

### Files to modify
- `proto/dleague/v1/envelope.proto` — add 7 message types
- `server/internal/ws/hub.go` — register match handlers
- `web/src/App.tsx` — add routes for `/m/{token}` (and `/leaderboard`)
- `server/internal/firestore/attempts.go` — call `ComputeAndPushResult` after both done

### Match-finding query patterns
| Use case | Query | Index needed |
|----------|-------|--------------|
| User's matches list | `where creator_uid==uid OR joiner_uid==uid order by created_at desc` | (Firestore can't OR) → split into 2 queries client-side OR add `participants array<string>` field |
| Find match by share_token | `where share_token==token limit 1` | single-field auto |
| Leaderboard top 100 | `collectionGroup('attempts') where won=true order by attempts_used,duration_ms limit 100` | composite (planned in phase-3) |

**Decision:** Add `participants: [creator_uid, joiner_uid]` array field to `/matches/{id}` doc; query `where participants array-contains uid order by created_at desc`. Single composite index. Phase-3 leaderboard plan unchanged.

## Implementation Steps

### Server
1. Update phase-3 schema: add `participants array<string>` to MatchDoc (TS + Go types) + composite index on `participants array-contains` + `created_at desc`. Re-deploy `firestore.indexes.json`.
2. Add 7 message types to envelope.proto + proto-gen
3. Implement `firestore/matches.go` CRUD using transactions where atomicity matters (joiner binding)
4. Implement `matches/create.go` (token generation w/ collision retry; 8-char base62)
5. Implement `matches/join.go` (transaction: read by token, check status, set joiner+participants+status)
6. Implement `matches/result.go`:
   - load both attempts; compare `(attempts_used, duration_ms)`; pick winner or draw
   - update match.winner_uid + status=completed
   - push MATCH_RESULT to both conns via hub broadcast (best-effort; doc remains canonical)
7. Implement `matches/leaderboard.go`:
   - collection-group query: `attempts.where(won==true).orderBy(attempts_used,duration_ms).limit(100)`
   - join with user docs for display_name (batch get)
   - write `/leaderboards/{date}_{gameId}`
8. Hook into phase-5 attempts.go: after MarkComplete, check if other attempt exists, call ComputeAndPushResult; if kind=='daily', also call RecomputeLeaderboard (rate-limited: at most 1 recomputation per 60s per `(date, game_id)`)
9. WS handlers: match_create.go, match_join.go, leaderboard.go; wire dispatch
10. Tests: result.go winner determination (table-driven: 8 cases incl. ties); leaderboard.go aggregation

### Client
1. Regen protobuf-ts
2. Implement `<Lobby/>` with "Create async match" + "Join via token" + "My matches" list
3. Implement `<MatchResult/>` modal triggered by MATCH_RESULT
4. Implement `<Leaderboard/>` page; sends LEADERBOARD_QUERY; renders table
5. Implement `<ShareLink/>` component (copy-to-clipboard) for after MATCH_CREATED
6. Add route `/m/:token` → on mount, send MATCH_JOIN, on MATCH_JOINED proceed to game
7. `use-matches` hook: queries Firestore directly via JS SDK with `where('participants', 'array-contains', uid)` (rules already allow participant read)
8. Smoke test full flow: create → share link → second device anon-sign-in → paste link → both play → result modal

## Todo List

### Server
- [ ] Add `participants` array to MatchDoc schema (Go + TS types)
- [ ] Update firestore.indexes.json with array-contains composite
- [ ] Add 7 message types to envelope.proto + regen
- [ ] firestore/matches.go (CRUD, transactional joiner)
- [ ] firestore/leaderboards.go
- [ ] matches/create.go + token generation
- [ ] matches/join.go (transactional)
- [ ] matches/result.go + winner logic + tests
- [ ] matches/leaderboard.go + aggregation
- [ ] WS handlers + hub dispatch
- [ ] Hook attempts.MarkComplete → result + leaderboard
- [ ] go build + lint + test

### Client
- [ ] Regen protobuf-ts
- [ ] <Lobby/>, <MatchResult/>, <Leaderboard/>, <ShareLink/>
- [ ] Route `/m/:token`
- [ ] use-matches hook (Firestore client query)
- [ ] Smoke test 2-device flow

## Success Criteria
- [ ] Creator can create async match; receives share token + URL
- [ ] Joiner can paste URL on different device; matches load same puzzle
- [ ] Both attempts independent; results compared after BOTH end
- [ ] MATCH_RESULT pushed to both conns (or read on next reconnect)
- [ ] My-matches list shows past + active matches; uses `participants` index
- [ ] Daily leaderboard shows top 100; refreshes when new daily attempt completes
- [ ] Share token collision impossible at testing scale (8-char base62 = 218T; bound check)
- [ ] No file >200 LOC

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Joiner == creator (self-join) abuse | Med | Low | join.go rejects when joiner_uid == creator_uid |
| Two joiners race on same share_token | Low | Med | Firestore transaction in join.go; first wins, second gets ErrAlreadyJoined |
| MATCH_RESULT not delivered if both clients offline | High | Low | Doc canonical; on next reconnect, client subscribes to `/matches/{id}` and renders MATCH_RESULT from doc state |
| Leaderboard recomputation thundering herd at midnight | Low | Med | Rate-limit per `(date, game_id)` to 1/60s; first request after each completion is cheap |
| Collection-group query for leaderboard scans many docs | Med | Med | At ≤100 attempts/day, 100 reads = 0.2% of budget; OK. Reconsider at 400 DAU |
| Share token leaked publicly | Med | Low | Anyone with token can join, but they only see the puzzle (which is daily public anyway) and their own attempt; no PII exposure |
| `participants` array eventually >2 if we ever extend to N-player | Low | Low | Out of scope at MVP; field is array-contains compatible with future N |
| Client direct Firestore read costs not bounded | Med | Low | use-matches limits to 20; Lobby UI paginated; rules require auth |

## Security Considerations
- Match writes server-mediated only — rules deny client writes
- Client direct READS allowed for participants — rules check `creator_uid` or `joiner_uid` matches `request.auth.uid`
- Adding `participants` array means rules become: `allow read: if request.auth != null && request.auth.uid in resource.data.participants`. Cleaner. Update phase-3 rules accordingly
- Leaderboard exposes display_name + stats; that's the point. No email/private data
- Share token is bearer credential for joining — but join requires auth; can't be used to read match without auth
- Rate-limit MATCH_CREATE per uid (max 10/hour) to prevent doc spam and write quota burn

## Next Steps
- **Unblocks:** phase-7 (sync PvP shares match doc model; only transport differs)
- **Unblocks:** phase-9 (parent phase-04 marked superseded)

## Unresolved Questions
1. Should "open" matches be discoverable in a "find a game" lobby, or only via share-link? MVP: share-link only (simpler)
2. Daily kind: do creator+joiner know each other, or "blind" daily with global leaderboard? MVP: daily = just leaderboard, no pairing
3. Should we expire matches automatically (e.g. cron-like cleanup)? Without Cloud Functions, server-on-demand cleanup runs on `/lobby` open ("expire stale matches"). Defer
4. Leaderboard time-window: today only, or weekly? MVP: today only
5. What if the joiner is anonymous and never returns? Match stays `in_progress` forever; cleanup deferred
