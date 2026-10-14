package joblog

import (
	"context"
	"testing"
)

type testLogger struct{}

func (testLogger) Log(context.Context, Event) {}

func TestLoggerContextRoundTrip(t *testing.T) {
	logger := testLogger{}
	ctx := WithLogger(context.Background(), logger)
	if _, ok := FromContext(ctx).(testLogger); !ok {
		t.Fatalf("FromContext() did not return attached logger")
	}
}

func TestFromContextUsesNoopLogger(t *testing.T) {
	if _, ok := FromContext(context.Background()).(*NoopLogger); !ok {
		t.Fatalf("FromContext() returned %T, want *NoopLogger", FromContext(context.Background()))
	}
}
