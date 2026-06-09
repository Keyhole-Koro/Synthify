'use client';

import { useParams } from 'next/navigation';
import { ShareLinkViewer } from '@/features/sharing/ShareLinkViewer';

export default function ShareLinkPage() {
  const params = useParams<{ token: string }>();
  const token = typeof params.token === 'string' ? params.token : '';
  return <ShareLinkViewer token={token} />;
}
