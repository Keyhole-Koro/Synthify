package httpjson

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWrite(t *testing.T) {
	rec := httptest.NewRecorder()
	Write(rec, map[string]string{"ok": "yes"})
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := rec.Body.String(); got != "{\"ok\":\"yes\"}\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, 422, "invalid input")
	if rec.Code != 422 {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"Unprocessable Entity"`) ||
		!strings.Contains(rec.Body.String(), `"message":"invalid input"`) {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestDecode(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"synthify"}`))
	var body struct {
		Name string `json:"name"`
	}
	if err := Decode(req, &body); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if body.Name != "synthify" {
		t.Fatalf("decoded name = %q", body.Name)
	}
}

func TestDecodeReturnsMalformedJSONError(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader("{"))
	var body map[string]any
	if err := Decode(req, &body); err == nil {
		t.Fatal("Decode() error = nil, want malformed JSON error")
	}
}
