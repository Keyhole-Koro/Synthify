import { buildPaperMap } from '@keyhole-koro/paper-in-paper';
import type { ContentNode, Paper, PaperMap } from '@keyhole-koro/paper-in-paper';
import { AuthPaper, type AuthMode } from './AuthPaper';
import { type User } from 'firebase/auth';
import { type Workspace } from '@/features/workspaces/api';

export const LANDING_ROOT_ID = 'root';

// helpers
const t = (value: string): ContentNode => ({ type: 'text', value });
const p = (...children: ContentNode[]): ContentNode => ({ type: 'paragraph', children });
const b = (...children: ContentNode[]): ContentNode => ({ type: 'bold', children });
const link = (paperId: string, label: string): ContentNode => ({ type: 'paper-link', paperId, label });
const card = (paperId: string, title: string, description: string): ContentNode => ({ type: 'card', paperId, title, description });
const section = (title: string, ...children: ContentNode[]): ContentNode => ({ type: 'section', title, children });
const list = (...items: ContentNode[][]): ContentNode => ({ type: 'list', items });
const table = (headers: string[], rows: string[][]): ContentNode => ({ type: 'table', headers, rows });
const callout = (...children: ContentNode[]): ContentNode => ({ type: 'callout', children });

const LANDING_PAPERS: Paper[] = [
  {
    id: 'root',
    title: 'Synthifyとは',
    description: 'ドキュメントを知識グラフに変換・探索するシステム',
    hue: 230,
    content: [
      section('Synthify',
        p(
          t('複数のドキュメントを読み込み、'),
          link('extraction', 'AIが概念・主張・根拠を抽出'),
          t('して'),
          link('graph', '知識グラフ'),
          t('を自動生成。そのまま'),
          link('auth', 'ワークスペースに入って'),
          link('explore', 'paper-in-paper形式で探索'),
          t('できます。'),
        ),
        card('auth', 'Start Here', 'まずは ログイン / 新規登録 を開いて、Synthify の入口をこの paper の中で試せます。'),
        table(
          ['機能', '説明'],
          [
            ['AI抽出', 'Geminiが概念・主張・根拠・反論を自動識別'],
            ['グラフ化', '階層・横断リンクを持つ知識グラフを構築'],
          ],
        ),
      ),
    ] satisfies ContentNode[],
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
    content: [
      section('6ステージ パイプライン',
        list(
          [t('テキスト正規化・チャンク分割')],
          [t('エンティティ・概念の抽出')],
          [link('canonicalization', 'エイリアス正規化')],
          [t('関係エッジの推論')],
          [t('重要度スコアリング')],
          [t('HTMLサマリ生成')],
        ),
        p(
          t('抽出深度は '),
          b(t('詳細')),
          t(' と '),
          b(t('要約のみ')),
          t(' から選択できます。'),
        ),
      ),
    ] satisfies ContentNode[],
    parentId: 'root',
    childIds: ['canonicalization', 'depth'],
  },
  {
    id: 'graph',
    title: '知識グラフ',
    description: '概念間の階層・横断リンクを可視化',
    hue: 140,
    content: [
      section('グラフ構造',
        p(
          link('hierarchy', '階層エッジ'),
          t('がツリー構造を定義し、'),
          link('crosslinks', '非階層エッジ'),
          t('（measured_by・contradicts・supports）が横断的な関係を表現します。'),
        ),
        table(
          ['ノード種別', '役割'],
          [
            ['concept', '抽象的な概念・テーマ'],
            ['claim', '主張・仮説'],
            ['evidence', '根拠・データ'],
            ['counter', '反論・制約'],
          ],
        ),
      ),
    ] satisfies ContentNode[],
    parentId: 'root',
    childIds: ['hierarchy', 'crosslinks'],
  },
  {
    id: 'explore',
    title: 'paper-in-paper 探索',
    description: 'ノードをクリックするだけで概念が展開',
    hue: 280,
    content: [
      section('インタラクティブ探索',
        p(
          t('ペーパー内のリンクをクリックすると、親の文脈を保ちながら子ノードがインラインで展開されます。'),
          link('datalink', 'data-paper-id リンク'),
          t('がグラフの横断リンクも再現します。'),
        ),
        callout(
          t('このページ自体が paper-in-paper のデモです。ペーパーをクリックして展開してみてください。'),
        ),
      ),
    ] satisfies ContentNode[],
    parentId: 'root',
    childIds: ['datalink', 'focusmode'],
  },
  {
    id: 'team',
    title: 'チームコラボレーション',
    description: 'ワークスペースを共有・閲覧履歴を追跡',
    hue: 10,
    content: [
      section('ロールベースアクセス',
        list(
          [b(t('owner')), t(' - 全権限・メンバー管理')],
          [b(t('editor')), t(' - アップロード・招待')],
          [b(t('viewer')), t(' - 閲覧のみ')],
        ),
        p(t('各ユーザーの閲覧ノード履歴・追加ノードが記録され、チームの探索状況を把握できます。')),
      ),
    ] satisfies ContentNode[],
    parentId: 'root',
    childIds: ['viewhistory', 'invite'],
  },
  {
    id: 'canonicalization',
    title: 'エイリアス正規化',
    description: '同義語・表記揺れを同一ノードに統合',
    hue: 200,
    content: [
      section('正規化の仕組み',
        p(t('Gemini が候補を提案し、コサイン類似度 + 人手ルールで同義語を一つの canonical ノードに統合します。元の document ノードは参照として残ります。')),
      ),
    ] satisfies ContentNode[],
    parentId: 'extraction',
    childIds: [],
  },
  {
    id: 'depth',
    title: '抽出深度',
    description: '詳細 vs 要約の2モード',
    hue: 200,
    content: [
      section('抽出深度の選択',
        p(
          b(t('詳細')),
          t('：全チャンクを処理し豊富なグラフを生成（時間がかかる）。'),
        ),
        p(
          b(t('要約のみ')),
          t('：高速だが粗めのグラフ。プロトタイプ確認に最適。'),
        ),
      ),
    ] satisfies ContentNode[],
    parentId: 'extraction',
    childIds: [],
  },
  {
    id: 'hierarchy',
    title: '階層エッジ',
    description: 'ツリー構造を定義するエッジ',
    hue: 150,
    content: [
      section('hierarchical エッジ',
        p(t('親子関係を表し、paper-in-paper のキャンバスツリーを決定します。ルートノード（level 0）から深くなるほど詳細な概念になります。')),
      ),
    ] satisfies ContentNode[],
    parentId: 'graph',
    childIds: [],
  },
  {
    id: 'crosslinks',
    title: '横断リンク',
    description: '階層を超えた概念間の関係',
    hue: 150,
    content: [
      section('非階層エッジ',
        p(t('supports・contradicts・measured_by など。HTMLサマリ内の data-paper-id リンクとして埋め込まれ、クリックで対象ノードが展開されます。')),
      ),
    ] satisfies ContentNode[],
    parentId: 'graph',
    childIds: [],
  },
  {
    id: 'datalink',
    title: 'data-paper-id リンク',
    description: 'HTMLリンクがグラフ展開をトリガー',
    hue: 265,
    content: [
      section('仕組み',
        p(t('ペーパーの HTML に <a data-paper-id="node_id"> を埋め込むと、クリック時に対象ノードが子として展開されます。非階層リンクもこの仕組みで再現されます。')),
      ),
    ] satisfies ContentNode[],
    parentId: 'explore',
    childIds: [],
  },
  {
    id: 'focusmode',
    title: 'フォーカスモード',
    description: '1つのノードに集中して読む',
    hue: 265,
    content: [
      section('フォーカスパネル',
        p(t('ノードを選択するとサイドパネルが開き、ソースチャンク・関連エッジ・HTMLサマリを詳しく確認できます。閲覧履歴にも自動記録されます。')),
      ),
    ] satisfies ContentNode[],
    parentId: 'explore',
    childIds: [],
  },
  {
    id: 'viewhistory',
    title: '閲覧履歴',
    description: 'ユーザーごとの探索状況を追跡',
    hue: 20,
    content: [
      section('user_node_views',
        p(t('ノードを開くたびに first_viewed_at・last_viewed_at・view_count が記録されます。チームで誰がどの概念を探索したかが一目で分かります。')),
      ),
    ] satisfies ContentNode[],
    parentId: 'team',
    childIds: [],
  },
  {
    id: 'invite',
    title: 'メンバー招待',
    description: 'メールアドレスで招待・ロール設定',
    hue: 20,
    content: [
      section('招待フロー',
        p(t('オーナーがメールアドレスとロールを指定して招待。is_dev フラグを付けると開発者モードが有効になり、内部メタデータへのアクセスが解放されます。')),
      ),
    ] satisfies ContentNode[],
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
