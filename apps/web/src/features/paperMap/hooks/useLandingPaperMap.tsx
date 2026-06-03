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
import { log } from '@/lib/observability/log';

interface UseLandingPaperMapProps {
  user: AuthUser | null;
  loading: boolean;
  workspaces: Workspace[];
  workspaceError: AppError | null;
  authError: AppError | null;
  workspacePaperGroups: Map<string, Paper[]>;
  handleGoogleSubmit: () => void;
  handleLogout: () => void;
  handleCreateWorkspace: (name: string) => Promise<Workspace | void>;
  handleRootUpload: (file: File) => Promise<void>;
  handleOpenWorkspace: (workspaceId: string) => Promise<void>;
  onRetryWorkspaces: () => void;
  buildWsPaper: (workspaceId: string, childPapers: { id: string; title: string }[]) => Paper;
}

const ABOUT_CHILD_IDS = [
  'synthify:about:promise',
  'synthify:about:getting-started',
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
        <span style={{ color: 'var(--muted)', fontSize: '0.68rem' }}>見出し / 根拠 / つながり</span>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 7 }}>
        {[
          ['主張', '市場変化の仮説'],
          ['根拠', '資料の該当箇所'],
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
    { id: 'synthify:about', title: 'Synthifyについて', body: '何ができて、どう使うのか', hue: CATEGORY_HUES.about },
    ...(user ? [{ id: WORKSPACES_ID, title: 'ワークスペース', body: '資料と知識ツリーの管理', hue: CATEGORY_HUES.workspaces }] : []),
    ...(hasBilling ? [{ id: 'billing', title: 'プラン・課金', body: '予算、使用量、支払い', hue: CATEGORY_HUES.billing }] : []),
  ];
}

