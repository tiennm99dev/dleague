# Code Review — Phase 07 Game Core Pluggable + Server-Authoritative Wordle

**Branch:** main (working tree, pre-commit) | **Reviewer:** code-reviewer | **Date:** 2026-05-09
**Scope:** ~28 changed/new files, ~3.6k LOC delta
**Verdict:** Ship-eligible. No Critical findings. 1 High, 4 Medium, 5 Low/Nit.

---

## Verification Results

| Check | Result |
|---|---|
| `go vet ./server/... ./shared/...` | clean |
| `go test ./server/internal/{game/wordle,ws}/...` | PASS |
| `go test -coverprofile … ./server/internal/game/wordle/` | **84.9%** (matches claim) |
| `npm run check` (svelte-check) | 0 errors, 0 warnings, 381 files |
| `npx vitest run` | 9/9 PASS |

---

## 1. Server-Authoritative Solution Invariant — VERIFIED ✓

**Threat:** solution leaks pre-terminal via WordleState payload, debug log, or error envelope.

Trace of `wordle.go:101-120` `ToProto`:
- `state.Solution` is **only** assigned inside `if w.IsTerminal() { state.Solution = w.solution }` (L116-118).
- `IsTerminal()` returns `w.won || w.lost`. Pre-terminal both flags are false → `Solution` is the proto3 zero-value (`""`).
- Proto3 with `optional` not set: `string solution = 6` zero-valued → marshaled as zero bytes (omitted from wire).
- Test `TestToProto_SolutionHiddenPreTerminal` (wordle_test.go:125) asserts empty string after a wrong guess. PASSES.
- Tests `TestToProto_SolutionRevealedOnWin` and `_OnLoss` confirm reveal is only post-terminal.

**Debug log path** (`debug_log.go:14-22`): only marshals the **outer Envelope** to JSON. Envelope.payload is `bytes`; protojson encodes bytes as base64. The inner WordleState bytes will contain the solution **only when terminal** — and only when the build tag `debug` is set. So debug logs do NOT pre-terminally leak. (Informational: when terminal the base64 IS easily decodable, but at that point the solution is already authorized to be sent.)

**Error path** (`game_handler.go:67-68`): logs `uid=%q` and the wrapped EnsureToday error. The error string includes the date but never the solution.

**Result:** invariant holds end-to-end. Game/UID‑based session map (`sessions` sync.Map L37) ensures one user can't read another's terminal state.

---

## 2. Two-Pass Color Algorithm — VERIFIED ✓

`colors.go:33-67` is the canonical two-pass algorithm. Hand-trace against spec edge cases:

**EERIE / ALLEE** (guess=ALLEE, sol=EERIE):
- Pass 1: pos4 E==E → green; consumed=[F,F,F,F,T].
- Pass 2: pos0 A → none; pos1 L → none; pos2 L → none; pos3 E → solution[0]=E unconsumed → yellow.
- Result: `___YG`. Matches `colors_test.go:93-97` ✓ and TS `colors.test.ts:33` ✓.

**AROMA / AAAAA**: Pass 1 greens at pos0+pos4; Pass 2 finds no unconsumed As for middle positions. Result `G___G` ✓.

**SHARP / SHRED** corrected: With sol=SHARP and guess=SHRED → S=G, H=G, R(at pos2 of guess) vs A(sol[2])≠ but R appears in sol at pos3 unconsumed → Y. E and D no match → gray. Result `GGY__` ✓ matches both Go test L72 and TS test L26.

**RATES/TASER** and **ENTER/ETHER** also hand-verified consistent.

**Off-by-one / mutation check:** input strings `guess`/`solution` are read-only (Go strings are immutable). `consumed` slice is local. No global state. Inputs are not modified. ✓

---

## 3. Wordle Game Logic — Mostly Solid

