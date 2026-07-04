// Package handlers provides HTTP handler functions for the REST API.
// Handlers must not import mixer, output, or any audio package.
// They communicate exclusively via the Command Bus and read state via the State Manager.
package handlers

import (
	"encoding/json"
	"net/http"
)

// writeJSON serialises v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// errResponse is the envelope used for HTTP-level errors (bad input, etc.).
type errResponse struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// writeError writes a JSON error envelope with the given HTTP status.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errResponse{
		OK:      false,
		Error:   code,
		Message: message,
	})
}
