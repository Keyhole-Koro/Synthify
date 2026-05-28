package usage

import (
	"fmt"
	"strconv"
)

// ComputeCostMinor turns token counts into a cost in the smallest currency
// unit. The formula is integer arithmetic by design: fractional minor units
// cannot be billed and we do not want any single event to land at half a
// cent or half a yen.
//
//	cost_minor = (input_tokens * input_rate + output_tokens * output_rate) / 1_000_000
func ComputeCostMinor(p *ModelPricing, inputTokens, outputTokens int64) int64 {
	const million = int64(1_000_000)
	return (inputTokens*p.InputCostPerMTokenMinor)/million + (outputTokens*p.OutputCostPerMTokenMinor)/million
}

// FormatMinor renders a minor-unit amount as the conventional decimal string
// for the currency: JPY is an integer, everything else uses two decimal
// places.
func FormatMinor(minor int64, currency string) string {
	if currency == "jpy" {
		return strconv.FormatInt(minor, 10)
	}
	neg := ""
	if minor < 0 {
		neg = "-"
		minor = -minor
	}
	return neg + strconv.FormatInt(minor/100, 10) + "." + fmt.Sprintf("%02d", minor%100)
}