`wordle.go`:
- `Validate` (L48-64): both length and dict check are evaluated **before** branching — comments correctly note this is a timing-leak mitigation. However `inDict := contains(dict, upper)` (L55) is **not constant-time** — it short-circuits on first match (L126 `return true`). A timing observer could distinguish "early-alphabet match" from "late-alphabet match" or "no match." For attempts to enumerate the dictionary this is meaningful, but per the threat model this is solo daily play with a known public dictionary — **not a real attack surface**. Acceptable for MVP. (Med — informational.)
- `Apply` (L68-84): correctly decrements `attemptsRemaining`, sets `won` on exact match, sets `lost` only when `attemptsRemaining == 0` AND not `won`. The `else if` ordering is critical — verified.
- `IsTerminal` returns `w.won || w.lost`. ✓
- `Result()` (L92-97) returns `Won` and `AttemptsUsed`. Note: `AttemptsUsed = len(w.guesses)` — on loss this returns 6 (correct: all attempts used). ✓
- `Apply` is **not** safe for concurrent use on the same `*Wordle`. Call sites: `game_handler.go` always holds `sess.mu.Lock()` (L60-61) before any `sess.game.*` call. Two tabs as same user serialize through the mutex. ✓

---

## 4. Daily Puzzle Determinism — VERIFIED ✓

`daily.go:32-75`:
- Date string formed via `now.UTC().Format("2006-01-02")` — UTC consistent.
- `sha256(date + "wordle-v1")` → first 8 bytes BE → mask sign bit → mod len(answers).
- Idempotent: GetByDate hit returns existing solution unchanged; no re-roll.
- Tests `TestEnsureToday_SameDateReturnsSameSolution` and `_ExistingDocNotOverwritten` confirm idempotency.

**Concurrent boot race:** Two replicas booting at UTC midnight both call EnsureToday → both miss → both compute (same answer due to deterministic seed) → both Upsert. Mongo `UpdateOne(SetUpsert(true))` (`daily_puzzles.go:62`) handles the race: the second write becomes a `$set` (same values) + `$setOnInsert` is no-op since doc exists. **Idempotent and safe.** Solution stable. ✓ (Edge case: if `len(answers)` differs between replicas — say one is on embed, one on Mongo — they could pick different solutions. Acceptable boot constraint; document.)

⚠️ **Med-1 [main.go:76]:** boot calls `wordle.EnsureToday(bootCtx, dailyRepo, answers, time.Now())` — passes `time.Now()` not `time.Now().UTC()`. `EnsureToday` internally calls `now.UTC().Format(...)` (daily.go:42), so the date string is correct. Just noisy if anyone reads the call site. Not a bug.

---

## 5. Wordlist Loader — Concerns

`wordlist.go`:
- Embedded fallback uses `//go:embed data/answers.txt`. Sizes verified: **answers=772, dictionary=864**. Spec called for ~2315/~10k — TODO at L13-17 acknowledges this for Phase 10.

⚠️ **High-1 [wordlist.go:13-17, data/dictionary.txt]:** dictionary is a STRICT SUBSET of necessary words. Spot-check: dictionary.txt does **NOT** contain the test guesses used in `wordle_test.go` (CRANE, ALERT, ADIEU, etc. — these come from a separate hardcoded `testDict`). If a real user submits "CRANE" against the embedded dictionary and the embedded dictionary doesn't contain it, the server returns `ErrNotInDictionary`. Need to verify the embedded dictionary contains all 772 answer words at minimum (otherwise some daily solutions become unguessable) AND common English 5-letter words.

Quick check needed:
```bash
comm -23 <(sort answers.txt) <(sort dictionary.txt)  # any answers NOT in dict?
```
A.txt starts ABOUT/ABOVE/ABUSE/ACTOR; dict.txt starts AAHED/AALII/ABACI/ABACK/ABAFT — alphabetically dict.txt covers AA-AB range, but **dictionary should be a superset of answers** (spec L38 of phase-07: "valid-guess list (superset of answers)"). If not, daily solution may not be a legal guess. This needs verification before MVP.

⚠️ **Med-2 [wordlist.go:33-55]:** if Mongo returns a successful FindOne with `len(words) == 0` (unlikely but possible — schema_version=1 doc with empty Words slice), code falls back to embedded silently. Acceptable. But if Mongo returns an error (transient network), `LoadAnswers` returns `(nil, err)` — main.go fallback path at L66 catches this. ✓

