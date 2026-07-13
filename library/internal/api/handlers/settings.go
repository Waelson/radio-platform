package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Waelson/radio-library-service/internal/store"
)

// SettingsReadWriter is the store subset required by the settings handlers.
// It extends SettingsStore (used by transmission log handlers) with write access.
type SettingsReadWriter interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	List(ctx context.Context) ([]store.SettingRow, error)
}

// ── JSON shapes ───────────────────────────────────────────────────────────────

type settingJSON struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// ListSettings handles GET /v1/settings.
// Returns all key/value pairs sorted by key.
func ListSettings(s SettingsReadWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := s.List(r.Context())
		if err != nil {
			slog.Error("ListSettings: store error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "list failed")
			return
		}

		out := make([]settingJSON, len(rows))
		for i, row := range rows {
			out[i] = settingJSON{Key: row.Key, Value: row.Value, UpdatedAt: row.UpdatedAt}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"data": out,
		})
	}
}

// GetSetting handles GET /v1/settings/{key}.
// Returns 404 when the key is not found.
func GetSetting(s SettingsReadWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		if key == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "key is required")
			return
		}

		value, err := s.Get(r.Context(), key)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "setting not found")
			return
		}
		if err != nil {
			slog.Error("GetSetting: store error", "key", key, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "get failed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"data": settingJSON{Key: key, Value: value},
		})
	}
}

// UpdateSetting handles PUT /v1/settings/{key}.
// Body: {"value": "..."}
// Returns 404 for unknown keys, 400 for invalid values.
func UpdateSetting(s SettingsReadWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		if key == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "key is required")
			return
		}

		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
			return
		}

		// Verify the key exists before updating.
		if _, err := s.Get(r.Context(), key); errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "setting not found")
			return
		} else if err != nil {
			slog.Error("UpdateSetting: get check error", "key", key, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "get failed")
			return
		}

		// Validate known keys with domain constraints.
		if msg := validateSetting(key, body.Value); msg != "" {
			writeError(w, http.StatusBadRequest, "invalid_value", msg)
			return
		}

		if err := s.Set(r.Context(), key, body.Value); err != nil {
			slog.Error("UpdateSetting: store error", "key", key, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "update failed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// validateSetting enforces domain constraints on known settings keys.
// Returns an error message string, or "" when valid.
func validateSetting(key, value string) string {
	switch key {
	case "transmission_log.retention_days":
		n, err := strconv.Atoi(value)
		if err != nil {
			return "retention_days deve ser um número inteiro"
		}
		if n < 7 {
			return "retention_days mínimo é 7"
		}
	case "transmission_log.poll_interval", "transmission_log.grace_period":
		if value == "" {
			return key + " não pode ser vazio"
		}
	}
	return ""
}
