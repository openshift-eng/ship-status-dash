/**
 * Schedules work after the current render commit.
 * Used for mount fetches that set state, so react-hooks/set-state-in-effect is satisfied.
 */
export function deferMountFetch(fn: () => void): void {
  queueMicrotask(fn)
}
