package util

import (
	"github.com/oklog/ulid/v2"
)

// NewULID generates a new globally unique identifier string using ULID.
func NewULID() string {
	return ulid.Make().String()
}
