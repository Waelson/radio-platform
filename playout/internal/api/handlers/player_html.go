package handlers

import "net/http"

// PlayerHTML returns a handler that serves the embedded player HTML page.
func PlayerHTML(content []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(content)
	}
}
