// /api/jobs/search — the handler lives in src/server/api/jobs/search.ts.
import { createFileRoute } from '@tanstack/react-router';
import { POST } from '@/server/api/jobs/search';

export const Route = createFileRoute('/api/jobs/search')({
  server: { handlers: { POST: ({ request }) => POST(request) } },
});
