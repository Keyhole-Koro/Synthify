
import { useCallback, useRef } from 'react';
import type { WorkspacePaperRuntimeState } from '@/features/workspaces/paper/WorkspacePaper';

interface UseWorkspaceRuntimeStateArgs {
  rebuildWorkspacePaper: (workspaceId: string) => void;
}

/**
 * Manages volatile in-memory runtime UI state (activeJobId, upload messages) per workspace
 * so upload progress indicators light up immediately upon creation.
 *
 * @param props - Callback used to rebuild the affected workspace paper after state injection.
 * @returns Stable callbacks for reading, injecting, clearing one workspace, and resetting all
 *   runtime state. The returned map is kept in a ref and does not itself trigger a render.
 */
export function useWorkspaceRuntimeState({ rebuildWorkspacePaper }: UseWorkspaceRuntimeStateArgs) {
  const ref = useRef<Map<string, WorkspacePaperRuntimeState>>(new Map());

  const getRuntimeState = useCallback((workspaceId: string): WorkspacePaperRuntimeState => {
    return ref.current.get(workspaceId) ?? {};
   
  }, []);

  const injectRuntimeState = useCallback((workspaceId: string, state: WorkspacePaperRuntimeState) => {
    ref.current = new Map(ref.current).set(workspaceId, state);
    rebuildWorkspacePaper(workspaceId);
   
  }, [rebuildWorkspacePaper]);

  const clearRuntimeState = useCallback((workspaceId: string) => {
    if (!ref.current.has(workspaceId)) return;
    const next = new Map(ref.current);
    next.delete(workspaceId);
    ref.current = next;
   
  }, []);

  const resetAll = useCallback(() => {
    ref.current = new Map();
   
  }, []);

  return { getRuntimeState, injectRuntimeState, clearRuntimeState, resetAll };
}
