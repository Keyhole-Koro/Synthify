// /api/dashboards/eval — the handler lives in src/server/api/dashboards/eval.ts.
import { createFileRoute } from '@tanstack/react-router';
import { GET } from '@/server/api/dashboards/eval';

export const Route = createFileRoute('/api/dashboards/eval/')({
  server: { handlers: { GET: ({ request }) => GET(request) } },
});
