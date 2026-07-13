// Package transmissionlog implements the LogWriter: an Event Bus subscriber
// that appends played-track records to hourly JSONL files on disk.
//
// # Design contract
//
//   - The LogWriter is a passive observer — it never touches the audio pipeline.
//   - File I/O occurs exclusively in the Run() goroutine. No mutex needed.
//   - If the write channel is full, the Event Bus drops the event silently (non-blocking).
//   - Files are rotated by UTC hour based on the entry's FinishedAt timestamp.
//   - Each line is flushed with Sync() before the next entry — no buffering.
package transmissionlog

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/events"
)

// LogEntry is a single record written to the JSONL file.
// It represents one completed (or interrupted) playback event.
// Written exactly once, after ItemFinished or CartStopped.
type LogEntry struct {
	StartedAt        time.Time `json:"started_at"`
	FinishedAt       time.Time `json:"finished_at"`
	EngineID         string    `json:"engine_id"`
	QueueItemID      string    `json:"queue_item_id"`
	AssetID          string    `json:"asset_id"`
	Title            string    `json:"title"`
	Artist           string    `json:"artist"`
	Type             string    `json:"type"`             // MUSIC|JINGLE|VINHETA|SPOT|CART
	DurationMS       int64     `json:"duration_ms"`
	DurationPlayedMS int64     `json:"duration_played_ms"`
	Result           string    `json:"result"`           // finished|skipped|failed
	ISRC             string    `json:"isrc,omitempty"`
	Composer         string    `json:"composer,omitempty"`
	Publisher        string    `json:"publisher,omitempty"`
	BreakID          string    `json:"break_id,omitempty"`
	BreakTitle       string    `json:"break_title,omitempty"`
	BreakRole        string    `json:"break_role,omitempty"` // open|spot|close
	BreakPosition    int       `json:"break_position,omitempty"`
}

// pendingEntry holds the NowPlaying metadata captured when a track starts.
// It is kept in memory until ItemFinished arrives, then merged into a LogEntry.
// Accessed exclusively by the Run() goroutine — no synchronisation needed.
type pendingEntry struct {
	startedAt time.Time
	meta      events.NowPlayingChangedPayload
}

// Config holds the LogWriter configuration.
type Config struct {
	// EngineID is the unique identifier of the playout engine instance.
	// Included in every log entry so the Library Service can distinguish
	// entries from multiple studio engines sharing the same import directory.
	EngineID string

	// Dir is the directory where JSONL files are written.
	Dir string

	// FileNameTemplate is the filename pattern for log files.
	// Supports two placeholders:
	//   {date} → yyyyMMdd  (UTC)
	//   {hour} → HH        (UTC, zero-padded)
	// Example: "transmission_{date}_{hour}.jsonl"
	FileNameTemplate string
}

// Writer subscribes to the Event Bus and appends log entries to hourly JSONL files.
type Writer struct {
	cfg Config
	bus *events.Bus
	log *slog.Logger
}

// New creates a Writer. Call Run(ctx) in a dedicated goroutine to start it.
func New(cfg Config, bus *events.Bus, log *slog.Logger) *Writer {
	return &Writer{cfg: cfg, bus: bus, log: log}
}

