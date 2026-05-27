/* eslint-disable react-refresh/only-export-components */
import { useMemo, type CSSProperties, type ReactNode } from 'react';
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
import { AuthUser } from '@/features/auth/session';
import { Workspace } from '@/features/workspaces/api';
import { Paper, PaperMap } from '@keyhole-koro/paper-in-paper';
import { AUTH_ID, ROOT_ID, WORKSPACES_ID } from '@/features/paperMap/defaultOpenState';

import { type AppError } from '@/lib/errors';

interface UseLandingPaperMapProps {
  user: AuthUser | null;
  loading: boolean;
  workspaces: Workspace[];
  workspaceError: AppError | null;
  authError: AppError | null;
  workspacePaperGroups: Map<string, Paper[]>;
  handleGoogleSubmit: () => void;
  handleLogout: () => void;
  handleCreateWorkspace: (name: string) => Promise<void>;
  handleRootUpload: (file: File) => Promise<{ workspaceId: string; workspaceName: string; jobId: string; documentId: string }>;
  handleRootUploadComplete: (workspaceId: string) => Promise<void>;
  handleSuggestedWorkspaceName: (workspaceId: string, name: string) => Promise<void>;
  handleOpenWorkspace: (workspaceId: string) => Promise<void>;
  onRetryWorkspaces: () => void;
  buildWsPaper: (workspaceId: string, childPapers: { id: string; title: string }[]) => Paper;
}

const ABOUT_CHILD_IDS = [
  'synthify:about:promise',
  'synthify:about:fit',
  'synthify:overview',
  'synthify:documents',
  'synthify:worker',
  'synthify:tree',
  'synthify:operations',
];

const CATEGORY_HUES = {
  auth: 285,
  about: 212,
  overview: 220,
  documents: 175,
  worker: 292,
  tree: 132,
  operations: 24,
  workspaces: 202,
  billing: 48,
} as const;

const sectionStyle: CSSProperties = {
  display: 'grid',
  gap: 10,
  fontSize: '0.84rem',
  lineHeight: 1.65,
};

const eyebrowStyle: CSSProperties = {
  margin: 0,
  color: 'var(--accent)',
  fontSize: '0.66rem',
  fontWeight: 700,
  letterSpacing: '0.08em',
  textTransform: 'uppercase',
};

const titleStyle: CSSProperties = {
  margin: 0,
  color: 'var(--text)',
  fontSize: '1rem',
  fontWeight: 700,
  letterSpacing: 0,
};

const cardGridStyle: CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fit, minmax(148px, 1fr))',
  gap: 8,
};

const panelStyle: CSSProperties = {
  border: '1px solid var(--soft-line)',
  borderRadius: 8,
  background:
    'linear-gradient(135deg, color-mix(in srgb, var(--panel) 92%, white), color-mix(in srgb, var(--link-bg) 56%, white))',
  boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.72)',
};

function PaperSection({
  eyebrow,
  title,
  children,
}: {
  eyebrow: string;
  title: string;
  children: ReactNode;
}) {
  return (
    <section style={sectionStyle}>
      <div>
        <p style={eyebrowStyle}>{eyebrow}</p>
        <h2 style={titleStyle}>{title}</h2>
      </div>
      {children}
    </section>
  );
}

function PaperLinkCard({
  id,
  title,
  body,
  hue,
}: {
  id: string;
  title: string;
  body: string;
  hue?: number;
}) {
  const accent = hue === undefined ? 'var(--accent)' : `hsl(${hue}, 68%, 45%)`;
  const border = hue === undefined ? 'var(--link-border)' : `hsl(${hue}, 42%, 82%)`;
  const background = hue === undefined
    ? 'linear-gradient(135deg, color-mix(in srgb, var(--link-bg) 78%, white), color-mix(in srgb, var(--surface) 82%, white))'
    : `linear-gradient(135deg, hsl(${hue}, 72%, 96%), hsl(${hue}, 58%, 99%))`;

  return (
    <a
      data-paper-id={id}
      tabIndex={0}
      style={{
        display: 'grid',
        gap: 5,
        padding: '10px 11px',
        border: `1px solid ${border}`,
        borderRadius: 8,
        background,
        color: 'var(--text)',
        cursor: 'pointer',
        textDecoration: 'none',
        boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.74)',
      }}
    >
      <span style={{ display: 'flex', alignItems: 'center', gap: 6, color: accent, fontSize: '0.76rem', fontWeight: 700 }}>
        <span style={{ width: 6, height: 6, borderRadius: 99, background: 'currentColor', flexShrink: 0 }} />
        {title}
      </span>
      <span style={{ color: 'var(--muted)', fontSize: '0.76rem', lineHeight: 1.45 }}>{body}</span>
    </a>
  );
}

