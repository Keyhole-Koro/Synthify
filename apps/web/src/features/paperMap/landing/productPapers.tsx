import type { Paper } from '@keyhole-koro/paper-in-paper';
import { ROOT_ID } from '@/features/paperMap/defaultOpenState';
import { HeroPanel, MetricRow, NodePreview, PaperLinkCard, PaperSection, PlainList, SignalGrid, StageRail } from './landingContent';
import { CATEGORY_HUES, cardGridStyle } from './landingTheme';

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

/**
 * The static explainer papers about Synthify. They depend on nothing, so they are built once:
 * the canvas then sees the same objects on every update and never re-renders them for it.
 */
export const PRODUCT_PAPERS: readonly Paper[] = [
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
