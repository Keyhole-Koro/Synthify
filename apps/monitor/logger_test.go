package joblog

import (
	"context"
	"testing"
)

type aliasLogger struct{}

func (aliasLogger) Log(context.Context, Event) {}

func TestAliasesDelegateToSharedLogger(t *testing.T) {
	logger := aliasLogger{}
	if _, ok := FromContext(WithLogger(context.Background(), logger)).(aliasLogger); !ok {
		t.Fatal("top-level aliases did not preserve logger")
	}
	if INFO != Level("INFO") || WARN != Level("WARN") || ERROR != Level("ERROR") {
		t.Fatal("log level aliases changed")
	}
}
