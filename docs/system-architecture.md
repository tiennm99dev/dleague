# System Architecture

**Status:** skeleton — diagrams + ERD landed by Phase 10.

## High-level

```
┌──────────────────────────┐         WebSocket           ┌──────────────────────────┐
│   Browser                │   binary protobuf envelope  │   Go server (Fly.io)     │
│ ┌──────────────────────┐ │ ◄───────────────────────►   │ ┌──────────────────────┐ │
│ │ SvelteKit (static)   │ │                             │ │ chi router           │ │
│ │   + Phaser canvas    │ │                             │ │   /health, /, /ws    │ │
│ │   + protobuf-es      │ │                             │ │ ws hub + dispatch    │ │
│ │   + firebase JS SDK  │ │ ─── Sec-WebSocket-Protocol ─┤ │ Firebase verifier    │ │
│ └──────────────────────┘ │     fb.<id_token>           │ │ Mongo repos          │ │
└──────────────────────────┘                             │ └──────────────────────┘ │
            │                                            └──────────────────────────┘
            │ ID token verify                                    │
            ▼                                                    ▼
   ┌──────────────────┐                              ┌──────────────────────┐
   │ Firebase Auth    │                              │ MongoDB Atlas (M0)   │
   │ (Google managed) │                              │ replica set, TLS     │
   └──────────────────┘                              └──────────────────────┘
```

TODO Phase 10: Mermaid version + deployment topology.

## Components

### Client (SvelteKit + Phaser)

**Stack:** SvelteKit 2 + adapter-static → `web/dist/` (single-page app); Phaser 3.88 canvas component; `@bufbuild/protobuf` v2 for binary protobuf encode/decode; Firebase JS SDK v11 for auth.

**Source layout:**
```
web/src/
├── routes/
│   ├── +layout.ts          — ssr:false, prerender:true (SPA mode)
│   ├── +layout.svelte      — Firebase init + auth gate: shows SignIn or <slot/>
│   └── +page.svelte        — mounts <PhaserGame/>; listens for title:start → goto('/play')
└── lib/
    ├── firebase.ts         — initializeApp, connectAuthEmulator (DEV), sign-in helpers
    ├── auth-store.ts       — writable<User|null>, onAuthStateChanged subscription, idToken()
    ├── ws.ts               — WS client (see below)
    ├── pb/dleague/v1/      — generated TS protobuf (committed; from buf generate)
    ├── phaser/
    │   ├── event-bus.ts    — typed Map<string,Set<Handler>> pub/sub (~30 LOC, no mitt)
    │   ├── phaser-game.svelte — Phaser.Game lifecycle (onMount create, onDestroy destroy)
    │   └── scenes/
    │       └── title-scene.ts  — "DLEAGUE" title + Start button → eventBus.emit('title:start')
    ├── game/
    │   ├── game.ts                 — pluggable Game<S,M> interface + State/Move/Result types
    │   └── wordle/
    │       ├── colors.ts           — two-pass color scoring (optimistic preview only)
    │       ├── colors.test.ts      — Vitest: 9 canonical edge cases
    │       └── wordle.ts           — WordleGame client preview (server is authoritative)
    └── components/
        ├── sign-in.svelte          — email/password form + Google popup + anonymous
        ├── connection-status.svelte — top-right badge (green/yellow/red) from connectionState store
        ├── board.svelte            — 5×6 Wordle grid (props: guesses, hints, currentInput)
        └── keyboard.svelte         — on-screen QWERTY; tracks best color per letter
```

**WS client (`web/src/lib/ws.ts`):**
- Native `WebSocket('/ws', ['dleague.v1', 'fb.<idToken>'])`, `binaryType='arraybuffer'`
- Request/response correlation: `Map<requestId, PendingRequest>` + `crypto.randomUUID()`
- Exponential-backoff reconnect: base 1s, max 30s, max 10 attempts
- Token refresh at 50 min: sends `MESSAGE_TYPE_AUTH_REFRESH{id_token}` (Phase 05 contract)
- Reactive `connectionState` Svelte store → `ConnectionStatus` badge

**Build pipeline:**
- Dev: `make web-dev` → Vite on `:5173`; proxies `/ws` (ws:true) and `/health` to `:8080`
- Prod: `make web-build` → `web/dist/`; Go FileServer serves it at `/` with SPA fallback
- Proto: `make proto-gen` (`buf generate`) emits both Go (`shared/pb/`) and TS (`web/src/lib/pb/`)

