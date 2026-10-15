// /api/jobs/trace — the handler lives in src/server/api/jobs/trace.ts.
import { createFileRoute } from '@tanstack/react-router';
import { GET } from '@/server/api/jobs/trace';

export const Route = createFileRoute('/api/jobs/trace')({
  server: { handlers: { GET: ({ request }) => GET(request) } },
});
