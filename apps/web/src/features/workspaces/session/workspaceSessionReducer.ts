import {
  WORKSPACE_COMPLETION_MESSAGE,
  WORKSPACE_UPLOAD_STARTED_MESSAGE,
  type WorkspaceSessionAction,
  type WorkspaceSessionState,
} from './workspaceSessionTypes';

export function workspaceSessionReducer(
  state: WorkspaceSessionState,
  action: WorkspaceSessionAction,
): WorkspaceSessionState {
  switch (action.type) {
    case 'workspace/propsSynced':
      return {
        ...state,
        rename: state.rename.editing
          ? state.rename
          : { ...state.rename, draftName: action.workspaceName },
      };

    case 'job/adopted':
      return {
        ...state,
        phase: action.reason === 'completedTreeChange' ? 'recovering' : 'processing',
        job: { ...state.job, activeJobId: action.jobId },
      };

    case 'job/suggestedWorkspaceNameObserved':
      return {
        ...state,
        job: {
          ...state.job,
          observedSuggestedWorkspaceName: action.suggestedWorkspaceName,
        },
      };

    case 'job/succeeded':
      return {
        ...state,
        phase: 'ready',
        job: {
          ...state.job,
          completedJobIds: {
            ...state.job.completedJobIds,
            [action.jobId]: true,
          },
        },
        upload: {
          ...state.upload,
          message: WORKSPACE_COMPLETION_MESSAGE,
        },
      };

    case 'upload/started':
      return {
        ...state,
        phase: 'uploading',
        upload: { ...state.upload, uploading: true, message: null },
      };

    case 'upload/enqueued':
      return {
        ...state,
        phase: 'processing',
        job: {
          ...state.job,
          activeJobId: action.jobId,
          observedSuggestedWorkspaceName: null,
        },
        upload: {
          uploading: false,
          message: WORKSPACE_UPLOAD_STARTED_MESSAGE,
        },
      };

    case 'upload/failed':
      return {
        ...state,
        phase: 'failed',
        upload: { uploading: false, message: action.message },
      };

    case 'upload/messageDismissed':
      return {
        ...state,
        upload: { ...state.upload, message: null },
      };

    case 'rename/started':
      return {
        ...state,
        rename: {
          ...state.rename,
          editing: true,
          draftName: action.currentName,
          error: null,
        },
      };

    case 'rename/inputChanged':
      return {
        ...state,
        rename: { ...state.rename, draftName: action.value },
      };

    case 'rename/saveStarted':
      return {
        ...state,
        rename: { ...state.rename, saving: true, error: null },
      };

    case 'rename/saveSucceeded':
      return {
        ...state,
        rename: {
          editing: false,
          draftName: action.name,
          saving: false,
          error: null,
        },
      };

    case 'rename/saveFailed':
      return {
        ...state,
        rename: { ...state.rename, saving: false, error: action.message },
      };

    case 'rename/cancelled':
      return {
        ...state,
        rename: {
          editing: false,
          draftName: action.currentName,
          saving: false,
          error: null,
        },
      };

    default:
      return state;
  }
}
