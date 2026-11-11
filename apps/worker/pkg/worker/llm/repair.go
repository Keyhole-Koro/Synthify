package llm

import "strings"

// RepairJSON cleans common LLM Markdown fencing and trailing commas from raw JSON strings.
func RepairJSON(input string) string {
	repaired := strings.TrimSpace(input)
	repaired = strings.TrimPrefix(repaired, "```json")
	repaired = strings.TrimPrefix(repaired, "```")
	repaired = strings.TrimSuffix(repaired, "```")
	repaired = strings.TrimSpace(repaired)
	repaired = strings.ReplaceAll(repaired, ",]", "]")
	repaired = strings.ReplaceAll(repaired, ",}", "}")
	return repaired
}
