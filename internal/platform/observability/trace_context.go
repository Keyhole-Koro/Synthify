package observability

import (
	"context"
	"net/http"

	"github.com/newrelic/go-agent/v3/newrelic"
	"go.opentelemetry.io/otel/propagation"
)

// traceContextPropagator intentionally carries only W3C Trace Context.
// Baggage is not propagated across the worker boundary so request content,
// tokens, or other user data cannot accidentally become telemetry metadata.
var traceContextPropagator propagation.TextMapPropagator = propagation.TraceContext{}

// InjectTraceContext writes W3C traceparent/tracestate headers into headers.
//
// Today API requests are instrumented by the New Relic Go agent. When a New
// Relic transaction is present, use its distributed-tracing headers to seed
// the vendor-neutral OpenTelemetry context before injecting it. If the caller
// already has an OpenTelemetry SpanContext, it is propagated directly. This
// keeps the Cloud Tasks boundary independent of the tracing backend.
func InjectTraceContext(ctx context.Context, headers http.Header) {
	if headers == nil {
		return
	}

	if txn := newrelic.FromContext(ctx); txn != nil {
		seed := make(http.Header)
		txn.InsertDistributedTraceHeaders(seed)
		ctx = traceContextPropagator.Extract(ctx, propagation.HeaderCarrier(seed))
	}

	traceContextPropagator.Inject(ctx, propagation.HeaderCarrier(headers))
}

// ExtractTraceContext restores W3C trace context from an inbound HTTP request.
// It does not propagate baggage; see traceContextPropagator above.
func ExtractTraceContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := traceContextPropagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// NewRelicHTTPHandler instruments a plain net/http endpoint with New Relic.
// SetWebRequestHTTP consumes the W3C headers propagated by OpenTelemetry, so
// the worker transaction remains part of the API trace across Cloud Tasks.
func NewRelicHTTPHandler(app *newrelic.Application, name string, next http.Handler) http.Handler {
	if app == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		txn := app.StartTransaction(name)
		txn.SetWebRequestHTTP(r)
		w = txn.SetWebResponse(w)
		defer txn.End()

		ctx := newrelic.NewContext(r.Context(), txn)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
