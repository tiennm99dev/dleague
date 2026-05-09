# Phase 03 — UX correctness

## Context Links
- [Web review](reports/code-reviewer-web-260509-1331.md) — H6, M6, M7, M18, M20, L8, L10, L16, L17, L23
- Depends on: Phase 01 (rejoin payload threading must land first)

## Overview
- **Priority:** P2
- **Status:** pending
- **Description:** Repair UX edges around mid-match navigation, accessibility, anonymous-user clarity, reconnect affordance, and results-screen edge cases (opponent left, disconnect mid-race). Not new features — fixing what's broken or missing-but-implied.

## Key Insights
- `web/src/routes/+layout.svelte:29-56` rejoin handler fires on every reconnect — visiting `/leaderboard` mid-match auto-bounces user back to `/sync` (web H6). **Phase 02 note:** Auth failures now surface via `AuthErrorToast` (non-blocking alert). Phase 03 can reference this store in connection-status or connection-error handling if deeper integration needed.
- `web/src/lib/components/board.svelte:36` uses `role="grid"` without keyboard nav model; aria-label `"empty"` per cell is screen-reader noise (web M6).
- `web/src/lib/components/keyboard.svelte:65-71` on-screen keys steal focus → autoscroll on Backspace (web M7).
- `web/src/lib/components/sign-in.svelte:83-85` "Continue anonymously" with no warning that progress isn't persisted on leaderboard (web M20).
- `web/src/lib/components/connection-status.svelte:31` `pointer-events:none` — disconnected user has no "reconnect now" affordance (web L8).
- `web/src/lib/components/results-screen.svelte:73-75` "Challenge a friend" button can be spam-clicked → multiple CHALLENGE_CREATE requests (web M18).
- `web/src/lib/components/sign-in.svelte:18` shows raw Firebase error strings (`"auth/wrong-password"`) (web L16).

## Requirements
- Mid-match navigation to non-game routes does not auto-bounce user back.
- Board accessible to screen readers without per-cell noise; keyboard nav coherent.
- On-screen keyboard does not steal focus / scroll the page.
- Anonymous users see clear "scores not saved" callout before / during play.
- Disconnected state has a visible Reconnect button.
- Results screen handles "opponent left" and "you disconnected mid-race" with appropriate messaging.
- Sign-in errors shown as friendly text.

## Related Code Files
**Modify**
- `web/src/routes/+layout.svelte` (gate rejoin on local mid-match flag)
- `web/src/lib/components/board.svelte` (role + aria)
- `web/src/lib/components/keyboard.svelte` (focus/tabindex)
- `web/src/lib/components/sign-in.svelte` (anon warning + friendly errors)
- `web/src/lib/components/connection-status.svelte` (reconnect affordance)
- `web/src/lib/components/results-screen.svelte` (opponent-left/disconnect copy; busy state)
- `web/src/routes/play/+page.svelte` (creating-challenge guard)
- `web/src/lib/components/sync-game-scene.svelte` (emit "opponent left" event from MATCH_FORFEIT/RESOLVED)

**Create**
- `web/src/lib/components/anonymous-warning.svelte` — small badge for sign-in flow + sticky banner on /play if anonymous.
- `web/src/lib/match-state-flag.ts` — writable `inMatch` flag set by sync-game-scene mount/unmount.

## Implementation Steps

