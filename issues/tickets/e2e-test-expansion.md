# E2Eテストの対象を主要ユーザーフローへ拡張する

## 背景

Playwrightの基盤と、以下の初期シナリオは導入済み。

- 未認証時にログイン導線が表示される
- workspaceを作成し、リロード後も復元される
- dev seed workspaceを作成し、永続化された内容を開ける

手動デバッグの負担をさらに減らすため、認証・アップロード・非同期処理・共有など、複数コンポーネントをまたぐ重要導線をE2Eで保護する。

## 方針

- 通常のE2EではFirebase Auth/Firestore、DB、fake GCSなどローカルのエミュレータを利用する
- Gemini、Googleログイン、Stripeなどの外部サービスには通常のPRテストから依存しない
- LLM処理は固定fixtureを返すworkerのE2Eモードを用意する
- 座標、アニメーション時間、CSSクラスではなく、ユーザーから見える状態遷移を検証する
- `waitForTimeout`は使用せず、表示状態またはAPI/Firestore上の完了条件を待つ
- 認証・課金・workspaceデータはテストユーザー単位で分離する

## Phase 1: 軽量で効果の高い導線

### Workspace CRUD

- [ ] workspace名を変更できる
- [ ] リロード後も変更後の名前が表示される
- [ ] workspaceを削除できる
- [ ] 削除後に一覧およびローカルキャッシュから消える
- [ ] リロードしても削除済みworkspaceが復活しない
- [ ] 空文字や上限超過など不正な名前に対するエラーが表示される

### セッション分離

- [ ] ログイン状態がリロード後も復元される
- [ ] ログアウト後に認証必須のworkspaceが表示されない
- [ ] ユーザーAのキャッシュがユーザーBに表示されない
- [ ] 別ユーザーへ切り替えた際にpaperのopen stateが混ざらない

### 公開リンク

- [ ] ownerがworkspaceの公開リンクを作成できる
- [ ] 未ログインの新しいbrowser contextで公開リンクを閲覧できる
- [ ] viewerには編集操作が表示されない、または実行できない
- [ ] revoke後は同じリンクで閲覧できない
- [ ] 不正または期限切れtokenで適切なエラーが表示される

## Phase 2: アップロードと非同期処理

### テスト用worker fixture

- [ ] local/test環境限定で固定された処理結果を返せる
- [ ] productionではfixtureモードを有効化できない
- [ ] queued/running/completed/failedを決定的に再現できる
- [ ] 固定fixtureから生成されたtree内容を検証できる

### アップロード正常系

- [ ] ファイル選択からfake GCSへの直接アップロードまで成功する
- [ ] `StartProcessing`後に進捗表示が出る
- [ ] Firestoreの完了通知後にworkspaceへ成果物が反映される
- [ ] 完了後にリロードしても成果物が表示される
- [ ] processing中のリロード後も進捗を復元できる

### アップロード異常系

- [ ] 実行形式など禁止ファイルがブラウザ上で拒否される
- [ ] サイズ超過時に適切なエラーが表示される
- [ ] GCSアップロード失敗時に再試行可能な状態になる
- [ ] processing開始失敗時にエラーが表示される
- [ ] worker失敗時に失敗理由と再試行導線が表示される
- [ ] Firestore購読エラー時に画面が無期限のloadingにならない

## Phase 3: Paper操作・認可・課金境界

### Paper-in-paper

- [ ] workspaceから子paperを開閉できる
- [ ] 複数階層のpaperを辿れる
- [ ] リロード後にopen stateが復元される
- [ ] iframe内の生成本文が表示される
- [ ] iframe内の子paperリンクから対象paperを開ける

### 複数ユーザー認可

- [ ] ownerがeditor/viewerを招待できる
- [ ] editorは許可された編集操作を実行できる
- [ ] viewerは閲覧のみ可能である
- [ ] member削除後はworkspaceへアクセスできない
- [ ] UIで操作を隠すだけでなく、APIも権限違反を拒否する

### 課金境界

- [ ] free planの制限とupgrade導線が表示される
- [ ] quota到達時にアップロードが拒否される
- [ ] Checkout開始要求が正しく送信される
- [ ] Billing Portal開始要求が正しく送信される
- [ ] budget設定を更新できる
- [ ] 課金API失敗時にユーザー向けエラーが表示される

## CIでの実行方針

- PR: ChromiumでPhase 1とアップロードの主要正常系を実行する
- main merge後: Chromiumの全E2Eを実行する
- nightly/manual: 外部サービスを使う最小限の統合確認を分離して実行する
- Firefox/WebKitはChromiumのテストが安定した後に追加する
- 失敗時はtrace、screenshot、videoをartifactとして保存する

## 完了条件

- [ ] 各シナリオが独立したテストデータで実行できる
- [ ] 同じsuiteを5回連続実行してflaky failureがない
- [ ] 通常E2EはGoogle、Gemini、Stripeの稼働状況に依存しない
- [ ] PR向けsuiteが10分以内、目標3分以内で完了する
- [ ] 失敗時にPlaywright traceからブラウザ操作と通信失敗を特定できる
- [ ] ローカル実行方法とCIでの調査方法がREADMEに記載されている

## 推奨実装順

1. Workspace rename/delete
2. ログアウトとユーザー間のセッション分離
3. 公開リンクの未ログイン閲覧とrevoke
4. 固定worker fixture
5. アップロード正常系とprocessing中リロード
6. アップロード異常系
7. Paper操作、複数ユーザー認可、課金境界
