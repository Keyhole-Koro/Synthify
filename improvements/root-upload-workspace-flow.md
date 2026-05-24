# Rootアップロードとワークスペース導線の改善

## 目的

ユーザーがワークスペースを先に開かなくても、トップ/root paper からすぐにドキュメントをアップロードできるようにする。

アップロード後は、Synthify が保存先ワークスペースを作成または選択し、処理 job を開始し、完了時に生成された knowledge tree を自動で表示する。

## 現状のUX課題

- ドキュメントアップロードが workspace paper を開いた後でないと使えない。
- 初回ユーザーが、コア体験の前に workspace 構造を理解する必要がある。
- workspace が無いユーザーは、アップロード前に workspace 名を考えて作成しないといけない。
- workspace 名は、アップロードされた資料の内容から LLM が自然に提案できる可能性が高い。
- root upload を別実装すると、既存の workspace upload と進捗/job 管理が二重化しやすい。

## 提案するプロダクト挙動

### Root直下のアップロードPaper

root 直下に `新規アップロード` paper を追加する。

root は大きな入口だけを持つ。

- `ログイン / アカウント`
- `Synthifyについて`
- `新規アップロード`
- `ワークスペース`
- `プラン・課金`

`新規アップロード` paper は以下の状態を持つ。

- 未ログイン: ログイン導線だけ表示し、アップロード操作は出さない。
- ログイン済み・workspaceなし: ファイルを受け取り、workspace を自動作成する。
- ログイン済み・workspaceあり: 既存 workspace の選択、または新規 workspace 作成を選べる。
- アップロード中/処理中: ファイルごとの upload/job 進捗を表示する。
- 完了: 対象 workspace を開き、生成された document root/tree を表示する。
- 失敗: ファイルごとの失敗理由と retry 導線を表示する。

### 複数ドキュメント同時アップロード

root upload UI は複数ドキュメントを同時に扱えるようにする。

期待挙動:

- file input の複数選択と、複数ファイルの drag/drop を受け付ける。
- ファイルごとに queue row を表示する。
- 各ファイルを独立してアップロードする。
- backend に batch job が入るまでは、document ごとに processing job を開始する。
- ファイルごとに `queued`, `uploading`, `processing`, `succeeded`, `failed` を持つ。
- 部分成功を許容する。1ファイルの失敗で他ファイルの処理を止めない。
- 新規 workspace に複数ファイルを投入する場合、同じ workspaceId を使う。
- job 完了時、可能なら新しく生成された document root をすべて表示する。

### Workspace選択

ログイン済みで既存 workspace がある場合:

- デフォルト保存先は「最後に使った workspace」があればそれを使う。
- last used が無い場合は最初の workspace を使う。
- 明示的に workspace を選べる。
- `新規ワークスペースを作成してアップロード` も選べる。

初回ユーザー、または明示的に新規 workspace を選んだ場合:

- workspace を即時作成する。
- 初期名は `新規ワークスペース` にする。
- `workspaceId` があるので、UI/API/Firestore 上の識別は問題ない。
- 人間にとって読みやすい名前が後から決まる前に、upload と processing を開始できる。

## LLMによるWorkspace名生成

workspace 名は LLM が非同期に生成してよい。

### 初期名

自動作成 workspace は以下の形にする。

- `name` は `新規ワークスペース`
- placeholder 名とユーザー指定名を区別できる metadata を持つ。

推奨フィールド:

```ts
workspace.name = "新規ワークスペース"
workspace.nameSource = "placeholder" | "llm" | "user"
```

任意フィールド:

```ts
workspace.nameUpdatedAt
workspace.nameSuggestionJobId
workspace.lastNameCandidate
```

### 名前の上書きルール

- LLM が上書きできるのは `nameSource === "placeholder"` のときだけ。
- ユーザーが名前を編集したら `nameSource = "user"` にする。
- `nameSource = "user"` は LLM が絶対に上書きしない。
- `nameSource = "llm"` の workspace を、追加ドキュメントで再命名するかは後で決める。初期仕様では自動上書きしない。

### 名前生成のタイミング

ファイル upload 直後は filename や metadata しかなく、良い名前を付ける材料が少ない。

より良いタイミング:

- text extraction と brief 生成の後。
- document title、filename、brief、主要 concept を使う。
- 複数ドキュメント upload の場合、最初に有用な brief ができたタイミングで命名する。
- ただし `placeholder` の間だけ更新し、ユーザー編集後は触らない。

## Firestore Snapshot 方針

