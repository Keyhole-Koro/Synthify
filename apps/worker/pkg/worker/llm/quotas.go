package llm

import "strings"

// ModelQuota defines published reference quota limits for an LLM model.
// Values are sourced from Gemini Developer API (ai.google.dev) published Tier 1 limits.
// On Vertex AI, actual quotas vary per project via Dynamic Shared Quotas; this struct
// serves as a baseline guideline for client-side rate limiting and backoff intervals.
//
// References:
//   - https://ai.google.dev/gemini-api/docs/rate-limits
//   - https://cloud.google.com/vertex-ai/generative-ai/docs/quotas
type ModelQuota struct {
	Model string
	// RPM specifies the maximum requests per minute used by client-side rate limiters.
	RPM int
	// Tier identifies the source tier label ("free", "tier1", "tier2", "tier3", "vertex-dsq").
	Tier string
}

// modelQuotas holds reference quota configurations for recognized Gemini models.
var modelQuotas = map[string]ModelQuota{
	// --- Gemini 2.5 series (Tier1 paid) ---
	"gemini-2.5-pro":        {Model: "gemini-2.5-pro", RPM: 150, Tier: "tier1"},
	"gemini-2.5-flash":      {Model: "gemini-2.5-flash", RPM: 300, Tier: "tier1"},
	"gemini-2.5-flash-lite": {Model: "gemini-2.5-flash-lite", RPM: 300, Tier: "tier1"},

	// --- Gemini 3.5 series (Tier1 paid) ---
	"gemini-3.5-flash": {Model: "gemini-3.5-flash", RPM: 300, Tier: "tier1"},

	// --- Gemini 2.0 series — provisional values matching 2.5 series ---
	"gemini-2.0-flash":      {Model: "gemini-2.0-flash", RPM: 300, Tier: "tier1"},
	"gemini-2.0-flash-lite": {Model: "gemini-2.0-flash-lite", RPM: 300, Tier: "tier1"},

	// --- Gemini 3 preview — conservative limits matching free tier ---
	"gemini-3-pro-preview":   {Model: "gemini-3-pro-preview", RPM: 5, Tier: "free"},
	"gemini-3-flash-preview": {Model: "gemini-3-flash-preview", RPM: 5, Tier: "free"},
}

// defaultQuota serves as a conservative fallback for unregistered models.
var defaultQuota = ModelQuota{Model: "unknown", RPM: 5, Tier: "fallback"}

// QuotaFor returns the quota reference for a model name by exact match, suffix stripping, or default fallback.
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

// stripPreviewSuffix removes preview or experimental suffixes to resolve to a base model name.
func stripPreviewSuffix(model string) (string, bool) {
	for _, suf := range []string{"-preview", "-exp", "-latest"} {
		if strings.HasSuffix(model, suf) {
			return strings.TrimSuffix(model, suf), true
		}
	}
	return model, false
}