**Bundle sizes (measured):**
- Phaser chunk: 1,485 KB min / **341 KB gzip** (full Phaser 3.88 — over 400 KB budget)
- Firebase chunk: ~177 KB min / **38 KB gzip**
- Total gzip across all chunks: ~411 KB — slightly over 400 KB spec target
- Mitigation (deferred): use Phaser custom build (audio/physics stripped) to reclaim ~100 KB gzip

**SPA fallback (server-side):** `server/internal/http/spa_fallback.go` wraps the Go FileServer. Any GET for a non-existent path that is not `/ws`, `/health`, or a static asset extension (`.js`, `.css`, `.png`, etc.) returns `web/dist/index.html` with 200 so SvelteKit client-side routing handles it.

**Security:** CSP `wasm-unsafe-eval` removed (no WASM). Firebase web config (`apiKey` etc.) is public and committed (`firebase.config.json`); restrict by Auth Domain in Firebase console. ID tokens live in JS memory only — never `localStorage`.

**Reference:** `plans/reports/researcher-260508-2300-svelte-phaser-protobuf-client.md`

### Server (Go)
TODO Phase 02–05.

### Persistence (MongoDB Atlas M0)

Driver: `go.mongodb.org/mongo-driver/v2` (v2.6.0+). One `*store.Client` per process; pool max 100. Atlas TLS is implicit via `mongodb+srv://`. Transactions are supported on M0's 3-node replica set.

#### Collections

| Collection | `_id` type | Purpose |
|---|---|---|
| `users` | Firebase UID string | Player profile + embedded stats |
| `games` | slug string ("wordle") | Game-type registry |
| `matches` | ObjectID | One PvP or solo match instance |
| `attempts` | ObjectID | Per-player guess log within a match |
| `daily_puzzles` | "YYYY-MM-DD" string | Daily puzzle seed + solution (server-only) + solution_hash |
| `leaderboards` | "{game}_{period}_{date}" string | Pre-computed ranking snapshots |
| `wordlists` | "wordle_answers" / "wordle_dictionary" string | Word lists; fallback to embedded if empty |

All documents carry `schema_version: 1` for lazy in-place migration (Option A).

#### Indexes (8 explicit, created by `store.EnsureIndexes` at boot)

| Collection | Keys | Options |
|---|---|---|
| `users` | `display_name ASC` | unique |
| `matches` | `players ASC` | — |
| `matches` | `created_at DESC` | — |
| `matches` | `state ASC, created_at DESC` | — (ESR compound) |
| `attempts` | `match_id ASC` | — |
| `attempts` | `match_id ASC, player_uid ASC` | — |
| `daily_puzzles` | `_id DESC` | — |
| `leaderboards` | `game_id ASC, period_end DESC` | — |

`EnsureIndexes` is idempotent — re-runnable on every boot without error.

### Auth (Firebase Auth)

Firebase Auth is the sole identity provider. The Go server verifies Firebase ID tokens using the Admin SDK (`firebase.google.com/go/v4/auth`). No session cookies; no server-side sessions.

#### WebSocket upgrade flow (Pattern A)

```
Client                                          Server (UpgradeHandler)
──────                                          ───────────────────────
firebase JS SDK: user.getIdToken() → idToken
WS open: ws://.../ws
  Sec-WebSocket-Protocol: dleague.v1, fb.<idToken>
                                    ──────────────►
                                                  1. extractFirebaseToken(header)
                                                     → reject 401 if missing/malformed
                                                  2. verifier.VerifyIDToken(ctx, idToken)
                                                     → reject 401 on exp/sig failure
                                                  3. conn.userID = token.UID
                                                     conn.isAnonymous (sign_in_provider)
                                                     conn.tokenExpiresAt
                                                  4. userRepo.UpsertByUID(uid, profile)
                                                     (idempotent Mongo upsert — non-fatal)
                                                  5. websocket.Accept → 101 Switching
```

#### Auth gate (every dispatch)

```go
if requiresAuth(env.GetType()) && c.userID == "" {
    return errorEnvelope(req_id, 401, "unauthenticated"), nil
}
```

Messages exempt from auth: `UNSPECIFIED`, `PING`, `PONG`, `AUTH_REFRESH`, `AUTH_REFRESH_ACK`, `ERROR`. All future game/match message types require auth by default.

#### Token refresh (at ~50 min)

