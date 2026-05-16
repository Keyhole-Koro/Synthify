'use client';

import { usePaperStoreSelector } from '@keyhole-koro/paper-in-paper';
import type { Paper } from '@keyhole-koro/paper-in-paper';

export const ROOT_ID = 'root';

const linkStyle: React.CSSProperties = {
  color: 'var(--accent)',
  background: 'var(--link-bg)',
  border: '1px solid var(--link-border)',
  borderRadius: 4,
  cursor: 'pointer',
  textDecoration: 'none',
  fontSize: 'inherit',
};

// Paper link — triggers inline expansion on click via data-paper-id
export function PL({ id, children, variant }: { id: string; children?: React.ReactNode; variant?: 'card' }) {
  const paper = usePaperStoreSelector(({ state }) => state.paperMap.get(id));

  if (variant === 'card') {
    return (
      <a
        data-paper-id={id}
        tabIndex={0}
        style={{ display: 'block', border: '1px solid var(--link-border)', borderRadius: 8, padding: '10px 12px', background: 'var(--link-bg)', cursor: 'pointer', textDecoration: 'none' }}
      >
        <p style={{ margin: '0 0 4px', fontSize: '0.7rem', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--accent)' }}>
          {paper?.title ?? id}
        </p>
        <p style={{ margin: 0, fontSize: '0.78rem', lineHeight: 1.55 }}>{paper?.description}</p>
      </a>
    );
  }

  return (
    <a data-paper-id={id} tabIndex={0} style={{ ...linkStyle, display: 'inline', padding: '1px 5px' }}>
      {children ?? paper?.title ?? id}
    </a>
  );
}

// --- Leaf papers ---

export const canonicalizationPaper: Paper = {
  id: 'canonicalization',
  title: 'エイリアス正規化',
  description: '同義語・表記揺れを同一アイテムに統合',
  hue: 200,
  parentId: 'extraction',
  childIds: [],
  content: (
    <section>
      <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>正規化の仕組み</h2>
      <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
        Gemini が候補を提案し、コサイン類似度 + 人手ルールで同義語を一つの canonical アイテムに統合します。元の document 出典は参照として残ります。
      </p>
    </section>
  ),
};

export const depthPaper: Paper = {
  id: 'depth',
  title: '抽出深度',
  description: '詳細 vs 要約の2モード',
  hue: 200,
  parentId: 'extraction',
  childIds: [],
  content: (
    <section>
      <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>抽出深度の選択</h2>
      <div style={{ display: 'grid', gap: 8 }}>
        <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
          <strong>詳細</strong>：全チャンクを処理し豊富なツリーを生成（時間がかかる）。
        </p>
        <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
          <strong>要約のみ</strong>：高速だが粗めの構造。プロトタイプ確認に最適。
        </p>
      </div>
    </section>
  ),
};

export const hierarchyPaper: Paper = {
  id: 'hierarchy',
  title: '階層構造',
  description: 'ツリーを決定する親子関係',
  hue: 150,
  parentId: 'tree',
  childIds: [],
  content: (
    <section>
      <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>hierarchical な関係</h2>
      <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
        親子関係を表し、paper-in-paper のキャンバスツリーを決定します。ルートアイテム（level 0）から深くなるほど詳細な概念になります。
      </p>
    </section>
  ),
};

export const crosslinksPaper: Paper = {
  id: 'crosslinks',
  title: '関連リンク',
  description: '階層を超えたアイテム間の関係',
  hue: 150,
  parentId: 'tree',
  childIds: [],
  content: (
    <section>
      <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>非階層リンク</h2>
      <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
        supports・contradicts・measured_by など。HTMLサマリ内の data-paper-id リンクとして埋め込まれ、クリックで対象アイテムが展開されます。
      </p>
    </section>
  ),
};

export const datalinkPaper: Paper = {
  id: 'datalink',
  title: 'data-paper-id リンク',
  description: 'HTMLリンクがアイテム展開をトリガー',
  hue: 265,
  parentId: 'explore',
  childIds: [],
  content: (
    <section>
      <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>仕組み</h2>
      <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
        ペーパーの HTML に{' '}
        <code style={{ fontSize: '0.8em', background: 'var(--surface-alt)', padding: '1px 4px', borderRadius: 3 }}>
          {'<a data-paper-id="item_id">'}
        </code>
        {' '}を埋め込むと、クリック時に対象アイテムが子として展開されます。関連リンクもこの仕組みで再現されます。
      </p>
    </section>
  ),
};

export const focusmodePaper: Paper = {
  id: 'focusmode',
  title: 'フォーカスモード',
  description: '1つのアイテムに集中して読む',
  hue: 265,
  parentId: 'explore',
  childIds: [],
  content: (
    <section>
      <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>フォーカスパネル</h2>
      <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
        アイテムを選択するとサイドパネルが開き、ソースチャンク・関連リンク・HTMLサマリを詳しく確認できます。閲覧履歴にも自動記録されます。
      </p>
    </section>
  ),
};

export const viewhistoryPaper: Paper = {
  id: 'viewhistory',
  title: '閲覧履歴',
  description: 'ユーザーごとの探索状況を追跡',
  hue: 20,
  parentId: 'team',
  childIds: [],
  content: (
    <section>
      <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>user_item_views</h2>
      <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
        アイテムを開くたびに first_viewed_at・last_viewed_at・view_count が記録されます。チームで誰がどの概念を探索したかが一目で分かります。
      </p>
    </section>
  ),
};

export const invitePaper: Paper = {
  id: 'invite',
  title: 'メンバー招待',
  description: 'メールアドレスで招待・ロール設定',
  hue: 20,
  parentId: 'team',
  childIds: [],
  content: (
    <section>
      <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>招待フロー</h2>
      <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
        オーナーがメールアドレスとロールを指定して招待。is_dev フラグを付けると開発者モードが有効になり、内部メタデータへのアクセスが解放されます。
      </p>
    </section>
  ),
};

// --- Branch papers ---

export const authPaper: Paper = {
  id: 'auth',
  title: 'アカウント',
  description: 'Synthify をはじめる',
  hue: 250,
  parentId: ROOT_ID,
  childIds: [],
  content: null,
};

export const workspacesPaper: Paper = {
  id: 'workspaces',
  title: 'ワークスペース',
  description: 'あなたのワークスペース一覧',
  hue: 200,
  parentId: ROOT_ID,
  childIds: [],
  content: null,
};

export const extractionPaper: Paper = {
  id: 'extraction',
  title: 'AI による概念抽出',
  description: 'Geminiがドキュメントを6ステージで解析',
  hue: 215,
  parentId: ROOT_ID,
  childIds: [canonicalizationPaper.id, depthPaper.id],
  content: (
    <section>
      <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>6ステージ パイプライン</h2>
      <div style={{ display: 'grid', gap: 8 }}>
        <ul style={{ margin: 0, paddingLeft: 16, lineHeight: 1.8, fontSize: '0.85rem' }}>
          <li>テキスト正規化・チャンク分割</li>
          <li>エンティティ・概念の抽出</li>
          <li><PL id={canonicalizationPaper.id} /></li>
          <li>親子関係の推論</li>
          <li>重要度スコアリング</li>
          <li>HTMLサマリ生成</li>
        </ul>
        <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
          抽出深度は <strong>詳細</strong> と <strong>要約のみ</strong> から選択できます。
        </p>
      </div>
    </section>
  ),
};

export const treePaper: Paper = {
  id: 'tree',
  title: '知識構造',
  description: '概念間の階層・関連リンクを可視化',
  hue: 140,
  parentId: ROOT_ID,
  childIds: [hierarchyPaper.id, crosslinksPaper.id],
  content: (
    <section>
      <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>ツリー構造</h2>
      <div style={{ display: 'grid', gap: 8 }}>
        <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
          <PL id={hierarchyPaper.id} />が基本構造を定義し、<PL id={crosslinksPaper.id}>関連リンク</PL>
          （measured_by・contradicts・supports）が補足的な関係を表現します。
        </p>
        <table style={{ width: '100%', fontSize: '0.75rem', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ background: 'var(--surface-raised)' }}>
              <th style={{ padding: '6px 8px', textAlign: 'left' }}>アイテム種別</th>
              <th style={{ padding: '6px 8px', textAlign: 'left' }}>役割</th>
            </tr>
          </thead>
          <tbody>
            {[
              ['concept', '抽象的な概念・テーマ'],
              ['claim', '主張・仮説'],
              ['evidence', '根拠・データ'],
              ['counter', '反論・制約'],
            ].map(([kind, role], i) => (
              <tr key={kind} style={{ background: i % 2 === 1 ? 'var(--surface-alt)' : 'transparent' }}>
                <td style={{ padding: '5px 8px' }}>{kind}</td>
                <td style={{ padding: '5px 8px', color: 'var(--muted)' }}>{role}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  ),
};

export const explorePaper: Paper = {
  id: 'explore',
  title: 'paper-in-paper 探索',
  description: 'アイテムをクリックするだけで概念が展開',
  hue: 280,
  parentId: ROOT_ID,
  childIds: [datalinkPaper.id, focusmodePaper.id],
  content: (
    <section>
      <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>インタラクティブ探索</h2>
      <div style={{ display: 'grid', gap: 8 }}>
        <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
          ペーパー内のリンクをクリックすると、親の文脈を保ちながら子アイテムがインラインで展開されます。
          <PL id={datalinkPaper.id} />が関連リンクも再現します。
        </p>
        <div style={{ borderLeft: '3px solid var(--line)', background: 'var(--surface-alt)', borderRadius: 4, padding: '8px 12px', fontSize: '0.8rem', color: 'var(--muted)', lineHeight: 1.6 }}>
          このページ自体が paper-in-paper のデモです。ペーパーをクリックして展開してみてください。
        </div>
      </div>
    </section>
  ),
};

export const teamPaper: Paper = {
  id: 'team',
  title: 'チームコラボレーション',
  description: 'ワークスペースを共有・閲覧履歴を追跡',
  hue: 10,
  parentId: ROOT_ID,
  childIds: [viewhistoryPaper.id, invitePaper.id],
  content: (
    <section>
      <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>ロールベースアクセス</h2>
      <div style={{ display: 'grid', gap: 8 }}>
        <ul style={{ margin: 0, paddingLeft: 16, lineHeight: 1.8, fontSize: '0.85rem' }}>
          <li><strong>owner</strong> - 全権限・メンバー管理</li>
          <li><strong>editor</strong> - アップロード・招待</li>
          <li><strong>viewer</strong> - 閲覧のみ</li>
        </ul>
        <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
          各ユーザーの閲覧履歴・追加アイテムが記録され、チームの探索状況を把握できます。
        </p>
      </div>
    </section>
  ),
};

// --- Root ---

export const rootPaper: Paper = {
  id: ROOT_ID,
  title: 'トップ',
  description: 'ドキュメントを知識構造に変換・探索するシステム',
  hue: 230,
  parentId: null,
  childIds: [authPaper.id, workspacesPaper.id, extractionPaper.id, treePaper.id, explorePaper.id, teamPaper.id],
  content: (
    <section>
      <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>Synthify</h2>
      <div style={{ display: 'grid', gap: 8 }}>
        <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
          複数のドキュメントを読み込み、<PL id={extractionPaper.id}>AIが概念・主張・根拠を抽出</PL>して
          <PL id={treePaper.id} />を自動生成。そのまま<PL id={authPaper.id}>ワークスペースに入って</PL>
          <PL id={explorePaper.id}>paper-in-paper形式で探索</PL>できます。
        </p>
        <PL id={authPaper.id} variant="card" />
        <table style={{ width: '100%', fontSize: '0.75rem', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ background: 'var(--surface-raised)' }}>
              <th style={{ padding: '6px 8px', textAlign: 'left' }}>機能</th>
              <th style={{ padding: '6px 8px', textAlign: 'left' }}>説明</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td style={{ padding: '5px 8px' }}>AI抽出</td>
              <td style={{ padding: '5px 8px', color: 'var(--muted)' }}>Geminiが概念・主張・根拠・反論を自動識別</td>
            </tr>
            <tr style={{ background: 'var(--surface-alt)' }}>
              <td style={{ padding: '5px 8px' }}>構造化</td>
              <td style={{ padding: '5px 8px', color: 'var(--muted)' }}>階層・関連リンクを持つ知識ツリーを構築</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  ),
};

export const STATIC_PAPERS: Paper[] = [
  rootPaper,
  authPaper,
  workspacesPaper,
  extractionPaper,
  treePaper,
  explorePaper,
  teamPaper,
  canonicalizationPaper,
  depthPaper,
  hierarchyPaper,
  crosslinksPaper,
  datalinkPaper,
  focusmodePaper,
  viewhistoryPaper,
  invitePaper,
];
