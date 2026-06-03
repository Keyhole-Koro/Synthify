'use client';

import { useCallback, useRef } from 'react';
import type { WorkspacePaperRuntimeState } from '@/features/workspaces/paper/WorkspacePaper';

interface UseWorkspaceRuntimeStateArgs {
  rebuildWorkspacePaper: (workspaceId: string) => void;
}

// useWorkspaceRuntimeState holds the per-workspace WorkspacePaper runtime
// state (initial activeJobId / uploadMessage / etc.) that the landing page
// injects into a freshly created workspace card so its upload progress UI
// lights up immediately. It is a thin in-memory keyed map; refresh on
// reload is the WorkspacePaper's own concern (it recovers from Firestore).
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
