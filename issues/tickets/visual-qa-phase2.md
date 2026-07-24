# Post-Deploy Visual QA — Phase 2(認証後画面)

## 背景

Phase 1 で stage デプロイ後の**未認証**画面(ランディング)に対する自動 QA を導入した(生存確認 `@smoke` + 視覚差分 `@visual`)。詳細は [docs/architecture/visual-qa.md](../../docs/architecture/visual-qa.md)。

Phase 2 では**認証後の主要画面**まで視覚差分の対象を広げ、QA の手動確認をさらに削減する。

## 課題

認証後画面を実 stage 相手に撮影するには「毎回同じ画面になる」保証が要る。現状の e2e 認証は Firebase Auth **エミュレータ**前提(`createTestUser` / `auth.setup.ts`)で、実 stage には使えない。

## やること

1. **専用 stage QA アカウント**を1つ用意し、資格情報を stage environment の secret(`E2E_QA_EMAIL` / `E2E_QA_PASSWORD` 等)に登録。
2. `visual` プロジェクト用に、実 stage へログインして `storageState` を生成する setup を追加(エミュレータ用 `auth.setup.ts` とは分離)。
3. **固定 Firestore シード**: `scripts/seed-firestore-jobs.mjs` を stage QA ユーザー向けに流用し、`visual-qa` job の頭で「リセット→シード」。一覧/詳細系を決定的にする。
4. 相対時刻・ランダムID等の残存非決定要素は `mask:` で塗る。
5. `@visual` の対象に主要な認証後画面(ワークスペース等)を追加し、Linux CI でベースラインを確定。

## 判断ポイント

- シードのリセット運用が重い/リスクがある場合は、認証後スコープを「空状態・設定画面」などデータ非依存の画面に絞るのが現実解。
- prod への横展開は stage でフレーキー率が十分低いと確認できてから。

## 関連

- Phase 1 実装: `apps/web/e2e/deploy-smoke.visual.ts`, `.github/workflows/deploy-frontend.yml`(`visual-qa` job)
- [e2e-test-expansion.md](./e2e-test-expansion.md)
