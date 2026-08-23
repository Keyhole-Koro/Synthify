# テストマトリクス: local provider daemon

Python daemon、Antigravity CLI adapter、generated ConnectRPC handler の決定論的な PR gate と、
認証済み subscription を使う manual release gate を分離する。通常テストは fake `agy` だけを使い、
quota を消費しない。

## PR gate

| 境界 / 分岐 | テスト | 状態 | 残る gap |
|---|---|---|---|
| CLI version / model discovery | minimum version、canonical model IDs、default model 完全一致 | OK | 実 CLI の出力変更 |
| Prompt transport | NDJSON stdin、argv に prompt なし、job ごとの空 directory | OK | OS ごとの process inspection |
| Structured output | schema 自体を実行前検証、返却 JSON を同じ schema で再検証 | OK | 実モデルの schema 対応品質 |
| Error normalization | quota、invalid output、malformed stream、raw message 非公開 | PARTIAL | 実 CLI の auth/rate/quota error 全形状 |
| Lifecycle | explicit cancel、timeout、malformed stream cleanup、process group stop | OK (POSIX) | Windows process-tree cleanup |
| Resource bound | 最大同時 generation、stdout/request size、bounded timeout | PARTIAL | 境界値、stderr 無限出力 |
| Token file | regular file、symlink 拒否、owner-only mode、長さ/ASCII、path/token 非公開 | OK (POSIX) | Windows ACL |
| ConnectRPC application | bearer auth、Protovalidate、全 unary RPC、typed detail、request task cancel、watchdog | PARTIAL | disconnect 後の実 TCP watchdog |
| Packaging | pinned runtime dependencies、console entry point | PARTIAL | supported OS ごとの wheel/entrypoint smoke |

## Gate policy

- Every PR: Python unit suite、Ruff、generated Go client ↔ production Python ASGI application の実 TCP test。
- Affected PR / pre-release: Go race suite、`buf lint/build/generate/breaking`、clean generated diff。
- Local-provider release: supported OS ごとの package install/start/check/cancel/cleanup smoke。
- Manual only: authenticated `agy` text/structured generation、overage `Never`、auth expiry、rate limit、
  CLI internal retry trace。個人 credential と subscription quota を PR CI に持ち込まない。
- Windows 対応を宣言するまでは Windows ACL と process-tree cleanup の GAP を許容するが、Windows artifact
  を配布しない。実 CLI error mapping と internal retry の GAP が残る間は local PoC flag の外へ出さない。