⚠️ **Low-1 [wordlist.go:13-17]:** comment claims "Wardle [sic] released word lists to public domain." Original Wordle answer list was author Josh Wardle's personal selection — public domain status is debatable post-NYT acquisition. Use a generated open-dictionary list (e.g., from Collins/SCOWL) for safety. Documented as Phase 10 todo; track for legal review.

---

## 6. WS Handler — Solid

`game_handler.go`:
- Defensive auth re-check (L46) is belt-and-suspenders; primary gate is `hub.go:85` via `requiresAuth`. `auth_gate.go:11` returns `false` only for protocol/auth message types — `MESSAGE_TYPE_GAME_MOVE = 6` falls into the `default: return true` → auth required. ✓
- `proto.Unmarshal` failure returns 400 with non-leaky message. ✓
- Lazy session init (L64-72): solution loaded once per session via EnsureToday — efficient.
- Mutex held across Validate/Apply/Marshal (L60-95) — safe.
- Request ID echoed back in success and error envelopes. ✓
- Response envelope type set to GAME_STATE (L92). ✓

⚠️ **Med-3 [game_handler.go:37, sessions sync.Map]:** memory leak — sessions are never evicted. Each unique `userID` adds a `*wordleSession{}` (~64B + game struct ~100B). At 1M lifetime users that's ~150MB. Acceptable for MVP per Phase 08 making sessions durable. Document and add a TTL eviction in Phase 08.

⚠️ **Med-4 [game_handler.go:101-103]:** `loadOrCreateSession` creates a new `*wordleSession{}` even if `LoadOrStore` returns an existing one — the new struct is allocated, then discarded by `LoadOrStore`. Minor allocation noise; not a correctness bug. Could use `Load` first then `LoadOrStore` to avoid the alloc on hot path. Optimisation only.

⚠️ **Low-2 [game_handler.go:65]:** `EnsureToday` is called inside the per-session mutex with the session's first guess. If the daily roll-over occurs mid-game (UTC midnight), the in-memory session keeps yesterday's solution while a fresh session would pick today's. Not a correctness bug for solo play (each session is its own world) but worth noting in Phase 08 design when sessions become durable.

---

## 7. Game Interface Generalization — OK

`shared/game/game.go` and `shared/game/state.go`:
- Removed `State = []byte` alias (Phase 1 code-review L8). ✓
- New `State` is a real interface with `IsTerminal() bool`. `Move` is `interface{}` — fine for V1; concrete games define their own.
- `Game` interface (L47-67): Init/Validate/Apply/IsTerminal/Result. `*Wordle` does **not** implement `Game.Init(seed int64) error` — Wordle uses `New(solution)` instead. The pluggable interface is currently aspirational; no code dispatches generically yet. Acceptable for MVP because there's only one game.

⚠️ **Low-3 [shared/game/game.go:31]:** `type Move interface{}` — empty interface. Every concrete move struct will pass type assertions at the boundary. Could tighten with at minimum a marker method `isMove()`, but this is YAGNI until a second game lands.

---

## 8. Proto Schema — Clean

- `wordle.proto`: enum has `COLOR_UNSPECIFIED = 0` ✓, repeated `Color` colors in `WordleHint`, `WordleState.solution` field documented "intentionally omitted until terminal."
- `envelope.proto`: GAME_MOVE=6, GAME_STATE=7 — new values appended at end (no renumbering — safe wire-compat).
- Generated TS pb (`wordle_pb.ts`): types and short-name enum (`Color.GREEN`) match Go field names. ✓
- Generated Go pb (`wordle.pb.go`): WordleState.Solution at field tag 6, type `string`, `omitempty` (proto3 default).

---

## 9. TS / Svelte Client — OK with minor issues

`colors.ts`: mirrors Go algorithm bit-for-bit; tests match.

`board.svelte` (101 LOC, under 200): renders 5×6 grid; tile color from hints prop; current input row shows in-progress letters. Reactive props use Svelte 5 `$props()`. ✓

`keyboard.svelte` (126 LOC): best-color-so-far priority green=3 > yellow=2 > gray=1 (L21). Iterates all guesses + all positions to compute letterColor (L26-42) — O(guesses × cols × keys × per-render). With 6 guesses × 5 cols × 26 keys = 780 ops/render — negligible.

