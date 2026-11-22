import { ConnectError } from '@connectrpc/connect';
import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createAppError } from '@/lib/errors';
import { ShareLinkViewer } from './ShareLinkViewer';

const api = vi.hoisted(() => ({ resolveShareLink: vi.fn() }));

vi.mock('@/features/workspaces/api', () => ({
  WorkspaceRole: { UNSPECIFIED: 0, EDITOR: 1, VIEWER: 2, OWNER: 3 },
  resolveShareLink: api.resolveShareLink,
}));
vi.mock('./ShareLinkTree', () => ({ ShareLinkTree: () => null }));

// What the client call rejects with: the AppError lib/connect.ts builds,
// wrapped by connect-es in a ConnectError with code Unknown.
function rpcError(kind: 'not_found' | 'server', code: string, message: string) {
  return ConnectError.from(createAppError({ kind, code, message }));
}

describe('ShareLinkViewer', () => {
  beforeEach(() => {
    api.resolveShareLink.mockReset();
  });

  it('says a revoked or expired link is no longer valid', async () => {
    api.resolveShareLink.mockRejectedValue(rpcError('not_found', 'NotFound', 'not found'));
    render(<ShareLinkViewer token="revoked" />);
    expect(await screen.findByText('この共有リンクは無効か、有効期限が切れています。')).toBeTruthy();
    expect(screen.queryByText('not found')).toBeNull();
  });

  it('shows the server message for other failures', async () => {
    api.resolveShareLink.mockRejectedValue(rpcError('server', 'Internal', 'internal server error'));
    render(<ShareLinkViewer token="broken" />);
    expect(await screen.findByText('internal server error')).toBeTruthy();
  });
});
