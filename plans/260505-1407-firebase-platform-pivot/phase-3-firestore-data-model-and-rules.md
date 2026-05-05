# Phase 3: Firestore data model + security rules

## Context Links
- Research: `plans/reports/researcher-260505-1407-firebase-as-backend-feasibility.md` (data model section, rules section)
- Source-of-truth schema (deprecating): `server/internal/store/migrations/0001_init.sql:6-72`
- Locked: UID = PK; server-mediated writes only; aggregation in Go (no Cloud Functions)

## Overview
- **Priority:** P1 (blocks phase-5/6/7)
- **Status:** pending
- **Effort:** 2d (design + rules + index plan; no client/server code)
- Maps every MySQL table from `0001_init.sql` to Firestore collections + subcollections. Defines indexes, denormalization, and Security Rules. Output is `firestore.rules` + `firestore.indexes.json` + this design doc.

## Key Insights
- Firestore charges 1 read per doc returned; subcollection queries do NOT include parent reads
- Denormalize aggressively: leaderboard top-N is a SINGLE doc (`/leaderboards/today`), updated by server, NOT a query against `/attempts`
- `users.id` (BINARY(16) UUIDv7) → Firebase UID (string, ~28 chars)
- `sessions` table → DELETED entirely (Firebase ID tokens replace)
- Composite index per query pattern; Firestore auto-builds single-field indexes; project limit = 200 composites (we'll have <10)
- Security rules: `request.auth.uid` only set after Firebase Auth client signs in; Admin SDK bypasses rules entirely
- Rules cannot do "exists in another collection" cheaply — embed `creator_uid` + `joiner_uid` ON the parent match doc to avoid extra `get()` in rules

## Requirements

### Functional
1. Document every collection path, doc shape, and field type
2. Map each MySQL table to a Firestore equivalent (or explicit deletion)
3. Define composite indexes for: leaderboard query, user match history, daily-puzzle resolution
4. Write `firestore.rules` enforcing server-mediated-write trust model
5. Write `firestore.indexes.json` for `firebase deploy --only firestore:indexes`
6. Document TTL/archive strategy (matches >7d move from Firestore... or just stay; testing scale = trivial volume)

### Non-functional
- Schema fits in 1 GiB total at 100 DAU (per research: ~500 MB headroom)
- Hot read paths (puzzle, user profile) cached client-side TTL=24h
- Read budget per user/day ≤3 ops (puzzle + profile + leaderboard)

## Architecture

### Collection map

| Path | Doc shape | Owner | Read | Write |
|------|-----------|-------|------|-------|
| `/users/{uid}` | UserDoc | Server | self only | server only |
| `/puzzles/{date}_{gameId}` | PuzzleDoc | Server | any auth | server only |
| `/matches/{matchId}` | MatchDoc | Server | participants only | server only |
| `/matches/{matchId}/attempts/{uid}` | AttemptDoc | Server | match participants | server only |
| `/leaderboards/{date}_{gameId}` | LeaderboardDoc | Server (aggregated nightly) | any auth | server only |
| `/_meta/health` | `{ ok: true, ts: ServerTimestamp }` | Server | any auth | server only |

### Doc shapes (TypeScript-style)

```ts
// /users/{uid}
type UserDoc = {
  uid: string;                  // == doc id; redundant for query
  display_name: string;
  is_anonymous: boolean;
  email: string | null;         // null for anonymous
  photo_url: string | null;
  created_at: Timestamp;
  last_seen_at: Timestamp;
  // Aggregations (denormalized; updated by server post-match)
  matches_played: number;
  matches_won: number;
};

// /puzzles/{date}_{gameId}     e.g. "2026-05-05_wordle"
type PuzzleDoc = {
  puzzle_date: string;          // ISO date "2026-05-05"
  game_id: string;              // "wordle"
  seed: number;                 // PRNG seed for reproducibility
  answer_hash: string;          // hex of SHA-256(answer); answer NEVER in this doc (server-side only)
  word_length: number;          // 5 for Wordle
  max_attempts: number;         // 6 for Wordle
  created_at: Timestamp;
};

// /matches/{matchId}
type MatchDoc = {
  match_id: string;
  kind: 'async' | 'sync' | 'daily';
  game_id: string;
  puzzle_date: string;
  creator_uid: string;          // denorm for rules; cheaper than get()
  joiner_uid: string | null;
  status: 'open' | 'in_progress' | 'completed' | 'expired';
  created_at: Timestamp;
  completed_at: Timestamp | null;
  // Result summary (filled when both attempts done)
  winner_uid: string | null;
  share_token: string | null;   // short opaque code for invite link
};

// /matches/{matchId}/attempts/{uid}
type AttemptDoc = {
  match_id: string;
  uid: string;
  attempts_used: number;        // 1..max_attempts
  duration_ms: number;
  won: boolean;
  guesses: string[];            // ["CRANE","SLATE",...]; verified by server
  feedback: string[];           // ["BBYGG", ...] color codes per guess
  finished_at: Timestamp;
};

// /leaderboards/{date}_{gameId}
type LeaderboardDoc = {
  date: string;
  game_id: string;
  // Top 100 entries; pre-sorted by (won DESC, attempts ASC, duration ASC)
  entries: Array<{
    uid: string;
    display_name: string;       // denorm; if user renames, leaderboard stale ≤24h
    attempts_used: number;
    duration_ms: number;
    won: boolean;
  }>;
  updated_at: Timestamp;
};
```

### Field-by-field mapping (MySQL → Firestore)

| MySQL (`0001_init.sql`) | Firestore | Notes |
|-------------------------|-----------|-------|
| `users.id BINARY(16)` | doc id (Firebase UID string) | UUIDv7 dropped |
| `users.email` | `users/{uid}.email` | string or null |
| `users.password_hash` | DELETED | Firebase Auth owns credentials |
| `users.display_name` | `users/{uid}.display_name` | unchanged |
| `users.created_at` | `users/{uid}.created_at` | Firestore Timestamp |
| `sessions.*` | DELETED | ID tokens replace |
| `puzzles.puzzle_date,game_id` (PK) | `puzzles/{date}_{gameId}` doc id | composite key joined |
| `puzzles.seed` | `puzzles/...seed` | unchanged |
| `puzzles.answer_hash BINARY(32)` | `puzzles/...answer_hash` | hex string |
| `matches.id BINARY(16)` | doc id (random 20-char string from `crypto/rand`) | URL-safer |
| `matches.kind ENUM` | `matches/...kind` | string union |
| `matches.creator_id`/`joiner_id` | `matches/...creator_uid`/`joiner_uid` | Firebase UIDs |
| `matches.status` | `matches/...status` | string union |
| `attempts.id` | `attempts/{uid}` doc id (UID, not random) | enforces "1 attempt per user per match" via doc-id uniqueness |
| `attempts.attempts_used` | unchanged | |
| `attempts.duration_ms` | unchanged | |
| `attempts.won` | unchanged | |
| `attempts.state JSON` | `attempts/...guesses[] + feedback[]` | flattened into doc fields |
| (NEW) leaderboards | `/leaderboards/{date}_{gameId}` | server-aggregated, replaces SQL `ORDER BY won, attempts_used` query |

### Composite indexes

| Index | Fields | Used by |
|-------|--------|---------|
| `matches` (collection) | `creator_uid` ASC, `created_at` DESC | "my matches" list (phase-6) |
| `matches` (collection) | `status` ASC, `kind` ASC, `created_at` DESC | "open async games" list (phase-6) |
| `attempts` (collection group) | `uid` ASC, `won` DESC, `finished_at` DESC | user history across matches |

Single-field indexes auto-built by Firestore — no config needed.

### Security rules

```
rules_version = '2';
service cloud.firestore {
  match /databases/{database}/documents {

    // Users: read self only; writes server-mediated only
    match /users/{uid} {
      allow read: if request.auth != null && request.auth.uid == uid;
      allow write: if false;
    }

    // Puzzles: any authenticated user can read; writes server-mediated only
    match /puzzles/{puzzleId} {
      allow read: if request.auth != null;
      allow write: if false;
    }

    // Matches: read if participant; writes server-mediated only
    match /matches/{matchId} {
      allow read: if request.auth != null
                  && (resource.data.creator_uid == request.auth.uid
                      || resource.data.joiner_uid == request.auth.uid);
      allow write: if false;

      // Attempts subcollection inherits match read scope
      match /attempts/{uid} {
        allow read: if request.auth != null
                    && (get(/databases/$(database)/documents/matches/$(matchId)).data.creator_uid == request.auth.uid
                        || get(/databases/$(database)/documents/matches/$(matchId)).data.joiner_uid == request.auth.uid);
        allow write: if false;
      }
    }

    // Leaderboards: any authenticated user reads
    match /leaderboards/{lbId} {
      allow read: if request.auth != null;
      allow write: if false;
    }

    // Health probe: any auth
    match /_meta/{doc} {
      allow read: if request.auth != null;
      allow write: if false;
    }
  }
}
```

**Note on `get()` cost:** Each `get()` in rules counts as 1 read. The match-id-bound subcollection rule does 1 extra read per attempt-doc access. Acceptable at testing scale (~100 attempt reads/day = 100 extra reads, well under 50k budget). At 400+ DAU, denormalize `creator_uid + joiner_uid` ONTO the attempt doc to remove the `get()` (already in our doc shape — the `match_id` field; we'd add UIDs).

## Related Code Files
- **Create:** `firestore.rules` (project root) — 50 LOC
- **Create:** `firestore.indexes.json` (project root) — 30 LOC
- **Create:** `firebase.json` (project root) — Firebase CLI config:
  ```json
  {
    "firestore": { "rules": "firestore.rules", "indexes": "firestore.indexes.json" },
    "hosting": { "public": "web/dist", "ignore": ["firebase.json"] }
  }
  ```
- **Update:** `docs/system-architecture.md` — add Firestore schema diagram (phase-9)

## Implementation Steps
1. Create `firebase.json` with firestore + hosting refs
2. Create `firestore.rules` per spec above; lint via `firebase emulators:start --only firestore`
3. Create `firestore.indexes.json` with the 3 composite indexes above
4. Document doc shapes in this file (DONE — see Architecture)
5. Write a TypeScript `web/src/types/firestore-docs.ts` (consumed in phase-4) that mirrors doc shapes — single source of truth for client + (manually-mirrored) server Go structs
6. Mirror the same shapes in Go: `server/internal/firestore/types.go` with `UserDoc`, `MatchDoc`, etc. structs annotated with `firestore:"..."` tags
7. Write `server/internal/firestore/seed.go` — function `SeedHealthDoc(ctx)` writes `/_meta/health` doc on boot for `/health` ping
8. Deploy rules to dev project: `firebase deploy --only firestore:rules`
9. Deploy indexes: `firebase deploy --only firestore:indexes` (async; takes minutes)
10. Smoke test from Firebase emulator: anon user reads `/puzzles/2026-05-05_wordle` (allowed), tries to write (denied)

## Todo List
- [ ] Create `firebase.json`
- [ ] Create `firestore.rules`
- [ ] Create `firestore.indexes.json`
- [ ] Create `web/src/types/firestore-docs.ts` (TS mirror)
- [ ] Create `server/internal/firestore/types.go` (Go mirror)
- [ ] Create `server/internal/firestore/seed.go` (health doc seeder)
- [ ] Run `firebase emulators:start --only firestore` for local rule lint
- [ ] Deploy rules + indexes to dev Firebase project
- [ ] Smoke test rules: anon read, denied write

## Success Criteria
- [ ] `firestore.rules` deploys without syntax errors
- [ ] All 3 composite indexes show "Enabled" in Firebase console after deploy
- [ ] Anon user can read `/puzzles/*` and `/_meta/health`; cannot read other user's `/users/*`; cannot write anything
- [ ] Admin SDK (server) can write any path (rules bypassed)
- [ ] TS types compile in `web/src/`; Go types compile in `server/internal/firestore/`
- [ ] Both type definitions stay in sync (manual review checklist in phase-9)

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| TS + Go shape drift | High | Med | Phase-9 cross-check; Codegen tool deferred (YAGNI); one source of truth = this doc |
| Index deploy takes 10–20min on first deploy; tests fail meanwhile | Med | Low | Deploy indexes BEFORE phase-6 starts |
| `get()` in rules eats read quota | Low | Low | <10% of budget at testing scale; refactor to denormalized UIDs at 400+ DAU |
| Doc id collision on `attempts/{uid}` if user retries | Low | Med | Server-side: refuse second write via `Set(ctx, doc, firestore.MergeAll)` only after status check |
| 1 GiB storage cap reached unexpectedly | Low | Med | Phase-8 monitoring; 7-day match cleanup if growth surprises |
| Match share_token collision | Very low | Low | 8-char base62 = ~218T combos; bound check before issuing |

## Security Considerations
- Rules deny ALL client writes — no path can be exploited to forge attempts or matches
- Server is sole mutation authority via Admin SDK
- `answer_hash` (not plain answer) stored client-readable in `/puzzles` so client can VERIFY a guess match without server, but cannot REVERSE the hash to discover the answer (preimage resistance of SHA-256)
- Wait — security flaw: SHA-256 of a 5-letter Wordle answer is brute-forceable (~12M combos). DO NOT trust client `answer_hash` as the only proof. Server holds plain answer in private collection `/server_only/answers/{date}_{gameId}` (rules: deny ALL client read+write); guess validation runs server-side only
- Update `PuzzleDoc.answer_hash` semantics: it's a tripwire for cache-poisoning detection, NOT a security boundary
- Anonymous accounts CAN play but have empty email; rule `request.auth.uid != null` covers them

## Next Steps
- **Unblocks:** phase-5 (game handlers write to `/puzzles` + read from `/server_only/answers`)
- **Unblocks:** phase-6 (async PvP creates `/matches/*`)
- **Phase-9:** add to `docs/system-architecture.md`

## Unresolved Questions
1. Schema for `/server_only/answers/{date}_{gameId}` — is `{ answer: "CRANE" }` enough, or do we need per-puzzle metadata (difficulty, theme)? Defer to phase-5 game logic
2. Match TTL: do we delete `/matches` >30d old, or keep forever (storage cheap until 1 GiB)? Recommend: keep until 800 MiB threshold, then archive/delete oldest 50%
3. Leaderboard scope: today only, weekly, all-time? Drives more `/leaderboards/*` doc shapes — defer to phase-6
4. Should we use a Firestore collection group query for "all user attempts ever" or denormalize attempts to `/users/{uid}/recent_attempts` subcollection? Defer; depends on history-page UX in phase-5
