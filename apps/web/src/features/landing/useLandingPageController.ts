'use client';

import { useCallback, useRef, useState } from 'react';
import type { Paper } from '@keyhole-koro/paper-in-paper';
import { useAuthState } from '@/features/auth/useAuthState';
import { signOutSession } from '@/features/auth/session';
import { createWorkspace, updateWorkspace, type Workspace } from '@/features/workspaces/api';
import { useWorkspaceTree, type WorkspacePaperRuntimeState } from '@/features/workspaces/useWorkspaceTree';
import { useHomeCanvasViewState } from '@/features/paperMap/hooks/useHomeCanvasViewState';
import { useLandingPaperMap } from '@/features/paperMap/hooks/useLandingPaperMap';
import { usePersistentPaperOpenState } from '@/features/paperMap/hooks/usePersistentPaperOpenState';
import { useViewportSize } from '@/features/landing/useViewportSize';
import { toAppError } from '@/lib/error_normalize';

const NEW_WORKSPACE_NAME = '新規ワークスペース';

export function useLandingPageController() {
  const {
    user,
    loading,
    workspaces,
    workspaceError,
    authError,
    setWorkspaces,
    handleGoogleSubmit,
    retryWorkspaces,
  } = useAuthState();
  const [canvasFullscreen, setCanvasFullscreen] = useState(false);
  const [workspacePaperGroups, setWorkspacePaperGroups] = useState<Map<string, Paper[]>>(new Map());
  const [pendingRevealNodeId, setPendingRevealNodeId] = useState<string | null>(null);
  const { hasMounted, winSize } = useViewportSize();

  const {
    canvasKey,
    defaultOpenState,
    hasDefaultOpenState,
    expansionMap,
    focusedNodeId,
    handleExpansionMapChange,
    handleFocusedNodeIdChange,
    resetToLoggedOutDefaults,
  } = usePersistentPaperOpenState({ user, loading, workspaces });

  const getWorkspaceName = useCallback(
    (id: string) => workspaces.find((w) => w.workspaceId === id)?.name ?? id,
    [workspaces],
  );

  const setWorkspacePapers = useCallback((workspaceId: string, papers: Paper[]) => {
    setWorkspacePaperGroups((prev) => {
      const current = prev.get(workspaceId);
      if (current === papers) return prev;
      const next = new Map(prev);
      next.set(workspaceId, papers);
      return next;
    });
  }, []);

  const clearWorkspacePapers = useCallback(() => {
    setWorkspacePaperGroups(new Map());
  }, []);

  const clearPendingRevealNodeId = useCallback(() => {
    setPendingRevealNodeId(null);
  }, []);

  const applyWorkspaceUpdate = useCallback((workspace: Workspace) => {
    setWorkspaces((prev) => prev.map((candidate) => (
      candidate.workspaceId === workspace.workspaceId ? workspace : candidate
    )));
  }, [setWorkspaces]);

  const handleRenameWorkspace = useCallback(async (workspaceId: string, name: string) => {
    const updated = await updateWorkspace(workspaceId, name);
    applyWorkspaceUpdate(updated);
    return updated;
  }, [applyWorkspaceUpdate]);

  const handleAutoNameWorkspace = useCallback(async (workspaceId: string, suggestedName: string) => {
    const current = workspaces.find((workspace) => workspace.workspaceId === workspaceId);
    if (!current || current.name !== NEW_WORKSPACE_NAME) return;
    const trimmed = suggestedName.trim();
    if (!trimmed || trimmed === current.name) return;
    await handleRenameWorkspace(workspaceId, trimmed.slice(0, 64));
  }, [handleRenameWorkspace, workspaces]);

  const workspacePaperRuntimeStateRef = useRef<Map<string, WorkspacePaperRuntimeState>>(new Map());
  const getWorkspacePaperRuntimeState = useCallback((workspaceId: string): WorkspacePaperRuntimeState => {
    return workspacePaperRuntimeStateRef.current.get(workspaceId) ?? {};
  }, []);
  const clearWorkspacePaperRuntimeState = useCallback((workspaceId: string) => {
    if (!workspacePaperRuntimeStateRef.current.has(workspaceId)) return;
    const nextRuntimeState = new Map(workspacePaperRuntimeStateRef.current);
    nextRuntimeState.delete(workspaceId);
    workspacePaperRuntimeStateRef.current = nextRuntimeState;
  }, []);

  const { handleOpenWorkspace, resetTree, buildWsPaper, rebuildWorkspacePaper, uploadWorkspaceFile } = useWorkspaceTree(
    getWorkspaceName,
    expansionMap,
    handleExpansionMapChange,
    handleFocusedNodeIdChange,
    setWorkspacePapers,
    clearWorkspacePapers,
    workspaces,
    handleRenameWorkspace,
    handleAutoNameWorkspace,
    getWorkspacePaperRuntimeState,
    clearWorkspacePaperRuntimeState,
  );

  const { isFullscreen } = useHomeCanvasViewState(expansionMap, canvasFullscreen);

  const injectWorkspaceRuntimeState = useCallback((workspaceId: string, state: WorkspacePaperRuntimeState) => {
    workspacePaperRuntimeStateRef.current = new Map(workspacePaperRuntimeStateRef.current).set(workspaceId, state);
    rebuildWorkspacePaper(workspaceId);
  }, [rebuildWorkspacePaper]);

  const handleLogout = useCallback(async () => {
    const previousUser = user;
    await signOutSession();
    workspacePaperRuntimeStateRef.current = new Map();
    resetTree();
    resetToLoggedOutDefaults(previousUser);
  }, [resetToLoggedOutDefaults, resetTree, user]);

  const handleCreateWorkspace = useCallback(async (name: string) => {
    const ws = await createWorkspace(name);
    setWorkspaces((prev) => [ws, ...prev]);
    void handleOpenWorkspace(ws.workspaceId, ws);
    setPendingRevealNodeId(ws.workspaceId);
  }, [handleOpenWorkspace, setWorkspaces]);

  const handleRootUpload = useCallback(async (file: File) => {
    const ws = await createWorkspace(NEW_WORKSPACE_NAME);
    setWorkspaces((prev) => [ws, ...prev]);
    await handleOpenWorkspace(ws.workspaceId, ws);
    setPendingRevealNodeId(ws.workspaceId);
    injectWorkspaceRuntimeState(ws.workspaceId, { initialUploading: true, initialUploadMessage: null });
    void (async () => {
      try {
        const uploaded = await uploadWorkspaceFile(ws.workspaceId, file);
        injectWorkspaceRuntimeState(ws.workspaceId, {
          initialActiveJobId: uploaded.jobId,
          initialDocumentId: uploaded.documentId,
          initialUploading: false,
          initialUploadMessage: 'アップロードしました。解析を開始します。',
        });
      } catch (err) {
        console.error('Root upload failed after workspace creation:', err);
        const appError = toAppError(err);
        injectWorkspaceRuntimeState(ws.workspaceId, {
          initialUploading: false,
          initialUploadMessage: `アップロードに失敗しました。${appError.message}`,
        });
      }
    })();
  }, [handleOpenWorkspace, injectWorkspaceRuntimeState, setWorkspaces, uploadWorkspaceFile]);

  const { paperMap } = useLandingPaperMap({
    user,
    loading,
    workspaces,
    workspaceError,
    authError,
    workspacePaperGroups,
    handleGoogleSubmit,
    handleLogout,
    handleCreateWorkspace,
    handleRootUpload,
    handleOpenWorkspace,
    buildWsPaper,
    onRetryWorkspaces: retryWorkspaces,
  });

  return {
    isReady: hasMounted && hasDefaultOpenState,
    isFullscreen,
    winSize,
    paperMap,
    canvasKey,
    defaultOpenState,
    expansionMap,
    focusedNodeId,
    pendingRevealNodeId,
    setCanvasFullscreen,
    handleExpansionMapChange,
    handleFocusedNodeIdChange,
    clearPendingRevealNodeId,
  };
}

export type LandingPageController = ReturnType<typeof useLandingPageController>;
