package handlers

import (
	"net/http"

	"github.com/Waelson/radio-playout-engine/internal/metrics"
)

// metricsProvider is the subset of metrics.Collector used by the handler.
type metricsProvider interface {
	Snapshot() metrics.Snapshot
}

// Metrics returns a handler for GET /v1/metrics.
func Metrics(col metricsProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, col.Snapshot())
	}
}
