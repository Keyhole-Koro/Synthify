package imageref

import (
	"strings"
	"testing"
)

func TestRender_RegisteredToken(t *testing.T) {
	reg := NewRegistry()
	id, ok := reg.Register("file-abc", "A diagram")
	if !ok || id != 1 {
		t.Fatalf("Register id=%d ok=%v", id, ok)
	}

	out := Render("<p>See [[img:1]] below.</p>", reg)
	want := `<p>See <img data-file-id="file-abc" alt="A diagram" loading="lazy"> below.</p>`
	if out != want {
		t.Fatalf("Render =\n  %q\nwant\n  %q", out, want)
	}
}

func TestRender_UnregisteredTokenDropped(t *testing.T) {
	reg := NewRegistry()
	reg.Register("file-1", "")

	// [[img:7]] was never registered — a hallucinated reference. It must vanish.
	out := Render("<p>a[[img:7]]b</p>", reg)
	if strings.Contains(out, "img:7") || strings.Contains(out, "<img") {
		t.Fatalf("unregistered token survived: %q", out)
	}
	if out != "<p>ab</p>" {
		t.Fatalf("Render = %q, want <p>ab</p>", out)
	}
}

func TestRender_StripsModelAuthoredImgTag(t *testing.T) {
	reg := NewRegistry()
	// The model disobeyed and wrote its own <img> with a fabricated URL.
	out := Render(`<p>x</p><img src="https://evil.example/hallucinated.png">`, reg)
	if strings.Contains(out, "<img") || strings.Contains(out, "hallucinated") {
		t.Fatalf("model-authored <img> survived: %q", out)
	}
}

func TestRender_EscapesAttributes(t *testing.T) {
	reg := NewRegistry()
	reg.Register(`f"><script>x`, `"><b>alt`)
	out := Render("[[img:1]]", reg)
	if strings.Contains(out, "<script>") || strings.Contains(out, "<b>") {
		t.Fatalf("attributes not escaped: %q", out)
	}
}

func TestRender_NilRegistryStripsEverything(t *testing.T) {
	out := Render(`hi [[img:1]]<img src="x.png"> bye`, nil)
	if strings.Contains(out, "<img") || strings.Contains(out, "img:1") {
		t.Fatalf("nil registry left image content: %q", out)
	}
	if out != "hi  bye" {
		t.Fatalf("Render = %q, want 'hi  bye'", out)
	}
}

func TestRender_TolerantTokenWhitespace(t *testing.T) {
	reg := NewRegistry()
	reg.Register("file-a", "alt")
	out := Render("[[ img : 1 ]]", reg)
	if !strings.Contains(out, "<img data-file-id=") {
		t.Fatalf("whitespace token not matched: %q", out)
	}
}

func TestRegistry_ResetRewindsIDs(t *testing.T) {
	reg := NewRegistry()
	reg.Register("file-a", "")
	reg.Register("file-b", "")
	reg.Reset()
	id, _ := reg.Register("file-c", "")
	if id != 1 {
		t.Fatalf("after Reset id=%d, want 1", id)
	}
}

func TestRegistry_RejectsBlankFileID(t *testing.T) {
	reg := NewRegistry()
	if _, ok := reg.Register("", "alt"); ok {
		t.Fatal("blank file id should not register")
	}
}

func TestPromptHint_EmptyWhenNoImages(t *testing.T) {
	reg := NewRegistry()
	if h := reg.PromptHint(); h != "" {
		t.Fatalf("PromptHint = %q, want empty", h)
	}
}

func TestPromptHint_ListsTokens(t *testing.T) {
	reg := NewRegistry()
	reg.Register("file-a", "First")
	reg.Register("file-b", "Second")
	h := reg.PromptHint()
	if !strings.Contains(h, "[[img:1]]") || !strings.Contains(h, "First") {
		t.Fatalf("hint missing img 1: %q", h)
	}
	if !strings.Contains(h, "[[img:2]]") || !strings.Contains(h, "Second") {
		t.Fatalf("hint missing img 2: %q", h)
	}
}
