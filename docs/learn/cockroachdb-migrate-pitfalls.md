# CockroachDB × golang-migrate で踏んだ落とし穴

このドキュメントは `db/init/*.sql` を golang-migrate ベースに移行し、`deploy-backend.yml` から CockroachDB Cloud に対して migration を流すまでに踏んだ非互換・silent fail パターンの記録です。同じ罠を再度踏まないための覚え書き。後半 (6.) は同じ移行を **dev の docker compose** にも徹底するまでに踏んだ罠。

関連:
- 適用スクリプト (prod/CI): `scripts/apply-db-migrations.sh`
- 適用スクリプト (dev compose): `scripts/dev-apply-migrations.sh`
- マイグレーション: `db/migrations/`
- CD 連携: `.github/workflows/deploy-backend.yml` の "Apply DB migrations" step

---

## 1. `pg_advisory_lock()` not implemented

- **症状**: `migrate up` 実行直後に `try lock failed in line 0: SELECT pg_advisory_lock($1) (details: pq: unknown function: pg_advisory_lock())` で即停止。
- **原因**: golang-migrate のデフォルト postgres driver は migration 適用前に `pg_advisory_lock()` を取って排他制御するが、**CockroachDB はこの関数を実装していない**。
- **対処**: DSN のスキームを `postgres://` → `cockroachdb://` に書き換える。migrate は CockroachDB 専用 driver を選び、advisory lock の代わりに CockroachDB ネイティブの方式を使うので問題は起きない。
- **実装**: 接続文字列を 2 系統用意する。
    - `MIGRATE_DSN`: `cockroachdb://...` (migrate 専用)
    - `PSQL_DSN`: `postgres://...` (psql は cockroachdb スキームを理解しないので残す)

## 2. `TRUNCATE` privilege not recognised

- **症状**: `REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON ... FROM monitor` で `pq: at or near "on": syntax error: not a valid privilege: "truncate"`。
- **原因**: **CockroachDB は TRUNCATE を独立した privilege として扱わない**。DELETE 権限の一部として表現される。PostgreSQL では GRANT/REVOKE 可能だが CockroachDB では構文エラーになる。
- **対処**: `TRUNCATE` を REVOKE 句から外す。`INSERT, UPDATE, DELETE` の REVOKE だけで write 操作は十分封じられる (TRUNCATE は DELETE 権限経由で制御される)。
- **教訓**: PostgreSQL の GRANT/REVOKE をコピペで持ち込まない。privilege キーワード単位で互換性を疑う。

## 3. Migration の順序依存は厳格 — psql-per-file の甘えに注意

- **症状**: `migrate up` が `0005_monitor_role` でエラー停止。`document_processing_jobs` への GRANT を試みたが、そのテーブルは後続の migration で作成される予定だった。
- **背景**: 旧 `db/init/004_monitor_role.sql` 時代は compose の `postgres-init` が `for f in *.sql; do cockroach sql ... < $f` で**ファイル単位に独立実行**していたため、GRANT がエラーになっても次の SQL に進んで結果的に動いていた。migrate にはこの寛容さは無い。
- **対処**: 参照テーブルが全部揃った後で GRANT する順序にする。`0005_monitor_role` → `0014_monitor_role` にリネームし、`db/migrations` の末尾に移動。
- **教訓**: psql-per-file 時代に動いてた SQL を migration 化するときは、**依存テーブルの作成順を必ず確認する**。失敗してたコマンドが silent skip で誤魔化されていた可能性がある。

## 4. dirty 検出が 2 回連続で silent fail した話

migration が途中失敗すると `schema_migrations.dirty = true` がセットされ、次回 `migrate up` は即停止する。自動回復のために「dirty 検出 → 直前バージョンに `migrate force` → 再 up」というロジックを書いたが、2 段階の silent fail で**ログには何も出ず**自動回復が機能しなかった。

### 4-1. 第 1 段: psql が `sslmode=verify-full` のまま接続失敗していた

- **症状**: probe ログに `Probe results: schema_migrations=<empty>, accounts=<empty>` のような空文字。検出ロジックがすべて空振り。
- **原因**: probe 関数は `psql ... 2>/dev/null || true` で書かれていた。GitHub runner には `~/.postgresql/root.crt` が無いので `sslmode=verify-full` の DSN は接続自体に失敗、エラーは `|| true` で握りつぶされ、空文字だけが返って silent に処理が進んでいた。
- **対処 (2 つ)**:
    1. `PSQL_DSN` を `sslmode=require` に sed で書き換え (probe は metadata 読みだけだから verify-full は不要)。
    2. **接続テストを最初に 1 回だけ実行**、失敗したら明示的に exit。
       ```bash
       if ! psql "$PSQL_DSN" -At -c "SELECT 1" >/dev/null 2>&1; then
           echo "psql could not connect — schema probes will not work." >&2
           exit 1
       fi
       ```
- **教訓**: 「エラーは握りつぶすが結果は信頼する」は脆い。`|| true` で逃がすなら、その結果が空文字でも別経路で **fail-fast の接続テスト** + **probe 結果の echo** を入れる。

### 4-2. 第 2 段: 検出クエリの型エラーがやはり silent fail

