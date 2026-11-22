import { act, cleanup, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { isIntakeClosed, setIntakeClosed, useIntakeClosed } from './intakeStatus';

describe('intake status', () => {
  // Unmount first: a mounted paper would react to the reset.
  afterEach(() => {
    cleanup();
    setIntakeClosed(false);
  });

  it('starts open and follows what the API says', () => {
    const { result } = renderHook(() => useIntakeClosed());
    expect(result.current).toBe(false);

    act(() => setIntakeClosed(true));
    expect(result.current).toBe(true);
    expect(isIntakeClosed()).toBe(true);

    // An operator reopened intake: the next SignInUser says so.
    act(() => setIntakeClosed(false));
    expect(result.current).toBe(false);
  });
});
