/**
 * Schedules work after the current render commit.
 * Returns a cancel function for use in effect cleanup (prevents stale
 * microtasks from firing after React StrictMode re-mounts the component).
 */
export function deferMountFetch(fn: () => void): () => void {
  let cancelled = false
  queueMicrotask(() => {
    if (!cancelled) fn()
  })
  return () => {
    cancelled = true
  }
}
