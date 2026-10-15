import { createFileRoute } from '@tanstack/react-router';
import { z } from 'zod';
import { ShareLinkViewer } from '@/features/sharing/ShareLinkViewer';

const viewSearchSchema = z.object({
  token: z.string().optional().default(''),
});

/**
 * Component rendering the public shared workspace viewer with token parsed from query parameters (`?token=...`).
 */
function ViewIndexRoute() {
  const { token } = Route.useSearch();
  return <ShareLinkViewer token={token} />;
}

/**
 * Route definition for public workspace share links accessed via query parameter `/view/?token=...`.
 */
export const Route = createFileRoute('/view/')({
  validateSearch: viewSearchSchema,
  component: ViewIndexRoute,
});