1. Create `web/src/lib/match-state-flag.ts`: `export const inMatch = writable(false)`.
2. `web/src/lib/components/sync-game-scene.svelte` — set `inMatch.set(true)` on mount; on resolve/forfeit/destroy set `inMatch.set(false)`.
3. `web/src/routes/+layout.svelte:29-56` — change rejoin gate: only attempt if `!get(inMatch) && sessionStorage.activeMatchID` AND user is on a routable game path. Remove the unconditional `goto('/sync')` after ack — instead, only navigate if user is on the **landing/leaderboard** routes (i.e., they refreshed away from /sync); never bounce away from /leaderboard mid-view.
4. `web/src/lib/components/board.svelte:36` — replace `role="grid"` with `<div role="region" aria-label="Wordle board" aria-live="polite">`. Drop per-cell `aria-label="empty"`; set `aria-hidden="true"` on empty cells. Add `aria-label` per filled cell summarising the row when colors arrive (use a hidden status div with row-level summary).
5. `web/src/lib/components/keyboard.svelte:65-71` — add `tabindex="-1"` to all key buttons; add `onpointerdown={(e) => e.preventDefault()}` so focus stays where it was.
6. Create `web/src/lib/components/anonymous-warning.svelte` — accepts `{ inline: boolean }`. Renders "Anonymous play — daily leaderboard scores will not be saved. Sign in to compete." Mount in `sign-in.svelte:83` above the button (inline) and in `+layout.svelte` as a sticky banner when `$authUser?.isAnonymous`.
7. `web/src/lib/components/sign-in.svelte:18` — map Firebase error codes (`auth/wrong-password`, `auth/user-not-found`, `auth/invalid-email`, `auth/too-many-requests`) to friendly strings; default fallback "Sign-in failed. Try again."
8. `web/src/lib/components/connection-status.svelte:31` — when state is `disconnected`, drop `pointer-events:none` and show a "Reconnect" button that calls `connect(await idToken())`. Hide button while `connecting`.
9. `web/src/lib/components/results-screen.svelte` — add prop `reason: 'win' | 'loss' | 'tie' | 'opponent-left' | 'self-disconnect'`. Render copy variants. For `opponent-left`, suppress "Challenge again" CTA; show "Find new match".
10. `web/src/routes/play/+page.svelte:createChallenge` — wrap in `let creating = $state(false)`; set true before sendRequest, false in finally; pass `creating` as a `disabled` prop to the button (separate from `attemptSubmitting`).
11. `web/src/lib/components/sync-game-scene.svelte` — on MATCH_RESOLVED, inspect payload for opponent-forfeit reason; pass appropriate `reason` to ResultsScreen.

## Todo List
- [ ] `inMatch` flag + rejoin gate (steps 1-3)
- [ ] Board a11y refactor (step 4)
- [ ] Keyboard focus fix (step 5)
- [ ] Anonymous warning component + mount points (step 6)
- [ ] Friendly sign-in error mapping (step 7)
- [ ] Reconnect affordance on connection-status (step 8)
- [ ] Results-screen edge-case copy + reason prop (steps 9, 11)
- [ ] Challenge-create busy state (step 10)

## Success Criteria
- Manual: start sync match, navigate to /leaderboard → leaderboard renders; clicking "Back to game" returns; no auto-bounce.
- axe-core scan on /play: no critical issues; per-cell empty noise gone.
- Tab through keyboard component → focus does not visibly land on visual-only keys; page does not scroll.
- Anonymous sign-in → banner visible on /play "scores not saved".
- Pull network plug mid-match → connection-status shows "Reconnect" button; click reconnects.
- Spam Challenge button → only one CHALLENGE_CREATE in network log.
- `auth/wrong-password` → user sees "Incorrect email or password", not raw code.

## Risk Assessment
- **Rejoin gating regression:** if `inMatch` flag is mis-set, refresh-during-match no longer rehydrates. Mitigation: keep sessionStorage as source of truth for "should rejoin"; `inMatch` only suppresses auto-navigation post-ack.
- **A11y change disrupts current users:** screen reader behaviour changes; verify with NVDA/VoiceOver smoke test.
- **Reconnect button race:** clicking during connecting state → double socket. Mitigation: button disabled while `connecting`.

## Security Considerations
- Friendly error mapping must not leak account-existence (`auth/user-not-found` should map to same generic message as `auth/wrong-password`).

## Next Steps
- Phase 06 adds component tests + a basic e2e covering anon banner, results-screen variants.
