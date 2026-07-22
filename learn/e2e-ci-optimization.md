# E2E CI を速くしたときの調査と改善

PR #16 で Web E2E CI を改善した記録です。

単なる変更一覧ではなく、どのようにボトルネックを切り分け、なぜその修正を選び、途中で何に失敗したかを残します。

## 最初の症状

Web CI はおよそ 9〜10 分かかっていました。

Playwright を2シャードに分割した直後の実行では、次のような偏りがありました。

| ジョブ | 実測の目安 |
|---|---:|
| checks | 約37秒 |
| e2e shard 2 | 約4分 |
| e2e shard 1 | 約7分28秒 |

シャーディング自体は動いていましたが、最長の shard 1 が全体の完了時刻を決めていました。

ここで重要なのは、平均時間ではなく **クリティカルパス上の最長ジョブ** を見ることです。

## 調査で分かったこと

遅さには、性質の異なる2つの原因がありました。

### 1. 各シャードが毎回払う固定コスト

`compose.yaml` の backend と worker は、コンテナ起動時にそれぞれ `go build` を実行していました。

```yaml
backend:
  image: golang:1.25-bookworm
  command: go build ... && exec ...

worker:
  image: golang:1.25-bookworm
  command: go build ... && exec ...
```

Go のモジュールキャッシュとビルドキャッシュには named volume が使われていました。これはローカルでは再利用できますが、GitHub Actions のランナーは実行ごとに破棄されるため、CI では毎回ほぼコールドビルドになります。

速いテストしか含まれない shard 2 でも約4分かかっていたことから、テスト件数とは無関係な「起動フロア」があると判断できました。

### 2. 遅いテストが片方に偏る可変コスト

Playwright のシャーディングは、過去の所要時間を見て自動的に均等化するとは限りません。

さらに `apps/web/e2e/upload.spec.ts` は次の設定でした。

```ts
test.describe.configure({ mode: 'serial' });
```

このファイルには、worker の完了を最大30秒待つテストや、失敗結果を最大20秒待つテストを含む7テストがあります。

`serial` の describe はシャーディング上も大きな一塊として扱われやすく、重いアップロード系テストが片方のシャードへ集中していました。

## 実施した変更

## 1. 静的チェックとE2Eを分離した

変更ファイル: `.github/workflows/web.yml`

もともと同じ流れにあった lint、unit test、Next build を `checks` ジョブに分離しました。

E2E は matrix で2シャードにし、別ランナーで並列実行します。

```yaml
strategy:
  fail-fast: false
  matrix:
    shard: [1, 2]
```

実行時には次の引数をPlaywrightへ渡します。

```sh
--shard=1/2
--shard=2/2
```

### 学び

並列化できる処理に依存関係を付けないことが重要です。

`checks` の成果物はE2Eの入力ではないため、E2Eを `needs: checks` にすると、不要な直列待ちが生まれます。

## 2. Go バイナリを一度だけ事前ビルドした

変更ファイル: `.github/workflows/web.yml`

`binaries` ジョブを追加し、API と worker をホストランナー上で一度だけビルドします。

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -buildvcs=false \
  -o .e2e-bin/synthify-api \
  ./apps/api/cmd/server

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -buildvcs=false \
  -o .e2e-bin/synthify-worker \
  ./apps/worker/cmd/server
```

静的バイナリにすることで、Compose 側の `golang:1.25-bookworm` コンテナ内でも追加の共有ライブラリ依存を増やさずに実行できます。

生成した2つのバイナリは GitHub Actions artifact にし、両E2Eシャードからダウンロードします。

### なぜキャッシュだけではなく事前ビルドにしたか

Go キャッシュを各シャードへ復元する方法でもビルド時間は短縮できます。

ただし、その方式では各シャードが依然として `go build` を実行します。

事前ビルドなら、コンパイル自体を1回にまとめられます。今回のように同一コミット・同一アーキテクチャのバイナリを複数ジョブで使う場合は、より直接的です。

## 3. Compose にプリビルドの fast path を追加した

変更ファイル: `compose.yaml`

backend と worker は、プリビルド済みバイナリが存在すれば直接 `exec` します。

```sh
if [ -x /workspace/.e2e-bin/synthify-api ]; then
  exec /workspace/.e2e-bin/synthify-api
fi
```

プリビルドがない場合は、従来の `go build` にフォールバックします。

### 学び

CI高速化のためにローカル開発手順を壊さないよう、既存経路を残しました。

CI専用の最適化を追加するときは、ローカル利用者に新しい必須手順を押し付けない設計が安全です。

## 4. 隠しディレクトリの artifact 問題を修正した

最初の実装では `binaries` ジョブ自体は成功したのに、E2Eジョブが artifact を見つけられず失敗しました。

原因は出力先が `.e2e-bin` という隠しディレクトリだったことです。

`actions/upload-artifact@v4` は、既定では隠しファイル・隠しディレクトリを含めません。

```yaml
- uses: actions/upload-artifact@v4
  with:
    name: e2e-binaries
    path: .e2e-bin
    include-hidden-files: true
