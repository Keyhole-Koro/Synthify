import { createFileRoute } from '@tanstack/react-router';
import { LandingPageView } from '@/features/landing/LandingPageView';
import { useLandingPageController } from '@/features/landing/useLandingPageController';

/**
 * URL search parameters accepted by the root landing page route.
 */
export interface LandingPageSearch {
  /** Optional active node ID in the paper map tree. */
  nodeId?: string;
  /** Optional open modal or sidebar panel. */
  panel?: 'billing' | 'sharing' | 'workspaces';
}

/**
 * Route definition for the home / workspace landing page with query parameter validation.
 */
export const Route = createFileRoute('/')({
  validateSearch: (search: Record<string, unknown>): LandingPageSearch => {
    const panel =
      search.panel === 'billing' || search.panel === 'sharing' || search.panel === 'workspaces'
        ? search.panel
        : undefined;
    const nodeId = typeof search.nodeId === 'string' && search.nodeId ? search.nodeId : undefined;
    return { panel, nodeId };
  },
  component: LandingPageRoute,
});

/**
 * Landing page view wrapper binding the landing page controller to the UI presentation.
 */
function LandingPageRoute() {
  const controller = useLandingPageController();
  return <LandingPageView controller={controller} />;
}
