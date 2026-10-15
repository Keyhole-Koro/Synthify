package util

import "testing"

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name string
		text string
		max  int
		want string
	}{
		{name: "trims without truncating", text: "  hello  ", max: 10, want: "hello"},
		{name: "truncates by rune", text: "あいうえお", max: 3, want: "あいう..."},
		{name: "non-positive max returns trimmed text", text: "  hello  ", max: 0, want: "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TruncateRunes(tt.text, tt.max); got != tt.want {
				t.Fatalf("TruncateRunes() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := FirstNonEmpty(" ", "", "  value  "); got != "  value  " {
		t.Fatalf("FirstNonEmpty() = %q, want original non-empty value", got)
	}
	if got := FirstNonEmpty("", "  "); got != "" {
		t.Fatalf("FirstNonEmpty() = %q, want empty", got)
	}
}

func TestHTMLEscape(t *testing.T) {
	got := HTMLEscape(`<script title="x">a & 'b'</script>`)
	want := "&lt;script title=&#34;x&#34;&gt;a &amp; &#39;b&#39;&lt;/script&gt;"
	if got != want {
		t.Fatalf("HTMLEscape() = %q, want %q", got, want)
	}
}

func TestNormalizeRiskTier(t *testing.T) {
	tests := map[string]string{
		"tier_3":              "tier_3",
		" approval_required ": "tier_3",
		"TIER_2":              "tier_2",
		"review_required":     "tier_2",
		"unknown":             "tier_1",
	}
	for input, want := range tests {
		if got := NormalizeRiskTier(input); got != want {
			t.Errorf("NormalizeRiskTier(%q) = %q, want %q", input, got, want)
		}
	}
}
