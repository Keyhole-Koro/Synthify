import React from 'react';
import type { Paper } from '@keyhole-koro/paper-in-paper';
import { WorkspacePaper } from '@/features/workspaces/paper/WorkspacePaper';
import type { WorkspacePaperRuntimeState } from '@/features/workspaces/paper/WorkspacePaper';
import { WORKSPACES_ID } from '@/features/paperMap/defaultOpenState';
import type { Workspace } from '@/features/workspaces/api';

export interface WorkspacePaperFactoryDeps {
  // getWorkspace looks up the workspace by id from whatever source the
  // orchestrator considers authoritative (the official list, falling back
  // to the optimistic newly-created cache).
  getWorkspace: (workspaceId: string) => Workspace | undefined;
  // isLoading reports whether the workspace list is still being fetched. While
  // it is, a getWorkspace miss is expected (the list just hasn't arrived yet),
  // so we render an empty placeholder instead of the "not found" message.
  isLoading: () => boolean;
  getWorkspaceName: (id: string) => string;
  getRuntimeState: (workspaceId: string) => WorkspacePaperRuntimeState;
  onUploadFile: (workspaceId: string, file: File) => Promise<{ jobId: string; documentId: string }>;
  onRenameWorkspace: (workspaceId: string, name: string) => Promise<Workspace>;
  onSuggestedWorkspaceName: (workspaceId: string, suggestedName: string) => Promise<void> | void;
  onProcessingComplete: (workspaceId: string) => Promise<void> | void;
}

// createWorkspacePaperFactory returns the synchronous Paper builder used by
// projectWorkspacePapers and handleOpenWorkspace. It is a pure function over
// its deps so the orchestrator can memoize it once at hook setup time. The
// runtime state lookup is deferred to render time so refs stay live.
export function createWorkspacePaperFactory(deps: WorkspacePaperFactoryDeps) {
  return function buildWsPaper(workspaceId: string, childPapers: { id: string; title: string }[]): Paper {
    const workspace = deps.getWorkspace(workspaceId);

    if (!workspace) {
      // The list is still loading: the workspace will likely appear once it
      // arrives. Render an empty placeholder so we never bake a stale
      // "not found" paper into workspacePaperGroups (which would stick).
      if (deps.isLoading()) {
        return {
          id: workspaceId,
          title: deps.getWorkspaceName(workspaceId),
          description: '読み込み中',
          hue: 200,
          parentId: WORKSPACES_ID,
          childIds: [],
          content: <div className="p-4" aria-busy="true" />,
        };
      }
      return {
        id: workspaceId,
        title: '不明なワークスペース',
        description: 'ワークスペースが見つかりません',
        hue: 0,
        parentId: WORKSPACES_ID,
        childIds: [],
        content: (
          <div className="flex flex-col gap-2 p-4 text-center">
            <p className="text-sm font-medium text-slate-500">ワークスペースが見つかりません。</p>
            <p className="text-xs text-slate-400">削除されたか、アクセス権限がない可能性があります。</p>
          </div>
        ),
      };
    }

    const workspaceName = workspace.name || deps.getWorkspaceName(workspaceId);
    const runtimeState = deps.getRuntimeState(workspaceId);
    const runtimeKey = [
      runtimeState.initialActiveJobId ?? '',
      runtimeState.initialDocumentId ?? '',
      runtimeState.initialUploading === true ? 'uploading' : '',
      runtimeState.initialUploadMessage ?? '',
    ].join(':');

    return {
      id: workspaceId,
      title: workspaceName,
      description: 'ドキュメントと知識構造',
      hue: 200,
      parentId: WORKSPACES_ID,
      childIds: childPapers.map((p) => p.id),
      content: (
        <WorkspacePaper
          key={`${workspaceId}:${runtimeKey}`}
          workspace={workspace}
          workspaceId={workspaceId}
          workspaceName={workspaceName}
          hasTree={childPapers.length > 0}
          childItems={childPapers}
          initialActiveJobId={runtimeState.initialActiveJobId ?? null}
          initialDocumentId={runtimeState.initialDocumentId ?? null}
          initialUploading={runtimeState.initialUploading}
          initialUploadMessage={runtimeState.initialUploadMessage}
          onUploadFile={(file) => deps.onUploadFile(workspaceId, file)}
          onRenameWorkspace={(name) => deps.onRenameWorkspace(workspaceId, name)}
          onSuggestedWorkspaceName={(name) => deps.onSuggestedWorkspaceName(workspaceId, name)}
          onProcessingComplete={() => deps.onProcessingComplete(workspaceId)}
        />
      ),
    };
  };
}

export type WorkspacePaperFactory = ReturnType<typeof createWorkspacePaperFactory>;
