// Package handlers contains HTTP handler functions for the Library Service API.
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

// TrackStore is the store subset required by the track handlers.
type TrackStore interface {
	FindByID(ctx context.Context, id string) (store.Track, error)
	Search(ctx context.Context, q store.SearchQuery) ([]store.Track, error)
	CountFiltered(ctx context.Context, q store.SearchQuery) (int, error)
	ListArtists(ctx context.Context, trackType string) ([]string, error)
	UpdateMeta(ctx context.Context, id string, patch store.TrackPatch) error
}

// NormalizationReader reads loudness-normalization settings from the store.
type NormalizationReader interface {
	NormalizationSettings(ctx context.Context) (store.NormalizationSettings, error)
}

// computeGainDB returns the gain (in dB) to apply to t so its loudness reaches
// the configured target. Returns 0 when normalization is disabled or when t has
// not yet been analysed (LoudnessLUFS == nil).
func computeGainDB(t store.Track, ns store.NormalizationSettings) float64 {
	if !ns.Enabled || t.LoudnessLUFS == nil {
		return 0.0
	}
	target := ns.TargetLUFS
	if ns.PerTypeEnabled {
		switch t.Type {
		case "MUSIC":
			target = ns.TargetMusic
		case "JINGLE":
			target = ns.TargetJingle
		case "VINHETA":
			target = ns.TargetVinheta
		case "SPOT":
			target = ns.TargetSpot
		}
	}
	gain := target - *t.LoudnessLUFS
	if gain > ns.MaxGainDB {
		gain = ns.MaxGainDB
	}
	return gain
}

// trackJSON is the JSON representation of a track.
type trackJSON struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Title      string    `json:"title"`
	Artist     string    `json:"artist"`
	Album      string    `json:"album"`
	Type       string    `json:"type"`
	ISRC       string    `json:"isrc"`
	Composer   string    `json:"composer"`
	Publisher  string    `json:"publisher"`
	DurationMS int64     `json:"duration_ms"`
	Category   string    `json:"category"`
	IndexedAt  time.Time `json:"indexed_at"`

	// Loudness fields (null until analysis completes).
	LoudnessLUFS       *float64   `json:"loudness_lufs"`
	TruePeakDBTP       *float64   `json:"true_peak_dbtp"`
	LoudnessStatus     string     `json:"loudness_status"`
	LoudnessAnalyzedAt *time.Time `json:"loudness_analyzed_at,omitempty"`

	// GainDB is the gain adjustment (dB) the playout engine must apply so this
	// track reaches the configured loudness target. 0.0 means no adjustment.
	GainDB float64 `json:"gain_db"`

	// Cue point fields (null when not set — playout engine falls back to defaults).
	CueInMS  *int64 `json:"cue_in_ms"`  // seek start (ms)
	IntroMS  *int64 `json:"intro_ms"`   // vocal intro end / announcer window (ms)
	OutroMS  *int64 `json:"outro_ms"`   // crossfade trigger (ms)
	CueOutMS *int64 `json:"cue_out_ms"` // playback stop (ms)
}

func toTrackJSON(t store.Track) trackJSON {
	return trackJSON{
		ID:                 t.ID,
		Path:               t.Path,
		Title:              t.Title,
		Artist:             t.Artist,
		Album:              t.Album,
		Type:               t.Type,
		ISRC:               t.ISRC,
		Composer:           t.Composer,
		Publisher:          t.Publisher,
		DurationMS:         t.DurationMS,
		Category:           t.Category,
		IndexedAt:          t.IndexedAt,
		LoudnessLUFS:       t.LoudnessLUFS,
		TruePeakDBTP:       t.TruePeakDBTP,
		LoudnessStatus:     t.LoudnessStatus,
		LoudnessAnalyzedAt: t.LoudnessAnalyzedAt,
		CueInMS:            t.CueInMS,
		IntroMS:            t.IntroMS,
		OutroMS:            t.OutroMS,
		CueOutMS:           t.CueOutMS,
	}
}