Firebase ID tokens expire after 1 hour. Clients send `AuthRefresh{id_token}` before expiry:

```
Client                                          Server (hub.dispatch)
──────                                          ─────────────────────
AuthRefresh{id_token: newToken}  ─────────────►
                                                verifier.VerifyIDToken(newToken)
                                                → on error: ERROR{401} + close conn
                                                → on ok: update conn.userID,
                                                         conn.tokenExpiresAt
                                                AuthRefreshAck{expires_at_unix}  ◄──
```

#### Anonymous users

Firebase Anonymous Auth issues a UID with `firebase.sign_in_provider == "anonymous"`. The server sets `conn.isAnonymous = true` and creates a Mongo user doc with no email/displayName. Anonymous users are excluded from leaderboards (Phase 08).

#### Credential management

| Environment | Method |
|---|---|
| Local dev | `FIREBASE_AUTH_EMULATOR_HOST=127.0.0.1:9099` (no creds needed) |
| CI | Same as local dev; emulator started in GH Actions before tests |
| Production (Fly.io) | Workload Identity via ambient OIDC; no JSON file |
| Dev/staging (non-Fly) | `GOOGLE_APPLICATION_CREDENTIALS=/path/to/serviceAccount.json` |

Service-account JSON files are `.gitignore`d (`serviceAccount*.json`). Never committed.

Revocation check (`VerifyIDTokenAndCheckRevoked`) deferred to Phase 10.

## Game flow (Wordle — Phase 07)

```
Client (SvelteKit)                             Server (Go)
──────────────────                             ───────────
user types "CRANE" + Enter
  ws.sendRequest(GAME_MOVE,
    WordleMove{guess:"CRANE"})  ─────────────►
                                               hub.dispatch → handleGameMove:
                                               1. auth gate (userID required)
                                               2. unmarshal WordleMove
                                               3. load/create wordleSession (sync.Map)
                                               4. lazy EnsureToday → solution (never sent pre-terminal)
                                               5. wordle.Validate(guess, dictionary)
                                                  → ERROR{400} on length/dict fail
                                               6. wordle.Apply(guess)
                                                  → two-pass color Score(guess,solution)
                                               7. marshal WordleState{guesses,hints,attemptsRemaining,won,lost}
                                                  solution field: EMPTY until IsTerminal()
  ◄─────────────────────────────────────────  GAME_STATE{WordleState}
applyServerState():
  update guesses/hints/attemptsRemaining
  eventBus.emit('wordle:flip-row', {row,colors})
  WordleScene.flipRow() → Phaser Y-rotation tween
  if won/lost → results screen
```

### Daily puzzle seeding

```
boot / make seed-wordlists
  wordle.EnsureToday(ctx, dailyRepo, answers, time.Now()):
    date = UTC "YYYY-MM-DD"
    dailyRepo.GetByDate(date)
    if exists → return solution
    seed = int64(sha256(date+"wordle-v1")[:8]) & 0x7FFF...
    solution = answers[seed % len(answers)]
    dailyRepo.Upsert({id:date, seed, solution, solution_hash:sha256(solution)})
    return solution
```

### Wordlist loading

Server startup: `wordle.LoadAnswers(ctx, wordlistRepo)` → Mongo `wordlists` collection first; if empty falls back to embedded `data/answers.txt` (772 words placeholder; Phase 10 replaces with full 2315-word public-domain list).

### Server-authoritative trust guarantee

- `WordleState.solution` is an empty string in all pre-terminal responses.
- Only when `won == true` or `lost == true` does the server set `solution`.
- Tests: `TestToProto_SolutionHiddenPreTerminal`, `TestToProto_SolutionRevealedOnWin/Loss`.

## Wire format
- **Envelope:** see `proto/dleague/v1/envelope.proto` (single oneof payload + request_id correlation).
- **Auth:** ID token piped via `Sec-WebSocket-Protocol: dleague.v1, fb.<id_token>` at upgrade.
- **Refresh:** client sends `AuthRefresh{id_token}` ~50 min into a connection.
- **Errors:** server emits `MESSAGE_TYPE_ERROR` envelope on malformed input — does NOT close the connection.
- **Game move:** `MESSAGE_TYPE_GAME_MOVE` (6) carries `WordleMove{guess}`.
- **Game state:** `MESSAGE_TYPE_GAME_STATE` (7) carries `WordleState{guesses,hints,attemptsRemaining,won,lost,solution?}`.

