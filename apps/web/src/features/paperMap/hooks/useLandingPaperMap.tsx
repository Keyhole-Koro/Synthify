import { useMemo } from 'react';
import type { AuthMode } from '@/features/auth/AuthPaper';
import { AuthPaper } from '@/features/auth/AuthPaper';
import { WorkspaceListContent } from '@/features/paperMap/WorkspaceListContent';
import { BillingSummary } from '@/features/billing/BillingSummary';
import { CurrentPlanPaper } from '@/features/billing/CurrentPlanPaper';
import { BudgetSettingsPaper } from '@/features/billing/BudgetSettingsPaper';
import { UsagePaper } from '@/features/billing/UsagePaper';
import { UpgradePaper } from '@/features/billing/UpgradePaper';
import { ManagePaper } from '@/features/billing/ManagePaper';
import { InvoicePaper } from '@/features/billing/InvoicePaper';
import { AuthUser } from '@/features/auth/session';
import { Workspace } from '@/features/workspaces/api';
import { Paper, PaperMap } from '@keyhole-koro/paper-in-paper';

interface UseLandingPaperMapProps {
  user: AuthUser | null;
  loading: boolean;
  workspaces: Workspace[];
  workspaceError: Error | null;
  authMode: AuthMode;
  workspacePaperGroups: Map<string, Paper[]>;
  setAuthMode: (mode: AuthMode) => void;
  handleEmailSubmit: () => void;
  handleGoogleSubmit: () => void;
  handleLogout: () => void;
  handleCreateWorkspace: (name: string) => Promise<void>;
  handleOpenWorkspace: (workspaceId: string) => Promise<void>;
  buildWsPaper: (workspaceId: string, childPapers: { id: string; title: string }[]) => Paper;
}

