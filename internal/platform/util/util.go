package util

import (
	"strings"
)

func TruncateRunes(text string, max int) string {
	runes := []rune(strings.TrimSpace(text))
	if max <= 0 || len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "..."
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func HTMLEscape(text string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;")
	return replacer.Replace(text)
}

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