function PlainList({ items }: { items: string[] }) {
  return (
    <ul style={{ margin: 0, paddingLeft: 17, display: 'grid', gap: 5 }}>
      {items.map((item) => (
        <li key={item}>{item}</li>
      ))}
    </ul>
  );
}

function MetricRow({ label, value }: { label: string; value: string }) {
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: '86px minmax(0, 1fr)',
        gap: 8,
        padding: '6px 0',
        borderBottom: '1px solid var(--soft-line)',
      }}
    >
      <span style={{ color: 'var(--muted)', fontSize: '0.75rem' }}>{label}</span>
      <span style={{ fontSize: '0.8rem' }}>{value}</span>
    </div>
  );
}

function HeroPanel({
  title,
  body,
  metrics,
}: {
  title: string;
  body: string;
  metrics: { label: string; value: string }[];
}) {
  return (
    <div
      style={{
        ...panelStyle,
        display: 'grid',
        gap: 12,
        padding: 14,
        background:
          'radial-gradient(circle at 8% 0%, color-mix(in srgb, var(--accent) 24%, transparent), transparent 38%), linear-gradient(135deg, color-mix(in srgb, var(--surface-raised) 92%, white), var(--surface))',
      }}
    >
      <div style={{ display: 'grid', gap: 5 }}>
        <h3 style={{ margin: 0, fontSize: '0.98rem', lineHeight: 1.35 }}>{title}</h3>
        <p style={{ margin: 0, color: 'var(--muted)', fontSize: '0.8rem', lineHeight: 1.6 }}>{body}</p>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 7 }}>
        {metrics.map((metric) => (
          <div key={metric.label} style={{ borderLeft: '2px solid var(--accent)', paddingLeft: 8, minWidth: 0 }}>
            <p style={{ margin: 0, color: 'var(--accent)', fontSize: '0.68rem', fontWeight: 700 }}>{metric.label}</p>
            <p style={{ margin: '2px 0 0', color: 'var(--text)', fontSize: '0.76rem', lineHeight: 1.35 }}>{metric.value}</p>
          </div>
        ))}
      </div>
    </div>
  );
}

function StageRail({ stages }: { stages: { label: string; title: string; body: string }[] }) {
  return (
    <div style={{ display: 'grid', gap: 7 }}>
      {stages.map((stage, index) => (
        <div
          key={stage.label}
          style={{
            ...panelStyle,
            display: 'grid',
            gridTemplateColumns: '34px minmax(0, 1fr)',
            gap: 9,
            padding: '9px 10px',
            alignItems: 'start',
          }}
        >
          <div
            style={{
              display: 'grid',
              placeItems: 'center',
              width: 28,
              height: 28,
              borderRadius: 7,
              background: 'color-mix(in srgb, var(--accent) 15%, var(--surface))',
              color: 'var(--accent)',
              fontSize: '0.68rem',
              fontWeight: 800,
            }}
          >
            {String(index + 1).padStart(2, '0')}
          </div>
          <div>
            <p style={{ margin: 0, color: 'var(--accent)', fontSize: '0.68rem', fontWeight: 800, letterSpacing: '0.06em', textTransform: 'uppercase' }}>{stage.label}</p>
            <p style={{ margin: '1px 0 0', fontSize: '0.82rem', fontWeight: 700 }}>{stage.title}</p>
            <p style={{ margin: '2px 0 0', color: 'var(--muted)', fontSize: '0.75rem', lineHeight: 1.45 }}>{stage.body}</p>
          </div>
        </div>
      ))}
    </div>
  );
}

function SignalGrid({ items }: { items: { label: string; value: string }[] }) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(118px, 1fr))', gap: 7 }}>
      {items.map((item) => (
        <div key={item.label} style={{ ...panelStyle, padding: '9px 10px' }}>
          <p style={{ margin: 0, color: 'var(--accent)', fontSize: '0.68rem', fontWeight: 800 }}>{item.label}</p>
          <p style={{ margin: '4px 0 0', color: 'var(--text)', fontSize: '0.78rem', lineHeight: 1.45 }}>{item.value}</p>
        </div>
      ))}
    </div>
  );
}

