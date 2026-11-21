# 0004: Antigravity CLI for Subscription-oriented Runs

**Status:** Accepted  
**Date:** 2026-08-28

## Context

Antigravity SDK の公式 quickstart は Gemini API key または Vertex credentials を使う。一方、Antigravity CLI は
signed-in Google account と headless stream を提供する。Insightify の初期目的は product subscription usage の利用である。

## Decision

subscription-oriented adapter は `agy -p <prompt> --output-format stream-json --sandbox` とする。SDK は BYOK/Vertex 用 Python sidecar として
後続実装する。CLI の interactive approval capability は false とし、run 前の permission profile と isolated worktree で守る。

## Alternatives

- SDK first: policy/hook API は優れるが subscription usage path ではない。
- CLI TUI scraping: terminal presentationへの依存が強い。

## Consequences

- provider間の capability差をUIに表示する必要がある。
- quotaは構造化percentを推測せず、公式のusage command結果をraw表示する。
- dangerous permission bypassを通常UIから提供しない。
