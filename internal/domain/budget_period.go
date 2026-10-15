package domain

import "time"

// budgetCalendarZone is the zone of the calendar month used when an account has
// no live billing period. Users are in Japan; JST has no DST, so a fixed zone
// avoids depending on tzdata in the runtime image.
var budgetCalendarZone = time.FixedZone("JST", 9*60*60)

// BudgetPeriodStart returns the start of the period the monthly budget is
// measured over. It is the Stripe billing period when one is recorded and
// contains now, so the budget lines up with what the invoice will charge.
// Otherwise (free accounts, or a period whose renewal has not arrived yet) it is
// the start of the current calendar month in JST.
//
// periodStartedAt and periodEnd are RFC 3339 strings; empty or unparsable
// values count as missing.
func BudgetPeriodStart(periodStartedAt, periodEnd string, now time.Time) time.Time {
	start, startErr := time.Parse(time.RFC3339, periodStartedAt)
	end, endErr := time.Parse(time.RFC3339, periodEnd)
	if startErr == nil && endErr == nil && !now.Before(start) && now.Before(end) {
		return start.UTC()
	}
	local := now.In(budgetCalendarZone)
	return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, budgetCalendarZone).UTC()
}

// BudgetExceeded reports whether usage in the current period is over the
// budget. A zero or negative limit means no budget.
func BudgetExceeded(limitMinor, periodUsageMinor int64) bool {
	return limitMinor > 0 && periodUsageMinor > limitMinor
}
