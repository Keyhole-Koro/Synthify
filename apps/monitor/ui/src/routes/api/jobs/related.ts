// /api/jobs/related — the handler lives in src/server/api/jobs/related.ts.
import { createFileRoute } from '@tanstack/react-router';
import { POST } from '@/server/api/jobs/related';

export const Route = createFileRoute('/api/jobs/related')({
  server: { handlers: { POST: ({ request }) => POST(request) } },
});
