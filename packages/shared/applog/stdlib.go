package applog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

type StdLogger struct {
	logger *slog.Logger
}

type entry struct {
	Timestamp        string         `json:"timestamp"`
	Level            string         `json:"level"`
	Event            string         `json:"event"`
	Error            string         `json:"error,omitempty"`
	Fields           map[string]any `json:"fields,omitempty"`
	DroppedFieldKeys []string       `json:"dropped_field_keys,omitempty"`
}

func NewStdLogger() Logger {
	return WrapSlogLogger(NewJSONSlogLogger(os.Stdout))
}

func NewJSONSlogLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

func WrapSlogLogger(logger *slog.Logger) Logger {
	return StdLogger{
		logger: logger,
	}
}

func NewSlogLogger(w io.Writer) Logger {
	return WrapSlogLogger(NewJSONSlogLogger(w))
}

func (l StdLogger) Info(ctx context.Context, event string, fields map[string]any) {
	write(ctx, l.logger, slog.LevelInfo, event, nil, fields)
}

func (l StdLogger) Warn(ctx context.Context, event string, err error, fields map[string]any) {
	write(ctx, l.logger, slog.LevelWarn, event, err, fields)
}

func (l StdLogger) Error(ctx context.Context, event string, err error, fields map[string]any) {
	write(ctx, l.logger, slog.LevelError, event, err, fields)
}

func write(ctx context.Context, logger *slog.Logger, level slog.Level, event string, err error, fields map[string]any) {
	safeFields, droppedKeys := sanitizeFields(fields)
	record := entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level.String(),
		Event:     event,
	}
	if len(safeFields) > 0 {
		record.Fields = safeFields
	}
	if len(droppedKeys) > 0 {
		record.DroppedFieldKeys = droppedKeys
	}
	if err != nil {
		record.Error = err.Error()
	}
	attrs := []any{
		"timestamp", record.Timestamp,
		"event", record.Event,
	}
	if record.Error != "" {
		attrs = append(attrs, "error", record.Error)
	}
	if len(record.Fields) > 0 {
		attrs = append(attrs, "fields", record.Fields)
	}
	if len(record.DroppedFieldKeys) > 0 {
		attrs = append(attrs, "dropped_field_keys", record.DroppedFieldKeys)
	}
	logger.Log(ctx, level, event, attrs...)
}

func sanitizeFields(fields map[string]any) (map[string]any, []string) {
	if len(fields) == 0 {
		return nil, nil
	}
	safe := make(map[string]any, len(fields))
	var dropped []string
	for key, value := range fields {
		safeValue, ok := sanitizeValue(value)
		if !ok {
			dropped = append(dropped, key)
		}
		safe[key] = safeValue
	}
	return safe, dropped
}

func sanitizeValue(value any) (any, bool) {
	if value == nil {
		return nil, true
	}
	if _, err := json.Marshal(value); err == nil {
		return value, true
	}
	if stringer, ok := value.(fmt.Stringer); ok {
		return stringer.String(), false
	}
	if err, ok := value.(error); ok {
		return err.Error(), false
	}
	return "<unmarshalable>", false
}
