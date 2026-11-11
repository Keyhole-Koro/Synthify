// /api/dashboards/cost — the handler lives in src/server/api/dashboards/cost.ts.
import { createFileRoute } from '@tanstack/react-router';
import { GET } from '@/server/api/dashboards/cost';

export const Route = createFileRoute('/api/dashboards/cost')({
  server: { handlers: { GET: ({ request }) => GET(request) } },
});
