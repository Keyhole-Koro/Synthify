import { useQuery } from '@tanstack/react-query';
import { ConnectError, Code } from '@connectrpc/connect';
import { WorkspaceRole } from '@/features/workspaces/api';
import { resolveShareLinkQueryOptions } from '@/features/workspaces/queries';
import { ShareLinkTree } from './ShareLinkTree';
import { formatConnectErrorMessage, isExpectedConnectError, UserFacingError } from '@/lib/errors';

/**
 * Props for the public shared workspace viewer.
 */
interface ShareLinkViewerProps {
  /** Share link access token. */
  token: string;
}

/**
 * Maps a workspace role enum to a friendly badge label.
 */
function roleLabel(role: WorkspaceRole): string {
  switch (role) {
    case WorkspaceRole.OWNER:
      return 'オーナー';
    case WorkspaceRole.EDITOR:
      return 'Read & write';
    case WorkspaceRole.VIEWER:
    default:
      return 'Read-only';
  }
}

/**
 * Public full-page viewer rendered when someone navigates to a workspace via share link.
 *
 * @param props - Public share token from the route.
 * @returns Loading, error, or shared workspace viewer content.
 */
export function ShareLinkViewer({ token }: ShareLinkViewerProps) {
  const { data, isLoading, error } = useQuery(resolveShareLinkQueryOptions(token));

  if (isLoading) {
    return (
      <main className="flex min-h-screen items-center justify-center">
        <p className="text-sm text-slate-500">共有リンクを読み込んでいます…</p>
      </main>
    );
  }

  if (error || !data?.workspace) {
    let message = '共有リンクを開けませんでした。';
    if (error instanceof UserFacingError) {
      message = error.message;
    } else if (error instanceof ConnectError) {
      if (error.code === Code.NotFound) {
        message = 'この共有リンクは無効か、有効期限が切れています。';
      } else if (error.code === Code.PermissionDenied || error.code === Code.Unauthenticated) {
        message = 'この共有リンクにアクセスする権限がありません。';
      } else if (isExpectedConnectError(error)) {
        message = formatConnectErrorMessage(error, message);
      }
    } else if (error) {
      console.error('Unexpected UI error', error);
    }

    return (
      <main className="flex min-h-screen items-center justify-center px-4">
        <div className="max-w-md rounded-lg border border-red-200 bg-red-50 p-4 text-center text-sm text-red-700">
          {message}
        </div>
      </main>
    );
  }

  const { workspace, role } = data;
  return (
    <main className="mx-auto max-w-3xl px-6 py-12">
      <div className="mb-2 inline-flex items-center rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-600">
        共有リンク · {roleLabel(role)}
      </div>
      <h1 className="text-2xl font-semibold text-slate-900">{workspace.name}</h1>
      <p className="mt-3 text-sm text-slate-600">
        このワークスペースは共有リンクで公開されています。閲覧専用です。
      </p>
      <ShareLinkTree workspaceId={workspace.workspaceId} token={token} />
    </main>
  );
}
