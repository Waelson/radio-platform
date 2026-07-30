package linein

import "time"

// RecordingEntry describes a completed line-in recording session.
// It is stored in memory by the Handler and exposed via ListRecordings().
type RecordingEntry struct {
	// Label is the human-readable name of the session (e.g. "A Voz do Brasil").
	Label string `json:"label"`

	// StartedAt is the UTC time when the session started.
	StartedAt time.Time `json:"started_at"`

	// StoppedAt is the UTC time when the session ended.
	StoppedAt time.Time `json:"stopped_at"`

	// DurationMS is the elapsed time in milliseconds.
	DurationMS int64 `json:"duration_ms"`

	// Path is the absolute path to the recorded file on disk.
	Path string `json:"path"`

	// Format is the file format: "wav" or "mp3".
	Format string `json:"format"`

	// SizeBytes is the size of the recorded file in bytes.
	SizeBytes int64 `json:"size_bytes"`

	// TriggeredBy is the origin of the session: "manual" or "scheduler".
	TriggeredBy string `json:"triggered_by"`
}
