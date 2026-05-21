import { useMemo } from 'react';
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
  workspacePaperGroups: Map<string, Paper[]>;
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
  workspacePaperGroups,
  handleGoogleSubmit,
  handleLogout,
  handleCreateWorkspace,
  handleOpenWorkspace,
  buildWsPaper,
}: UseLandingPaperMapProps) {
  const paperMap = useMemo<PaperMap>(() => {
    if (typeof window !== 'undefined' && (window as { __pipDebug?: boolean }).__pipDebug) {
      console.log('[pip-debug] useLandingPaperMap regenerate', { loading, userId: user?.id, workspacesCount: workspaces.length });
    }
    const map = new Map<string, Paper>();

    const hasBilling = user != null && workspaces.length > 0;
    const synthifyChildIds = [
      'synthify:overview',
      'synthify:documents',
      'synthify:worker',
      'synthify:tree',
      'synthify:collaboration',
    ];
    const rootChildIds = user
      ? ['auth', ...synthifyChildIds, 'workspaces', ...(hasBilling ? ['billing'] : [])]
      : ['auth', ...synthifyChildIds];
    map.set('root', {
      id: 'root',
      title: 'Synthify',
      description: 'Document Intelligence Platform',
      hue: 220,
      parentId: null,
      childIds: rootChildIds,
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>Synthify</h2>
          <div style={{ display: 'grid', gap: 8, fontSize: '0.85rem', lineHeight: 1.65 }}>
            <p style={{ margin: 0 }}>
              Synthify は、アップロードした資料を AI worker が読み解き、概念・主張・根拠・反論を paper-in-paper で辿れる知識構造へ変換するプロダクトです。
            </p>
            <div style={{ display: 'grid', gap: 6 }}>
              {[
                ['auth', user ? 'アカウント' : 'ログイン'],
                ['synthify:overview', 'プロダクト像'],
                ['synthify:documents', 'ドキュメント入力'],
                ['synthify:worker', 'AI worker'],
                ['synthify:tree', '知識ツリー'],
                ['synthify:collaboration', '共有と運用'],
              ].map(([id, label]) => (
                <a
                  key={id}
                  data-paper-id={id}
                  tabIndex={0}
                  style={{ color: 'var(--accent)', background: 'var(--link-bg)', border: '1px solid var(--link-border)', borderRadius: 4, padding: '5px 8px', cursor: 'pointer', textDecoration: 'none' }}
                >
                  {label}
                </a>
              ))}
            </div>
          </div>
        </section>
      ),
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

    map.set('synthify:overview', {
      id: 'synthify:overview',
      title: 'プロダクト像',
      description: 'ドキュメントを読むための知識キャンバス',
      hue: 220,
      parentId: 'root',
      childIds: ['synthify:overview:problem', 'synthify:overview:workflow'],
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>何をするものか</h2>
          <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
            Synthify は、長い資料を単なる要約で終わらせず、あとから探索できる知識ノードに分解します。各ノードは出典・関係・要約を持ち、読む順番を固定しない調査用の地図になります。
          </p>
        </section>
      ),
    });

    map.set('synthify:overview:problem', {
      id: 'synthify:overview:problem',
      title: '解く課題',
      description: '長文資料をあとから使える知識にする',
      hue: 220,
      parentId: 'synthify:overview',
      childIds: [],
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>要約だけでは足りない</h2>
          <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
            調査資料や仕様書は、一度読んで終わりではありません。後から「根拠はどこか」「反論は何か」「関連する概念は何か」を辿れる形にすることが Synthify の主眼です。
          </p>
        </section>
      ),
    });

    map.set('synthify:overview:workflow', {
      id: 'synthify:overview:workflow',
      title: '基本ワークフロー',
      description: 'upload -> job -> tree -> exploration',
      hue: 220,
      parentId: 'synthify:overview',
      childIds: [],
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>使い方の流れ</h2>
          <ol style={{ margin: 0, paddingLeft: 18, fontSize: '0.85rem', lineHeight: 1.7 }}>
            <li>ワークスペースを作る</li>
            <li>資料をアップロードする</li>
            <li>AI worker が知識ツリーを生成する</li>
            <li>paper node を開きながら読み進める</li>
          </ol>
        </section>
      ),
    });

    map.set('synthify:documents', {
      id: 'synthify:documents',
      title: 'ドキュメント入力',
      description: 'PDF・テキスト・メディアを処理対象にする',
      hue: 190,
      parentId: 'root',
      childIds: ['synthify:documents:storage', 'synthify:documents:chunking'],
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>入力から処理単位へ</h2>
          <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
            アップロードされたファイルは storage に保存され、document と job に紐づきます。worker はファイルを読み込み、テキスト抽出・チャンク分割・メタデータ化を行って、LLM が扱える単位へ整えます。
          </p>
        </section>
      ),
    });

    map.set('synthify:documents:storage', {
      id: 'synthify:documents:storage',
      title: '保存とURL',
      description: 'GCS URL と document record を分けて扱う',
      hue: 190,
      parentId: 'synthify:documents',
      childIds: [],
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>ファイルの置き場所</h2>
          <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
            API はアップロードURLを発行し、worker は内部URLや mount path 経由で処理します。外部向けURLと内部処理用URLを分けることで、ローカル・Cloud Run・emulator の差分を吸収します。
          </p>
        </section>
      ),
    });

    map.set('synthify:documents:chunking', {
      id: 'synthify:documents:chunking',
      title: 'チャンク分割',
      description: 'LLM が扱える粒度へ資料を分解',
      hue: 190,
      parentId: 'synthify:documents',
      childIds: [],
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>意味単位を作る</h2>
          <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
            長い入力は見出し、段落、ファイル境界を使って chunk に分けます。後続の tree item は source chunk を参照するので、生成結果から元の資料へ戻れます。
          </p>
        </section>
      ),
    });

    map.set('synthify:worker', {
      id: 'synthify:worker',
      title: 'AI worker',
      description: '抽出・要約・構造化を実行する非同期処理',
      hue: 260,
      parentId: 'root',
      childIds: ['synthify:worker:tools', 'synthify:worker:lifecycle'],
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>非同期パイプライン</h2>
          <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
            worker は job plan に従い、抽出、セマンティックチャンク、brief、knowledge tree 生成、永続化を進めます。LLM 使用量や tool 実行はログに残り、失敗時は job lifecycle が状態を更新します。
          </p>
        </section>
      ),
    });

    map.set('synthify:worker:tools', {
      id: 'synthify:worker:tools',
      title: 'tool 実行',
      description: 'builtin tool と dynamic tool を同じ形で扱う',
      hue: 260,
      parentId: 'synthify:worker',
      childIds: [],
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>Tool layer</h2>
          <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
            抽出、chunking、brief、tree 生成、persist などを tool として実行します。builtin と dynamic transform を共通の入出力 schema で扱い、worker の orchestration から呼び出します。
          </p>
        </section>
      ),
    });

    map.set('synthify:worker:lifecycle', {
      id: 'synthify:worker:lifecycle',
      title: 'job lifecycle',
      description: 'queued/running/completed/failed を管理',
      hue: 260,
      parentId: 'synthify:worker',
      childIds: [],
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>状態管理</h2>
          <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
            job は実行状態、失敗理由、通知 payload を持ちます。API は job を作り、worker が処理し、UI は進捗や結果を paper node として反映します。
          </p>
        </section>
      ),
    });

    map.set('synthify:tree', {
      id: 'synthify:tree',
      title: '知識ツリー',
      description: '概念・主張・根拠を paper node として展開',
      hue: 140,
      parentId: 'root',
      childIds: ['synthify:tree:item', 'synthify:tree:links'],
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>構造化された読み物</h2>
          <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
            生成された item は親子関係を持つ tree として保存されます。node ごとに title、description、HTML content、source chunk があり、関連する概念へ paper-in-paper 形式で広げながら読めます。
          </p>
        </section>
      ),
    });

    map.set('synthify:tree:item', {
      id: 'synthify:tree:item',
      title: 'item model',
      description: 'title, description, content, source chunks',
      hue: 140,
      parentId: 'synthify:tree',
      childIds: [],
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>Paper node の中身</h2>
          <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
            各 item は title と description で一覧性を持ち、content に HTML summary を持ちます。source chunk ids によって、AI がどの資料断片を根拠にしたかを追跡できます。
          </p>
        </section>
      ),
    });

    map.set('synthify:tree:links', {
      id: 'synthify:tree:links',
      title: '関係リンク',
      description: '階層だけでなく横断関係も表す',
      hue: 140,
      parentId: 'synthify:tree',
      childIds: [],
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>関連を辿る</h2>
          <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
            tree の親子関係は読み筋を作り、関連リンクは supports、contradicts、measured_by などの横断関係を表します。paper-in-paper はその両方を同じ操作で展開できます。
          </p>
        </section>
      ),
    });

    map.set('synthify:collaboration', {
      id: 'synthify:collaboration',
      title: '共有と運用',
      description: 'ワークスペース、権限、課金、ログ',
      hue: 35,
      parentId: 'root',
      childIds: ['synthify:collaboration:workspace', 'synthify:collaboration:observability'],
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>チームで使うための層</h2>
          <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
            ワークスペース単位で資料・メンバー・権限を管理します。閲覧履歴、LLM 使用量、請求、job log を分けて持つことで、探索体験と運用管理を同じプロダクト内に収めています。
          </p>
        </section>
      ),
    });

    map.set('synthify:collaboration:workspace', {
      id: 'synthify:collaboration:workspace',
      title: 'workspace',
      description: '資料と知識ツリーの共有単位',
      hue: 35,
      parentId: 'synthify:collaboration',
      childIds: [],
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>共有境界</h2>
          <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
            workspace は document、tree、member、billing の基準です。owner/editor/viewer の権限で、アップロード、閲覧、招待、管理の範囲を分けます。
          </p>
        </section>
      ),
    });

    map.set('synthify:collaboration:observability', {
      id: 'synthify:collaboration:observability',
      title: 'ログと使用量',
      description: 'job log と LLM metering',
      hue: 35,
      parentId: 'synthify:collaboration',
      childIds: [],
      content: (
        <section>
          <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>運用の見える化</h2>
          <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
            job log は処理の進行と失敗原因を残し、LLM metering はモデル・token・コストを記録します。探索体験だけでなく、運用上の説明責任も支えます。
          </p>
        </section>
      ),
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
          loading={loading}
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
    user, workspaces, workspaceError, loading,
    handleGoogleSubmit, handleLogout, handleCreateWorkspace,
    handleOpenWorkspace, buildWsPaper,
    workspacePaperGroups,
  ]);

  return { paperMap };
}
