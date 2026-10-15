// /api/dashboards/errors — the handler lives in src/server/api/dashboards/errors.ts.
import { createFileRoute } from '@tanstack/react-router';
import { GET } from '@/server/api/dashboards/errors';

export const Route = createFileRoute('/api/dashboards/errors')({
  server: { handlers: { GET: ({ request }) => GET(request) } },
});
