// /api/auth/me — the handler lives in src/server/api/auth/me.ts.
import { createFileRoute } from '@tanstack/react-router';
import { GET } from '@/server/api/auth/me';

export const Route = createFileRoute('/api/auth/me')({
  server: { handlers: { GET: ({ request }) => GET(request) } },
});
