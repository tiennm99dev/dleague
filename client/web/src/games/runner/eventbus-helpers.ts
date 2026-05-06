// Typed wrappers around the shared EventBus so variants and the runner
// don't ping each other with stringly-typed events scattered across files.

import { EventBus } from '../../game/EventBus';

export interface AttemptCompletePayload {
  guesses: string[];
  evaluations: ('hit' | 'present' | 'miss')[][];
  status: 'won' | 'lost';
}

export type GameRunnerEvent =
  | { kind: 'attempt-complete'; payload: AttemptCompletePayload }
  | { kind: 'guess-submitted'; payload: { guess: string } };

export const Events = {
  ATTEMPT_COMPLETE: 'attempt-complete' as const,
  GUESS_SUBMITTED: 'guess-submitted' as const,
};

export function emitAttemptComplete(payload: AttemptCompletePayload): void {
  EventBus.emit(Events.ATTEMPT_COMPLETE, payload);
}

export function onAttemptComplete(fn: (p: AttemptCompletePayload) => void): () => void {
  EventBus.on(Events.ATTEMPT_COMPLETE, fn);
  return () => EventBus.off(Events.ATTEMPT_COMPLETE, fn);
}

export function emitGuessSubmitted(guess: string): void {
  EventBus.emit(Events.GUESS_SUBMITTED, { guess });
}

export function onGuessSubmitted(fn: (p: { guess: string }) => void): () => void {
  EventBus.on(Events.GUESS_SUBMITTED, fn);
  return () => EventBus.off(Events.GUESS_SUBMITTED, fn);
}
