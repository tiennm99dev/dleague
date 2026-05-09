// Stores state recovered from a MATCH_REJOIN_ACK so the sync route can
// pre-populate the board without a second server round-trip.
import { writable } from 'svelte/store';
import type { WordleState, WordleHint } from './pb/dleague/v1/wordle_pb';

export const matchRejoinStore = writable<{
	ownState?: WordleState;
	opponentHints?: WordleHint[];
} | null>(null);
