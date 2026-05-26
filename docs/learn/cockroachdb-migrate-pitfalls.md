# CockroachDB × golang-migrate で踏んだ落とし穴

このドキュメントは `db/init/*.sql` を golang-migrate ベースに移行し、`deploy-backend.yml` から CockroachDB Cloud に対して migration を流すまでに踏んだ非互換・silent fail パターンの記録です。同じ罠を再度踏まないための覚え書き。

関連:
- 適用スクリプト: `scripts/apply-db-migrations.sh`
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

## 5. まとめると

- **CockroachDB は PostgreSQL ワイヤ互換だが SQL レベルでは別物**: advisory lock, TRUNCATE privilege, ほか DDL transaction 周りでも差異がある。PostgreSQL の SQL をコピペで持ち込まない。
- **Migration 化の前に「順序依存」を必ず洗う**: psql-per-file の寛容さに甘えていた SQL は移行で必ず詰まる。
- **silent fail を生む `|| true` には fail-fast の保険を**: 握りつぶすなら必ず別経路でログ・接続テスト・型キャストを入れる。

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
