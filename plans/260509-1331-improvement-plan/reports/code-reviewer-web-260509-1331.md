# Web Client Code Review — dleague (SvelteKit + Phaser + TS)

Scope: `web/src/**` and `web/{vite,svelte,ts,package}.config.*`. Skips deploy/static-host concerns. ~30 source files, ~2.7K LOC excl. generated pb.

---

## High-impact issues

### H1. WebSocket never connects on `/quick-match` or `/sync` — page hangs indefinitely

`web/src/routes/quick-match/+page.svelte:11` calls `sendQueueJoin('wordle')` immediately on mount but never calls `connect()`. Same for `web/src/routes/sync/+page.svelte` which mounts `<SyncGameScene>` — neither calls `connect()`.

`+layout.svelte` does not call `connect()` either; it only subscribes to `connectionState` for rejoin.

Outcome:
- User hits `/quick-match` directly (deep link, refresh, or link from a fresh nav) → `sendQueueJoin` early-returns at `ws.ts:224` (`socket.readyState !== OPEN`) → spinner spins forever, no error surfaced. `cancel()` is the only escape.
- `sendMatchMove` in `sync-game-scene.svelte:79` silently no-ops the same way (`ws.ts:254`).
- Only `/play`, `/leaderboard`, `/m/[token]` call `connect()` themselves. Whether quick-match works at all currently depends on the user happening to come from one of those routes (and `disconnect()` not having fired on their unmount, which it does — see H2).

