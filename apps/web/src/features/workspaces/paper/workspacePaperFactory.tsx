import type { Paper } from '@keyhole-koro/paper-in-paper';
import { WorkspacePaper, WORKSPACE_PAPER_HUE } from '@/features/workspaces/paper/WorkspacePaper';
import type { WorkspacePaperRuntimeState } from '@/features/workspaces/paper/WorkspacePaper';
import { WORKSPACES_ID } from '@/features/paperMap/defaultOpenState';
import type { Workspace } from '@/features/workspaces/api';

/**
 * External dependencies and callbacks required to construct workspace paper definitions.
 */
export interface WorkspacePaperFactoryDeps {
  /** Accessor to look up the authoritative workspace record by ID. */
  getWorkspace: (workspaceId: string) => Workspace | undefined;
  /** Accessor reporting whether workspace data is currently loading. */
  isLoading: () => boolean;
  /** Accessor to get a fallback display name for a workspace ID. */
  getWorkspaceName: (id: string) => string;
  /** Accessor for the active runtime/job state of a workspace. */
  getRuntimeState: (workspaceId: string) => WorkspacePaperRuntimeState;
  /** Callback to upload a document to the workspace. */
  onUploadFile: (workspaceId: string, file: File) => Promise<{ jobId: string; documentId: string }>;
  /** Callback to rename the workspace. */
  onRenameWorkspace: (workspaceId: string, name: string) => Promise<Workspace>;
  /** Callback to update workspace name based on AI suggestions. */
  onSuggestedWorkspaceName: (workspaceId: string, suggestedName: string) => Promise<void> | void;
  /** Callback to delete the workspace. */
  onDeleteWorkspace: (workspaceId: string) => Promise<void>;
  /** Callback invoked when an active processing job completes. */
  onProcessingComplete: (workspaceId: string) => Promise<void> | void;
}

/**
 * Root node projection data representing the synthesized workspace summary report.
 */
export interface WorkspaceRootNode {
  /** Node identifier. */
  id: string;
  /** Synthesized HTML content. */
  content: string;
  /** Optional custom CSS stylesheet. */
  overrideCss?: string;
}

/**
 * Creates a synchronous Paper builder function for generating workspace container papers
 * on the PaperCanvas.
 *
 * @param deps - Factory dependencies and event handlers.
 * @returns A builder function accepting `workspaceId`, `childPapers`, and optional `rootNode`.
 */
export function createWorkspacePaperFactory(deps: WorkspacePaperFactoryDeps) {
  return function buildWsPaper(
    workspaceId: string,
    childPapers: { id: string; title: string }[],
    rootNode?: WorkspaceRootNode,
  ): Paper {
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
          hue: WORKSPACE_PAPER_HUE,
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
      hue: WORKSPACE_PAPER_HUE,
      // Neutral (white/grey) surface: the workspace's knowledge papers host
      // arbitrary LLM-authored content that reads on white. Children inherit
      // this, so the whole workspace subtree renders without a hue tint.
      saturationScale: 0,
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
          rootContent={rootNode?.content ?? null}
          rootOverrideCss={rootNode?.overrideCss ?? null}
          initialActiveJobId={runtimeState.initialActiveJobId ?? null}
          initialDocumentId={runtimeState.initialDocumentId ?? null}
          initialUploading={runtimeState.initialUploading}
          initialUploadMessage={runtimeState.initialUploadMessage}
          onUploadFile={(file) => deps.onUploadFile(workspaceId, file)}
          onRenameWorkspace={(name) => deps.onRenameWorkspace(workspaceId, name)}
          onDeleteWorkspace={() => deps.onDeleteWorkspace(workspaceId)}
          onSuggestedWorkspaceName={(name) => deps.onSuggestedWorkspaceName(workspaceId, name)}
          onProcessingComplete={() => deps.onProcessingComplete(workspaceId)}
        />
      ),
    };
  };
}

export type WorkspacePaperFactory = ReturnType<typeof createWorkspacePaperFactory>;
