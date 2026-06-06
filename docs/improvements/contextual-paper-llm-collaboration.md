# Contextual Paper LLM Collaboration

既存 paper の中で、クリック位置・選択範囲・現在の paper node を文脈として LLM と共同作業するための設計メモ。
従来案の「ヘッダ + から LLM 対話 child を作る」よりも、paper 上の操作に直接寄せた方向性として扱う。

## Goal

- paper 内のクリック、テキスト選択、またはヘッダの `+` から同じ contextual composer を起動する。
- ユーザーは `Ask` / `Search` / `Edit` / `Pin note` のような操作種別を選ばない。
- LLM が現在の文脈と自然文入力から、質問回答・検索・既存 node 編集・子 node 作成・ピン付きメモ作成を判断する。
- 既存 node に変更を加えるのか、その下に子 node を作るのかが曖昧な場合は、LLM が勝手に実行せず選択肢を提示する。

## UX

- paper 本文をクリックすると、その位置に小さな composer を開く。
- テキスト選択中にクリックまたは `+` を押した場合は、選択範囲を anchor として composer を開く。
- ヘッダの `+` は現在 paper 全体を対象に composer を開く補助入口にする。
- composer は自然文入力だけを持つ。例: `ここ詳しく`, `関連箇所を探して`, `この表現を直して`, `この下に背景を追加`.
- `answer` / `search` はその場で回答と関連 paper / source を表示する。
- `revise_node` / `create_child` は preview card を表示し、ユーザーが `Apply` した時だけ反映する。
- Apply 後は paper map を即時更新し、DB 成功後に確定する。失敗時は元に戻す。
- 関連 paper / source はクリックで既存の `OPEN_NODE` 導線に接続する。

## Intent Model

LLM は contextual turn ごとに次の intent を返す。

- `answer`: 指定箇所や現在 paper について回答する。
- `search`: grep、vector search、tree item search の結果を優先して返す。
- `revise_node`: 選択箇所または現在 node の本文を改善・修正する。
- `create_child`: 現在 node の下に新しい子 paper を作る。
- `pin_note`: クリック位置にメモ、未解決論点、TODO などを置く。
- `needs_decision`: 既存 node 更新か子 node 作成かなど、変更方針が曖昧な場合に候補を提示する。

既存 node 編集と子 node 作成の初期判定ルール:

- `revise_node`: `直して`, `短く`, `言い換えて`, `追記して`, `この表現を改善` など、対象本文の変更が主目的。
- `create_child`: `詳しく`, `展開して`, `背景を追加`, `別観点を作って`, `この下にまとめて` など、情報を増やすことが主目的。
- 判断できない場合は `needs_decision` とし、直接変更しない。

## Data Model

想定テーブル:

- `collab_anchors`
  - `anchor_id`, `workspace_id`, `item_id`, `kind`, `selected_text`, `selected_html`, `start_selector`, `end_selector`, `x`, `y`, `note`, `created_by`, `created_at`
  - `kind`: `selection`, `node`, `pin`
- `collab_threads`
  - `thread_id`, `workspace_id`, `item_id`, `anchor_id`, `created_by`, `created_at`, `updated_at`
- `collab_messages`
  - `message_id`, `thread_id`, `role`, `content`, `intent`, `status`, `created_at`
- `item_content_revisions`
  - `revision_id`, `item_id`, `before_html`, `after_html`, `anchor_id`, `prompt`, `model`, `created_by`, `created_at`

anchor selector は初版では iframe 内の安定 HTML selector と selected text fallback を併用する。selector で復元できない場合は paper 全体に fallback する。

## API / Worker

### App API

新規 `CollaborationService` を追加する。

- `CreateAnchor`
- `AskOnAnchor`
- `ProposeEdit`
- `ApplyEdit`
- `ListAnchorThreads`

response は次の情報を返す。

- `intent`
- `answer`
- `sources`
- `edit_patch`
- `child_draft`
- `pin_draft`
- `decision_options`

### Worker

worker 内部 RPC として `RunContextualCollabTurn` を追加する。

入力:

- `workspace_id`
- `item_id`
- `anchor`
- `nearby_text`
- `selected_text`
- `user_message`

検索:

- 既存 `grep_search`
- 既存 vector search
- tree item search
- 指定 anchor 周辺文脈
- 現在 item 全体

編集出力:

- full HTML 全体ではなく、`before_fragment`, `after_fragment`, `confidence`, `rationale` を返す。
- API 側で対象断片を検証してから preview / apply する。

## Open Questions

- anchor selector の精度: LLM が生成した rich HTML や iframe 内 DOM の変化にどこまで耐えるか。
- undo 範囲: 直前 revision だけ戻すか、thread 単位で複数 revision を戻せるようにするか。
- `locked` / `human_curated` item の扱い: 直接編集を拒否して提案だけ返す方針を初期値にする。
- streaming 形状: Firestore 経由の chunk streaming に寄せるか、Connect streaming を導入するか。
- pin 表示: paper content HTML 内に marker を埋め込むか、host 側 overlay として表示するか。

## Test Plan

- iframe 内クリック位置と選択範囲が anchor として取得できる。
- ヘッダ `+`、本文クリック、範囲選択の 3 入口が同じ composer を開く。
- intent が `answer`, `search`, `revise_node`, `create_child`, `pin_note`, `needs_decision` に分類される。
- `answer` / `search` が grep、vector、tree item search の source 付きで返る。
- `revise_node` preview apply で `tree_items.content` と paper 表示が更新される。
- `create_child` preview apply で現在 node 配下に新 paper が作られる。
- 曖昧な依頼では直接変更せず decision card が出る。
- undo で直前 revision を戻せる。
- `locked` / `human_curated` item は直接編集されず、提案だけ返る。
