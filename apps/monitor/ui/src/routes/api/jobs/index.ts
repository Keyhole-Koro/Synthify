// /api/jobs — the handler lives in src/server/api/jobs.ts.
import { createFileRoute } from '@tanstack/react-router';
import { GET } from '@/server/api/jobs';

export const Route = createFileRoute('/api/jobs/')({
  server: { handlers: { GET: ({ request }) => GET(request) } },
});
