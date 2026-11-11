import { renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { Paper } from '@keyhole-koro/paper-in-paper';
import type { AuthUser } from '@/features/auth/session';
import type { Workspace } from '@/features/workspaces/api';
import { AUTH_ID, ROOT_ID, WORKSPACES_ID } from '@/features/paperMap/defaultOpenState';
import { PRODUCT_PAPERS } from '@/features/paperMap/landing/productPapers';
import { useLandingPaperMap } from './useLandingPaperMap';

// The papers' contents are not rendered here; only the map is under test.
vi.mock('@/lib/observability/log', () => ({ log: { debug: vi.fn() } }));
vi.mock('@/features/auth/AuthPaper', () => ({ AuthPaper: () => null }));
vi.mock('@/features/paperMap/WorkspaceListContent', () => ({ WorkspaceListContent: () => null }));
vi.mock('@/features/paperMap/components/RootUploadPaper', () => ({ RootUploadPaper: () => null }));
vi.mock('@/features/billing/BillingSummary', () => ({ BillingSummary: () => null }));
vi.mock('@/features/billing/CurrentPlanPaper', () => ({ CurrentPlanPaper: () => null }));
vi.mock('@/features/billing/BudgetSettingsPaper', () => ({ BudgetSettingsPaper: () => null }));
vi.mock('@/features/billing/UsagePaper', () => ({ UsagePaper: () => null }));
vi.mock('@/features/billing/UpgradePaper', () => ({ UpgradePaper: () => null }));
vi.mock('@/features/billing/ManagePaper', () => ({ ManagePaper: () => null }));
vi.mock('@/features/billing/InvoicePaper', () => ({ InvoicePaper: () => null }));

const alice: AuthUser = { id: 'alice', email: 'alice@example.com', name: 'Alice', accountId: 'acc-1' };
const BILLING_IDS = ['billing', 'billing:plan', 'billing:budget', 'billing:usage', 'billing:invoice', 'billing:upgrade', 'billing:manage'];

const workspace = (workspaceId: string) => ({ workspaceId, name: workspaceId }) as unknown as Workspace;
const wsPaper = (workspaceId: string): Paper => ({
  id: workspaceId, title: workspaceId, description: '', content: '', parentId: WORKSPACES_ID, childIds: [],
});

type Props = Parameters<typeof useLandingPaperMap>[0];

function setup(overrides: Partial<Props> = {}) {
  const props: Props = {
    user: alice,
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
    buildWsPaper: (workspaceId) => wsPaper(workspaceId),
    ...overrides,
  };
  const hook = renderHook((p: Props) => useLandingPaperMap(p).paperMap, { initialProps: props });
  return { ...hook, props };
}

describe('useLandingPaperMap', () => {
  it('builds the landing tree for a signed-out visitor', () => {
    const { result } = setup({ user: null });
    const map = result.current;
    expect(map.get(ROOT_ID)?.childIds).toEqual([AUTH_ID, 'synthify:about']);
    expect(map.get(AUTH_ID)?.title).toBe('ログイン');
    for (const paper of PRODUCT_PAPERS) expect(map.get(paper.id)).toBe(paper);
    expect(map.has(WORKSPACES_ID)).toBe(false);
    expect(map.has('billing')).toBe(false);
  });

  it('adds workspaces and billing for a signed-in user with an account', () => {
    const { result } = setup({ workspaces: [workspace('ws-1')] });
    const map = result.current;
    expect(map.get(ROOT_ID)?.childIds).toEqual([AUTH_ID, 'synthify:about', WORKSPACES_ID, 'billing']);
    expect(map.get(WORKSPACES_ID)?.childIds).toEqual(['ws-1']);
    expect(map.get('billing')?.childIds).toEqual(BILLING_IDS.slice(1));
    for (const id of BILLING_IDS.slice(1)) expect(map.get(id)?.parentId).toBe('billing');
    // A workspace whose tree has not loaded yet gets a placeholder paper.
    expect(map.get('ws-1')).toEqual(wsPaper('ws-1'));
  });

  it('keeps every paper whose inputs did not change, so the canvas does not re-render it', () => {
    const { result, rerender, props } = setup();
    const before = result.current;

    // A workspace arrives: only the workspace list (and the new workspace) change.
    rerender({ ...props, workspaces: [workspace('ws-1')], workspacePaperGroups: new Map([['ws-1', [wsPaper('ws-1')]]]) });
    const after = result.current;
    expect(after).not.toBe(before);
    expect(after.get(WORKSPACES_ID)).not.toBe(before.get(WORKSPACES_ID));
    for (const id of [ROOT_ID, AUTH_ID, ...BILLING_IDS, ...PRODUCT_PAPERS.map((p) => p.id)]) {
      expect(after.get(id), id).toBe(before.get(id));
    }

    // Loading touches the papers that show it, and nothing else.
    rerender({ ...props, workspaces: [workspace('ws-1')], workspacePaperGroups: new Map([['ws-1', [wsPaper('ws-1')]]]), loading: true });
    const loading = result.current;
    expect(loading.get(ROOT_ID)).not.toBe(after.get(ROOT_ID));
    expect(loading.get(AUTH_ID)).not.toBe(after.get(AUTH_ID));
    for (const id of [...BILLING_IDS, ...PRODUCT_PAPERS.map((p) => p.id)]) {
      expect(loading.get(id), id).toBe(after.get(id));
    }
  });
});
