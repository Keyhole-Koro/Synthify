import { describe, expect, test } from 'bun:test';
import { periodInterval } from './dashboard-queries';

describe('periodInterval', () => {
  test("'today' truncates to the start of the current day", () => {
    expect(periodInterval('today')).toBe("date_trunc('day', NOW())");
  });

  test("'7d' / '30d' go back the right interval", () => {
    expect(periodInterval('7d')).toBe("NOW() - INTERVAL '7 days'");
    expect(periodInterval('30d')).toBe("NOW() - INTERVAL '30 days'");
  });
});