- **症状**: 接続は通った (`schema_migrations=t`) のに、dirty 検出のログが出ず `up` が即エラー。
- **原因**: 検出クエリが
    ```sql
    SELECT version || '|' || dirty FROM schema_migrations LIMIT 1
    ```
    で、`dirty` (BOOL) を `||` で text 連結しようとして型エラー。`2>/dev/null` で潰されて空文字を返し、`dirty_flag != "t"` で素通り。
- **対処**: クエリを単純化。
    ```sql
    SELECT version::text FROM schema_migrations WHERE dirty = true LIMIT 1
    ```
    結果が非空なら dirty、空ならクリーンと判定できる。
- **教訓**: SQL の型推論を probe に頼らない。型キャスト (`::text`) を明示し、ロジックは「行があるか / 無いか」のように単純化する。

## 6. dev compose が migration を毎回 replay して forward-only DROP と衝突 (2026-06-10)

prod/CI は 1.〜5. で golang-migrate に移行済みだったが、**dev の `docker compose`** はずっと取り残されていた。`compose.yaml` の `postgres-init` が

```sh
for f in /migrations/*.up.sql; do cockroach sql ... < "$f"; done
```

で**起動のたびに全 `*.up.sql` を頭から replay** していた (3. で触れた psql-per-file の甘えがそのまま残存)。

- **症状**: `docker compose up` で `postgres-init` が `exit 1`。ログ末尾は
    ```
    Applying migration /migrations/0008_tree.up.sql...
    NOTICE: relation "tree_items" already exists, skipping
    ERROR: column "kind" does not exist (SQLSTATE 42703)
    ```
- **原因**: forward-only migration の `0018_node_direct_tree` が `tree_items.kind` を DROP した後、replay で `0008_tree` の `CREATE INDEX ... ON tree_items(workspace_id, kind)` が再実行され、もう存在しない `kind` を参照して落ちた。`CREATE TABLE IF NOT EXISTS` はスキップされ、後続の index 作成だけが死ぬ。
- **罠の質**: **空ボリュームでは通り、2 回目の `up` で初めて落ちる**。`8で作る→18で消す` は 1 パスなら順序が通るが、replay で `8` を再実行すると `18` の DROP 後の状態と衝突する。`down -v` すると直って見えるので「ボリュームが古い」と誤診しやすい。
- **対処**: dev も golang-migrate に統一。`scripts/dev-apply-migrations.sh` (prod 版のローカル insecure 版) を作り、`postgres-init` から実行。`schema_migrations` でバージョン追跡し**各 migration を一度だけ**適用する。
    - migrate バイナリは cockroach イメージに無いので runtime で取得 (CI と同じ release)。
    - baseline 判定は「テーブル有無」でなく **version 行が記録されているか** で見る。旧 replay 製ボリュームは `schema_migrations` が存在するのに**空**という中途半端な状態があり得るため (`has_accounts=t && applied_version 空 → migrate force HEAD`)。
- **教訓**:
    - **forward-only な DROP は「各 migration を 1 回だけ適用」が大前提**。冪等 replay 機構 (`IF NOT EXISTS` の羅列) とは構造的に両立しない。`IF NOT EXISTS` は後続 DROP と組むと冪等でなくなる。
    - **スキーマ適用は dev/CI/prod で同じ機構を共有する**。片方だけ旧方式で残すと、そこでしか踏まないバグが生まれる。
    - **「初回は通る」バグは状態依存**。`down -v` での復旧は応急処置。2 回目の実行まで確認しないと直ったとは言えない。
    - 過去 migration は squash/書き換えしない。「作ってすぐ DROP」の往復は正常コスト。

## 7. まとめると

- **CockroachDB は PostgreSQL ワイヤ互換だが SQL レベルでは別物**: advisory lock, TRUNCATE privilege, ほか DDL transaction 周りでも差異がある。PostgreSQL の SQL をコピペで持ち込まない。
- **Migration 化の前に「順序依存」を必ず洗う**: psql-per-file の寛容さに甘えていた SQL は移行で必ず詰まる。
- **silent fail を生む `|| true` には fail-fast の保険を**: 握りつぶすなら必ず別経路でログ・接続テスト・型キャストを入れる。
- **適用機構は全環境で統一する**: forward-only migration を採用したら、dev compose も含めて「各 migration を 1 回だけ適用」を保証する。replay 方式を 1 箇所でも残すと DROP 系 migration で詰む。

---

## 参考: 今回構築したフロー

1. CD (`deploy-backend.yml`) が secret から DSN を取得。
2. `MIGRATE_DSN` (cockroachdb://) と `PSQL_DSN` (postgres://, sslmode=require) に分割。
3. `psql "$PSQL_DSN" -c "SELECT 1"` で fail-fast 接続テスト。
4. metadata probe で
   - `schema_migrations` が無く `accounts` が有る → 既存 DB → `migrate force HEAD` で baseline
   - `schema_migrations.dirty = true` → 直前バージョンに `migrate force` して up を retry させる
5. `migrate -database "$MIGRATE_DSN" up` 本体。
6. `migrate version` で適用結果を出力。

Cloud Run のロールアウトより前に走らせるので、「新しいコードが古いスキーマに当たる」事故は構造的に防げる。
