# テストマトリクス: `local_provider_test.go`

このマトリクスは Worker の `LocalProviderClient` seam に対する決定論的な Go contract test と、
generated Go client ↔ Python fake server の実 TCP gate、後続の live-provider gate を分離する。
カバレッジ数値は未計測 (2026-08-22)。

## インターフェース網羅チェック

| API / 経路 | 専用テスト | 状態 | 未テストの主分岐 |
| --- | --- | --- | --- |
| `NewLocalProviderClient` | ✅ readiness/capabilities/auth を各 client setup で通過 | PARTIAL | unreachable、not-ready、malformed protobuf response |
| `GenerateText` | ✅ success、cancel、timeout、typed error、source拒否、request validation | OK | response size 上限ちょうど、empty response |
| `GenerateStructured` | ✅ success、schema送信、worker-side response validation | PARTIAL | invalid JSON、schema size境界 |
| `Capabilities` | ◐ generation test setup と semantic helper test | PARTIAL | defensive copy の変更耐性、refresh |
| `LocalProviderError.RetryablePreTurnRateLimit` | ✅ typed rate-limit detail | PARTIAL | `turn_started=true`、別code/reason の全組み合わせ |
| `InitProcessClient` | ✅ Gemini identity、production fail-closed | PARTIAL | local success は constructor tests で間接確認 |
| token file reader | ✅ `0600` / `0644` | PARTIAL | symlink、短いtoken、巨大file、Windows ACL |

## 依存エラー軸

| 経路 | Connect error | context cancel | deadline | invalid response | auth header |
| --- | --- | --- | --- | --- | --- |
| `GenerateText` | ☑ typed `resource_exhausted` | ☑ explicit cancel RPC | ☑ explicit cancel RPC | ◐ request validation中心 | ☑ 全RPCで観測 |
| `GenerateStructured` | ❌ | ❌ | ❌ | ☑ JSON Schema mismatch | ☑ |
| startup | ❌ | ❌ | ◐ bounded context | ◐ semantic capabilities | ☑ Check/capabilities |

## テストケース表

| テストケース | 主保証 | 副作用 assertion | 状態 |
| --- | --- | --- | --- |
| `TestLocalProviderClientGenerateText` | model/prompt/generation ID と usage を protobuf 経由で保持 | bearer token が startup/generation 全RPCに付く | OK |
| `TestLocalProviderClientGenerateStructuredValidatesResponse` | Go type から JSON Schema を生成し provider へ送り、response を同じ schema で再検証 | schema不一致を成功扱いしない | OK |
| `TestLocalProviderClientGenerateStructured` | schema-valid payload と usage を保持 | bearer token が付く | OK |
| `TestLocalProviderClientCancellationCallsExplicitRPC` | caller cancel 時に同じ generation ID の `CancelGeneration` を呼ぶ | detached RPC 完了後に `context.Canceled` | OK |
| `TestLocalProviderClientTimeoutCallsExplicitRPC` | client deadline 時にも explicit cancel を呼ぶ | `context.DeadlineExceeded` | OK |
| `TestLocalProviderClientDoesNotRetryGeneration` | typed rate-limit detail をdecodeし raw provider message を隠す | generation call は1回のみ | OK |
| `TestLocalProviderClientRejectsSourceFilesBeforeRPC` | shared job volume 前は source files を拒否 | generation call 0回 | OK |
| `TestLocalProviderClientValidatesTextBeforeRPC` | Protovalidate 境界違反を client 側で拒否 | generation call 0回 | OK |
| `TestReadLocalProviderTokenRequiresOwnerOnlyFile` | token file の owner-only permission | token/path をerrorへ出さない | PARTIAL |
| `TestValidateLocalProviderCapabilitiesFailsClosed` | default/model prefix/duplicate/structured support の semantic gate | invalid capabilities をcacheしない | OK |
| `TestInitProcessClientGeminiKeepsExistingClient` | default Gemini path の identity を維持 | 新RPCなし | OK |
| `TestInitProcessClientLocalFailsClosedInProduction` | production local-provider 設定を startup 前に拒否 | token file / networkへ到達しない | OK |
| `TestLocalProviderCrossLanguageContract` | generated Go client と generated Python WSGI server の全 unary RPC、typed detail、Protovalidate wire 互換 | bearer auth、同一 generation ID の explicit cancel、no live credentials | OK |

## 未テスト分岐 (GAP)

| GAP | 必要な次のテスト | Gate |
| --- | --- | --- |
| daemon timeout / orphan watchdog | Worker crash を模擬して provider turn が最大期限で停止 | release gate |
| explicit pre-turn rate-limit retry | `turn_started=false` のみ bounded retry、その他は1回 | retry実装時のPR gate |
| Windows token ACL | supported Windows runner で owner-only ACL を確認 | Windows配布 gate |
| source-file shared volume | traversal/symlink escape、cleanup、job isolation | Phase 3 gate |

Python runtime を install した上で `SYNTHIFY_LOCAL_PROVIDER_PYTHON=python go test ./...` を CI の
cross-language PR gate とする。`go test -race ./apps/worker/cmd/server ./apps/worker/pkg/worker/config
./apps/worker/pkg/worker/llm` もこの seam の affected-PR gate とする。live Antigravity/Codex session は
通常PR gateに含めない。
