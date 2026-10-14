
import React, { useCallback, useEffect, useRef, useState } from 'react';
import { isJobFailed, isJobSucceeded, isJobTerminal } from '@/features/jobs/contracts/jobStatusContract';
import { WorkspaceHeader } from './components/WorkspaceHeader';
import { WorkspaceDropzone } from './components/WorkspaceDropzone';
import { WorkspaceJobProgress } from './components/WorkspaceJobProgress';
import { WorkspaceJobList } from './components/WorkspaceJobList';
import { WorkspaceRootContent } from './components/WorkspaceRootContent';
import { WorkspaceEmptyHeader } from './components/WorkspaceEmptyHeader';
import { getWorkspace, WorkspaceRole, type Workspace } from '@/features/workspaces/api';
import { ConnectError, Code } from '@connectrpc/connect';
import { getInitialAuthUser } from '@/features/auth/session';
import { SharePanel } from '@/features/sharing/SharePanel';
import { InlineError } from '@/components/error/InlineError';
import { formatConnectErrorMessage, isExpectedConnectError, UserFacingError } from '@/lib/errors';
import { useWorkspaceSession } from '../session/useWorkspaceSession';
import { WorkspaceChatPanel } from '@/features/workspaceChat/WorkspaceChatPanel';

/**
 * Constant defining the primary HSL hue (200) for workspace papers on canvas.
 */
export const WORKSPACE_PAPER_HUE = 200;

/**
 * Initial runtime parameters passed into a workspace paper to restore job progress and uploads.
 */
export interface WorkspacePaperRuntimeState {
  /** Identifier of the active job initiated prior to mounting. */
  initialActiveJobId?: string | null;
  /** Identifier of the document being processed. */
  initialDocumentId?: string | null;
  /** Whether an upload is actively occurring upon mount. */
  initialUploading?: boolean;
  /** Status message associated with the ongoing upload. */
  initialUploadMessage?: string | null;
}

/**
 * Props for the {@link WorkspacePaper} canvas card component.
 */
interface WorkspacePaperProps extends WorkspacePaperRuntimeState {
  /** The full Protobuf Workspace entity. */
  workspace: Workspace;
  /** Unique workspace ID string. */
  workspaceId: string;
  /** Current display name of the workspace. */
  workspaceName: string;
  /** Whether any knowledge trees or root nodes currently exist. */
  hasTree: boolean;
  /** Child papers associated with top-level root nodes. */
  childItems: { id: string; title: string }[];
  /** HTML cover report string for the workspace root, or null if empty. */
  rootContent: string | null;
  /** Optional custom CSS styling for the sandboxed root iframe. */
  rootOverrideCss: string | null;
  /** Callback invoked when a user drops or selects a file to upload. */
  onUploadFile: (file: File) => Promise<{ jobId: string; documentId: string }>;
  /** Callback invoked when renaming the workspace. */
  onRenameWorkspace: (name: string) => Promise<Workspace>;
  /** Callback invoked when deleting the workspace. */
  onDeleteWorkspace: () => Promise<void>;
  /** Callback triggered when AI generates an updated workspace title. */
  onSuggestedWorkspaceName: (name: string) => Promise<void> | void;
  /** Optional callback triggered when an active job reaches completion. */
  onProcessingComplete?: (jobId: string) => Promise<void> | void;
}

/**
 * Main container paper for an individual workspace on the canvas map.
 * Houses the workspace title header, document upload dropzone, live job progress bar,
 * sandboxed cover report iframe, and Q&A chat panel.
 *
 * @param props - Component properties.
 * @returns The rendered workspace paper component.
 */
