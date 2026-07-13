package handlers

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Waelson/radio-library-service/internal/store"
)

// ── Store interfaces ──────────────────────────────────────────────────────────

// TransmissionLogStore is the store subset required by these handlers.
type TransmissionLogStore interface {
	List(ctx context.Context, q store.TransmissionLogQuery) ([]store.TransmissionLogEntry, int, error)
	Summary(ctx context.Context, date time.Time) (store.TransmissionLogSummary, error)
	ExportCSV(ctx context.Context, from, to time.Time, w io.Writer) error
	ExportECAD(ctx context.Context, from, to time.Time, station store.StationInfo, w io.Writer) error
}

// TransmissionImportLogStore is the store subset required by the imports endpoint.
type TransmissionImportLogStore interface {
	List(ctx context.Context, limit, offset int) ([]store.TransmissionImportLogEntry, error)
}

// SettingsStore is the subset required by these handlers (station info for ECAD).
type SettingsStore interface {
	StationInfo(ctx context.Context) store.StationInfo
}

// ── JSON shapes ───────────────────────────────────────────────────────────────

type transmissionLogEntryJSON struct {
	ID               string     `json:"id"`
	QueueItemID      string     `json:"queue_item_id"`
	AssetID          string     `json:"asset_id"`
	Title            string     `json:"title"`
	Artist           string     `json:"artist"`
	Type             string     `json:"type"`
	ISRC             string     `json:"isrc"`
	Composer         string     `json:"composer"`
	Publisher        string     `json:"publisher"`
	DurationMS       int64      `json:"duration_ms"`
	DurationPlayedMS int64      `json:"duration_played_ms"`
	Result           string     `json:"result"`
	Status           string     `json:"status"`
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	BreakID          string     `json:"break_id,omitempty"`
	BreakTitle       string     `json:"break_title,omitempty"`
	BreakRole        string     `json:"break_role,omitempty"`
	BreakPosition    int        `json:"break_position,omitempty"`
	ImportFileName   string     `json:"import_file_name,omitempty"`
}

type importLogEntryJSON struct {
	ID              string     `json:"id"`
	FileName        string     `json:"file_name"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	Status          string     `json:"status"`
	RecordsTotal    int        `json:"records_total"`
	RecordsImported int        `json:"records_imported"`
	ErrorMessage    string     `json:"error_message,omitempty"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// ListTransmissionLog handles GET /v1/transmission-log
// Query params: from, to (YYYY-MM-DD), type, status, q, limit, offset.
func ListTransmissionLog(tls TransmissionLogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		from, to, ok := parseDateRange(w, q.Get("from"), q.Get("to"))
		if !ok {
			return
		}

		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))
		if limit <= 0 {
			limit = 100
		}
		if limit > 500 {
			limit = 500
		}

		sq := store.TransmissionLogQuery{
			From:   from,
			To:     to,
			Type:   q.Get("type"),
			Status: q.Get("status"),
			Search: q.Get("q"),
			Limit:  limit,
			Offset: offset,
		}

		entries, total, err := tls.List(r.Context(), sq)
		if err != nil {
			slog.Error("ListTransmissionLog: store error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "list failed")
			return
		}

		out := make([]transmissionLogEntryJSON, len(entries))
		for i, e := range entries {
			out[i] = toTransmissionLogEntryJSON(e)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true,
			"data": map[string]any{
				"entries": out,
				"total":   total,
				"limit":   limit,
				"offset":  offset,
			},
		})
	}
}

// GetTransmissionLogSummary handles GET /v1/transmission-log/summary?date=YYYY-MM-DD.
func GetTransmissionLogSummary(tls TransmissionLogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dateStr := r.URL.Query().Get("date")
		var date time.Time
		if dateStr == "" {
			date = time.Now().UTC()
		} else {
			var err error
			date, err = time.Parse("2006-01-02", dateStr)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_date", "date must be YYYY-MM-DD")
				return
			}
		}

		sum, err := tls.Summary(r.Context(), date)
		if err != nil {
			slog.Error("GetTransmissionLogSummary: store error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "summary failed")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true,
			"data": map[string]any{
				"date":            sum.Date,
				"total":           sum.Total,
				"by_type":         sum.ByType,
				"total_played_ms": sum.TotalPlayedMS,
			},
		})
	}
}

