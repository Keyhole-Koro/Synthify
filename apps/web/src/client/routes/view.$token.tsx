import { createFileRoute } from '@tanstack/react-router';
import { ShareLinkViewer } from '@/features/sharing/ShareLinkViewer';
import { resolveShareLinkQueryOptions } from '@/features/workspaces/queries';

/**
 * Route definition for public workspace share links accessed via path token `/view/$token`.
 */
export const Route = createFileRoute('/view/$token')({
  loader: ({ context: { queryClient }, params: { token } }) => {
    return queryClient.ensureQueryData(resolveShareLinkQueryOptions(token));
  },
  component: ViewTokenRoute,
});

/**
 * Component rendering the public shared workspace viewer with token from URL parameters.
 */
function ViewTokenRoute() {
  const { token } = Route.useParams();
  return <ShareLinkViewer token={token} />;
}