function NodePreview() {
  return (
    <div style={{ ...panelStyle, padding: 12, display: 'grid', gap: 9 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 }}>
        <span style={{ color: 'var(--accent)', fontSize: '0.72rem', fontWeight: 800 }}>paper node</span>
        <span style={{ color: 'var(--muted)', fontSize: '0.68rem' }}>title / source / relation</span>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 7 }}>
        {[
          ['主張', '市場変化の仮説'],
          ['根拠', 'source chunk 12'],
          ['反論', '前提条件の制約'],
          ['関連', 'supports / measured_by'],
        ].map(([label, value]) => (
          <div key={label} style={{ border: '1px solid var(--soft-line)', borderRadius: 6, padding: '7px 8px', background: 'color-mix(in srgb, var(--surface) 78%, white)' }}>
            <p style={{ margin: 0, color: 'var(--accent)', fontSize: '0.66rem', fontWeight: 800 }}>{label}</p>
            <p style={{ margin: '2px 0 0', fontSize: '0.74rem', color: 'var(--muted)', lineHeight: 1.35 }}>{value}</p>
          </div>
        ))}
      </div>
    </div>
  );
}

function buildIntroCards(user: AuthUser | null, hasBilling: boolean) {
  return [
    { id: AUTH_ID, title: user ? 'アカウント' : 'ログイン', body: user ? '利用中のユーザーとセッション' : 'Google アカウントで開始', hue: CATEGORY_HUES.auth },
    { id: 'synthify:about', title: 'Synthifyについて', body: '価値、全体像、入力、処理、知識ツリー', hue: CATEGORY_HUES.about },
    ...(user ? [{ id: WORKSPACES_ID, title: 'ワークスペース', body: '資料と知識ツリーの管理', hue: CATEGORY_HUES.workspaces }] : []),
    ...(hasBilling ? [{ id: 'billing', title: 'プラン・課金', body: '予算、使用量、支払い', hue: CATEGORY_HUES.billing }] : []),
  ];
}

