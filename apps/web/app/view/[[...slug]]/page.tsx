/* eslint-disable react-refresh/only-export-components */
import { ShareLinkPageClient } from './ShareLinkPageClient';

export function generateStaticParams() {
  return [{ slug: [] }];
}

export default function ShareLinkPage() {
  return <ShareLinkPageClient />;
}
