# Eval Runner 道A メモ

Eval runner は `apps/eval` に閉じた CLI 評価基盤であり、実装契約の正本は
`apps/eval/runner` の Go 型と doc コメントに置く。

## 現行方針

- 全 tool は `Tool{Name, IOSchema, Run}` の単一概念に統一する。
- `Run` は input JSON を受け取り、output JSON、usage、tool-level error を返す。
- builtin tool と dynamic tool は登録経路だけが違い、runner は同じ型として扱う。
- case YAML の `input:` は prepare 層で JSON 化する。`knowledge_tree` の `chunks` path 展開も prepare 層の責務。
- 判定は output JSON の schema validation と JSON rule のみで行う。
- golden 判定、golden 更新、golden diff は現行 runner の責務から外す。

## CLI

```bash
go run ./apps/eval/cmd --cases apps/eval/cases --format json
go run ./apps/eval/cmd --case apps/eval/cases/knowledge_tree_api_spec.yaml --variant concise-v1
```

主な flag:

| flag | 内容 |
| :--- | :--- |
| `--case` / `--cases` | 単一 case または case directory。どちらか一方を指定する |
| `--format` | `table` または `json` |
| `--variant` | `apps/eval/variants/{name}` の prompt template を使う |
| `--out` / `--out-gcs` | report の保存先 |

## Case Expect

```yaml
expect:
  schema_valid: true
  json:
    - path: $.items
      op: count_gte
      value: 3
    - path: $.items
      op: tree_depth_lte
      value: 4
    - path: $.items[*].title
      op: contains_all
      value:
        - 認証
        - エラーハンドリング
```

JSON path は軽量 subset のみを扱う: `$.x`, `$.x[*].y`, `$.x[i]`。
