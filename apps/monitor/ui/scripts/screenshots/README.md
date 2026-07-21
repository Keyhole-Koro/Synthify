# Monitor screenshot harness

`monitor` のダッシュボード画面を Playwright で撮影するための最小ハーネス。
実 DB / Firebase を立てずに、モックデータで各タブを撮影できる。

## 仕組み

- **認証**: `AuthGate` の dev 限定プレビューフラグ (`NEXT_PUBLIC_MONITOR_PREVIEW=1`)
  でサインインを素通りする。`NODE_ENV === 'production'` では常に無効なので本番
  ビルドで認証がバイパスされることはない。
- **データ**: 実 API を叩かず、`fixtures/*.json` を Playwright の
  `page.route()` インターセプトで返す。Postgres は不要。
- **ブラウザ**: `playwright-core` + プリインストール済み Chromium
  (`CHROMIUM_PATH`、既定 `/opt/pw-browsers/chromium`) を使用。ブラウザの
  ダウンロードは行わない。

## 実行

```sh
cd apps/monitor/ui
bun install          # playwright-core を入れる
bun run screenshots  # next dev を起動 → 撮影 → 後始末
```

出力は `docs/monitor-screenshots/*.png` (job-health / cost / workspace /
errors / logs)。

既に dev サーバーが動いている場合は capture だけ実行できる:

```sh
BASE_URL=http://127.0.0.1:5174 node scripts/screenshots/capture.mjs
```

## ファイル

| ファイル | 役割 |
| --- | --- |
| `run.mjs` | next dev をプレビューモードで起動し capture を回すランナー |
| `capture.mjs` | ルートをモックしてタブを巡回・撮影する Playwright スクリプト |
| `fixtures/*.json` | 各ダッシュボード / ジョブ一覧 / ログのモック応答 |
