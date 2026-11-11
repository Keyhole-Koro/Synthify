// /api/dashboards/workspace — the handler lives in src/server/api/dashboards/workspace.ts.
import { createFileRoute } from '@tanstack/react-router';
import { GET } from '@/server/api/dashboards/workspace';

export const Route = createFileRoute('/api/dashboards/workspace')({
  server: { handlers: { GET: ({ request }) => GET(request) } },
});