function addProductPapers(map: Map<string, Paper>) {
  const productPapers: Paper[] = [
    {
      id: 'synthify:about',
      title: 'Synthifyについて',
      description: '長い資料を、あとから辿れる形にする',
      hue: CATEGORY_HUES.about,
      parentId: ROOT_ID,
      childIds: ABOUT_CHILD_IDS,
      content: (
        <PaperSection eyebrow="About" title="Synthifyは、資料を読みっぱなしにしないための場所です">
          <HeroPanel
            title="要約だけで終わらせず、根拠まで戻れるようにする"
            body="資料を読んでいると、結論だけでなく「なぜそう言えるのか」「どの話とつながるのか」も残したくなります。Synthifyは、その手がかりを paper node として並べ、あとから開いて辿れるようにします。"
            metrics={[
              { label: 'Problem', value: '読んだ内容が散らばる' },
              { label: 'Method', value: '資料から論点を取り出す' },
              { label: 'Result', value: 'あとから辿れる地図にする' },
            ]}
          />
          <div style={cardGridStyle}>
            <PaperLinkCard id="synthify:about:promise" title="できること" body="読む、確かめる、共有する" />
            <PaperLinkCard id="synthify:about:getting-started" title="はじめかた" body="ログインして、資料を置く" />
            <PaperLinkCard id="synthify:about:fit" title="向いている資料" body="何度も読み返す資料" />
            <PaperLinkCard id="synthify:overview" title="全体の流れ" body="入れる、読ませる、辿る" hue={CATEGORY_HUES.overview} />
            <PaperLinkCard id="synthify:documents" title="資料の取り込み" body="アップロードして処理へ渡す" hue={CATEGORY_HUES.documents} />
            <PaperLinkCard id="synthify:worker" title="AIの処理" body="時間のかかる読み取りを任せる" hue={CATEGORY_HUES.worker} />
            <PaperLinkCard id="synthify:tree" title="知識ツリー" body="論点と根拠をつなぐ" hue={CATEGORY_HUES.tree} />
            <PaperLinkCard id="synthify:operations" title="共有と運用" body="チームで使い続ける" hue={CATEGORY_HUES.operations} />
          </div>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:about:promise',
      title: 'できること',
      description: '読んだ内容を、あとで使える形にする',
      hue: CATEGORY_HUES.about,
      parentId: 'synthify:about',
      childIds: [],
      content: (
        <PaperSection eyebrow="Value" title="読んだ後に、もう一度使える形で残す">
          <SignalGrid
            items={[
              { label: '探す', value: '気になる論点から、根拠や関連する話へ移動できる' },
              { label: '確かめる', value: 'AIが出した内容を、元の資料に戻って確認できる' },
              { label: '共有する', value: '同じ資料の理解を、チームで同じ形で見られる' },
            ]}
          />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:about:getting-started',
      title: 'はじめかた',
      description: 'まずは資料をひとつ置いてみる',
      hue: CATEGORY_HUES.about,
      parentId: 'synthify:about',
      childIds: [],
      content: (
        <PaperSection eyebrow="Start" title="最初は、小さな資料から始めるのがおすすめです">
          <StageRail
            stages={[
              { label: '1', title: 'Googleでログインする', body: 'アカウントを作ると、ワークスペースを使えるようになります。' },
              { label: '2', title: 'ワークスペースを作る', body: '調査テーマやプロジェクトごとに、資料を置く場所を分けます。' },
              { label: '3', title: '資料をアップロードする', body: 'まずはPDFやメモをひとつ置いて、処理の進み方を見ます。' },
              { label: '4', title: '知識ツリーを開く', body: '気になる論点から根拠へ戻りながら、資料を読み直します。' },
            ]}
          />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:about:fit',
      title: '向いている用途',
      description: '一度読んで終わりにしたくない資料',
      hue: CATEGORY_HUES.about,
      parentId: 'synthify:about',
      childIds: [],
      content: (
        <PaperSection eyebrow="Use cases" title="読み返すたびに迷子になりやすい資料に向いています">
          <PlainList
            items={[
              '調査レポートや市場分析を、あとから説明できる形にしたいとき',
              '仕様書や設計書を読み、レビューの論点を整理したいとき',
              '研究資料や論文メモのつながりを残したいとき',
              '会議録やヒアリングから、次に見るべき論点を拾いたいとき',
            ]}
          />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:overview',
      title: '全体像',
      description: '資料を入れてから、辿れる形になるまで',
      hue: CATEGORY_HUES.overview,
      parentId: 'synthify:about',
      childIds: ['synthify:overview:workflow', 'synthify:overview:reading'],
      content: (
        <PaperSection eyebrow="Product" title="最初から最後まで読むだけではなく、気になる場所から開いていけます">
          <StageRail
            stages={[
              { label: 'Input', title: '資料を入れる', body: '読みたいファイルをワークスペースに置きます。' },
              { label: 'Process', title: 'AIが読み解く', body: '大事な論点、根拠、関連する話を取り出します。' },
              { label: 'Explore', title: '地図のように辿る', body: '全体像から、根拠や反論へ開いて読めます。' },
            ]}
          />
          <div style={cardGridStyle}>
            <PaperLinkCard id="synthify:overview:workflow" title="ワークフロー" body="資料を入れてから読むまで" />
            <PaperLinkCard id="synthify:overview:reading" title="読み方" body="文脈を残したまま深掘りする" />
          </div>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:overview:workflow',
      title: '基本ワークフロー',
      description: '資料を置き、AIに読ませ、結果を辿る',
      hue: CATEGORY_HUES.overview,
      parentId: 'synthify:overview',
      childIds: [],
      content: (
        <PaperSection eyebrow="Flow" title="まず資料を置く場所を作ります">
          <StageRail
            stages={[
              { label: 'Workspace', title: '資料をまとめる場所を作る', body: 'プロジェクトや調査テーマごとに資料を分けます。' },
              { label: 'Upload', title: 'ファイルをアップロードする', body: '処理の進み具合は画面上で追えます。' },
              { label: 'Synthesis', title: 'AIが論点を組み立てる', body: '長い資料を、見出しや根拠のある単位へ整理します。' },
              { label: 'Explore', title: 'paper node として読む', body: '気になる論点を開きながら読み進めます。' },
            ]}
          />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:overview:reading',
      title: '探索体験',
      description: 'どこから読んでいたかを見失いにくい',
      hue: CATEGORY_HUES.overview,
      parentId: 'synthify:overview',
      childIds: [],
      content: (
        <PaperSection eyebrow="Reading" title="閉じたページではなく、広がる地図として読む">
          <NodePreview />
          <p style={{ margin: 0, color: 'var(--muted)' }}>
            詳細を開いても、元の文脈は画面に残ります。全体像を見ながら、必要なところだけ深掘りできます。
          </p>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:documents',
      title: '資料の取り込み',
      description: 'ファイルを読みやすい材料に変える',
      hue: CATEGORY_HUES.documents,
      parentId: 'synthify:about',
      childIds: ['synthify:documents:intake', 'synthify:documents:chunks'],
      content: (
        <PaperSection eyebrow="Input" title="まずは、資料を安心して預けられる形にします">
          <SignalGrid
            items={[
              { label: '保存', value: 'アップロードしたファイルを安全に保管する' },
              { label: '資料', value: 'ファイルごとに状態や情報を持つ' },
              { label: '処理', value: '読み取りの進み具合を追えるようにする' },
            ]}
          />
          <div style={cardGridStyle}>
            <PaperLinkCard id="synthify:documents:intake" title="取り込み" body="ファイルを受け取り、処理を始める" />
            <PaperLinkCard id="synthify:documents:chunks" title="分割" body="長い資料を根拠に戻れる単位へ" />
          </div>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:documents:intake',
      title: '取り込み',
      description: 'アップロードと読み取りを分けて扱う',
      hue: CATEGORY_HUES.documents,
      parentId: 'synthify:documents',
      childIds: [],
      content: (
        <PaperSection eyebrow="Storage" title="画面は軽く、重い処理は裏側で進めます">
          <MetricRow label="画面" value="ファイルを選ぶ、落とす、進捗を見る" />
          <MetricRow label="API" value="保存先を用意し、処理を開始する" />
          <MetricRow label="worker" value="保存された資料を読み、構造化する" />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:documents:chunks',
      title: 'チャンク分割',
      description: '元の資料へ戻れる小さな単位',
      hue: CATEGORY_HUES.documents,
      parentId: 'synthify:documents',
      childIds: [],
      content: (
        <PaperSection eyebrow="Chunks" title="長い資料を意味単位に分ける">
          <p style={{ margin: 0 }}>
            見出しや段落を手がかりに、長い資料を小さな断片に分けます。あとでAIの出力を見たときに、どの部分の資料から来た話なのかを辿れるようにするためです。
          </p>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:worker',
      title: 'AI worker',
      description: '時間のかかる読み取りを担当する',
      hue: CATEGORY_HUES.worker,
      parentId: 'synthify:about',
      childIds: ['synthify:worker:pipeline', 'synthify:worker:lifecycle'],
      content: (
        <PaperSection eyebrow="Worker" title="長い資料を読む仕事は、裏側のworkerに任せます">
          <StageRail
            stages={[
              { label: 'Extract', title: '本文を取り出す', body: 'ファイルから読めるテキストと構造を取り出します。' },
              { label: 'Brief', title: '重要点をつかむ', body: '長い文脈を、次に扱いやすい粒度へ整えます。' },
              { label: 'Tree', title: '論点をつなぐ', body: '概念、主張、根拠、反論の関係を作ります。' },
            ]}
          />
          <div style={cardGridStyle}>
            <PaperLinkCard id="synthify:worker:pipeline" title="処理の流れ" body="抽出から保存まで" />
            <PaperLinkCard id="synthify:worker:lifecycle" title="処理状態" body="待機中、実行中、完了、失敗" />
          </div>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:worker:pipeline',
      title: '処理の流れ',
      description: '資料を少しずつ読みやすい形へ変える',
      hue: CATEGORY_HUES.worker,
      parentId: 'synthify:worker',
      childIds: [],
      content: (
        <PaperSection eyebrow="Pipeline" title="一度に全部を決めず、段階ごとに整えます">
          <PlainList
            items={[
              'まず本文を取り出し、読みやすい形に整える',
              '長い資料を意味のあるまとまりに分ける',
              '概念、主張、根拠、反論を見つける',
              'あとから開ける paper node として保存する',
            ]}
          />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:worker:lifecycle',
      title: '処理状態',
      description: '今どこまで進んでいるかを見る',
      hue: CATEGORY_HUES.worker,
      parentId: 'synthify:worker',
      childIds: [],
      content: (
        <PaperSection eyebrow="Status" title="待っている間も、何が起きているか分かるようにします">
          <MetricRow label="queued" value="処理待ち。アップロード直後の状態です" />
          <MetricRow label="running" value="読み取り中。進捗とメッセージを表示します" />
          <MetricRow label="succeeded" value="完了。新しい知識ツリーを開けます" />
          <MetricRow label="failed" value="失敗。理由を残し、やり直しの判断に使います" />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:tree',
      title: '知識ツリー',
      description: '論点と根拠を、開ける形で並べる',
      hue: CATEGORY_HUES.tree,
      parentId: 'synthify:about',
      childIds: ['synthify:tree:item', 'synthify:tree:sources', 'synthify:tree:links'],
      content: (
        <PaperSection eyebrow="Tree" title="資料の中身を、紙片のように開いて読めます">
          <NodePreview />
          <div style={cardGridStyle}>
            <PaperLinkCard id="synthify:tree:item" title="paper node" body="ひとつの論点や根拠" />
            <PaperLinkCard id="synthify:tree:sources" title="出典" body="元資料へ戻る手がかり" />
            <PaperLinkCard id="synthify:tree:links" title="つながり" body="支える、反論する、関連する" />
          </div>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:tree:item',
      title: 'paper node',
      description: '知識ツリーの小さな読み物',
      hue: CATEGORY_HUES.tree,
      parentId: 'synthify:tree',
      childIds: [],
      content: (
        <PaperSection eyebrow="Item" title="短い見出しと本文を分けて持ちます">
          <MetricRow label="title" value="何についての紙片かを示す見出し" />
          <MetricRow label="description" value="一覧で読める短い説明" />
          <MetricRow label="content" value="詳しい本文。関連する紙片へも移動できます" />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:tree:sources',
      title: '出典',
      description: 'AIの出力から元資料へ戻る',
      hue: CATEGORY_HUES.tree,
      parentId: 'synthify:tree',
      childIds: [],
      content: (
        <PaperSection eyebrow="Evidence" title="AI の出力に根拠の足場を残す">
          <p style={{ margin: 0 }}>
            要約や主張だけを見ると、どこまで信じてよいか判断しづらくなります。Synthifyでは、元の資料のどのあたりから来た話なのかを辿れるようにしておきます。
          </p>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:tree:links',
      title: 'つながり',
      description: '親子関係だけではない読み筋',
      hue: CATEGORY_HUES.tree,
      parentId: 'synthify:tree',
      childIds: [],
      content: (
        <PaperSection eyebrow="Relations" title="話と話の関係も残します">
          <PlainList
            items={[
              'ある主張を支える根拠',
              '反論や制約になる情報',
              '指標やデータに接続する関係',
            ]}
          />
        </PaperSection>
      ),
    },
    {
      id: 'synthify:operations',
      title: '共有と運用',
      description: 'チームで使い続けるための仕組み',
      hue: CATEGORY_HUES.operations,
      parentId: 'synthify:about',
      childIds: ['synthify:operations:workspace', 'synthify:operations:observability'],
      content: (
        <PaperSection eyebrow="Operations" title="資料を読む体験だけでなく、使い続けるための管理も持ちます">
          <SignalGrid
            items={[
              { label: 'Workspace', value: '資料と知識ツリーをまとめる単位' },
              { label: 'Access', value: '役割ごとにできる操作を分ける' },
              { label: 'Metering', value: 'AIの使用量とコストを見えるようにする' },
            ]}
          />
          <div style={cardGridStyle}>
            <PaperLinkCard id="synthify:operations:workspace" title="ワークスペース" body="共有と権限のまとまり" />
            <PaperLinkCard id="synthify:operations:observability" title="ログと使用量" body="処理とコストを見える化" />
          </div>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:operations:workspace',
      title: 'ワークスペース',
      description: '資料と知識ツリーの共有単位',
      hue: CATEGORY_HUES.operations,
      parentId: 'synthify:operations',
      childIds: [],
      content: (
        <PaperSection eyebrow="Boundary" title="同じテーマの資料を、同じ場所で扱います">
          <p style={{ margin: 0 }}>
            誰が資料を追加できるか、誰が読めるか、誰を招待できるかをワークスペース単位で分けます。調査テーマやプロジェクトごとに切り分けやすい形です。
          </p>
        </PaperSection>
      ),
    },
    {
      id: 'synthify:operations:observability',
      title: 'ログと使用量',
      description: '処理の様子とコストを見る',
      hue: CATEGORY_HUES.operations,
      parentId: 'synthify:operations',
      childIds: [],
      content: (
        <PaperSection eyebrow="Visibility" title="裏側で何が起きたかを後から確認できます">
          <p style={{ margin: 0 }}>
            処理がどこまで進んだか、どこで失敗したか、どのくらいAIを使ったかを残します。うまくいかなかった時の確認や、予算管理に使います。
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
  handleOpenWorkspace,
  onRetryWorkspaces,
  buildWsPaper,
}: UseLandingPaperMapProps) {
  const paperMap = useMemo<PaperMap>(() => {
    if (typeof window !== 'undefined' && (window as { __pipDebug?: boolean }).__pipDebug) {
      log.debug('[pip-debug] useLandingPaperMap regenerate', { source: 'pip_debug', loading, userId: user?.id, workspacesCount: workspaces.length });
    }

    const map = new Map<string, Paper>();
    const accountId = user?.accountId;
    const hasBilling = user != null && accountId != null;
    const rootChildIds = [
      AUTH_ID,
      'synthify:about',
      ...(user ? [WORKSPACES_ID] : []),
      ...(hasBilling ? ['billing'] : []),
    ];

    map.set(ROOT_ID, {
      id: ROOT_ID,
      title: 'Synthify',
      description: '資料を、あとから辿れる知識にする',
      hue: CATEGORY_HUES.overview,
      parentId: null,
      childIds: rootChildIds,
      content: (
        <PaperSection eyebrow="Synthify" title="資料を読んで、使える形で残す">
          <p style={{ margin: 0, color: 'var(--muted)' }}>
            Synthifyは、長い資料から論点や根拠を取り出し、あとから開いて辿れる地図のように整理します。まずはプロダクトの考え方を見るか、ワークスペースに資料を置いて始めてください。
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
          />
        ),
      });

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
    handleRootUpload,
    handleOpenWorkspace, buildWsPaper, onRetryWorkspaces,
    workspacePaperGroups,
  ]);



  return { paperMap };
}