// ExportTransmissionLog handles GET /v1/transmission-log/export?from=YYYY-MM-DD&to=YYYY-MM-DD.
// Streams a CSV file.
func ExportTransmissionLog(tls TransmissionLogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		from, to, ok := parseDateRange(w, q.Get("from"), q.Get("to"))
		if !ok {
			return
		}
		if from.IsZero() {
			from = time.Now().UTC().Truncate(24 * time.Hour)
		}
		if to.IsZero() {
			to = from.Add(24 * time.Hour)
		}

		fname := fmt.Sprintf("transmission_log_%s.csv", from.Format("2006-01-02"))
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fname))

		if err := tls.ExportCSV(r.Context(), from, to, w); err != nil {
			slog.Error("ExportTransmissionLog: export error", "error", err)
			// Headers already sent; cannot write a JSON error.
		}
	}
}

// ExportECAD handles GET /v1/transmission-log/export/ecad?from=YYYY-MM-DD&to=YYYY-MM-DD.
// Streams a semicolon-separated CSV in ECAD format.
func ExportECAD(tls TransmissionLogStore, settings SettingsStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		from, to, ok := parseDateRange(w, q.Get("from"), q.Get("to"))
		if !ok {
			return
		}
		if from.IsZero() {
			from = time.Now().UTC().Truncate(24 * time.Hour)
		}
		if to.IsZero() {
			to = from.Add(24 * time.Hour)
		}

		// File name uses month of 'from': ecad_2026-07_declaracao.csv
		fname := fmt.Sprintf("ecad_%s_declaracao.csv", from.Format("2006-01"))
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fname))

		station := settings.StationInfo(r.Context())
		if err := tls.ExportECAD(r.Context(), from, to, station, w); err != nil {
			slog.Error("ExportECAD: export error", "error", err)
		}
	}
}

// ListImportLog handles GET /v1/transmission-log/imports?limit=&offset=.
func ListImportLog(ils TransmissionImportLogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))
		if limit <= 0 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}

		entries, err := ils.List(r.Context(), limit, offset)
		if err != nil {
			slog.Error("ListImportLog: store error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "list failed")
			return
		}

		out := make([]importLogEntryJSON, len(entries))
		for i, e := range entries {
			out[i] = importLogEntryJSON{
				ID:              e.ID,
				FileName:        e.FileName,
				StartedAt:       e.StartedAt,
				FinishedAt:      e.FinishedAt,
				Status:          e.Status,
				RecordsTotal:    e.RecordsTotal,
				RecordsImported: e.RecordsImported,
				ErrorMessage:    e.ErrorMessage,
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true,
			"data": map[string]any{
				"entries": out,
				"limit":   limit,
				"offset":  offset,
			},
		})
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func toTransmissionLogEntryJSON(e store.TransmissionLogEntry) transmissionLogEntryJSON {
	return transmissionLogEntryJSON{
		ID:               e.ID,
		QueueItemID:      e.QueueItemID,
		AssetID:          e.AssetID,
		Title:            e.Title,
		Artist:           e.Artist,
		Type:             e.Type,
		ISRC:             e.ISRC,
		Composer:         e.Composer,
		Publisher:        e.Publisher,
		DurationMS:       e.DurationMS,
		DurationPlayedMS: e.DurationPlayedMS,
		Result:           e.Result,
		Status:           e.Status,
		StartedAt:        e.StartedAt,
		FinishedAt:       e.FinishedAt,
		BreakID:          e.BreakID,
		BreakTitle:       e.BreakTitle,
		BreakRole:        e.BreakRole,
		BreakPosition:    e.BreakPosition,
		ImportFileName:   e.ImportFileName,
	}
}

// parseDateRange parses optional from/to query params (YYYY-MM-DD).
// Returns zero times when params are empty (caller decides defaults).
// Writes a 400 and returns ok=false on parse error.
func parseDateRange(w http.ResponseWriter, fromStr, toStr string) (from, to time.Time, ok bool) {
	if fromStr != "" {
		var err error
		from, err = time.Parse("2006-01-02", fromStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_from", "from must be YYYY-MM-DD")
			return time.Time{}, time.Time{}, false
		}
	}
	if toStr != "" {
		var err error
		to, err = time.Parse("2006-01-02", toStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_to", "to must be YYYY-MM-DD")
			return time.Time{}, time.Time{}, false
		}
		// Make 'to' inclusive of the whole day.
		to = to.Add(24 * time.Hour)
	}
	return from, to, true
}