// SearchTracks handles GET /v1/tracks
// Query params: q, type, artist, album, category, limit (default 50, max 200), offset,
// loudness_status, loudness_min, loudness_max.
func SearchTracks(ts TrackStore, nr NormalizationReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))

		sq := store.SearchQuery{
			Q:              q.Get("q"),
			Type:           q.Get("type"),
			Artist:         q.Get("artist"),
			Album:          q.Get("album"),
			Category:       q.Get("category"),
			Limit:          limit,
			Offset:         offset,
			LoudnessStatus: q.Get("loudness_status"),
		}
		if v := q.Get("loudness_min"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				sq.LoudnessMin = &f
			}
		}
		if v := q.Get("loudness_max"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				sq.LoudnessMax = &f
			}
		}

		tracks, err := ts.Search(r.Context(), sq)
		if err != nil {
			slog.Error("SearchTracks: store error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "search failed")
			return
		}

		total, err := ts.CountFiltered(r.Context(), sq)
		if err != nil {
			slog.Error("SearchTracks: count error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "count failed")
			return
		}

		ns, _ := nr.NormalizationSettings(r.Context())

		out := make([]trackJSON, len(tracks))
		for i, t := range tracks {
			tj := toTrackJSON(t)
			tj.GainDB = computeGainDB(t, ns)
			out[i] = tj
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"tracks": out,
			"count":  len(out),
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

// GetTrack handles GET /v1/tracks/{id}.
func GetTrack(ts TrackStore, nr NormalizationReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "id is required")
			return
		}

		t, err := ts.FindByID(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "track not found")
			return
		}
		if err != nil {
			slog.Error("GetTrack: store error", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "lookup failed")
			return
		}

		ns, _ := nr.NormalizationSettings(r.Context())
		tj := toTrackJSON(t)
		tj.GainDB = computeGainDB(t, ns)
		writeJSON(w, http.StatusOK, tj)
	}
}

// PatchTrack handles PATCH /v1/tracks/{id}.
// Accepts a JSON body with any subset of: title, artist, category, type.
func PatchTrack(ts TrackStore, nr NormalizationReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "id is required")
			return
		}

		var body struct {
			Title     *string `json:"title"`
			Artist    *string `json:"artist"`
			Category  *string `json:"category"`
			Type      *string `json:"type"`
			ISRC      *string `json:"isrc"`
			Composer  *string `json:"composer"`
			Publisher *string `json:"publisher"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
			return
		}

		if body.Type != nil {
			switch *body.Type {
			case "MUSIC", "VINHETA", "JINGLE", "SPOT", "EFEITOS":
				// valid
			default:
				writeError(w, http.StatusBadRequest, "bad_request",
					"type must be one of MUSIC, VINHETA, JINGLE, SPOT, EFEITOS")
				return
			}
		}

		patch := store.TrackPatch{
			Title:     body.Title,
			Artist:    body.Artist,
			Category:  body.Category,
			Type:      body.Type,
			ISRC:      body.ISRC,
			Composer:  body.Composer,
			Publisher: body.Publisher,
		}

		if err := ts.UpdateMeta(r.Context(), id, patch); errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "track not found")
			return
		} else if err != nil {
			slog.Error("PatchTrack: store error", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "update failed")
			return
		}

		t, err := ts.FindByID(r.Context(), id)
		if err != nil {
			slog.Error("PatchTrack: FindByID after update", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "fetch after update failed")
			return
		}

		ns, _ := nr.NormalizationSettings(r.Context())
		tj := toTrackJSON(t)
		tj.GainDB = computeGainDB(t, ns)
		writeJSON(w, http.StatusOK, tj)
	}
}

// ListArtists handles GET /v1/tracks/artists.
// Optional query param: type (MUSIC | VINHETA | JINGLE | SPOT).
func ListArtists(ts TrackStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		trackType := r.URL.Query().Get("type")

		artists, err := ts.ListArtists(r.Context(), trackType)
		if err != nil {
			slog.Error("ListArtists: store error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "list artists failed")
			return
		}

		if artists == nil {
			artists = []string{}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"artists": artists,
			"count":   len(artists),
		})
	}
}

// writeJSON and writeError live in the parent api package; re-export here via
// package-level vars so handlers stay in their own package without an import cycle.
var writeJSON = func(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON: encode failed", "error", err)
	}
}

var writeError = func(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"error":   code,
		"message": message,
	})
}
