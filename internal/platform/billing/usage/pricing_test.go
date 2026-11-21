package usage

import (
	"testing"

	"github.com/synthify/backend/internal/domain"
)

func TestComputeCostMinor(t *testing.T) {
	pricing := &domain.ModelPricing{InputCostPerMTokenMinor: 3_000, OutputCostPerMTokenMinor: 5_000}
	if got := ComputeCostMinor(pricing, 1_000_000, 2_000_000); got != 13_000 {
		t.Fatalf("ComputeCostMinor() = %d, want 13000", got)
	}
}

func TestFormatMinor(t *testing.T) {
	if got := FormatMinor(1234, "jpy"); got != "1234" {
		t.Fatalf("JPY format = %q", got)
	}
	if got := FormatMinor(1234, "usd"); got != "12.34" {
		t.Fatalf("USD format = %q", got)
	}
	if got := FormatMinor(-5, "usd"); got != "-0.05" {
		t.Fatalf("negative format = %q", got)
	}
}

func TestProratedTokens(t *testing.T) {
	if in, out := ProratedTokens(100, 50, 100, 25); in != 25 || out != 12 {
		t.Fatalf("ProratedTokens() = (%d, %d), want (25, 12)", in, out)
	}
	if in, out := ProratedTokens(100, 50, 0, 10); in != 100 || out != 50 {
		t.Fatalf("zero total = (%d, %d), want original tokens", in, out)
	}
	if in, out := ProratedTokens(100, 50, 100, 150); in != 100 || out != 50 {
		t.Fatalf("full stripe = (%d, %d), want original tokens", in, out)
	}
}
