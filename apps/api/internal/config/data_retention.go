package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// DataRetention says how long deleted data is kept before it is purged for
// good (docs/data-retention.md).
type DataRetention struct {
	// DeletedWorkspaces is how long a deleted workspace keeps its rows and
	// stored files. DELETED_WORKSPACE_RETENTION_DAYS, default 30; 0 purges it
	// at the next sweep.
	DeletedWorkspaces time.Duration
}

const defaultDeletedWorkspaceRetentionDays = 30

// loadDataRetention panics on a malformed value, like mustBaseURL: purging on
// a schedule nobody chose is worse than refusing to start.
func loadDataRetention() DataRetention {
	days := defaultDeletedWorkspaceRetentionDays
	if raw := strings.TrimSpace(os.Getenv("DELETED_WORKSPACE_RETENTION_DAYS")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			panic(fmt.Sprintf("DELETED_WORKSPACE_RETENTION_DAYS must be a whole number of days >= 0, got %q", raw))
		}
		days = n
	}
	return DataRetention{DeletedWorkspaces: time.Duration(days) * 24 * time.Hour}
}