## Sync PvP flow (Phase 09)

### Matchmaking
```
client A  →  QUEUE_JOIN(wordle)  →  server pushes conn onto Queue
client B  →  QUEUE_JOIN(wordle)  →  Queue.PopPair() returns (A, B)
                                   MatchRepo.CreateSync(A.uid, B.uid, seed, gameID)
                                   RoomsRegistry.Add(matchID, Room{A, B, wordle1, wordle2})
client A  ←  QUEUE_MATCHED{matchID, seed, opponentName=B.name}
client B  ←  QUEUE_MATCHED{matchID, seed, opponentName=A.name}
```

### Live race
```
client A  →  MATCH_MOVE{matchID, guess="CRANE"}
              Room.HandleMove(A, "CRANE"):
                wordle[A].Apply("CRANE") → hint colors
                A.send ← GAME_STATE{guesses, hints, won, lost}         (own full state)
                B.send ← MATCH_OPPONENT_PROGRESS{attemptNum, colors}   (colors only, NO letters)
                if terminal → CompleteSync + broadcast MATCH_RESOLVED
```

### Letters-never-leak invariant
- `MatchOpponentProgress` proto has no string field for guesses.
- Server only reads `w.hints[last].colors` — the guess word is never placed in this message.
- Verified by `TestMatchRoom_LettersNeverLeakToOpponent` (byte-level scan for "CRANE" in wire payload).

### Disconnect grace
```
Conn close while activeMatchID != "":
  GraceTimers.Schedule(conn, deps)   → 30s timer
  on timer fire  → Room.HandleForfeit(loserUID) → MATCH_RESOLVED{winnerUID, reason="forfeit"}
  on MATCH_REJOIN within 30s → GraceTimers.Cancel → rebind conn → MATCH_REJOIN_ACK
```

### Match timeout (5 min hard cap)
- `main.go` spawns a 1s ticker iterating `RoomsRegistry.All()`.
- If `room.Deadline < now && !room.resolved` → `room.HandleTimeout(deps)`.
- Both players receive `MATCH_RESOLVED{winnerUID="", reason="timeout"}` when neither solved.

### Per-conn rate limiting
- Token bucket: 10 tokens burst, refill 10/sec.
- Gate at top of `Conn.handleFrame` before any proto parsing.
- On denial: enqueue `ERROR{429, "rate limit exceeded"}`; frame dropped; conn stays open.

### Atomic match-end
- `MatchRepo.CompleteSync` opens a Mongo session and calls `session.WithTransaction`:
  1. `matches.UpdateOne` → state="complete", winner_uid, completed_at, reason.
- `UserRepo.IncrementStats` (win/loss counters) called best-effort outside the transaction.
- Anonymous users are skipped by `IncrementStats` (filter `is_anonymous: {$ne: true}`).

### New envelope types (Phase 09)
| Value | Name                         | Direction       |
|-------|------------------------------|-----------------|
| 16    | QUEUE_JOIN                   | client → server |
| 17    | QUEUE_LEAVE                  | client → server |
| 18    | QUEUE_MATCHED                | server → client |
| 19    | MATCH_MOVE                   | client → server |
| 20    | MATCH_OPPONENT_PROGRESS      | server → client |
| 21    | MATCH_RESOLVED               | server → client |
| 22    | MATCH_REJOIN                 | client → server |
| 23    | MATCH_REJOIN_ACK             | server → client |

## Concurrency model
- One goroutine per WS connection for reads.
- One writer goroutine per connection drains a bounded `send` channel (Phase 02).
- Hub fans out broadcasts to per-conn channels with non-blocking sends (drop on slow client).
- Sync match rooms: `sync.Mutex` per room; single lock protects `HandleMove`, `HandleForfeit`, `HandleTimeout` and the `resolved` flag.
- `Queue`: `sync.Mutex`; `Push`/`PopPair`/`Remove` atomic.
- `RoomsRegistry`: `sync.RWMutex`; reads parallel, writes serialised.
- `GraceTimers`: `sync.Mutex`; `time.AfterFunc` goroutines hold no locks when firing.

## Failure domains
TODO Phase 10. Atlas pause, Firebase outage, Fly region failure, etc.

## Security boundaries
TODO Phase 10. WS origin allowlist, conn cap, request_id length cap, security headers, etc.

## Observability
TODO Phase 10. Structured logs, metrics, tracing (if any).
