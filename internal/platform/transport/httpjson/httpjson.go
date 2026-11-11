// Package httpjson provides utilities for encoding, decoding, and writing JSON HTTP responses and requests.
package httpjson

import (
	"encoding/json"
	"net/http"
)

// Write writes the given value as a JSON response with a Content-Type header of application/json.
func Write(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError sends a JSON response with the specified HTTP status code, status text, and error message.
func WriteError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":    http.StatusText(code),
		"message": msg,
	})
}

// Decode decodes the JSON body of an HTTP request into the target value v.
func Decode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
