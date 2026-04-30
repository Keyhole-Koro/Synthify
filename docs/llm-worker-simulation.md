# LLM Worker 処理シミュレーション

実際のドキュメントを例に、エージェントがどのようにツールを呼ぶかをターンごとに追う。

## 入力ドキュメント例

```
# API設計書 v2.3
## 1. 概要
本システムはマイクロサービスアーキテクチャを採用し...

## 2. 認証フロー
JWTトークンを用いる。有効期限は24時間。
リフレッシュトークンは30日間有効。

### 2.1 エンドポイント
| Method | Path           | 説明          |
|--------|----------------|---------------|
| POST   | /auth/login    | ログイン       |
| POST   | /auth/refresh  | トークン更新   |
| DELETE | /auth/logout   | ログアウト     |

## 3. レート制限
1IPあたり100req/min。超過時は429を返す。
認証済みユーザーは1000req/min。

## 4. エラーハンドリング
標準エラーフォーマット:
{"code": "ERR_XXX", "message": "..."}

### 4.1 エラーコード一覧
ERR_401: 未認証
ERR_429: レート制限超過
ERR_500: サーバーエラー
```

## LLM への初期メッセージ

```
job_id: job_001
document_id: doc_api_design
workspace_id: ws_001
file_uri: gs://bucket/api-design-v2.3.pdf
filename: api-design-v2.3.pdf
mime_type: application/pdf

Follow your workflow: extract text, chunk, generate brief,
synthesize items, critique, then persist.
```

---

## ターン1 — 計画

LLM の判断：「PDFだ、まず計画を立てよう」

```
→ journal_add_task("extract and chunk the document")
→ journal_add_task("generate document brief")
→ journal_add_task("synthesize knowledge tree")
→ journal_add_task("critique and persist")
```

Working Memory（以降のターンに自動注入）:
```
### Tasks
- [ ] task_1: extract and chunk the document
- [ ] task_2: generate document brief
- [ ] task_3: synthesize knowledge tree
- [ ] task_4: critique and persist
```

---

## ターン2 — テキスト抽出・チャンキング

```
→ extract_text(file_uri="gs://bucket/api-design-v2.3.pdf", mime_type="application/pdf")

結果: raw_text = "# API設計書 v2.3\n## 1. 概要\n..."

→ semantic_chunking(document_id="doc_api_design", raw_text="...")

結果:
chunks = [
  {index:0, heading:"概要",             text:"本システムはマイクロサービス..."},
  {index:1, heading:"認証フロー",        text:"JWTトークンを用いる..."},
  {index:2, heading:"エンドポイント",    text:"| Method | Path | ..."},
  {index:3, heading:"レート制限",        text:"1IPあたり100req/min..."},
  {index:4, heading:"エラーハンドリング", text:"標準エラーフォーマット..."},
  {index:5, heading:"エラーコード一覧",  text:"ERR_401: 未認証..."},
]
outline = ["概要", "認証フロー", "エンドポイント", "レート制限",
           "エラーハンドリング", "エラーコード一覧"]

→ journal_update_task(task_id="task_1", status="completed")
```

---

## ターン3 — Brief 生成・依存解析

```
→ analyze_dependencies(outline=["概要", "認証フロー", "エンドポイント", ...])

結果:
dependencies = [
  {task_id:"認証フロー",       depends_on:"概要",          reason:"subsection"},
  {task_id:"エンドポイント",   depends_on:"認証フロー",     reason:"subsection"},
  {task_id:"エラーコード一覧", depends_on:"エラーハンドリング", reason:"subsection"},
]
priorities = ["概要", "認証フロー", "エンドポイント", "レート制限",
              "エラーハンドリング", "エラーコード一覧"]

→ generate_brief(outline=["概要", "認証フロー", ...])

結果（Brief に書き込まれる）:
  Topic:        "APIシステム設計書"
  ClaimSummary: "JWT認証・レート制限・標準エラーフォーマットを定義するAPI仕様"
  Entities:     ["JWT", "リフレッシュトークン", "レート制限", "エラーコード"]

→ glossary_register(term="JWT",
    definition="JSON Web Token。24時間有効な認証トークン")
→ glossary_register(term="レート制限",
    definition="1IPあたり100req/min、認証済みユーザーは1000req/min")

→ journal_update_task(task_id="task_2", status="completed")
```

