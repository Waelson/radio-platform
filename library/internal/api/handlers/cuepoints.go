package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Waelson/radio-library-service/internal/store"
)

// SaveCuePoints handles PUT /v1/tracks/{id}/cuepoints.
// The request body may contain any subset of the four cue point fields.
// A JSON null value clears the marker; an omitted field also clears it
// (PUT semantics — the body fully replaces all four markers).
func SaveCuePoints(ts TrackStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "id is required")
			return
		}

		var cp store.CuePoints
		if err := json.NewDecoder(r.Body).Decode(&cp); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}

		if err := cp.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_cuepoints", err.Error())
			return
		}

		// Verify track exists and, when cue_out_ms is given, that it does not
		// exceed the track's known duration.
		t, err := ts.FindByID(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "track not found")
			return
		}
		if err != nil {
			slog.Error("SaveCuePoints: FindByID", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "lookup failed")
			return
		}

		if cp.CueOutMS != nil && t.DurationMS > 0 && *cp.CueOutMS > t.DurationMS {
			writeError(w, http.StatusBadRequest, "invalid_cuepoints",
				"cue_out_ms exceeds track duration")
			return
		}

		if err := ts.SaveCuePoints(r.Context(), id, cp); err != nil {
			slog.Error("SaveCuePoints: store error", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "save failed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":         true,
			"track_id":   id,
			"cue_in_ms":  cp.CueInMS,
			"intro_ms":   cp.IntroMS,
			"outro_ms":   cp.OutroMS,
			"cue_out_ms": cp.CueOutMS,
		})
	}
}
