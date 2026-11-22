import { render, renderHook, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ROOT_ID } from '@/features/paperMap/defaultOpenState';
import { useLandingPaperMap } from './useLandingPaperMap';

// The paper map imports every paper's component; only the root content is
// rendered here, so their API modules just need to load.
vi.mock('@/features/workspaces/api', () => ({ canSeedDevWorkspace: () => false, seedDevWorkspace: vi.fn() }));
vi.mock('@/features/billing/api', () => ({}));
vi.mock('@/features/auth/userApi', () => ({ deleteAccount: vi.fn() }));
vi.mock('@/lib/observability/log', () => ({ log: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() } }));

function buildPaperMap(intakeClosed: boolean) {
  const { result } = renderHook(() => useLandingPaperMap({
    user: { id: 'owner', email: 'owner@example.test', name: 'Owner', accountId: 'acc' },
    loading: false,
    workspaces: [],
    workspaceError: null,
    authError: null,
    workspacePaperGroups: new Map(),
    handleGoogleSubmit: vi.fn(),
    handleLogout: vi.fn(),
    handleCreateWorkspace: vi.fn(),
    handleRootUpload: vi.fn(),
    handleOpenWorkspace: vi.fn(),
    onRetryWorkspaces: vi.fn(),
    buildWsPaper: vi.fn(),
    intakeClosed,
  }));
  return result.current.paperMap;
}

describe('useLandingPaperMap', () => {
  it('offers the root upload drop to a signed-in user', () => {
    const paperMap = buildPaperMap(false);
    render(<>{paperMap.get(ROOT_ID)?.content}</>);
    expect(screen.getByText('ファイルをアップロード')).toBeTruthy();
    expect(paperMap.get('billing:upgrade')?.title).toBe('アップグレード');
  });

  it('drops the root upload drop and the upgrade offer once intake is closed', () => {
    const paperMap = buildPaperMap(true);
    render(<>{paperMap.get(ROOT_ID)?.content}</>);
    expect(screen.queryByText('ファイルをアップロード')).toBeNull();
    expect(paperMap.get('billing:upgrade')?.title).toBe('有料プラン');
    // Everything else the account had stays.
    expect(paperMap.has('billing:manage')).toBe(true);
  });
});