Working Memory（次ターン以降に注入）:
```
### Document Brief
- Topic: APIシステム設計書
- Summary: JWT認証・レート制限・標準エラーフォーマットを定義するAPI仕様

### Glossary
- JWT: JSON Web Token。24時間有効な認証トークン
- レート制限: 1IPあたり100req/min、認証済みユーザーは1000req/min

### Tasks
- [x] task_1: extract and chunk the document
- [x] task_2: generate document brief
- [ ] task_3: synthesize knowledge tree
- [ ] task_4: critique and persist
```

---

## ターン4 — Synthesis

LLM は priorities の順でチャンクを処理。テーブルを含むチャンクを検出して先に構造化する。

```
→ extract_table_data(
    chunk_id="doc_api_design_chunk_2",
    text="| Method | Path | 説明 |..."
  )

結果:
table_json = {
  "rows": [
    ["POST",   "/auth/login",   "ログイン"],
    ["POST",   "/auth/refresh", "トークン更新"],
    ["DELETE", "/auth/logout",  "ログアウト"]
  ]
}

→ goal_driven_synthesis(
    chunks=[chunk_0, chunk_1, chunk_2, chunk_3, chunk_4, chunk_5],
    instruction="エンドポイント一覧は table_json の構造を保持すること。
                 エラーコードはエラーハンドリングの子ノードとして配置すること。"
  )

結果:
items = [
  {local_id:"item_1", label:"API概要",          level:1, parent_local_id:""},
  {local_id:"item_2", label:"認証フロー",        level:1, parent_local_id:""},
  {local_id:"item_3", label:"認証エンドポイント", level:2, parent_local_id:"item_2"},
  {local_id:"item_4", label:"レート制限",         level:1, parent_local_id:""},
  {local_id:"item_5", label:"エラーハンドリング", level:1, parent_local_id:""},
  {local_id:"item_6", label:"エラーコード一覧",  level:2, parent_local_id:"item_5"},
]

→ journal_update_task(task_id="task_3", status="completed")
```

---

## ターン5 — 品質チェック・永続化

```
→ quality_critique(
    target_data="{items の JSON}",
    criteria="エラーコードが親ノードと正しく紐づいているか。
               テーブルの情報が失われていないか。"
  )

結果: {valid: true, issues: [], suggestions: []}

→ persist_knowledge_tree(
    job_id="job_001",
    document_id="doc_api_design",
    workspace_id="ws_001",
    items=[item_1, ..., item_6]
  )

→ journal_update_task(task_id="task_4", status="completed")
```

---

## 観察

**ツールが連携した点**
- テーブル検出 → `extract_table_data` → `instruction` に反映という流れが自然に成立する
- Glossary が Working Memory に蓄積されて synthesis 時に暗黙的に参照される
- `analyze_dependencies` が処理順を決め、LLM がそれに従って synthesis の instruction を構成する
- Journal が状態管理として機能し、LLM が自分の進捗を把握できる

**現在のスタブでは動かない点**
- `generate_brief` — 単純結合なので Entities が空になる
- `goal_driven_synthesis` — LLM を使わないので instruction が無視される
- `quality_critique` — 静的チェックのみで構造的な問題を検出できない

シミュレーション上の流れは正しく、`process/` ツール群の LLM 化が次のステップとなる。
詳細設計は [improvements/process-tools-llm-implementation.md](improvements/process-tools-llm-implementation.md) を参照。
