package joblog

import (
	"context"
	"testing"
)

type testLogger struct{}

func (testLogger) Log(context.Context, Event) {}

func TestLoggerContextRoundTrip(t *testing.T) {
	logger := testLogger{}
	if _, ok := FromContext(WithLogger(context.Background(), logger)).(testLogger); !ok {
		t.Fatal("FromContext() did not return the attached logger")
	}
}

func TestFromContextUsesNoopLogger(t *testing.T) {
	if _, ok := FromContext(context.Background()).(*NoopLogger); !ok {
		t.Fatalf("FromContext() returned %T, want *NoopLogger", FromContext(context.Background()))
	}
}
