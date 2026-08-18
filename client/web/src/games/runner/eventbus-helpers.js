// Typed wrappers around the shared EventBus so variants and the runner
// don't ping each other with stringly-typed events scattered across files.

import { EventBus } from '../../game/EventBus';

/**
 * @typedef {Object} AttemptCompletePayload
 * @property {string[]} guesses
 * @property {('hit' | 'present' | 'miss')[][]} evaluations
 * @property {'won' | 'lost'} status
 */

/**
 * @typedef {{ kind: 'attempt-complete', payload: AttemptCompletePayload } | { kind: 'guess-submitted', payload: { guess: string } }} GameRunnerEvent
 */

export const Events = {
  ATTEMPT_COMPLETE: /** @type {'attempt-complete'} */ ('attempt-complete'),
  GUESS_SUBMITTED: /** @type {'guess-submitted'} */ ('guess-submitted'),
};

/**
 * @param {AttemptCompletePayload} payload
 * @returns {void}
 */
export function emitAttemptComplete(payload) {
  EventBus.emit(Events.ATTEMPT_COMPLETE, payload);
}

/**
 * @param {(p: AttemptCompletePayload) => void} fn
 * @returns {() => void}
 */
export function onAttemptComplete(fn) {
  EventBus.on(Events.ATTEMPT_COMPLETE, fn);
  return () => EventBus.off(Events.ATTEMPT_COMPLETE, fn);
}

/**
 * @param {string} guess
 * @returns {void}
 */
export function emitGuessSubmitted(guess) {
  EventBus.emit(Events.GUESS_SUBMITTED, { guess });
}

/**
 * @param {(p: { guess: string }) => void} fn
 * @returns {() => void}
 */
export function onGuessSubmitted(fn) {
  EventBus.on(Events.GUESS_SUBMITTED, fn);
  return () => EventBus.off(Events.GUESS_SUBMITTED, fn);
}
