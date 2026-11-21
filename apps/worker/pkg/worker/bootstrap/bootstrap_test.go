package bootstrap

import (
	"context"
	"io"
	"log/slog"
	"testing"

	workermock "github.com/synthify/backend/apps/worker/pkg/worker/repository/mock"
)

func TestInitStoreUsesMockWhenDatabaseDSNIsUnset(t *testing.T) {
	t.Setenv("DATABASE_DSN", "")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := InitStore(context.Background(), logger, nil)
	if _, ok := store.(*workermock.Store); !ok {
		t.Fatalf("InitStore() returned %T, want *mock.Store", store)
	}
}
