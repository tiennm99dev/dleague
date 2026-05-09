# Phase 03 — UX correctness diff review

## Verdict
**APPROVE_WITH_FIXES** — core UX goals met; svelte-check clean; one user-visible copy bug (exhausted → "tie"), one dead store (`inMatch`), and small spec drifts on a11y. None blocking; fix as follow-ups.

## Spec compliance

| Step | Action | Status | Note |
|------|--------|--------|------|
| 1 | Create `match-state-flag.ts` | Done | 4 lines, writable bool. |
| 2 | sync-game-scene set inMatch on mount/resolve/destroy | Done | L95, L138, L152. |
| 3 | Layout rejoin gate using `!inMatch` + landingRoutes | Partial | `inMatch` NEVER read; gate uses `landingRoutes` only. Functionally equivalent, but flag is dead code. See I1. |
| 4 | Board role=region + aria changes | Partial | `role=region` ✓, `aria-hidden` on empty ✓, BUT filled cells use `aria-label={letter}` only (no row-summary), and `aria-live="polite"` is on whole board (spec said hidden status div with row summary). See I3. |
| 5 | Keyboard tabindex=-1 + onpointerdown preventDefault | Done | L68-69. |
| 6 | anonymous-warning component + mount points | Done | inline in sign-in L108; sticky banner in layout L97-99 gated to `/play`, `/sync`, `/quick-match`. /m/* not gated but redirects to /play almost immediately. |
| 7 | Friendly Firebase error mapping | Done | Anti-enumeration honored: wrong-password / user-not-found / invalid-credential collapse to one message. No sign-up flow exists, so email-already-in-use is moot. |
| 8 | Reconnect button on connection-status | Partial | Hidden (not disabled) while connecting per spec; double-click race possible (button visible only when state='disconnected', so click→connect()→state='connecting'→button hides; tight race tolerable). See I4. |
| 9 | results-screen reason prop + variants | Partial | New variants render correctly for forfeit. BUT `exhausted` reason maps to `'tie'` headline ("It's a tie!") — wrong copy, both players actually lost. See I2 (highest sev). |
| 10 | `creating` guard in play/+page.svelte | Done | L57, L127, L251. Passed as part of `submitting` to ResultsScreen. |
| 11 | Pass reason to ResultsScreen | Done | sync-game-scene L169. |

## Issues

### I2 — `exhausted` reason renders "It's a tie!" (Medium, user-visible bug)
**File:** `web/src/lib/components/sync-game-scene.svelte:131-132` + `results-screen.svelte:48,57-58`

Server (`server/internal/ws/match_room.go:200-205`) emits `winner_uid=""` only for `reason=="exhausted"` (both players ran out of guesses). True ties (both solved) set winner to player 0 with reason `"solved"`, so `winner_uid` is non-empty. Therefore the client guard `if (!msg.winnerUid) resultReason = 'tie'` is fired only on **exhaustion**, never on a real tie — but ResultsScreen shows "It's a tie!" headline.

Also: `reason=="timeout"` with both timeouts simultaneously could likewise produce empty winner → also mis-rendered as "tie".

**Fix:** Either add a `'both-lost'` reason variant or map empty-winner to a "Better luck — both ran out" copy. Don't reuse `'tie'`.

```ts
} else if (!msg.winnerUid) {
    resultReason = 'loss'; // both lost on exhaustion/timeout
}
```
Then drop `isTie` branch in results-screen, or keep it for a future genuine-tie code path.

### I1 — `inMatch` flag is set but never read (Low, dead code / YAGNI)
**Files:** `web/src/lib/match-state-flag.ts`, `web/src/lib/components/sync-game-scene.svelte:95,138,152`, `web/src/routes/+layout.svelte`

Phase 03 step 3 said the layout rejoin gate should check `!get(inMatch)` before navigating. The implemented gate uses `landingRoutes.includes(currentPath)` instead — which works correctly because `/sync` is excluded from landingRoutes. But `inMatch` is now an unused export.

**Fix (pick one):**
- Wire it up: `if (landingRoutes.includes(currentPath) && !get(inMatch)) { goto(...) }` (defense in depth).
- OR delete `match-state-flag.ts` and the three sync-game-scene set calls.

### I3 — Board a11y drifts from spec (Low)
**File:** `web/src/lib/components/board.svelte:36-46`

Spec step 4 asked for: hidden status div with **row-level summary** when colors arrive. Implementation:
- `aria-live="polite"` on the whole board container (not a hidden status div).
- Filled cells `aria-label={letter}` — letter only, no status (e.g. just `"A"`, not `"A correct"`).

Effect: every state change re-announces the entire board contents — noisy on screen readers; users hear letters without color context.

**Fix:** Keep board `aria-label` static; add a separate visually-hidden `<div role="status" aria-live="polite">` that summarizes only the most recent submitted row, e.g. "Row 3: A green, P yellow, P gray, L gray, E gray".

### I4 — Reconnect button race (Low)
**File:** `web/src/lib/components/connection-status.svelte:13-19,28-30`

Button only renders when `$connectionState === 'disconnected'`. Single-click triggers `connect()` which sets state to `'connecting'` synchronously and hides the button. A real double-click (sub-Svelte-tick) could fire `handleReconnect` twice. `connect()` resets `reconnectAttempt` and `closed`; `openSocket()` closes existing socket. Two sequential opens are tolerable (second close+open replaces first), but `reconnectAttempt` resets twice and `idToken()` is called twice (extra Firebase round-trip).

**Fix:** Add a local `reconnecting` flag in `handleReconnect` to short-circuit re-entry:
```ts
let reconnecting = $state(false);
async function handleReconnect() { if (reconnecting) return; reconnecting = true; try { connect(await idToken()); } catch {} finally { reconnecting = false; } }
```

### I5 — `matchSolution` from MatchResolved is dead read (Low, no user impact)
**File:** `web/src/lib/components/sync-game-scene.svelte:124`

`matchSolution = (msg as unknown as { solution?: string }).solution ?? '';` — `MatchResolved` proto (match.proto L110-114) has no `solution` field, so this always reads `undefined`. The `solution` lives on `WordleState.solution` (wordle.proto L34) which is captured by the GAME_STATE handler (L98-108) but never assigned to `matchSolution`. Net effect: ResultsScreen's "Answer:" line never appears in sync mode.

**Fix:** Inside the GAME_STATE handler, also set `matchSolution = state.solution ?? '';` when the state is terminal. Drop the `as unknown` cast.

### I6 — `creating` button has no busy label (Low, UX polish)
**File:** `web/src/lib/components/results-screen.svelte:97-99`

When `submitting=true` (which now includes `creating`), the "Challenge a friend" button is disabled but the label still reads "Challenge a friend". User has no visual confirmation the click registered.

**Fix:** Show "Creating…" while `submitting && !isChallenge && !shareToken`.

## Strengths
- Anti-enumeration in Firebase error mapping is correctly implemented and commented (sign-in.svelte L13-14, L19-22).
- WS lifecycle in layout untouched (L25-44); rejectAllPending + force-refresh cap (Phase 01) intact.
- Quick-match's `joined` once-guard preserved.
- File sizes all under 200 lines except sync-game-scene (236, but mostly CSS) and sign-in (206, mostly CSS) — acceptable.
- svelte-check 0/0.
- `creating` correctly guarded synchronously before any await (play/+page.svelte L127-128). No double-CHALLENGE_CREATE.
- Anonymous banner gating uses `startsWith` so /m/[token] doesn't briefly flash banner before redirect.
- Reconnect button properly delegates to AuthErrorToast on idToken failure (no console-leak path).

## Open follow-ups
1. (Q) Should `/leaderboard` be in landingRoutes? Spec text in step 3 contradicts itself ("only navigate from landing/leaderboard … never bounce away from /leaderboard mid-view"). Current impl bounces to /sync if user refreshes while on /leaderboard with active match. Acceptable on refresh? Confirm with PO.
2. Verify behavior with a real screen reader after I3 fix — current `aria-live` on board may already be working in practice; spec drift may be tolerable.
3. Phase 06 should add tests for: exhausted-reason copy, anon-banner gating per route, reconnect button visibility transitions.

**Status:** DONE_WITH_CONCERNS
**Summary:** 11/11 spec steps land; 1 user-visible copy bug (exhausted→"tie"), 1 dead store (`inMatch`), and a11y/UX polish items. Approve after I2 fix; rest can ride next phase.
**Concerns/Blockers:** I2 (exhausted reason mis-renders as "tie") should land before user-facing release.
