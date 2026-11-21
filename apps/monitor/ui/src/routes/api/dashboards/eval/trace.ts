// /api/dashboards/eval/trace — the handler lives in src/server/api/dashboards/eval/trace.ts.
import { createFileRoute } from '@tanstack/react-router';
import { GET } from '@/server/api/dashboards/eval/trace';

export const Route = createFileRoute('/api/dashboards/eval/trace')({
  server: { handlers: { GET: ({ request }) => GET(request) } },
});
