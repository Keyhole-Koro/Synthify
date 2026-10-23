package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestInjectTraceContextFromOpenTelemetryContext(t *testing.T) {
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatalf("trace id: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatalf("span id: %v", err)
	}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanContext)

	headers := make(http.Header)
	InjectTraceContext(ctx, headers)

	const want = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	if got := headers.Get("traceparent"); got != want {
		t.Fatalf("traceparent = %q, want %q", got, want)
	}
	if got := headers.Get("baggage"); got != "" {
		t.Fatalf("baggage unexpectedly propagated: %q", got)
	}
}

func TestExtractTraceContextAddsSpanContextToRequest(t *testing.T) {
	const traceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

	var got trace.SpanContext
	handler := ExtractTraceContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = trace.SpanContextFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/internal/dispatch-job", nil)
	req.Header.Set("traceparent", traceparent)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusNoContent)
	}
	if !got.IsValid() {
		t.Fatal("expected valid span context")
	}
	if got.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace id = %s", got.TraceID())
	}
	if !got.IsRemote() {
		t.Fatal("inbound trace context should be remote")
	}
}

func TestNewRelicHTTPHandlerWithoutAppPassesThrough(t *testing.T) {
	called := false
	handler := NewRelicHTTPHandler(nil, "test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	}))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/", nil))

	if !called {
		t.Fatal("wrapped handler was not called")
	}
	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusAccepted)
	}
}