export function useLandingPaperMap({
  user,
  loading,
  workspaces,
  workspaceError,
  authMode,
  workspacePaperGroups,
  setAuthMode,
  handleEmailSubmit,
  handleGoogleSubmit,
  handleLogout,
  handleCreateWorkspace,
  handleOpenWorkspace,
  buildWsPaper,
}: UseLandingPaperMapProps) {
  const paperMap = useMemo<PaperMap>(() => {
    const map = new Map<string, Paper>();

    const hasBilling = user != null && workspaces.length > 0;
    const rootChildIds = user ? ['auth', 'workspaces', ...(hasBilling ? ['billing'] : [])] : ['auth'];
    map.set('root', {
      id: 'root',
      title: 'Synthify',
      description: 'Document Intelligence Platform',
      hue: 220,
      parentId: null,
      childIds: rootChildIds,
      content: '<p>Synthify へようこそ。ドキュメントを知識構造へ変換します。</p>',
      layout: ({ openChildIds, focusedNodeId, paperMap: pm }) => {
        const focusInWorkspaces = (() => {
          if (!focusedNodeId) return false;
          let cursor: string | null = focusedNodeId;
          while (cursor) {
            if (cursor === 'workspaces') return true;
            cursor = pm.get(cursor)?.parentId ?? null;
          }
          return false;
        })();

        if (openChildIds.includes('workspaces')) {
          const contentShare = 0.06;
          const workspacesShare = focusInWorkspaces ? 0.88 : 0.78;
          const others = openChildIds.filter((id) => id !== 'workspaces');
          const evenOther = others.length > 0 ? (1 - workspacesShare - contentShare) / others.length : 0;
          const childShares: Record<string, number> = { workspaces: workspacesShare };
          for (const id of others) childShares[id] = evenOther;
          return { contentShare, childShares };
        }

        const contentShare = openChildIds.length > 0 ? 0.08 : 1;
        const childShare = openChildIds.length > 0 ? (1 - contentShare) / openChildIds.length : 0;
        const childShares: Record<string, number> = {};
        for (const id of openChildIds) childShares[id] = childShare;
        return { contentShare, childShares };
      },
    });

    map.set('auth', {
      id: 'auth',
      title: user ? 'アカウント' : 'ログイン',
      description: '認証とプロファイル',
      hue: 280,
      parentId: 'root',
      childIds: [],
      content: (
        <AuthPaper
          user={user}
          mode={authMode}
          loading={loading}
          onModeChange={setAuthMode}
          onEmailSubmit={handleEmailSubmit}
          onGoogleSubmit={handleGoogleSubmit}
          onLogout={handleLogout}
        />
      ),
    });

    if (user) {
      map.set('workspaces', {
        id: 'workspaces',
        title: 'ワークスペース',
        description: 'あなたのプロジェクト一覧',
        hue: 200,
        parentId: 'root',
        childIds: workspaces.map((w) => w.workspaceId),
        content: (
          <WorkspaceListContent
            workspaces={workspaces}
            loading={loading}
            error={workspaceError}
            onOpenWorkspace={handleOpenWorkspace}
            onCreateWorkspace={handleCreateWorkspace}
            onLogout={handleLogout}
          />
        ),
      });

      const accountId = workspaces[0]?.ownerId;
      if (accountId) {
        map.set('billing', {
          id: 'billing',
          title: 'プラン・課金',
          description: 'コントロールパネル',
          hue: 40,
          parentId: 'root',
          childIds: [
            'billing:plan',
            'billing:budget',
            'billing:usage',
            'billing:invoice',
            'billing:upgrade',
            'billing:manage',
          ],
          content: <BillingSummary accountId={accountId} />,
        });

        map.set('billing:plan', {
          id: 'billing:plan',
          title: '現在のプラン',
          description: 'プラン詳細とストレージ上限',
          hue: 40,
          parentId: 'billing',
          childIds: [],
          content: <CurrentPlanPaper accountId={accountId} />,
        });

        map.set('billing:budget', {
          id: 'billing:budget',
          title: '予算設定',
          description: '月次予算とアラート',
          hue: 40,
          parentId: 'billing',
          childIds: [],
          content: <BudgetSettingsPaper accountId={accountId} />,
        });

        map.set('billing:usage', {
          id: 'billing:usage',
          title: '使用量',
          description: '当月の LLM コスト内訳',
          hue: 40,
          parentId: 'billing',
          childIds: [],
          content: <UsagePaper accountId={accountId} />,
        });

        map.set('billing:invoice', {
          id: 'billing:invoice',
          title: '請求・支払い',
          description: '請求書・支払い方法・今月の請求予定額',
          hue: 40,
          parentId: 'billing',
          childIds: [],
          content: <InvoicePaper accountId={accountId} />,
        });

        map.set('billing:upgrade', {
          id: 'billing:upgrade',
          title: 'アップグレード',
          description: 'Usage-Based プランへ移行',
          hue: 40,
          parentId: 'billing',
          childIds: [],
          content: <UpgradePaper accountId={accountId} />,
        });

        map.set('billing:manage', {
          id: 'billing:manage',
          title: 'サブスクリプション管理',
          description: 'Stripe ポータルで変更・キャンセル',
          hue: 40,
          parentId: 'billing',
          childIds: [],
          content: <ManagePaper accountId={accountId} />,
        });
      }
    }

    if (user) {
      for (const ws of workspaces) {
        const workspacePapers = workspacePaperGroups.get(ws.workspaceId);
        if (workspacePapers && workspacePapers.length > 0) {
          for (const paper of workspacePapers) {
            map.set(paper.id, paper);
          }
        } else {
          map.set(ws.workspaceId, buildWsPaper(ws.workspaceId, []));
        }
      }
    }

    return map;
  }, [
    user, workspaces, workspaceError, authMode, loading,
    handleEmailSubmit, handleGoogleSubmit, handleLogout, handleCreateWorkspace,
    handleOpenWorkspace, buildWsPaper,
    workspacePaperGroups, setAuthMode,
  ]);

  return { paperMap };
}
