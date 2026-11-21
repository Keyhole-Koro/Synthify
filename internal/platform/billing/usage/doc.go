// Package usage owns the usage-billing pipeline: pricing lookup, credit
// drawdown, accounting persistence, and (in PR 8b) the Stripe meter
// outbox. Both api and worker call into it so the same accounting rules
// apply regardless of which service produced the event.
//
// The event, pricing, credit and result types are the ones in
// internal/domain; this package does not redefine them.
package usage
