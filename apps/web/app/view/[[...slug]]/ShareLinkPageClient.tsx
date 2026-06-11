'use client';

import { useEffect, useMemo, useState } from 'react';
import { ShareLinkViewer } from '@/features/sharing/ShareLinkViewer';

function getCurrentLocation() {
  if (typeof window === 'undefined') {
    return { pathname: '/view', search: '' };
  }
  return {
    pathname: window.location.pathname,
    search: window.location.search,
  };
}

export function ShareLinkPageClient() {
  const [location, setLocation] = useState(getCurrentLocation);

  useEffect(() => {
    const update = () => setLocation(getCurrentLocation());
    window.addEventListener('popstate', update);
    return () => window.removeEventListener('popstate', update);
  }, []);

  const token = useMemo(() => {
    const queryToken = new URLSearchParams(location.search).get('token')?.trim();
    if (queryToken) return queryToken;

    const segments = location.pathname.split('/').filter(Boolean);
    if (segments[0] !== 'view') return '';
    return decodeURIComponent(segments[1] ?? '');
  }, [location.pathname, location.search]);

  return <ShareLinkViewer token={token} />;
}