// Run subscribes to the Event Bus and processes events until ctx is cancelled.
// All file I/O happens inside this goroutine exclusively.
func (w *Writer) Run(ctx context.Context) error {
	ch, cancel := w.bus.Subscribe(256)
	defer cancel()

	// In-memory state — owned exclusively by this goroutine.
	pending     := make(map[string]pendingEntry) // queue_item_id → pending
	cartPending := make(map[string]pendingEntry) // cart_id      → pending

	var (
		curFile *os.File
		curHour = -1
		curDay  string
	)

	closeFile := func() {
		if curFile == nil {
			return
		}
		if err := curFile.Sync(); err != nil {
			w.log.Warn("transmissionlog: sync on close failed", "err", err)
		}
		if err := curFile.Close(); err != nil {
			w.log.Warn("transmissionlog: close failed", "err", err)
		}
		curFile = nil
		curHour = -1
		curDay = ""
	}
	defer closeFile()

	writeEntry := func(entry LogEntry) {
		t    := entry.FinishedAt.UTC()
		day  := t.Format("20060102")
		hour := t.Hour()

		// Rotate file when day or hour changes.
		if curFile == nil || day != curDay || hour != curHour {
			closeFile()

			if err := os.MkdirAll(w.cfg.Dir, 0o755); err != nil {
				w.log.Error("transmissionlog: mkdir failed", "dir", w.cfg.Dir, "err", err)
				return
			}

			name := BuildFileName(w.cfg.FileNameTemplate, day, hour)
			path := filepath.Join(w.cfg.Dir, name)
			f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				w.log.Error("transmissionlog: open failed", "path", path, "err", err)
				return
			}
			curFile = f
			curDay  = day
			curHour = hour
		}

		line, err := json.Marshal(entry)
		if err != nil {
			w.log.Error("transmissionlog: marshal failed", "err", err)
			return
		}
		line = append(line, '\n')

		if _, err := curFile.Write(line); err != nil {
			w.log.Error("transmissionlog: write failed", "err", err)
			closeFile() // force reopen on next entry
			return
		}
		if err := curFile.Sync(); err != nil {
			w.log.Warn("transmissionlog: sync failed", "err", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil

		case evt := <-ch:
			switch evt.Type {

			case events.EvtNowPlayingChanged:
				p, ok := evt.Payload.(events.NowPlayingChangedPayload)
				if !ok || p.QueueItemID == "" || p.Title == "" {
					// Engine transitioning to IDLE — no actionable metadata.
					continue
				}
				pending[p.QueueItemID] = pendingEntry{
					startedAt: evt.Timestamp,
					meta:      p,
				}

			case events.EvtItemFinished:
				p, ok := evt.Payload.(events.ItemFinishedPayload)
				if !ok {
					continue
				}
				pe, found := pending[p.QueueItemID]
				if !found {
					// Engine restarted mid-track — no NowPlayingChanged captured.
					// Cannot produce a complete ECAD record; skip.
					continue
				}
				delete(pending, p.QueueItemID)
				writeEntry(LogEntry{
					StartedAt:        pe.startedAt,
					FinishedAt:       evt.Timestamp,
					EngineID:         w.cfg.EngineID,
					QueueItemID:      p.QueueItemID,
					AssetID:          p.AssetID,
					Title:            pe.meta.Title,
					Artist:           pe.meta.Artist,
					Type:             pe.meta.Type,
					DurationMS:       pe.meta.DurationMS,
					DurationPlayedMS: p.DurationPlayedMS,
					Result:           p.Result,
					ISRC:             pe.meta.ISRC,
					Composer:         pe.meta.Composer,
					Publisher:        pe.meta.Publisher,
					BreakID:          pe.meta.BreakID,
					BreakTitle:       pe.meta.BreakTitle,
					BreakRole:        pe.meta.BreakRole,
					BreakPosition:    pe.meta.BreakPosition,
				})

			case events.EvtCartStarted:
				p, ok := evt.Payload.(events.CartStartedPayload)
				if !ok {
					continue
				}
				cartPending[p.CartID] = pendingEntry{
					startedAt: evt.Timestamp,
					meta: events.NowPlayingChangedPayload{
						QueueItemID: p.CartID,
						AssetID:     p.CartID,
						Title:       p.Title,
						Artist:      p.Artist,
						Type:        "CART",
						DurationMS:  p.DurationMS,
					},
				}

			case events.EvtCartStopped:
				p, ok := evt.Payload.(events.CartStoppedPayload)
				if !ok {
					continue
				}
				pe, found := cartPending[p.CartID]
				if !found {
					continue
				}
				delete(cartPending, p.CartID)
				result := "finished"
				if p.Reason == "manual" || p.Reason == "replaced" {
					result = "skipped"
				}
				writeEntry(LogEntry{
					StartedAt:        pe.startedAt,
					FinishedAt:       evt.Timestamp,
					EngineID:         w.cfg.EngineID,
					QueueItemID:      pe.meta.QueueItemID,
					AssetID:          pe.meta.AssetID,
					Title:            pe.meta.Title,
					Artist:           pe.meta.Artist,
					Type:             "CART",
					DurationMS:       pe.meta.DurationMS,
					DurationPlayedMS: evt.Timestamp.Sub(pe.startedAt).Milliseconds(),
					Result:           result,
				})
			}
		}
	}
}

// BuildFileName substitutes {date} and {hour} placeholders in the template.
// date must be in "yyyyMMdd" format; hour is zero-padded to two digits.
//
// Example:
//
//	BuildFileName("transmission_{date}_{hour}.jsonl", "20260720", 8)
//	→ "transmission_20260720_08.jsonl"
func BuildFileName(template, date string, hour int) string {
	s := strings.ReplaceAll(template, "{date}", date)
	s  = strings.ReplaceAll(s, "{hour}", fmt.Sprintf("%02d", hour))
	return s
}
