package util

import (
	"strings"
)

// TruncateRunes trims surrounding whitespace from text and limits it to at most
// max runes. If the text exceeds max runes, it is truncated and appended with "...".
func TruncateRunes(text string, max int) string {
	runes := []rune(strings.TrimSpace(text))
	if max <= 0 || len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "..."
}

// FirstNonEmpty returns the first non-empty, non-whitespace string among the provided values.
// Returns an empty string if all provided values are empty or contain only whitespace.
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// HTMLEscape replaces special HTML characters (&, <, >, ", ') in text with their
// corresponding HTML entities to prevent injection attacks.
func HTMLEscape(text string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;")
	return replacer.Replace(text)
}

// NormalizeRiskTier standardizes an input risk tier string into canonical tier identifiers:
// "tier_3" (for high risk / approval required), "tier_2" (for medium risk / review required),
// and "tier_1" (default / low risk).
func NormalizeRiskTier(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "tier_3", "approval_required":
		return "tier_3"
	case "tier_2", "review_required":
		return "tier_2"
	default:
		return "tier_1"
	}
}
