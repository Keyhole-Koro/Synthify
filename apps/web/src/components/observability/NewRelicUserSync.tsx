'use client';

import { useEffect } from 'react';
import { setBrowserUserId } from '@/lib/newrelic/browser';
import { subscribeAuthUser } from '@/features/auth/session';

export function NewRelicUserSync() {
  useEffect(() => {
    return subscribeAuthUser((user) => {
      setBrowserUserId(user?.id ?? null);
    });
  }, []);

  return null;
}