function addProductPapers(map: Map<string, Paper>) {
  const productPapers: Paper[] = [
    {
      id: 'synthify:about',
      title: 'Synthifyについて',
      description: 'ドキュメントを知識構造に変える理由',
      hue: CATEGORY_HUES.about,
      parentId: ROOT_ID,
      childIds: ABOUT_CHILD_IDS,
      content: (
        <PaperSection eyebrow="About" title="Synthifyは、資料を読む作業を探索可能にする">
          <HeroPanel
            title="要約ではなく、後から辿れる知識構造を作る"
            body="長い資料から概念、主張、根拠、反論を取り出し、paper node として接続します。読む人は結論だけでなく、根拠や関連論点まで同じ画面で開いて確認できます。"
            metrics={[
              { label: 'Problem', value: '長文資料が再利用しづらい' },
              { label: 'Method', value: 'AI worker が構造化' },
              { label: 'Result', value: '探索できる knowledge tree' },
            ]}
          />
          <div style={cardGridStyle}>
            <PaperLinkCard id="synthify:about:promise" title="提供価値" body="読む、検証する、共有する" />
            <PaperLinkCard id="synthify:about:fit" title="向いている用途" body="調査、仕様、研究、議事録" />
            <PaperLinkCard id="synthify:overview" title="全体像" body="入力から探索までの流れ" hue={CATEGORY_HUES.overview} />
            <PaperLinkCard id="synthify:documents" title="入力" body="ファイル、抽出、チャンク化" hue={CATEGORY_HUES.documents} />
            <PaperLinkCard id="synthify:worker" title="処理" body="worker と job lifecycle" hue={CATEGORY_HUES.worker} />
            <PaperLinkCard id="synthify:tree" title="知識ツリー" body="item、出典、関係リンク" hue={CATEGORY_HUES.tree} />
            <PaperLinkCard id="synthify:operations" title="運用" body="共有、ログ、使用量" hue={CATEGORY_HUES.operations} />
          </div>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:about:promise',
      title: '提供価値',
      description: '資料理解をチームで再利用できる形にする',
      hue: CATEGORY_HUES.about,
      parentId: 'synthify:about',
      childIds: [],
      content: (
        <PaperSection eyebrow="Value" title="読むだけで終わらせない">
          <SignalGrid
            items={[
              { label: '探索', value: '概念から根拠、反論、関連論点へ移動できる' },
              { label: '検証', value: 'source chunk を使って生成結果の由来を確認する' },
              { label: '共有', value: 'workspace 単位で同じ知識ツリーを扱う' },
            ]}
          />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:about:fit',
      title: '向いている用途',
      description: '複数回読み返す資料や複数人で扱う資料',
      hue: CATEGORY_HUES.about,
      parentId: 'synthify:about',
      childIds: [],
      content: (
        <PaperSection eyebrow="Use cases" title="あとから根拠を辿りたい資料に向いている">
          <PlainList
            items={[
              '調査レポートや市場分析の論点整理',
              '仕様書、設計書、RFC の理解とレビュー',
              '研究資料や論文メモの関係整理',
              '会議録やヒアリング記録からの知識化',
            ]}
          />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:overview',
      title: '全体像',
      description: '資料を知識キャンバスに変える流れ',
      hue: CATEGORY_HUES.overview,
      parentId: 'synthify:about',
      childIds: ['synthify:overview:workflow', 'synthify:overview:reading'],
      content: (
        <PaperSection eyebrow="Product" title="読む順番を固定しない調査環境">
          <StageRail
            stages={[
              { label: 'Input', title: '資料を入れる', body: 'workspace にファイルを置き、document と job を作る。' },
              { label: 'Process', title: 'AI worker が読む', body: 'chunk、brief、item、relation に変換する。' },
              { label: 'Explore', title: 'paper node で辿る', body: '全体像から根拠や反論へ開いて読む。' },
            ]}
          />
          <div style={cardGridStyle}>
            <PaperLinkCard id="synthify:overview:workflow" title="ワークフロー" body="upload から tree 探索まで" />
            <PaperLinkCard id="synthify:overview:reading" title="読み方" body="親子関係と横断リンクで読む" />
          </div>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:overview:workflow',
      title: '基本ワークフロー',
      description: 'workspace -> upload -> job -> tree',
      hue: CATEGORY_HUES.overview,
      parentId: 'synthify:overview',
      childIds: [],
      content: (
        <PaperSection eyebrow="Flow" title="処理はワークスペースから始まる">
          <StageRail
            stages={[
              { label: 'Workspace', title: '読みたい資料を置く', body: '資料、tree、member、billing の境界を作る。' },
              { label: 'Upload', title: 'document と job を作る', body: '保存先と処理状態を分け、UI は進捗を追う。' },
              { label: 'Synthesis', title: 'AI worker が構造化する', body: 'chunk、brief、item、relation を生成する。' },
              { label: 'Explore', title: 'paper node として読む', body: '文脈を保ったまま論点を開いていく。' },
            ]}
          />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:overview:reading',
      title: '探索体験',
      description: '文脈を保ったまま paper node を展開',
      hue: CATEGORY_HUES.overview,
      parentId: 'synthify:overview',
      childIds: [],
      content: (
        <PaperSection eyebrow="Reading" title="閉じたページではなく、広がる地図として読む">
          <NodePreview />
          <p style={{ margin: 0, color: 'var(--muted)' }}>
            開いた node は親の文脈の中に残るため、全体像から詳細へ進んでも、どこから来たかを見失いにくい構造です。
          </p>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:documents',
      title: 'ドキュメント入力',
      description: 'ファイルを処理可能な単位へ変換',
      hue: CATEGORY_HUES.documents,
      parentId: 'synthify:about',
      childIds: ['synthify:documents:intake', 'synthify:documents:chunks'],
      content: (
        <PaperSection eyebrow="Input" title="資料を worker が扱える形に整える">
          <SignalGrid
            items={[
              { label: 'Storage', value: 'アップロード先と内部処理URLを分ける' },
              { label: 'Document', value: 'ファイル単位の metadata と状態' },
              { label: 'Job', value: '非同期処理の進捗と結果' },
            ]}
          />
          <div style={cardGridStyle}>
            <PaperLinkCard id="synthify:documents:intake" title="取り込み" body="保存URLと document record" />
            <PaperLinkCard id="synthify:documents:chunks" title="分割" body="source chunk と出典追跡" />
          </div>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:documents:intake',
      title: '取り込み',
      description: 'アップロードURLと内部処理URLを分離',
      hue: CATEGORY_HUES.documents,
      parentId: 'synthify:documents',
      childIds: [],
      content: (
        <PaperSection eyebrow="Storage" title="保存と処理の境界を分ける">
          <MetricRow label="UI" value="ファイル選択、drag and drop、upload status" />
          <MetricRow label="API" value="document 作成、upload URL 発行、job 起動" />
          <MetricRow label="worker" value="内部URLまたは mount path から読み込み" />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:documents:chunks',
      title: 'チャンク分割',
      description: '根拠へ戻れる処理単位',
      hue: CATEGORY_HUES.documents,
      parentId: 'synthify:documents',
      childIds: [],
      content: (
        <PaperSection eyebrow="Chunks" title="長い資料を意味単位に分ける">
          <p style={{ margin: 0 }}>
            見出し、段落、ファイル境界を使って source chunk を作ります。tree item は chunk を参照するため、生成結果から元の資料断片に戻れる設計です。
          </p>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:worker',
      title: 'AI worker',
      description: '抽出・要約・構造化を実行',
      hue: CATEGORY_HUES.worker,
      parentId: 'synthify:about',
      childIds: ['synthify:worker:pipeline', 'synthify:worker:lifecycle'],
      content: (
        <PaperSection eyebrow="Worker" title="重い処理は job として非同期に進む">
          <StageRail
            stages={[
              { label: 'Extract', title: 'テキストと構造を取り出す', body: 'ファイルを読み、処理しやすい入力へ変換する。' },
              { label: 'Brief', title: '重要点を圧縮する', body: '長い文脈を後続処理に渡せる粒度へ整える。' },
              { label: 'Tree', title: 'item と relation を生成', body: '概念、主張、根拠、反論を接続する。' },
            ]}
          />
          <div style={cardGridStyle}>
            <PaperLinkCard id="synthify:worker:pipeline" title="pipeline" body="抽出から永続化まで" />
            <PaperLinkCard id="synthify:worker:lifecycle" title="lifecycle" body="queued/running/succeeded/failed" />
          </div>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:worker:pipeline',
      title: '処理 pipeline',
      description: 'tool 実行を組み合わせる',
      hue: CATEGORY_HUES.worker,
      parentId: 'synthify:worker',
      childIds: [],
      content: (
        <PaperSection eyebrow="Pipeline" title="同じ job の中で複数段階を進める">
          <PlainList
            items={[
              'テキスト抽出と正規化',
              'semantic chunk と brief の生成',
              '概念・主張・根拠・反論の構造化',
              'tree item と relation の永続化',
            ]}
          />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:worker:lifecycle',
      title: 'job lifecycle',
      description: '進捗、失敗、再取得の基準',
      hue: CATEGORY_HUES.worker,
      parentId: 'synthify:worker',
      childIds: [],
      content: (
        <PaperSection eyebrow="Status" title="UI に返すべき状態を job に集める">
          <MetricRow label="queued" value="処理待ち。アップロード後の初期状態" />
          <MetricRow label="running" value="進捗とメッセージを workspace paper に表示" />
          <MetricRow label="succeeded" value="tree を再取得し、新しい document root を開く" />
          <MetricRow label="failed" value="失敗理由を残し、再試行判断に使う" />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:tree',
      title: '知識ツリー',
      description: '概念・主張・根拠を paper node に投影',
      hue: CATEGORY_HUES.tree,
      parentId: 'synthify:about',
      childIds: ['synthify:tree:item', 'synthify:tree:sources', 'synthify:tree:links'],
      content: (
        <PaperSection eyebrow="Tree" title="API の item を paper node として読む">
          <NodePreview />
          <div style={cardGridStyle}>
            <PaperLinkCard id="synthify:tree:item" title="item model" body="title, description, content" />
            <PaperLinkCard id="synthify:tree:sources" title="source" body="根拠 chunk を追跡" />
            <PaperLinkCard id="synthify:tree:links" title="relation" body="supports / contradicts など" />
          </div>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:tree:item',
      title: 'item model',
      description: 'paper node の最小単位',
      hue: CATEGORY_HUES.tree,
      parentId: 'synthify:tree',
      childIds: [],
      content: (
        <PaperSection eyebrow="Item" title="一覧性と本文を分けて持つ">
          <MetricRow label="title" value="node header と breadcrumb に表示" />
          <MetricRow label="description" value="縮小表示や一覧で読める短い説明" />
          <MetricRow label="content" value="本文 HTML。data-paper-id で関連 node を開ける" />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:tree:sources',
      title: 'source chunk',
      description: '生成結果から元資料へ戻る',
      hue: CATEGORY_HUES.tree,
      parentId: 'synthify:tree',
      childIds: [],
      content: (
        <PaperSection eyebrow="Evidence" title="AI の出力に根拠の足場を残す">
          <p style={{ margin: 0 }}>
            各 item は source chunk ids を持てます。要約や主張がどの資料断片に由来するのかを辿れるため、後から検証する読み方に向いています。
          </p>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:tree:links',
      title: '関係リンク',
      description: '階層以外の読み筋',
      hue: CATEGORY_HUES.tree,
      parentId: 'synthify:tree',
      childIds: [],
      content: (
        <PaperSection eyebrow="Relations" title="親子関係だけでは表せないつながり">
          <PlainList
            items={[
              'supports: ある主張を支える根拠',
              'contradicts: 反論や制約になる情報',
              'measured_by: 指標やデータに接続する関係',
            ]}
          />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:operations',
      title: '共有と運用',
      description: 'ワークスペース、権限、ログ、使用量',
      hue: CATEGORY_HUES.operations,
      parentId: 'synthify:about',
      childIds: ['synthify:operations:workspace', 'synthify:operations:observability'],
      content: (
        <PaperSection eyebrow="Operations" title="探索体験と運用管理を同じ単位で扱う">
          <SignalGrid
            items={[
              { label: 'Workspace', value: '資料、tree、member の共有境界' },
              { label: 'Access', value: 'owner / editor / viewer で操作を分ける' },
              { label: 'Metering', value: 'LLM token とコストを説明可能にする' },
            ]}
          />
          <div style={cardGridStyle}>
            <PaperLinkCard id="synthify:operations:workspace" title="workspace" body="共有と権限の境界" />
            <PaperLinkCard id="synthify:operations:observability" title="observability" body="job log と使用量" />
          </div>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:operations:workspace',
      title: 'workspace',
      description: '資料と知識ツリーの共有単位',
      hue: CATEGORY_HUES.operations,
      parentId: 'synthify:operations',
      childIds: [],
      content: (
        <PaperSection eyebrow="Boundary" title="権限とデータのまとまり">
          <p style={{ margin: 0 }}>
            owner、editor、viewer のロールで操作範囲を分け、アップロード、閲覧、招待、管理を workspace 単位で扱います。
          </p>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:operations:observability',
      title: 'ログと使用量',
      description: '処理状況とコストの見える化',
      hue: CATEGORY_HUES.operations,
      parentId: 'synthify:operations',
      childIds: [],
      content: (
        <PaperSection eyebrow="Visibility" title="job と LLM 使用量を追跡する">
          <p style={{ margin: 0 }}>
            job log は処理の進行と失敗原因を残し、LLM metering はモデル、token、コストの把握に使います。探索だけでなく運用判断にも必要な層です。
          </p>
        </PaperSection>
      ),
    },
  ];

  for (const paper of productPapers) {
    map.set(paper.id, paper);
  }
}

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
  handleRootUploadComplete,
  handleSuggestedWorkspaceName,
  handleOpenWorkspace,
  onRetryWorkspaces,
  buildWsPaper,
}: UseLandingPaperMapProps) {
  const paperMap = useMemo<PaperMap>(() => {
    if (typeof window !== 'undefined' && (window as { __pipDebug?: boolean }).__pipDebug) {
      console.log('[pip-debug] useLandingPaperMap regenerate', { loading, userId: user?.id, workspacesCount: workspaces.length });
    }

    const map = new Map<string, Paper>();
    const hasBilling = user != null && workspaces.length > 0;
    const rootChildIds = [
      AUTH_ID,
      'synthify:about',
      ...(user ? [WORKSPACES_ID] : []),
      ...(hasBilling ? ['billing'] : []),
    ];

    map.set(ROOT_ID, {
      id: ROOT_ID,
      title: 'Synthify',
      description: 'Document Intelligence Platform',
      hue: CATEGORY_HUES.overview,
      parentId: null,
      childIds: rootChildIds,
      content: (
        <PaperSection eyebrow="Synthify" title="ドキュメントから、知識構造へ">
          <p style={{ margin: 0, color: 'var(--muted)' }}>
            まずはログイン、プロダクト説明、処理の仕組み、ワークスペースへ進めます。
          </p>
          <div style={cardGridStyle}>
            {buildIntroCards(user, hasBilling).map((card) => (
              <PaperLinkCard key={card.id} id={card.id} title={card.title} body={card.body} hue={card.hue} />
            ))}
          </div>
          {user && (
            <RootUploadPaper
              disabled={loading}
              onUpload={handleRootUpload}
              onProcessingComplete={handleRootUploadComplete}
              onSuggestedWorkspaceName={handleSuggestedWorkspaceName}
            />
          )}
        </PaperSection>
      ),
      layout: ({ openChildIds, focusedNodeId, paperMap: pm }) => {
        const focusInWorkspaces = (() => {
          if (!focusedNodeId) return false;
          let cursor: string | null = focusedNodeId;
          while (cursor) {
            if (cursor === WORKSPACES_ID) return true;
            cursor = pm.get(cursor)?.parentId ?? null;
          }
          return false;
        })();

        if (openChildIds.includes(WORKSPACES_ID)) {
          const contentShare = 0.06;
          const workspacesShare = focusInWorkspaces ? 0.64 : 0.52;
          const others = openChildIds.filter((id) => id !== WORKSPACES_ID);
          const evenOther = others.length > 0 ? (1 - workspacesShare - contentShare) / others.length : 0;
          const childShares: Record<string, number> = { [WORKSPACES_ID]: workspacesShare };
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

    addProductPapers(map);

    map.set(AUTH_ID, {
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
    });

    if (user) {
      map.set(WORKSPACES_ID, {
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
            onLogout={handleLogout}
          />
        ),
      });

      const accountId = workspaces[0]?.ownerId;
      if (accountId) {
        map.set('billing', {
          id: 'billing',
          title: 'プラン・課金',
          description: '予算、使用量、支払い管理',
          hue: CATEGORY_HUES.billing,
          parentId: ROOT_ID,
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
          hue: CATEGORY_HUES.billing,
          parentId: 'billing',
          childIds: [],
          content: <CurrentPlanPaper accountId={accountId} />,
        });

        map.set('billing:budget', {
          id: 'billing:budget',
          title: '予算設定',
          description: '月次予算とアラート',
          hue: CATEGORY_HUES.billing,
          parentId: 'billing',
          childIds: [],
          content: <BudgetSettingsPaper accountId={accountId} />,
        });

        map.set('billing:usage', {
          id: 'billing:usage',
          title: '使用量',
          description: '当月の LLM コスト内訳',
          hue: CATEGORY_HUES.billing,
          parentId: 'billing',
          childIds: [],
          content: <UsagePaper accountId={accountId} />,
        });

        map.set('billing:invoice', {
          id: 'billing:invoice',
          title: '請求・支払い',
          description: '請求書・支払い方法・今月の請求予定額',
          hue: CATEGORY_HUES.billing,
          parentId: 'billing',
          childIds: [],
          content: <InvoicePaper accountId={accountId} />,
        });

        map.set('billing:upgrade', {
          id: 'billing:upgrade',
          title: 'アップグレード',
          description: 'Usage-Based プランへ移行',
          hue: CATEGORY_HUES.billing,
          parentId: 'billing',
          childIds: [],
          content: <UpgradePaper accountId={accountId} />,
        });

        map.set('billing:manage', {
          id: 'billing:manage',
          title: 'サブスクリプション管理',
          description: 'Stripe ポータルで変更・キャンセル',
          hue: CATEGORY_HUES.billing,
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
    user, workspaces, workspaceError, authError, loading,
    handleGoogleSubmit, handleLogout, handleCreateWorkspace,
    handleRootUpload, handleRootUploadComplete, handleSuggestedWorkspaceName,
    handleOpenWorkspace, buildWsPaper, onRetryWorkspaces,
    workspacePaperGroups,
  ]);



  return { paperMap };
}