workspace 名や processing 状態が非同期に変わるため、frontend は Firestore snapshot 購読に寄せていくのが自然。

段階的な進め方:

1. 現在ユーザーがアクセスできる workspace list を snapshot 購読にする。
2. job status は一旦既存 polling のまま維持する。
3. root upload 成功時は既存の tree refresh/reveal 処理を呼ぶ。
4. 後続で job status と tree/document root 追加も snapshot/event 駆動へ寄せる。

workspace list snapshot で反映したいもの:

- root upload で作成された新規 workspace
- LLM による placeholder 名の置き換え
- ユーザーによる workspace 名変更
- workspace の updatedAt

## 実装計画

### 1. Uploadロジックを共通化する

`WorkspacePaper` の upload/job logic を root upload にコピーしない。

共通の upload primitives を作る。

- `useDocumentUploadQueue`
- `DocumentUploadDropzone`
- `DocumentUploadProgressList`

共通ロジックで扱うこと:

- 単一ファイルと複数ファイル
- ファイルごとの status
- document 作成
- signed URL への upload
- processing job 開始
- job 完了 callback
- failed file の retry

`WorkspacePaper` と root の `新規アップロード` paper は、どちらもこの共通部品を使う。

### 2. Root Upload Paper を追加する

root child として追加する。

- id: `upload`
- title: `新規アップロード`
- parent: `root`
- hue: 330 や 16 など、他カテゴリと区別できる色

content で扱うこと:

- 未ログイン時はログイン導線
- ログイン済みなら workspace selector
- drag/drop と複数ファイル選択
- queue status 表示
- 必要に応じた placeholder workspace 作成
- 処理完了後の target workspace/tree reveal

### 3. API / Data Model 対応

追加または確認したい API:

- placeholder 名で workspace を作成する
- workspace 名を更新する
- ユーザー編集名として lock/source 設定する
- LLM rename は placeholder の場合だけ適用する

既存 workspace record に `nameSource` がない場合の扱い:

- 既存の名前付き workspace は基本的に `user` 扱いにする。
- ただし auto-upload flow で作られ、名前が完全に `新規ワークスペース` のものは `placeholder` とみなせる余地がある。

### 4. Workerに命名ステップを追加する

brief 生成後に workspace 名候補を作る。

処理内容:

- workspace 名候補を生成する。
- `nameSource === "placeholder"` の場合だけ workspace を更新する。
- 必要なら name suggestion の元になった job/document metadata を残す。

命名 prompt の方針:

- 短い日本語名
- 資料の領域やテーマが分かる名前
- 汎用的すぎる `資料まとめ` のような名前を避ける
- 後から文書が追加されても破綻しにくい名前

### 5. 複数Upload後のTree Reveal

1つ以上の job が完了したら:

- 対象 workspace tree を refresh する。
- 新しく追加された document root node を表示する。
- progress list はユーザーが閉じるか、別画面へ移動するまで残す。
- 複数 job が別々のタイミングで完了する場合、完了ごとに新しい root を追加表示する。

## 未決定事項

- root upload のデフォルト保存先は last used workspace か、常に新規 workspace か。
- 複数ファイル upload は、デフォルトで1つの workspace にまとめるか、batch ごとに workspace を作るか。
- upload 前に、複数ファイルを別々の workspace へ振り分ける UI を入れるか。
- LLM 生成 workspace 名は自動適用するか、候補としてユーザー確認を挟むか。
- job status snapshot 化を root upload 初回リリース前にやるか、まず polling のまま出すか。
- `新規ワークスペース` が複数ある間、workspace list 上でどう見分けやすくするか。

## 初期リリース案

最初は保守的に出す。

- root に `新規アップロード` を追加する。
- ログイン済みユーザーは複数ファイルを upload できる。
- workspace が無い場合、`新規ワークスペース` を1つ作る。
- workspace がある場合、first/last-used workspace を default にして変更可能にする。
- 複数ファイル upload は1つの選択 workspace に入れる。
- 各ファイルは document と job をそれぞれ持つ。
- job status は一旦 polling のまま。
- 完了時に対象 workspace を refresh し、tree を reveal する。
- `nameSource` を追加し、brief 生成後に LLM rename する。

## 初期リリースではやらないこと

- batch job orchestration
- 1つの upload queue 内での cross-workspace 振り分け
- job/tree の完全な Firestore subscription 化
- 複雑な命名 approval workflow
- ユーザー編集済み workspace 名の自動上書き
