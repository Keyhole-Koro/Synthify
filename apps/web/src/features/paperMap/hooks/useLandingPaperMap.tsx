import { useMemo } from 'react';
import { AuthPaper } from '@/features/auth/AuthPaper';
import { WorkspaceListContent } from '@/features/paperMap/WorkspaceListContent';
import { RootUploadPaper } from '@/features/paperMap/components/RootUploadPaper';
import { BillingSummary } from '@/features/billing/BillingSummary';
import { CurrentPlanPaper } from '@/features/billing/CurrentPlanPaper';
import { BudgetSettingsPaper } from '@/features/billing/BudgetSettingsPaper';
import { UsagePaper } from '@/features/billing/UsagePaper';
import { UpgradePaper } from '@/features/billing/UpgradePaper';
import { ManagePaper } from '@/features/billing/ManagePaper';
import { InvoicePaper } from '@/features/billing/InvoicePaper';
import type { AuthUser } from '@/features/auth/session';
import type { Workspace } from '@/features/workspaces/api';
import type { Paper, PaperMap } from '@keyhole-koro/paper-in-paper';
import { AUTH_ID, ROOT_ID, WORKSPACES_ID } from '@/features/paperMap/defaultOpenState';
import { PaperLinkCard, PaperSection } from '@/features/paperMap/landing/landingContent';
import { CATEGORY_HUES, cardGridStyle } from '@/features/paperMap/landing/landingTheme';
import { PRODUCT_PAPERS } from '@/features/paperMap/landing/productPapers';

import { log } from '@/lib/observability/log';

/**
 * Properties passed to the {@link useLandingPaperMap} hook to construct the root PaperMap.
 */
interface UseLandingPaperMapProps {
  /** The authenticated user, or null if unauthenticated. */
  user: AuthUser | null;
  /** Whether authentication or workspace data is actively loading. */
  loading: boolean;
  /** List of workspaces belonging to the authenticated user. */
  workspaces: Workspace[];
  /** Error encountered while fetching workspaces, if any. */
  workspaceError: Error | null;
  /** Error encountered during authentication or user session restoration, if any. */
  authError: Error | null;
  /** Map of workspace IDs to their corresponding pre-built paper lists. */
  workspacePaperGroups: Map<string, Paper[]>;
  /** Callback triggered to begin Google OAuth login. */
  handleGoogleSubmit: () => void;
  /** Callback triggered to log out the user. */
  handleLogout: () => void;
  /** Callback triggered to create a new workspace with the given name. */
  handleCreateWorkspace: (name: string) => Promise<Workspace | void>;
  /** Callback triggered to upload a document directly to the root landing paper. */
  handleRootUpload: (file: File) => Promise<void>;
  /** Callback triggered to open a selected workspace in the paper canvas. */
  handleOpenWorkspace: (workspaceId: string) => Promise<void>;
  /** Callback triggered to retry workspace loading on failure. */
  onRetryWorkspaces: () => void;
  /** Factory function to build an individual workspace paper instance. */
  buildWsPaper: (workspaceId: string, childPapers: { id: string; title: string }[]) => Paper;
}

/**
 * Builds the top-level navigation cards rendered inside the root landing paper.
 */
function buildIntroCards(signedIn: boolean, hasBilling: boolean) {
  return [
    { id: AUTH_ID, title: signedIn ? 'アカウント' : 'ログイン', body: signedIn ? '利用中のユーザーとセッション' : 'Google アカウントで開始', hue: CATEGORY_HUES.auth },
    { id: 'synthify:about', title: 'Synthifyについて', body: '何ができて、どう使うのか', hue: CATEGORY_HUES.about },
    ...(signedIn ? [{ id: WORKSPACES_ID, title: 'ワークスペース', body: '資料と知識ツリーの管理', hue: CATEGORY_HUES.workspaces }] : []),
    ...(hasBilling ? [{ id: 'billing', title: 'プラン・課金', body: '予算、使用量、支払い', hue: CATEGORY_HUES.billing }] : []),
  ];
}

const BILLING_CHILDREN = [
  { id: 'billing:plan', title: '現在のプラン', description: 'プラン詳細とストレージ上限', Content: CurrentPlanPaper },
  { id: 'billing:budget', title: '予算設定', description: '月次予算とアラート', Content: BudgetSettingsPaper },
  { id: 'billing:usage', title: '使用量', description: '当月の LLM コスト内訳', Content: UsagePaper },
  { id: 'billing:invoice', title: '請求・支払い', description: '請求書・支払い方法・今月の請求予定額', Content: InvoicePaper },
  { id: 'billing:upgrade', title: 'アップグレード', description: 'Usage-Based プランへ移行', Content: UpgradePaper },
  { id: 'billing:manage', title: 'サブスクリプション管理', description: 'Stripe ポータルで変更・キャンセル', Content: ManagePaper },
] as const;

