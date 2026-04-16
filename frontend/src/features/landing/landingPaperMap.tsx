import { buildPaperMap, usePaperStore } from '@keyhole-koro/paper-in-paper';
import type { Paper, PaperMap } from '@keyhole-koro/paper-in-paper';
import { AuthPaper, type AuthMode } from './AuthPaper';
import { type User } from 'firebase/auth';
import { type Workspace } from '@/features/workspaces/api';

export const LANDING_ROOT_ID = 'root';

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
//
// <PL id="graph" />                   chip:   paper.title を自動表示
// <PL id="graph">知識グラフ</PL>      inline: children をラベルとして文中に埋め込む
// <PL id="auth" variant="card" />     card:   paper.title + paper.description をブロック表示
function PL({ id, children, variant }: { id: string; children?: React.ReactNode; variant?: 'card' }) {
  const { state } = usePaperStore();
  const paper = state.paperMap.get(id);

  if (variant === 'card') {
    return (
      <a
        data-paper-id={id}
        tabIndex={0}
        style={{
          display: 'block',
          border: '1px solid var(--link-border)',
          borderRadius: 8,
          padding: '10px 12px',
          background: 'var(--link-bg)',
          cursor: 'pointer',
          textDecoration: 'none',
        }}
      >
        <p style={{ margin: '0 0 4px', fontSize: '0.7rem', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--accent)' }}>
          {paper?.title ?? id}
        </p>
        <p style={{ margin: 0, fontSize: '0.78rem', lineHeight: 1.55 }}>
          {paper?.description}
        </p>
      </a>
    );
  }

  return (
    <a
      data-paper-id={id}
      tabIndex={0}
      style={{ ...linkStyle, display: 'inline', padding: '1px 5px' }}
    >
      {children ?? paper?.title ?? id}
    </a>
  );
}

const LANDING_PAPERS: Paper[] = [
  {
    id: 'root',
    title: 'Synthifyとは',
    description: 'ドキュメントを知識グラフに変換・探索するシステム',
    hue: 230,
    content: (
      <section>
        <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>Synthify</h2>
        <div style={{ display: 'grid', gap: 8 }}>
          <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
            複数のドキュメントを読み込み、<PL id="extraction">AIが概念・主張・根拠を抽出</PL>して
            <PL id="graph" />を自動生成。そのまま<PL id="auth">ワークスペースに入って</PL>
            <PL id="explore">paper-in-paper形式で探索</PL>できます。
          </p>
          <PL id="auth" variant="card" />
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
                <td style={{ padding: '5px 8px' }}>グラフ化</td>
                <td style={{ padding: '5px 8px', color: 'var(--muted)' }}>階層・横断リンクを持つ知識グラフを構築</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    ),
    parentId: null,
    childIds: ['auth', 'extraction', 'graph', 'explore', 'team'],
  },
  {
    id: 'auth',
    title: 'ログイン / 新規登録',
    description: 'Synthify をはじめる',
    hue: 250,
    content: null,
    parentId: 'root',
    childIds: [],
  },
  {
    id: 'extraction',
    title: 'AI による概念抽出',
    description: 'Geminiがドキュメントを6ステージで解析',
    hue: 215,
    content: (
      <section>
        <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>6ステージ パイプライン</h2>
        <div style={{ display: 'grid', gap: 8 }}>
          <ul style={{ margin: 0, paddingLeft: 16, lineHeight: 1.8, fontSize: '0.85rem' }}>
            <li>テキスト正規化・チャンク分割</li>
            <li>エンティティ・概念の抽出</li>
            <li><PL id="canonicalization" /></li>
            <li>関係エッジの推論</li>
            <li>重要度スコアリング</li>
            <li>HTMLサマリ生成</li>
          </ul>
          <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
            抽出深度は <strong>詳細</strong> と <strong>要約のみ</strong> から選択できます。
          </p>
        </div>
      </section>
    ),
    parentId: 'root',
    childIds: ['canonicalization', 'depth'],
  },
  {
    id: 'graph',
    title: '知識グラフ',
    description: '概念間の階層・横断リンクを可視化',
    hue: 140,
    content: (
      <section>
        <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>グラフ構造</h2>
        <div style={{ display: 'grid', gap: 8 }}>
          <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
            <PL id="hierarchy" />がツリー構造を定義し、<PL id="crosslinks">非階層エッジ</PL>
            （measured_by・contradicts・supports）が横断的な関係を表現します。
          </p>
          <table style={{ width: '100%', fontSize: '0.75rem', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ background: 'var(--surface-raised)' }}>
                <th style={{ padding: '6px 8px', textAlign: 'left' }}>ノード種別</th>
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
    parentId: 'root',
    childIds: ['hierarchy', 'crosslinks'],
  },
  {
    id: 'explore',
    title: 'paper-in-paper 探索',
    description: 'ノードをクリックするだけで概念が展開',
    hue: 280,
    content: (
      <section>
        <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>インタラクティブ探索</h2>
        <div style={{ display: 'grid', gap: 8 }}>
          <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
            ペーパー内のリンクをクリックすると、親の文脈を保ちながら子ノードがインラインで展開されます。
            <PL id="datalink" />がグラフの横断リンクも再現します。
          </p>
          <div style={{ borderLeft: '3px solid var(--line)', background: 'var(--surface-alt)', borderRadius: 4, padding: '8px 12px', fontSize: '0.8rem', color: 'var(--muted)', lineHeight: 1.6 }}>
            このページ自体が paper-in-paper のデモです。ペーパーをクリックして展開してみてください。
          </div>
        </div>
      </section>
    ),
    parentId: 'root',
    childIds: ['datalink', 'focusmode'],
  },
  {
    id: 'team',
    title: 'チームコラボレーション',
    description: 'ワークスペースを共有・閲覧履歴を追跡',
    hue: 10,
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
            各ユーザーの閲覧ノード履歴・追加ノードが記録され、チームの探索状況を把握できます。
          </p>
        </div>
      </section>
    ),
    parentId: 'root',
    childIds: ['viewhistory', 'invite'],
  },
  {
    id: 'canonicalization',
    title: 'エイリアス正規化',
    description: '同義語・表記揺れを同一ノードに統合',
    hue: 200,
    content: (
      <section>
        <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>正規化の仕組み</h2>
        <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
          Gemini が候補を提案し、コサイン類似度 + 人手ルールで同義語を一つの canonical ノードに統合します。元の document ノードは参照として残ります。
        </p>
      </section>
    ),
    parentId: 'extraction',
    childIds: [],
  },
  {
    id: 'depth',
    title: '抽出深度',
    description: '詳細 vs 要約の2モード',
    hue: 200,
    content: (
      <section>
        <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>抽出深度の選択</h2>
        <div style={{ display: 'grid', gap: 8 }}>
          <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
            <strong>詳細</strong>：全チャンクを処理し豊富なグラフを生成（時間がかかる）。
          </p>
          <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
            <strong>要約のみ</strong>：高速だが粗めのグラフ。プロトタイプ確認に最適。
          </p>
        </div>
      </section>
    ),
    parentId: 'extraction',
    childIds: [],
  },
  {
    id: 'hierarchy',
    title: '階層エッジ',
    description: 'ツリー構造を定義するエッジ',
    hue: 150,
    content: (
      <section>
        <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>hierarchical エッジ</h2>
        <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
          親子関係を表し、paper-in-paper のキャンバスツリーを決定します。ルートノード（level 0）から深くなるほど詳細な概念になります。
        </p>
      </section>
    ),
    parentId: 'graph',
    childIds: [],
  },
  {
    id: 'crosslinks',
    title: '横断リンク',
    description: '階層を超えた概念間の関係',
    hue: 150,
    content: (
      <section>
        <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>非階層エッジ</h2>
        <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
          supports・contradicts・measured_by など。HTMLサマリ内の data-paper-id リンクとして埋め込まれ、クリックで対象ノードが展開されます。
        </p>
      </section>
    ),
    parentId: 'graph',
    childIds: [],
  },
  {
    id: 'datalink',
    title: 'data-paper-id リンク',
    description: 'HTMLリンクがグラフ展開をトリガー',
    hue: 265,
    content: (
      <section>
        <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>仕組み</h2>
        <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
          ペーパーの HTML に{' '}
          <code style={{ fontSize: '0.8em', background: 'var(--surface-alt)', padding: '1px 4px', borderRadius: 3 }}>
            {'<a data-paper-id="node_id">'}
          </code>
          {' '}を埋め込むと、クリック時に対象ノードが子として展開されます。非階層リンクもこの仕組みで再現されます。
        </p>
      </section>
    ),
    parentId: 'explore',
    childIds: [],
  },
  {
    id: 'focusmode',
    title: 'フォーカスモード',
    description: '1つのノードに集中して読む',
    hue: 265,
    content: (
      <section>
        <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>フォーカスパネル</h2>
        <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
          ノードを選択するとサイドパネルが開き、ソースチャンク・関連エッジ・HTMLサマリを詳しく確認できます。閲覧履歴にも自動記録されます。
        </p>
      </section>
    ),
    parentId: 'explore',
    childIds: [],
  },
  {
    id: 'viewhistory',
    title: '閲覧履歴',
    description: 'ユーザーごとの探索状況を追跡',
    hue: 20,
    content: (
      <section>
        <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>user_node_views</h2>
        <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
          ノードを開くたびに first_viewed_at・last_viewed_at・view_count が記録されます。チームで誰がどの概念を探索したかが一目で分かります。
        </p>
      </section>
    ),
    parentId: 'team',
    childIds: [],
  },
  {
    id: 'invite',
    title: 'メンバー招待',
    description: 'メールアドレスで招待・ロール設定',
    hue: 20,
    content: (
      <section>
        <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>招待フロー</h2>
        <p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
          オーナーがメールアドレスとロールを指定して招待。is_dev フラグを付けると開発者モードが有効になり、内部メタデータへのアクセスが解放されます。
        </p>
      </section>
    ),
    parentId: 'team',
    childIds: [],
  },
];

export function buildLandingPaperMap({
  user,
  workspaces,
  authMode,
  loading,
  onAuthModeChange,
  onEmailSubmit,
  onGoogleSubmit,
  onLogout,
  onEnterWorkspace,
}: {
  user: User | null;
  workspaces: Workspace[];
  authMode: AuthMode;
  loading: boolean;
  onAuthModeChange: (mode: AuthMode) => void;
  onEmailSubmit: () => void;
  onGoogleSubmit: () => void;
  onLogout: () => void;
  onEnterWorkspace: () => void;
}): PaperMap {
  const paperMap = buildPaperMap(LANDING_PAPERS);
  const authPaper = paperMap.get('auth');

  if (authPaper) {
    paperMap.set('auth', {
      ...authPaper,
      content: (
        <AuthPaper
          user={user}
          workspaces={workspaces}
          mode={authMode}
          loading={loading}
          onModeChange={onAuthModeChange}
          onEmailSubmit={onEmailSubmit}
          onGoogleSubmit={onGoogleSubmit}
          onLogout={onLogout}
          onEnterWorkspace={onEnterWorkspace}
        />
      ),
    });
  }

  return paperMap;
}