export function WorkspacePaper(props: WorkspacePaperProps) {
  const {
    workspaceId,
    workspaceName,
    hasTree,
    childItems,
    rootContent,
    rootOverrideCss,
    initialActiveJobId,
    initialUploading,
    initialUploadMessage,
    onUploadFile,
    onRenameWorkspace,
    onDeleteWorkspace,
    onSuggestedWorkspaceName,
    onProcessingComplete,
  } = props;

  const {
    jobStatus,
    jobStatusError,
    workspaceJobs,
    activeJobId,
    isRunning,
    isFailed,
    uploading,
    uploadMessage,
    isJustCompleted,
    isPopulated,
    handleUpload,
    editingName,
    draftName,
    savingName,
    nameError,
    setDraftName,
    commitName,
    startRename,
    cancelRename,
  } = useWorkspaceSession({
    workspaceId,
    workspaceName,
    hasTree,
    childItemsCount: childItems.length,
    initialActiveJobId,
    initialUploading,
    initialUploadMessage,
    onUploadFile,
    onRenameWorkspace,
    onSuggestedWorkspaceName,
    onProcessingComplete,
  });

  const fileInputRef = useRef<HTMLInputElement>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [isSharing, setIsSharing] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [accessRole, setAccessRole] = useState<WorkspaceRole>(WorkspaceRole.OWNER);

  useEffect(() => {
    const userId = getInitialAuthUser()?.id;
    getWorkspace(workspaceId)
      .then(({ members }) => {
        const membership = members.find((member) => member.userId === userId);
        setAccessRole(membership?.role ?? WorkspaceRole.OWNER);
      })
      .catch(() => setAccessRole(WorkspaceRole.VIEWER));
  }, [workspaceId]);

  const canWrite = accessRole === WorkspaceRole.OWNER || accessRole === WorkspaceRole.EDITOR;
  const canManage = accessRole === WorkspaceRole.OWNER;

  const openShare = useCallback(() => {
    setIsSharing(true);
  }, []);

  const handleDeleteWorkspace = useCallback(async () => {
    setDeleting(true);
    setDeleteError(null);
    try {
      await onDeleteWorkspace();
    } catch (error) {
      setDeleting(false);
      if (error instanceof UserFacingError) {
        setDeleteError(error.message);
        return;
      }
      if (error instanceof ConnectError) {
        if (error.code === Code.PermissionDenied) {
          setDeleteError('ワークスペースを削除する権限がありません。');
          return;
        }
        if (error.code === Code.NotFound) {
          setDeleteError('ワークスペースが見つかりませんでした。');
          return;
        }
        if (error.code === Code.Unavailable || error.code === Code.DeadlineExceeded) {
          setDeleteError('通信環境を確認して、再度お試しください。');
          return;
        }
        if (isExpectedConnectError(error)) {
          setDeleteError(formatConnectErrorMessage(error, 'ワークスペースの削除に失敗しました。'));
          return;
        }
      }
      console.error('Unexpected UI error', error);
      setDeleteError('ワークスペースの削除に失敗しました。');
    }
  }, [onDeleteWorkspace]);

  const isTreeMissing = !hasTree;
  const isExpanded = !isPopulated || isFailed;

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (!file) return;
    await handleUpload(file);
  };

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    if (!e.currentTarget.contains(e.relatedTarget as Node)) {
      setIsDragging(false);
    }
  }, []);

  const handleDrop = useCallback(async (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    const file = e.dataTransfer.files[0];
    if (!file) return;
    await handleUpload(file);
  }, [handleUpload]);

  return (
    <div
      className="flex h-full flex-col"
    >
      <input
        ref={fileInputRef}
        type="file"
        className="sr-only"
        onChange={handleFileChange}
        onClick={(e) => {
          (e.target as HTMLInputElement).value = '';
        }}
      />

      {/* Share control. Kept outside the populated/empty branches so it stays
          mounted regardless of isPopulated (which briefly flips to false while
          the tree re-projects), avoiding the button flickering in and out. */}
      <div className="flex justify-end gap-2 px-5 pt-3">
        {canManage && (confirmingDelete ? (
          <>
            <button
              type="button"
              disabled={deleting}
              onClick={() => setConfirmingDelete(false)}
              className="h-7 rounded-md border border-stone-200 bg-white px-2.5 text-[12px] text-stone-500 disabled:opacity-60"
            >
              キャンセル
            </button>
            <button
              type="button"
              disabled={deleting}
              onClick={() => void handleDeleteWorkspace()}
              className="h-7 rounded-md border border-red-200 bg-red-50 px-2.5 text-[12px] font-medium text-red-600 disabled:opacity-60"
            >
              {deleting ? '削除中…' : '削除を確定'}
            </button>
          </>
        ) : (
          <button
            type="button"
            onClick={() => setConfirmingDelete(true)}
            className="h-7 rounded-md border border-stone-200 bg-white px-2.5 text-[12px] text-stone-400 hover:border-red-200 hover:text-red-500"
          >
            削除
          </button>
        ))}
        {canManage && <button
          type="button"
          onClick={openShare}
          className="flex h-7 items-center gap-1.5 rounded-md border border-stone-200 bg-white px-2.5 text-[12px] font-medium text-stone-500 transition-colors hover:border-indigo-300 hover:text-indigo-500"
          title="共有"
        >
          <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 8a3 3 0 10-2.977-2.63l-4.94 2.47a3 3 0 100 4.319l4.94 2.47a3 3 0 10.895-1.789l-4.94-2.47a3.027 3.027 0 000-.74l4.94-2.47C13.456 7.68 14.19 8 15 8z" />
          </svg>
          共有
        </button>}
      </div>
      {deleteError && <InlineError message={deleteError} className="px-5 pt-2" />}
      {isSharing && (
        <SharePanel
          workspaceId={workspaceId}
          onClose={() => setIsSharing(false)}
        />
      )}

      {/* Compact header (populated mode only) */}
      {isPopulated && (
        <>
          <WorkspaceHeader
            workspaceName={workspaceName}
            draftName={draftName}
            editingName={editingName}
            savingName={savingName}
            childItemsCount={childItems.length}
            isRunning={isRunning}
            isFailed={isFailed}
            jobProgress={jobStatus?.progress}
            isJustCompleted={isJustCompleted}
            onStartRename={startRename}
            onDraftNameChange={setDraftName}
            onCommitName={commitName}
            onCancelRename={cancelRename}
            canEdit={canWrite}
          />
          {nameError && <InlineError message={nameError} className="-mt-2 px-5 pb-2" />}

          {/* Always-visible section. The workspace's single root node content
              is its cover report, rendered as a CSS-isolated iframe. data-paper-id
              links embedded in the content are clickable: the iframe forwards the
              click to the host, which opens the child paper. Any child not
              referenced in the content is appended as a related link. Recent jobs
              stay visible below regardless of hover state.

              The iframe is pulled out to the room's left/right edges (-mx-5) so
              the cover report fills the content width; its inner body padding
              keeps the text readable. The job list keeps the px-5 gutter. */}
          <div className="flex flex-col gap-4 px-5 pb-4">
            {rootContent && (
              <div className="-mx-5">
                <WorkspaceRootContent
                  nodeId={workspaceId}
                  content={rootContent}
                  overrideCss={rootOverrideCss}
                  childItems={childItems}
                />
              </div>
            )}
            <WorkspaceJobList workspaceJobs={workspaceJobs} />
          </div>
        </>
      )}

      {/* Expanded content */}
      {isExpanded && (
        <div className={['flex flex-col gap-5 overflow-y-auto', isPopulated ? 'px-5 pb-5' : 'flex-1 p-5'].join(' ')}>
          {!isPopulated && (
            <WorkspaceEmptyHeader
              workspaceName={workspaceName}
              editingName={editingName}
              draftName={draftName}
              savingName={savingName}
              nameError={nameError}
              onDraftNameChange={setDraftName}
              onStartRename={startRename}
              onCommitName={commitName}
              onCancelRename={cancelRename}
              canEdit={canWrite}
            />
          )}

          {/* Upload zone / add button */}
          {canWrite && <div className={!isPopulated ? 'flex flex-1 flex-col justify-center' : ''}>
            <WorkspaceDropzone
              isTreeMissing={isTreeMissing}
              hasChildItems={childItems.length > 0}
              uploading={uploading}
              activeJobId={activeJobId}
              isDragging={isDragging}
              jobStatusMessage={jobStatusError?.message ?? jobStatus?.errorMessage ?? jobStatus?.message}
              jobStatusProgress={jobStatus?.progress}
              jobStatusFailed={isJobFailed(jobStatus)}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onDrop={handleDrop}
              onClick={() => fileInputRef.current?.click()}
            />

            {uploadMessage && (
              <div className="mt-2.5 text-center">
                {uploadMessage.includes('失敗') || uploadMessage.includes('エラー') ? (
                  <InlineError message={uploadMessage} className="justify-center" />
                ) : (
                  <p className="text-[11px] text-indigo-500">
                    {uploadMessage}
                  </p>
                )}
              </div>
            )}

            {/* Progress: populated mode only (empty mode shows it inline inside the drop zone).
                Hide once succeeded so the bar disappears after completion; the
                completion toast / Recent jobs list takes over from there. */}
            {isPopulated && (jobStatusError || (jobStatus && !isJobSucceeded(jobStatus))) && (
              <WorkspaceJobProgress
                message={jobStatusError?.message ?? jobStatus?.errorMessage ?? jobStatus?.message}
                progress={jobStatus?.progress}
                isFailed={isJobFailed(jobStatus)}
                startedAt={jobStatus?.startedAt}
                completedAt={jobStatus?.completedAt}
                isTerminal={isJobTerminal(jobStatus)}
                latestActivity={jobStatus?.latestActivity}
              />
            )}
          </div>}
        </div>
      )}

      {/* Read-only Q&A over this workspace's processed documents. Placed below
          the upload control per docs/architecture/workspace-chat-design.md §1.
          It is not gated on canWrite: any workspace reader may ask, since
          asking changes nothing. */}
      <WorkspaceChatPanel workspaceId={workspaceId} />
    </div>
  );
}