Fix: call `connect()` in `+layout.svelte` once `authUser` is non-null, and treat the WS as a layout-owned singleton. Remove per-route connect/disconnect entirely (they're already racy — see H2).

Severity: high (broken core UX path).

---

### H2. `connect/disconnect` per-route causes reconnect-loop and races on every navigation

`web/src/routes/play/+page.svelte:234` calls `disconnect()` in `onDestroy`. Same in `leaderboard/+page.svelte:75` and `m/[token]/+page.svelte:92`.

Because SvelteKit unmounts the previous page before mounting the next, every nav `/play` → `/leaderboard` triggers `disconnect()` (which sets `closed=true`) immediately followed by a fresh `connect()` (which sets `closed=false` and opens a new socket). Problems:

1. Pending `sendRequest` promises from the previous page are never rejected on intentional `disconnect()` — `disconnect()` only clears timers and the socket; `pending` map entries leak. Their setTimeouts will fire and reject "request timeout" against a dead promise; no orphan cleanup. See `ws.ts:81-89` vs. `ws.ts:358` `rejectAllPending` — the latter is only called when max reconnect attempts hit, never on intentional disconnect.
2. Module-level `pending` and `handlers` Maps survive across the disconnect→connect cycle. Stale `onMessage` handlers from `/play` (`ws.ts:223`-`227`) remain registered even after `removeHandler` because each route only removes the keys it knows about — but `+layout.svelte:62` also calls `removeHandler(MATCH_REJOIN_ACK)`, leaving overlap risk. Net: handler from old page can fire after navigating away.
3. After `disconnect()`, `closed=true` means `scheduleReconnect` is bypassed; the very next `connect()` resets it to `false`. But if the new `connect()` happens while the old close handshake is still in flight, the new socket assignment overrides `socket` immediately at `ws.ts:101` while the old `onclose` is still firing. The old `onclose` will then run for the new socket's `connectionState` → spurious "disconnected" flicker.
4. `/m/[token]` `onDestroy` (`+page.svelte:91`) only disconnects when status `!== 'redirecting'` — but `disconnect()` is also a no-op race vs. the new `/play` mount. After `redirecting` it leaves the socket open relying on `/play` to call `connect()` again immediately, which closes the existing one (`ws.ts:92-94`) and opens a new one with a fresh token. Unnecessary churn and any in-flight `ATTEMPT_SUBMIT` from a previous match would be dropped.

Fix: hoist the WS lifecycle into `+layout.svelte` (auth-resolved → connect; on signOut → disconnect). Routes only `sendRequest` / `onMessage`. On unmount, routes only call `removeHandler` for handlers they registered.

Severity: high (data loss possible — abandoned promises; UX flicker; complex race surface).

---

### H3. `sendRequest` promises never rejected on socket close — abandoned across reconnect

`ws.ts:174-202`: when WS is `OPEN` we register a `pending[requestId]`. If the socket closes while in-flight (server restart, network blip), `onclose` schedules reconnect but does **not** reject the pending promise. The 5-second `setTimeout` will eventually reject it as "request timeout", but only after the full timeout — meanwhile the user sees the screen frozen, a "Submitting…" overlay stays up, and if a reconnect succeeds with auto-rejoin, the request was already lost (server has no record because the close happened before/during write).

Worse: the `setTimeout` keeps the closure alive even after a real reconnect and re-issue of the request. No backpressure: if user mashes Enter we accumulate pending entries indefinitely.

Fix: in `onclose` (`ws.ts:117`), if `!closed`, call `rejectAllPending(new Error('connection lost'))` so callers can retry/show a "lost connection" toast. Currently rejectAllPending only runs after max attempts (`ws.ts:122-124`) — too late.

Severity: high (silent data loss in `/play` `submitGuess` and `submitAttempt`).

---

### H4. Token race: WS opens with possibly-empty token, no retry on auth

`/play/+page.svelte:216-221`, `/leaderboard:55-60`, `/m/[token]:80-85` all do:
```ts
try { const t = await idToken(); connect(t); }
catch { connect(''); }
```

`idToken()` (`auth-store.ts:23`) throws if `authUser` is null. Layout gates rendering on `authResolved && $authUser`, so by the time the route mounts, a user *should* exist. But:

- Anonymous sign-in is async; if the user has no Firebase session yet and the layout flips `authResolved=true` after the *first* `onAuthStateChanged` callback fires with `null`, the catch branch runs and connects with an empty token. The server (per protocol comment) requires `fb.<idToken>` — with empty token the server should reject the protocol negotiation.
- Even when token fetch succeeds, an upgrade later (sign in / sign out / token expiry mid-fetch) is not propagated to ws.ts; the existing socket keeps using the old subprotocol token until the 50-min refresh timer.
- `+layout.svelte:23` has `authUser.subscribe(() => { authResolved = true })` — `authResolved` is set on *every* auth-state change, not just first. This is a no-op flag-wise but the subscription's callback is leaked nowhere... wait, `unsub` is called on cleanup. OK.

Fix: don't connect with empty token. If `idToken()` throws, surface an error to the user; don't silently connect anonymously — server will close, user will see only a red badge with no explanation.

Severity: high (auth bypass / silent failure UX).

---

### H5. Token refresh timer never re-armed after reconnect

`ws.ts:319-339`: `scheduleTokenRefresh()` is called in `onopen` (`ws.ts:106`) and chains itself recursively at the end of the timeout. Good. But:

- If a reconnect happens before the 50-min timer fires (very common — page reload, network blip), the new `onopen` correctly re-schedules. But `clearTokenRefresh` is called in `onclose` (`ws.ts:119`) BEFORE the reconnect — if reconnect fails permanently (max attempts hit), no timer is scheduled. If it eventually succeeds via a future page mount/connect, scheduleTokenRefresh runs fine.
- If the user keeps a tab open >60 min and the reconnect path hits at minute 65, the token is already expired by then. `idToken()` will fetch a fresh one (Firebase auto-refreshes when <5min to expiry), but the reconnect code at `ws.ts:128-137` uses the **stale captured `token` parameter** — `openSocket(token)` uses the original `token` from the original `connect()` call. The Sec-WebSocket-Protocol value will be a stale token; server rejects.

Fix: on each reconnect attempt, fetch a fresh token via `idToken()` rather than reusing the captured one. Currently `scheduleReconnect(token, ...)` (`ws.ts:128`) propagates the stale token forward indefinitely.

Severity: high (after >1h tab idle, reconnect always fails until user navigates).

---

### H6. `+layout.svelte` rejoin handler executes on every route mount/connect transition

`+layout.svelte:29-56`: `connectionState.subscribe` triggers `sendMatchRejoin` whenever state becomes `'connected'`. Because routes connect/disconnect in their own onMount/onDestroy (H2), every route navigation that opens a fresh socket will re-fire MATCH_REJOIN as long as `sessionStorage.activeMatchID` is set. Implications:

- Visiting `/leaderboard` mid-match → leaderboard mounts, calls `connect`, layout-level subscribe sees `connected` → sends MATCH_REJOIN → on success, navigates back to `/sync` (`+layout.svelte:38-44`). User cannot view leaderboard mid-match without losing it.
- Worse: if the user is *currently on /sync* and the WS reconnects after a blip, the same code path navigates back to /sync (no-op `if (!currentPath.startsWith('/sync'))`), but `sendMatchRejoin` still fires and the in-flight match state may double-process the REJOIN_ACK.

Fix: only attempt rejoin when the user is on a routable game route AND the local in-memory state shows no active session. Better: gate on a `mid-match` flag set by `sync-game-scene` rather than sessionStorage alone.

Severity: high (broken nav UX during a match).

---

### H7. `sendQueueLeave` in `quick-match` `onDestroy` fires after `goto('/sync')` — leaves queue immediately after matchmaking

`/quick-match/+page.svelte:27-31`: `onDestroy` calls `sendQueueLeave()` if `searching` is true. After QUEUE_MATCHED (`:14-23`), `searching=false`, then `goto('/sync')`. Order is OK there.

But if QUEUE_MATCHED arrives in the same tick as the user clicks Cancel (or vice versa), there's a TOCTOU on `searching`. More important issue: `goto('/sync')` triggers unmount → `onDestroy` → if `searching` was set false in the same microtask, fine. If the user navigates away via browser back button mid-search, `cancel()` is not called (only invoked via the button click), so sendQueueLeave fires correctly via onDestroy. OK.

But: nothing protects against `QUEUE_MATCHED` arriving *after* unmount. Since the handler is registered globally on `ws.ts` handlers Map, it stays set until `removeHandler` runs in onDestroy. Order matters: `removeHandler(QUEUE_MATCHED)` first, then `sendQueueLeave`. Currently both run in onDestroy in that order — probably fine — but a server message arriving *between* the route unmounting and `removeHandler` executing could still call the captured `goto`. Probably benign but worth noting.

Severity: medium-high (race window is small but exists).

---

### H8. `MATCH_REJOIN_ACK` handler never triggers UI rehydration

`+layout.svelte:34` calls `sendMatchRejoin` (Promise) and uses the ack payload to `goto('/sync?matchId=...&seed=...')`, but `sync-game-scene.svelte` does **not** subscribe to MATCH_REJOIN_ACK to rehydrate `initialState`/`initialOpponentHints` props. Those props are documented as "Initial own state (populated on MATCH_REJOIN_ACK)" (`sync-game-scene.svelte:25-28`) but `/sync/+page.svelte:20` passes only `{matchId, seed, opponentName}`, never the rejoin payload.

Result: after rejoin, board shows empty, opponent panel shows empty — user appears to be at attempt 0 even though server has retained their progress. First MATCH_OPPONENT_PROGRESS or GAME_STATE will repopulate own state but opponent's prior attempts are lost until they make a new move.

Fix: in layout, stash `ack.ownState` and `ack.opponentHints` in a shared store (or pass via `goto` query state) and feed into `sync-game-scene` as initial props.

Severity: high (functional bug — rejoin is broken).

---

### H9. `applyServerState` row-flip emit uses **only the latest** row, but `WordleScene` is mounted on `/play` only — solo mode emits work; challenge mode also fine. But on hot-path duplicate dispatch:

`play/+page.svelte:79-98` `applyServerState` is called from BOTH (a) the `sendRequest` promise resolution at line 184 and (b) `onMessage(GAME_STATE)` push handler at 223-227. Server may send GAME_STATE both as a request response AND as a separate push (the WS handler fires both at `ws.ts:151-164`). Result: each move triggers `applyServerState` twice → each `eventBus.emit('wordle:flip-row', ...)` fires twice → two flip animations stacked, second one starts when first is mid-flip (rect destroyed in tween's onComplete).

Even worse: `submitAttempt` is called from `applyServerState` at line 96, so when terminal it could fire twice → ATTEMPT_SUBMIT sent twice. There's an `attemptSubmitting` guard (`:104`), but it's set inside the async function — the two calls run nearly simultaneously, second sees `attemptSubmitting=false`, sets it true, sends. Race-prone.

Fix: choose one delivery path. Either (a) drop the GAME_STATE push handler in play (already getting via Promise), or (b) drop the resolve-based path. Simpler: server should not double-deliver; on client, dedupe via a `lastAppliedSeq` if the protocol grows a sequence number.

Severity: high (correctness — duplicate ATTEMPT_SUBMIT, redundant animations).

---

### H10. `applyServerState` mutates derived row index assuming row=last guess — wrong on rejoin / state reload

`play/+page.svelte:80-97`: computes `row = s.guesses.length - 1` and emits `flip-row` only for the **last** row. On a fresh WordleScene mount where `s.guesses` arrives with multiple guesses already (e.g. challenge re-acceptance, or future rejoin in solo), only the last row animates; previous rows show no flip and remain unstyled by the Phaser overlay. They still render via DOM tile colors so it's visually OK, but the comment at line 89 ignores this case.

Severity: low (cosmetic, no broken state).

---

### H11. `connectionState.subscribe` callback in `+layout.svelte` runs **synchronously on subscribe** — fires for current state too

Svelte stores call subscribers immediately with the current value. `+layout.svelte:29` subscribes inside `onMount`; at that moment state is `'disconnected'`, so the if-guard `state !== 'connected'` returns early. OK. But after the rejoin Promise resolves and the user is on /sync, if the WS reconnects (e.g. server restart 1001), the subscriber fires again → another rejoin attempt → potentially navigates via `goto`. See H6.

Severity: medium (already covered by H6).

---

### H12. `firebase.config.json` is a real-looking but placeholder config

`firebase.config.json` shows `apiKey: "demo-api-key"`. Fine as placeholder; matches Firebase Auth emulator's expected demo project name ('demo-*' projects skip remote calls). But:
- `authDomain: dleague-dev.firebaseapp.com` and `projectId: dleague-dev` — if a real project exists with this ID, anyone running this client could pollute it. Verify the prod build pipeline overrides these (currently no env override is wired in `firebase.ts`).
- `.env.example` says env-var override is *not* wired up; firebase.ts hardcodes the import. This is OK as long as the JSON is updated per-environment, but it's brittle.

Recommend wiring `import.meta.env.VITE_FIREBASE_*` reads with the JSON as fallback, so you can build with different configs without rewriting the committed file.

Severity: medium (deploy hygiene; not strictly a code bug).

---

## Medium-impact issues

### M1. `play/+page.svelte` is 345 lines — exceeds 200-line cap from project rules

`docs` style guide caps files at 200 LOC. Extract: (a) WS message handlers into a dedicated `lib/play-controller.ts`; (b) `submitAttempt` / `createChallenge` into a `lib/challenge-api.ts`. The component should just bind UI to the controller.

### M2. `WordleScene` stale tween on flip-row before previous animation ends

`wordle-scene.ts:56-97`: nested tween chain. If `flipRow(0, ...)` fires twice (see H9) before first chain completes, the second invocation creates a new rectangle on top — the old one's onComplete still destroys its own rect (good), but during the overlap the user sees two stacked tiles flipping out of sync. Add an early-return guard: track an in-flight set of `(row,col)` keys; bail if already flipping.

### M3. `EventBus` typing is `unknown` everywhere — defeats type safety at the bus

`event-bus.ts:7`: `type Handler = (...args: unknown[]) => void;` and emit accepts any payload. Then `wordle-scene.ts:29` casts `payload as FlipRowPayload`. Replace with a typed map:

```ts
type Events = { 'title:start': []; 'wordle:flip-row': [FlipRowPayload]; };
class EventBus {
  on<K extends keyof Events>(e: K, h: (...args: Events[K]) => void): void { ... }
  emit<K extends keyof Events>(e: K, ...args: Events[K]): void { ... }
}
```

### M4. `+layout.svelte` `onMount` returns a cleanup func — but `onMount` only runs cleanup on component destroy, and `+layout.svelte` is rarely destroyed in an SPA

This is correct behavior actually — the cleanup runs on full app unmount (rarely). But there's no place where `disconnect()` runs on sign-out: signing out leaves the WS open with a now-stale Firebase token. Should subscribe to `authUser`: when it transitions non-null → null, call `disconnect()`.

### M5. `auth-store.ts` `idToken()` doesn't pass `forceRefresh: true` — relies on Firebase cache only

`auth-store.ts:26` calls `user.getIdToken()` (no forceRefresh). Firebase only fetches a fresh token if <5 min to expiry. If the server returned 401 (token revoked / claims changed / device-clock skew), there's no path to force a refresh. Add an overload `idToken(force = false)` and call with `force=true` from a 401 handler — currently no 401 handler exists.

### M6. Accessibility — board uses `role="grid"` but no keyboard navigation

`board.svelte:36`: ARIA grid pattern usually expects arrow-key nav and `aria-rowindex`/`aria-colindex`. The board is purely visual; users type via physical or on-screen keyboard. Either drop `role="grid"` (it's confusing for screen readers when there's no focus model) and use a `region` with descriptive aria-label, or implement focus handling. Empty tile aria-labels say literal `"empty"` which screen readers will speak for every empty cell — noisy.

Concretely:
- Replace `role="grid"` with a single `role="region" aria-label="..."` + `aria-live="polite"` on each row's class, announcing the latest guess when colors update.
- Drop per-cell aria-label or set to `aria-hidden="true"` when empty.

### M7. Keyboard component: on-screen keys are buttons, but no keyboard activation; physical keyboard handling lives in `+page.svelte`

`keyboard.svelte:65-71`: buttons with `onclick`. Hitting `Enter` on a focused on-screen button triggers it (good). However, focus management is missing — after clicking Backspace, the page sometimes auto-scrolls because the page focus moves to the button. Add `event.preventDefault()` and `tabindex=-1` for visual-only on-screen buttons, OR keep focus on a hidden text input that captures all input.

### M8. `opponent-panel.svelte` uses raw `Color` enum integers

`opponent-panel.svelte:22-33`: `case 3: return 'tile-green'` etc. Magic numbers. Import the `Color` enum and use `Color.GREEN` symbolically. Currently the `Color` proto type is imported but the integer cases are used directly — fragile against any pb regeneration that re-orders.

Same in `sync-game-scene.svelte:56-62`.

### M9. `LeaderboardEntry.uid` keyed each block — `entry.uid` may be empty for anonymous entries

`leaderboard/+page.svelte:114`, `leaderboard-table.svelte:33` use `(entry.uid)` as the keyed-each key. If two anonymous users happen to both appear with `uid=""`, the each block will throw a duplicate-key error. The display fallback is `entry.uid.slice(0, 8)` (`:117`) — also empty string for anonymous. Filter or coerce server-side, or use `(entry.rank)` as the key.

### M10. Two leaderboard table implementations

`leaderboard/+page.svelte` has its own table inline (`:104-124`), and `lib/components/leaderboard-table.svelte` is unused. DRY violation. Delete the unused component or refactor the page to use it.

### M11. `play/+page.svelte` initialises Phaser AFTER awaiting `idToken()` — race vs. message handlers

`play/+page.svelte:211-228`:
```ts
onMount(async () => {
  window.addEventListener('keydown', ...);
  initPhaser();
  gameStartMs = Date.now();
  try { const token = await idToken(); connect(token); } catch { connect(''); }
  onMessage(GAME_STATE, ...);
});
```

`onMessage` handler is registered AFTER `connect()`. If a server-pushed GAME_STATE arrives between `connect()` resolving onopen and the next microtask, it's dropped (no handler registered). Likely rare but reorder: register handlers first, then connect.

### M12. `sync-game-scene.svelte:106-110` mutates `opponentRows` then reassigns

```ts
while (opponentRows.length < msg.attemptNum - 1) {
    opponentRows.push([]);
}
opponentRows = [...opponentRows.slice(0, msg.attemptNum - 1), [...msg.colors]];
```

The `push()` mutates the reactive proxy in-place, which works in Svelte 5 runes mode but is needless: the next line replaces the array entirely. Drop the push loop. Also: `attemptNum - 1` indexing is brittle if the server ever sends out-of-order pushes (will overwrite a good earlier row). Sanity-check `msg.attemptNum >= prevAttemptNum + 1`.

### M13. `+page.svelte` (root) uses Phaser **only** for the title screen — pure overhead

`/` mounts `<PhaserGame>` purely to show "DLEAGUE" text and a button. Phaser is ~1MB+ gzipped. Replace with plain HTML/CSS for the title; reserve Phaser for actual game scenes (or lazy-load it). Currently Phaser is imported eagerly in `phaser-game.svelte` (top-level), so visiting `/` pays the full Phaser cost.

Quick win: dynamic-import Phaser inside `onMount`:
```ts
const { default: Phaser } = await import('phaser');
const { TitleScene } = await import('./scenes/title-scene');
```

Or replace the title scene entirely with Svelte markup — animation needs there are minimal.

### M14. `wordle-scene.ts` flip animation hardcoded for board layout 56px tile, 6px gap, 12px pad — duplicates `board.svelte` constants

Magic constants in two files (`board.svelte:65-67` CSS vs. `wordle-scene.ts:15-17`). If someone edits one, the overlay misaligns. Centralize in a `board-layout.ts` module.

### M15. No test coverage beyond `colors.test.ts`

Only one test file exists (9 score cases). Missing: ws.ts reconnect behavior, message dispatch, request timeout; auth-store; component smoke tests (board renders, keyboard fires); routing.

For a multiplayer real-time client, this is risky. Recommended minimum:
- ws.ts: vitest with mocked WebSocket (`vitest --workspace` / `happy-dom`) — verify reconnect backoff caps, pending rejection on close, token refresh chaining.
- Each route: ssr=false, mount in jsdom, verify the basic happy path.

### M16. No ESLint / Prettier config

`package.json` has `check` (svelte-check + tsc) but no `lint` script and no eslint/prettier deps. Project rule says "don't be too harsh on linting" but having an autoformatter prevents merge-conflict noise. Recommend adding `eslint-plugin-svelte` and `prettier-plugin-svelte`.

### M17. `tsconfig.json` `strict: true` is good, but `noUnusedLocals/Parameters` are explicitly false

Both flags off (`tsconfig.json:16-17`). Recommend turning at least `noUnusedLocals` on to catch dead code; e.g. `play/+page.svelte` declares `submitting` and never visibly uses it in templates apart from the brief setter — verify it's wired correctly.

### M18. `ResultsScreen` "Challenge a friend" button — requires server round-trip but no busy state

`results-screen.svelte:73-75`: `disabled={submitting}` is set, but `submitting` is the `attemptSubmitting` flag from the parent — it does not gate `createChallenge` itself. Spamming the button could send multiple CHALLENGE_CREATE requests. Add a local `creating` flag inside `play/+page.svelte::createChallenge` and wire it through.

### M19. `sync-game-scene.svelte` keyboard handler ENTER/BACKSPACE casing inconsistency with other components

`sync-game-scene.svelte:122-123` uses `'BACKSPACE'`/`'ENTER'` (uppercase), while `play/+page.svelte:151` uses `'Enter'`/`'Backspace'`. Two different conventions. The on-screen `Keyboard` component emits `'Enter'`/`'Backspace'` (`keyboard.svelte:17`), so `sync-game-scene.svelte:74-87` `handleKeyPress` doesn't actually match Enter clicks — only physical-keyboard upper-cased events. **Sync mode's on-screen keyboard's Enter button is broken.**

```ts
// keyboard.svelte emits 'Enter' / 'Backspace'
// sync-game-scene.svelte expects 'ENTER' / 'BACKSPACE'
```

Fix: normalize at one place. Make `Keyboard` always emit uppercase tokens, or normalize inside handleKeyPress.

Severity: high (broken UX in sync mode, on-screen keyboard) — re-classifying as **High**, not medium. Adding to high section retroactively.

### M20. Anonymous sign-in surfaces in UI but no warning that progress isn't persisted

`sign-in.svelte:83-85`: "Continue anonymously" button with no caveat. Anonymous users are excluded from leaderboards (per code comment in firebase.ts:65) — surface this in the UI. Otherwise users wonder why their daily score doesn't show.

---

## Low-impact / nits

- **L1.** `ws.ts:113` empty `onerror` — at least `console.warn` for visibility.
- **L2.** `ws.ts:160`: `pending` rejects with bare `'server error for request <id>'` — include the server's error payload (`env.payload` may carry an Error proto). Currently the payload is silently discarded.
- **L3.** `ws.ts:208` `onMessage` doc says "later registrations overwrite earlier ones" — silent override is hostile. `console.warn` if overwriting.
- **L4.** `play/+page.svelte:160`: `e.key === 'Backspace' ? 'Backspace' : e.key.length === 1 ? e.key : e.key` — last branch is a no-op. Simplify.
- **L5.** `play/+page.svelte:46` `$derived` returns `$page.url.searchParams.get('match') ?? ''`. Empty string for "no match" is OK but consider `null` for clarity; isChallengeMode coerces fine either way.
- **L6.** `play/+page.svelte:40` imports `Phaser` AND `WordleScene` at top level — Phaser ~1MB. Consider dynamic import on demand (per M13).
- **L7.** `share-button.svelte:19`: when clipboard fails, error message includes raw URL — user must select-and-copy. Nice-to-have: render the URL inside an `<input readonly>` for one-click select.
- **L8.** `connection-status.svelte:31`: `pointer-events: none` makes the badge non-interactive but also non-selectable; if "Disconnected" persists, user has no obvious "reconnect now" affordance.
- **L9.** `+layout.svelte:23`: `authUser.subscribe(() => { authResolved = true })` — sets the same flag repeatedly. Use `onAuthStateChanged` directly with a flag flip on first call, or just check `if (!authResolved) authResolved = true`.
- **L10.** `play/+page.svelte:165` toast "Word must be 5 letters" — also fires when input is empty (Enter on blank). Show a different message for empty: "Type a 5-letter word".
- **L11.** `app.html:11`: `<div style="display: contents">` works but is a weird wrapper. Either wrap in a real container (semantic) or use `display: contents` with rationale comment.
- **L12.** `vite.config.ts` `server.proxy['/health']` — not used in client code currently. If only for dev smoke tests, leaving it is fine but add a comment. Same `proxy['/ws']` only proxies during dev — confirm Go server in prod accepts the upgrade (looks correct based on earlier phases).
- **L13.** `sync-game-scene.svelte` `untrack()` use is a smart pattern but not documented for future maintainers — add a comment explaining why props are read once.
- **L14.** `leaderboard/+page.svelte:64` subscribes inside onMount but `unsub()` is called inside the callback only when state becomes connected — if the user leaves the page before connecting, the subscription leaks until WS connects. Move `unsub` cleanup to onDestroy.
- **L15.** `firebase.config.json:6` `messagingSenderId: "000000000000"` and `appId` placeholders — confirm intent: this is shipped to clients in `dist/`. If the project ID `dleague-dev` isn't a real one, prod will fail. Document this in `.env.example` more clearly.
- **L16.** `sign-in.svelte:18` shows raw Firebase error messages (e.g. `"Firebase: Error (auth/wrong-password)"`) — translate to friendly text.
- **L17.** `sign-in.svelte`: no "create account" or password reset flow. Users with no account hit the wall.
- **L18.** Duplicate `formatTime` function in `leaderboard/+page.svelte:41` and `leaderboard-table.svelte:11`. DRY (also see M10).
- **L19.** `quick-match/+page.svelte` cancel button: clicking it sets `searching=false`, then onDestroy runs and skips `sendQueueLeave` — but `cancel()` already calls `sendQueueLeave` so OK. Just confirm both paths do exactly one leave call. ✅ (verified).
- **L20.** `play/+page.svelte:202` Phaser config `scale: { mode: NONE }`, but `phaser-game.svelte` config uses FIT — inconsistent. Document why play uses NONE (overlay alignment) vs root uses FIT.
- **L21.** No favicon variants (just `static/favicon.png`). Not critical; Apple/touch icons missing.
- **L22.** Generated pb files have `/* eslint-disable */` — fine, but verify they're truly not edited (confirmed: top-level `// @generated by protoc-gen-es v2.12.0`).
- **L23.** Responsive: board tile size is fixed 56×56 (`board.svelte:66`). On <360px viewports the board overflows. Add `clamp()` or media queries.
- **L24.** No `lang` per-page; only `<html lang="en">` is set. Acceptable.
- **L25.** `+page.svelte` (root): Phaser title scene + a separate HTML "Quick Match" button below it — visually disjoint. Either pull the button into the Phaser scene (keep all CTAs in one render layer) or remove the title from Phaser entirely (M13).
- **L26.** `web/.gitignore` doesn't ignore `*.tsbuildinfo` — minor.
- **L27.** `sync-game-scene.svelte:99-100` empty if-block with just a comment — remove or implement.
- **L28.** `auth-store.ts` exports `idToken` (a function) named the same as common variable patterns — slight readability cost, consider `getIdToken`.

---

## Strengths

- WS module is well-structured: clear sections, no `any`, exhaustive switch on Color enum, named constants for backoff/timeout. Token refresh chaining is the right pattern.
- `Game<S, M>` interface mirrors the server, with clear "server is authoritative" comments throughout.
- Color scoring algorithm is correctly two-pass and matches the server unit tests verbatim.
- Clean separation of generated pb files into `pb/` folder; no hand-edits.
- Phaser scene shutdown handler removes EventBus listener — explicitly addresses leak risk.
- TypeScript `strict: true` with no `any` types in non-generated code. Good discipline.
- Auth provider abstraction in `firebase.ts` is small and focused.
- `+layout.ts` correctly disables SSR for an auth-gated SPA with WebSocket.

---

## Unresolved questions

1. Is Phaser intended to remain part of the title screen long-term, or is the title screen a transitional stub that should become pure HTML? (Affects M13 priority.)
2. What's the server contract for double-delivery of GAME_STATE — is the request/response also a server-push, or are they intentionally separate channels? (Affects H9 fix direction.)
3. Should anonymous users be able to play challenge matches? Currently `m/[token]` flow sends ChallengeJoin with whatever ID token the client has — server-side enforcement details not visible from this review.
4. Is there a planned offline/PWA story? Currently no service worker; reload during a match relies on sessionStorage which is per-tab.
5. Is there a way to retrieve a fresh `MATCH_REJOIN_ACK` payload-shaped state into the sync-game-scene (H8)? Plan suggests yes per code comments, but the wiring isn't there.
6. The `closed` module-level flag in ws.ts is reset by every `connect()` call — is that intended for the per-route connect/disconnect pattern (H2), or a vestige of a previous design?

---

**Status:** DONE_WITH_CONCERNS
**Summary:** Core WS lifecycle has multiple high-severity bugs (no connect on `/quick-match` / `/sync`, abandoned promises on disconnect, stale token on long-lived reconnect, broken Enter key in sync mode, broken rejoin rehydration, double-delivery of terminal state). Recommend hoisting WS to layout, registering handlers before connect, and adding the missing rejoin-state plumbing before any new feature work.
