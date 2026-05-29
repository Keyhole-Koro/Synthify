package llm

import "strings"

// ModelQuota は 1 モデルあたりの公開クォータ参照値。
// 値は Gemini Developer API (ai.google.dev) が tier ごとに公開している
// 固定値を Tier1 ベースで取り込んだもの。Vertex AI 側は Dynamic Shared
// Quota で project ごとに変動するため、Cloud Console の Quotas ページ
// （Service: aiplatform.googleapis.com）の実値を正としつつ、本構造体は
// バックオフ間隔やセマフォ容量を決めるための「目安」として使う。
//
// 出典:
//   - https://ai.google.dev/gemini-api/docs/rate-limits
//   - https://cloud.google.com/vertex-ai/generative-ai/docs/quotas
type ModelQuota struct {
	Model string
	// RPM = requests per minute
	RPM int
	// TPM = input tokens per minute
	TPM int
	// RPD = requests per day
	RPD int
	// Tier は値の出所 tier ラベル（"free" / "tier1" / "tier2" / "tier3" / "vertex-dsq"）。
	Tier string
}

// modelQuotas は固定参照値。preview モデルは公開クォータが無いので
// もっとも近い GA モデルの値にフォールバックさせる（resolveQuota 参照）。
var modelQuotas = map[string]ModelQuota{
	// --- Gemini 2.5 series (Tier1 paid) ---
	"gemini-2.5-pro":        {Model: "gemini-2.5-pro", RPM: 150, TPM: 1_000_000, RPD: 1_000, Tier: "tier1"},
	"gemini-2.5-flash":      {Model: "gemini-2.5-flash", RPM: 300, TPM: 2_000_000, RPD: 1_500, Tier: "tier1"},
	"gemini-2.5-flash-lite": {Model: "gemini-2.5-flash-lite", RPM: 300, TPM: 2_000_000, RPD: 1_500, Tier: "tier1"},

	// --- Gemini 2.0 series — Vertex AI 公式表に固定値が無いので 2.5 同等で暫定 ---
	"gemini-2.0-flash":      {Model: "gemini-2.0-flash", RPM: 300, TPM: 2_000_000, RPD: 1_500, Tier: "tier1"},
	"gemini-2.0-flash-lite": {Model: "gemini-2.0-flash-lite", RPM: 300, TPM: 2_000_000, RPD: 1_500, Tier: "tier1"},

	// --- Gemini 3 preview — preview 帯は限度が更にきついので Free 並みに見積もる ---
	"gemini-3-pro-preview":   {Model: "gemini-3-pro-preview", RPM: 10, TPM: 250_000, RPD: 100, Tier: "free"},
	"gemini-3-flash-preview": {Model: "gemini-3-flash-preview", RPM: 10, TPM: 250_000, RPD: 250, Tier: "free"},
}

// defaultQuota は未登録モデル用フォールバック。バックオフ計算が止まらない
// 程度の保守的な値にしてある（=preview 系 free tier 相当）。
var defaultQuota = ModelQuota{Model: "unknown", RPM: 10, TPM: 250_000, RPD: 100, Tier: "fallback"}

// QuotaFor は指定モデルに紐づくクォータ参照値を返す。完全一致 → preview
// サフィックス剥がし → defaultQuota の順で解決する。
func QuotaFor(model string) ModelQuota {
	if q, ok := modelQuotas[model]; ok {
		return q
	}
	if base, ok := stripPreviewSuffix(model); ok {
		if q, ok := modelQuotas[base]; ok {
			return q
		}
	}
	q := defaultQuota
	q.Model = model
	return q
}

func stripPreviewSuffix(model string) (string, bool) {
	for _, suf := range []string{"-preview", "-exp", "-latest"} {
		if strings.HasSuffix(model, suf) {
			return strings.TrimSuffix(model, suf), true
		}
	}
	return model, false
}
