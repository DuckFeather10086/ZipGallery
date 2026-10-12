import { test, expect } from "bun:test";
import { pagingState, pagingPulse } from "../src/wheel";
test("continuous scrolling repeats instead of locking a gesture", () => {
  const s = pagingState();
  let turns = 0;
  for (let now = 0; now <= 900; now += 10) turns += pagingPulse(s, 3.5, now);
  expect(turns).toBe(11);
});
test("wheel sampling rates produce the same sustained paging cadence", () => {
  for (const interval of [4, 8, 16, 30]) {
    const s = pagingState();
    let turns = 0;
    for (let now = 0; now <= 900; now += interval)
      turns += pagingPulse(s, 0.5, now);
    expect(turns).toBeGreaterThanOrEqual(10);
    expect(turns).toBeLessThanOrEqual(11);
  }
});
test("held keys and wheel use the same repeat clock", () => {
  const keys = pagingState(),
    wheel = pagingState();
  for (let now = 0; now <= 900; now += 30)
    expect(pagingPulse(keys, 1, now, now !== 0)).toBe(
      pagingPulse(wheel, 120, now),
    );
});
test("reverse, new gesture and individual key presses respond immediately", () => {
  const s = pagingState();
  expect(pagingPulse(s, 1, 0)).toBe(1);
  expect(pagingPulse(s, -1, 20)).toBe(-1);
  expect(pagingPulse(s, -1, 40, false)).toBe(-1);
  expect(pagingPulse(s, -1, 500)).toBe(-1);
});
test("no catch-up burst after an input stall; invalid inputs ignored", () => {
  const s = pagingState();
  pagingPulse(s, 1, 0);
  expect(pagingPulse(s, 1, 500)).toBe(1);
  expect(pagingPulse(s, 1, 501)).toBe(0);
  expect(pagingPulse(s, 0, 502)).toBe(0);
  expect(pagingPulse(s, NaN, 503)).toBe(0);
});