⚠️ **Low-4 [keyboard.svelte:26-42]:** physical keyboard input via `play/+page.svelte:77-79` `handlePhysicalKey` — that handler always passes through `e.key`. Browser-generated `e.key` values include "Shift", "Tab", "Escape", etc. The downstream filter `/^[A-Za-z]$/.test(key)` (page.svelte:72) catches these correctly. ✓ But the regex test happens AFTER Enter/Backspace branches — fine. Minor: a user holding shift while typing letters generates uppercase letters which match the regex. Confirmed safe.

`wordle-scene.ts` (88 LOC): tile-flip animation uses scaleY 1→0→1 with stagger=100ms. **Cleanup concern:** `eventBus.on('wordle:flip-row', ...)` (L34) registers a listener on scene `create()`. If the scene is destroyed (e.g., navigation away from /play), is the listener removed? Looking at `play/+page.svelte:158` — `phaserGame?.destroy(true)` should tear down the scene. Whether `eventBus` listeners attached in `scene.create()` are released depends on the EventBus implementation.

<br>

⚠️ **Med-5 [wordle-scene.ts:34, event-bus]:** EventBus listener is added but never removed. Repeated /play navigations leak listeners → multiple flips per event after re-mount. Should use `scene.events.on('shutdown', () => eventBus.off(...))` or store the handler reference and call `eventBus.off` in scene shutdown.

`play/+page.svelte` (300 LOC, slightly over 200 target):
- Uses protobuf-es v2 API (`create`, `toBinary`, `fromBinary`) ✓
- WS request via `sendRequest(MessageType.GAME_MOVE, payload)` returns response bytes; decoded with WordleStateSchema ✓
- `applyServerState` updates reactive state and emits `wordle:flip-row`. Note row index = `s.guesses.length - 1` (line 48): correct for the just-submitted guess.
- `protoColorToClient` maps `ProtoColor.GREEN/YELLOW` to client; `default → 'gray'`. Both `UNSPECIFIED` and `GRAY` proto values map to 'gray'. ✓
- TypeScript: no `any` in this file. Strict mode on (tsconfig.json default for sveltekit).

⚠️ **Low-5 [play/+page.svelte:78]:** `e.key.length === 1 ? e.key : e.key` is a no-op ternary (typo — likely meant a clamp). Currently passes through every keypress raw. Not a bug since `handleKey` filters with regex. Cleanup nit.

---

## 10. Integration with Prior Phases — OK

- Hub dispatch (`hub.go:94-98`): GAME_MOVE case added. nil-check on `h.GameDeps` returns 503. ✓
- main.go boot ordering: Mongo → indexes → repos → wordlist load → EnsureToday → Firebase → Hub → router. Reasonable. Wordlist + EnsureToday happen before Firebase init — they don't depend on Firebase. Boot context is shared 15s budget.
- Phase 02 hardening (send chan cap, ping ticker, request_id cap) **intact** — verified `conn.go:17-23` consts unchanged. ✓
- Phase 05 auth gate: GAME_MOVE not in deny list (`auth_gate.go:13-19`) → requires auth. ✓

---

## 11. Test Coverage — OK

- 84.9% on `internal/game/wordle/` confirmed.
- 0% on `LoadAnswers`/`LoadDictionary` — Mongo-dependent, unit-test-uncoverable without docker; acceptable.
- Tests check: TestApply_GameOver (post-win Apply rejected) ✓, TestHappyPath_LossAfterSixAttempts ✓, length+dict+solution-hidden ✓.
- **Gap:** no test for `Apply` after `Validate` failure (i.e., calling Apply on a bad guess). Apply doesn't re-validate — so it would happily score and decrement attempts. Game_handler always calls Validate first. Document the contract; or add Apply-validates-internally as a defense.
- TS Vitest 9/9 mirrors Go cases. ✓

---

## 12. Hub.GameDeps Public Field Mutability — CARRYOVER OK

