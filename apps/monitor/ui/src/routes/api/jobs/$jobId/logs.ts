// /api/jobs/$jobId/logs — the handler lives in src/server/api/jobs/logs.ts.
import { createFileRoute } from '@tanstack/react-router';
import { GET } from '@/server/api/jobs/logs';

export const Route = createFileRoute('/api/jobs/$jobId/logs')({
  server: { handlers: { GET: ({ request, params }) => GET(request, { params }) } },
});
