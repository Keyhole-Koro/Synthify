// /api/dashboards/job-health — the handler lives in src/server/api/dashboards/job-health.ts.
import { createFileRoute } from '@tanstack/react-router';
import { GET } from '@/server/api/dashboards/job-health';

export const Route = createFileRoute('/api/dashboards/job-health')({
  server: { handlers: { GET: ({ request }) => GET(request) } },
});