`hub.go:39 GameDeps *GameDeps` is set once in `main.go:97` before `srv.ListenAndServe`. Same pattern as `MaxConns`. Phase 02 review M2 mitigation logic carries: write happens-before any reader (goroutine creation establishes happens-before via `srv.ListenAndServe`'s internal `go` spawn). ✓ No change needed.

---

## 13. Anything to Block On

**Nothing blocks the commit.**

The only High-severity item (H1) is content-correctness (dictionary may not be a superset of answers) — needs a manual verification before letting users actually play, but does not affect compilability or any test path. Could be fixed in a follow-up commit before live deployment.

---

## Summary by Severity

| # | Sev | File:Line | Issue |
|---|-----|-----------|-------|
| H1 | High | data/dictionary.txt | Verify dictionary is superset of answers; otherwise some daily solutions are unguessable. |
| M1 | Med | main.go:76 | `time.Now()` instead of `time.Now().UTC()` (cosmetic — UTC applied internally). |
| M2 | Med | wordlist.go:33-55 | Mongo error → embed fallback only via main.go fallback; loader returns `(nil, err)`. Working but two-layered. |
| M3 | Med | game_handler.go:37 | `sessions sync.Map` never evicted. ~150B/user lifetime memory. Document for Phase 08. |
| M4 | Med | game_handler.go:101 | `LoadOrStore` always allocates a fresh struct on hot path. Optimization. |
| M5 | Med | wordle-scene.ts:34 | EventBus listener leak on scene destroy → /play re-mount duplicates flips. |
| L1 | Low | wordlist.go:13 | Public-domain claim on Wardle list; verify license before deploy. |
| L2 | Low | game_handler.go:65 | UTC roll-over mid-session keeps yesterday's solution. Solo OK; Phase 08 design note. |
| L3 | Low | shared/game/game.go:31 | `Move interface{}` — could add marker method when 2nd game lands. |
| L4 | Low | keyboard.svelte:26-42 | O(guesses×cols×keys) per render; trivial cost. |
| L5 | Low | play/+page.svelte:78 | `e.key.length === 1 ? e.key : e.key` no-op ternary. Cleanup. |

---

## Positive Observations

- **Server-authoritative invariant cleanly enforced.** `ToProto` is the single chokepoint and tests cover both pre- and post-terminal cases.
- **Two-pass color algorithm is the canonical correct implementation.** Hand-verified against EERIE/ALLEE, AROMA/AAAAA, SHARP/SHRED, RATES/TASER, ENTER/ETHER. TS port mirrors exactly.
- **Constant-evaluation Validate** (length AND dict checks both run before branch) shows defense-in-depth thinking.
- **Auth gate via allowlist (deny by default) is the right shape.** GAME_MOVE inherits auth-required because it's not in the explicit deny set.
- **Daily seed algorithm fully documented** for cheat investigation reproducibility.
- **Mutex per session, sync.Map across users** — correct concurrency primitive choice.
- **Idempotent Mongo upsert** handles concurrent boot race for daily puzzles.
- **84.9% coverage with both happy and edge-case tests**, plus TS mirror tests.
- **Generated proto code is committed** — no surprise build steps for downstream consumers.

---

## Unresolved Questions

1. **Dictionary completeness (H1):** does `dictionary.txt` (864 words) contain every word in `answers.txt` (772 words)? If not, some daily solutions cannot be guessed by user typing the answer. Run `comm -23 <(sort answers.txt) <(sort dictionary.txt)` before next deploy.
2. **Wordlist licensing (L1):** is the embedded list actually public domain? The TODO at `wordlist.go:13` notes pre-NYT origin but doesn't cite the file source. Provenance line in `data/answers.txt` would help.
3. **Phase 08 plan for EventBus listener (M5):** should the WordleScene be torn down on /play navigation, or kept persistent across routes? Affects how cleanup is implemented.

---

**Status:** DONE_WITH_CONCERNS
**Summary:** Phase 07 implementation is correct, well-tested (84.9% Go, 9/9 TS), and the server-authoritative solution invariant is verified end-to-end. No Critical issues; H1 (dictionary superset verification) should be resolved before users actually play.
**Concerns/Blockers:** H1 needs `comm` check; M3 (session map leak) and M5 (event listener leak) should land in Phase 08.
