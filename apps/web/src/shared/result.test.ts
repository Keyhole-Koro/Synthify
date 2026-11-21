import { describe, it, expect } from 'vitest';
import { ok, err, isOk, isErr } from './result';

describe('Result type helpers', () => {
  it('creates an ok result', () => {
    const res = ok('success');
    expect(res.ok).toBe(true);
    if (res.ok) {
      expect(res.value).toBe('success');
    }
  });

  it('creates an err result', () => {
    const res = err('failure');
    expect(res.ok).toBe(false);
    if (!res.ok) {
      expect(res.error).toBe('failure');
    }
  });

  it('checks with isOk and isErr guards', () => {
    const success = ok(123);
    const failure = err('error');

    expect(isOk(success)).toBe(true);
    expect(isErr(success)).toBe(false);

    expect(isOk(failure)).toBe(false);
    expect(isErr(failure)).toBe(true);
  });
});