/** The billing summary paper and its children, for one account. */
function buildBillingPapers(accountId: string): Paper[] {
  return [
    {
      id: 'billing',
      title: 'プラン・課金',
      description: '予算、使用量、支払い管理',
      hue: CATEGORY_HUES.billing,
      parentId: ROOT_ID,
      childIds: BILLING_CHILDREN.map((child) => child.id),
      content: <BillingSummary accountId={accountId} />,
    },
    ...BILLING_CHILDREN.map(({ id, title, description, Content }): Paper => ({
      id,
      title,
      description,
      hue: CATEGORY_HUES.billing,
      parentId: 'billing',
      childIds: [],
      content: <Content accountId={accountId} />,
    })),
  ];
}

/**
 * React hook that constructs the comprehensive `PaperMap` for the landing canvas,
 * encompassing the root overview paper, product explanation nodes, authentication card,
 * user workspaces, and billing/subscription management papers.
 *
 * Each paper (or group of papers) is rebuilt only when its own inputs change, and the static
 * explainer papers never are: the canvas re-renders a paper whenever it gets a new object for
 * it, so unchanged papers keep their objects.
 *
 * @param props - User session state, loaded workspaces, and interaction callbacks.
 * @returns Object providing the compiled `paperMap`.
 */
export function useLandingPaperMap({
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
  onRetryWorkspaces,
  buildWsPaper,
}: UseLandingPaperMapProps) {
  const signedIn = user !== null;
  const accountId = user?.accountId;
  // The root links to billing once there is an account id; its papers need a non-empty one.
  const hasBilling = signedIn && accountId != null;

  const rootPaper = useMemo((): Paper => ({
    id: ROOT_ID,
    title: 'Synthify',
    description: '資料を、あとから辿れる知識にする',
    hue: CATEGORY_HUES.overview,
    parentId: null,
    childIds: [
      AUTH_ID,
      'synthify:about',
      ...(signedIn ? [WORKSPACES_ID] : []),
      ...(hasBilling ? ['billing'] : []),
    ],
    content: (
      <PaperSection eyebrow="Synthify" title="資料を読んで、使える形で残す">
        <p style={{ margin: 0, color: 'var(--muted)' }}>
          Synthifyは、長い資料から論点や根拠を取り出し、あとから開いて辿れる地図のように整理します。まずはプロダクトの考え方を見るか、ワークスペースに資料を置いて始めてください。
        </p>
        <div style={cardGridStyle}>
          {buildIntroCards(signedIn, hasBilling).map((card) => (
            <PaperLinkCard key={card.id} id={card.id} title={card.title} body={card.body} hue={card.hue} />
          ))}
        </div>
        {signedIn && (
          <RootUploadPaper
            disabled={loading}
            onUpload={handleRootUpload}
          />
        )}
      </PaperSection>
    ),
  }), [signedIn, hasBilling, loading, handleRootUpload]);

  const authPaper = useMemo((): Paper => ({
    id: AUTH_ID,
    title: user ? 'アカウント' : 'ログイン',
    description: user ? '現在のセッションとプロフィール' : 'Synthify をはじめる',
    hue: CATEGORY_HUES.auth,
    parentId: ROOT_ID,
    childIds: [],
    content: (
      <AuthPaper
        user={user}
        loading={loading}
        error={authError}
        onGoogleSubmit={handleGoogleSubmit}
        onLogout={handleLogout}
      />
    ),
  }), [user, loading, authError, handleGoogleSubmit, handleLogout]);

  const workspacesPaper = useMemo((): Paper | null => (signedIn ? {
    id: WORKSPACES_ID,
    title: 'ワークスペース',
    description: '資料と知識ツリーの一覧',
    hue: CATEGORY_HUES.workspaces,
    parentId: ROOT_ID,
    childIds: workspaces.map((w) => w.workspaceId),
    content: (
      <WorkspaceListContent
        workspaces={workspaces}
        loading={loading}
        error={workspaceError}
        onOpenWorkspace={handleOpenWorkspace}
        onCreateWorkspace={handleCreateWorkspace}
        onRetry={onRetryWorkspaces}
      />
    ),
  } : null), [signedIn, workspaces, loading, workspaceError, handleOpenWorkspace, handleCreateWorkspace, onRetryWorkspaces]);

  const billingPapers = useMemo(
    () => (signedIn && accountId ? buildBillingPapers(accountId) : []),
    [signedIn, accountId],
  );

  const paperMap = useMemo<PaperMap>(() => {
    if (typeof window !== 'undefined' && (window as { __pipDebug?: boolean }).__pipDebug) {
      log.debug('[pip-debug] useLandingPaperMap regenerate', { source: 'pip_debug', loading, userId: user?.id, workspacesCount: workspaces.length });
    }

    const map = new Map<string, Paper>();
    map.set(ROOT_ID, rootPaper);
    for (const paper of PRODUCT_PAPERS) map.set(paper.id, paper);
    map.set(AUTH_ID, authPaper);
    if (workspacesPaper) map.set(WORKSPACES_ID, workspacesPaper);
    for (const paper of billingPapers) map.set(paper.id, paper);

    if (signedIn) {
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
    rootPaper, authPaper, workspacesPaper, billingPapers,
    signedIn, loading, user?.id, workspaces, workspacePaperGroups, buildWsPaper,
  ]);

  return { paperMap };
}