```

### 学び

「ビルドステップが成功した」と「成果物が次のジョブへ渡った」は別の検証項目です。

artifactを使うパイプラインでは、次の3点を個別に確認します。

1. ファイルが生成されたか
2. artifact に実ファイルが入ったか
3. consumer ジョブが期待したパスへ展開できたか

警告だけを出して成功扱いになるActionもあるため、ジョブの緑色だけでは不十分です。

## 5. 重い upload テストをシャード可能にした

変更ファイル: `apps/web/e2e/upload.spec.ts`

ローカルでは従来どおり直列実行を維持し、CIだけ `parallel` にしました。

```ts
test.describe.configure({
  mode: process.env.CI ? 'parallel' : 'serial',
});
```

Playwright設定ではCIの `workers` は1なので、同じシャード内で複数アップロードを同時実行するわけではありません。

この変更の目的は、7テストを1つの不可分な塊にせず、2つのGitHub Actionsシャードへ分配可能にすることです。

### 学び

次の2つは別物です。

- 1台のマシン内でテストを同時実行する並列性
- 複数ジョブへテストを分配するシャーディング可能性

今回はComposeスタックの競合を避けるため、各シャード内は `workers: 1` のままにし、ジョブ間だけで並列化しました。

## 結果

### 起動フロアの除去後

- artifact修正後の Web workflow run #234 は成功
- API / worker の事前ビルド、artifact upload、両シャードでのdownloadが成功
- ジョブ表示値の目安では、最長シャードは約8分から約6分へ短縮
- およそ2分、約25%の短縮

コンパイルは、両シャードで毎回行う方式から、キャッシュを使った事前ビルド1回へ変わりました。

### シャード偏りの調整後

- Web workflow run #250 は成功
- `checks`、`binaries`、`e2e (1)`、`e2e (2)` がすべて成功
- 以前は shard 2 が終わった後も shard 1 が長く残っていたが、変更後は両シャードがほぼ同じタイミングで完了

GitHubコネクタの取得結果には秒単位のジョブ開始・終了時刻が含まれないため、ここでは根拠のない秒数を作らず、「長い末尾待ちが解消された」という確認結果を記録します。

## 今回の改善を一般化すると

CIが遅いときは、次の順番で考えると効果的です。

### 1. 最長ジョブを特定する

合計CPU時間ではなく、ユーザーが待つ wall-clock time を決めるジョブを探します。

### 2. 固定コストと可変コストを分ける

- 固定コスト: 環境起動、コンパイル、イメージpull、依存インストール
- 可変コスト: テスト本体、リトライ、データ量、待機処理

固定コストが大きい状態でシャード数だけ増やすと、同じ固定コストを複数回払うことがあります。

### 3. 固定コストを共有または除去する

今回なら、各シャードでの `go build` を共有artifactへ置き換えました。

### 4. シャーディングの単位を確認する

テスト数が同じでも、1件あたりの所要時間は同じではありません。

`serial` グループ、beforeAll、依存プロジェクト、巨大specなどが不可分な塊になっていないか確認します。

### 5. ローカルとCIの要求を分ける

ローカルでは安定性を優先して直列、CIでは独立ランナーを使って分配、というように環境ごとに適切な設定を選べます。

### 6. 改善後に必ず再計測する

ボトルネックを1つ消すと、次のボトルネックが見えるようになります。

今回も、最初はGoコンパイルが支配的でしたが、それを消した後にアップロードテストの偏りが目立つようになりました。

## 変更したファイル

| ファイル | 変更内容 |
|---|---|
| `.github/workflows/web.yml` | checks分離、2シャード化、事前ビルド、artifact受け渡し、hidden file対応 |
| `compose.yaml` | API / worker のプリビルド fast path と従来buildへのフォールバック |
| `.gitignore` | `.e2e-bin/` を除外 |
| `apps/web/e2e/upload.spec.ts` | CIではテスト単位でシャード可能、ローカルではserial維持 |

## 関連コミット

- `0ac6789`: Goバイナリ事前ビルドとComposeへの注入
- `3248c0f`: 隠しディレクトリをartifactへ含める修正
- `3800d91`: upload E2EをCIでシャード可能にする修正

## 次に改善するなら

次の候補は、Playwrightの結果をJSONやblob reporterで保存し、spec・test単位の実測時間を継続的に収集することです。

実測履歴があれば、テストの追加で再び偏りが生まれたときに検知しやすくなり、固定の手動グループ分けより長期的に保守しやすくなります。
