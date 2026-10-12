export type PagingState = {
  direction: number;
  lastInput: number;
  nextPage: number;
};
export const PAGE_REPEAT_MS = 90;
export const INPUT_RELEASE_MS = 160;
export function pagingState(): PagingState {
  return { direction: 0, lastInput: -Infinity, nextPage: 0 };
}
// Sample held keys and wheel input on the same clock. No queued turns or timer:
// when input stops, paging stops, even if a frame or image was slow to render.
export function pagingPulse(
  state: PagingState,
  direction: number,
  now: number,
  repeat = true,
): -1 | 0 | 1 {
  if (!Number.isFinite(direction) || direction === 0) return 0;
  const d = Math.sign(direction) as -1 | 1;
  const fresh =
    !repeat ||
    d !== state.direction ||
    now - state.lastInput > INPUT_RELEASE_MS;
  state.lastInput = now;
  state.direction = d;
  if (fresh) {
    state.nextPage = now + PAGE_REPEAT_MS;
    return d;
  }
  if (now < state.nextPage) return 0;
  // Preserve cadence across different input sampling rates, but never catch up
  // with a burst of old turns after a stall.
  state.nextPage +=
    (Math.floor((now - state.nextPage) / PAGE_REPEAT_MS) + 1) * PAGE_REPEAT_MS;
  return d;
}
