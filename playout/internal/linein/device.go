// Package linein implements line-in (external audio source) capture.
// It follows the same subprocess model as internal/audio/decoder/ffmpeg.go:
// FFmpeg reads from a hardware input device and emits PCM float32 LE 48 kHz
// stereo on stdout, which the engine reads frame by frame.
package linein

import (
	"context"
)

// ── Config ────────────────────────────────────────────────────────────────────

// LineInConfig holds the parameters for a single line-in capture session.
type LineInConfig struct {
	// DeviceID is the platform-specific capture device identifier.
	// Use "default" to let FFmpeg choose the system default input.
	DeviceID string

	// Label is a human-readable name for logs and UI display (e.g. "Voz do Brasil").
	Label string

	// DurationMS is the maximum capture duration in milliseconds.
	// 0 means unlimited — the session runs until Stop is called.
	DurationMS int64

	// OnSilence controls behaviour when silence is detected.
	// "alert"  (default) — emit EvtLineInError but keep capturing.
	// "stop"              — emit EvtLineInError then auto-stop.
	OnSilence string

	// SilenceThresholdDBFS is the RMS level (in dBFS) below which audio is
	// considered silence. Default: -60.0.
	SilenceThresholdDBFS float64

	// SilenceThresholdMS is the continuous silence duration (ms) that triggers
	// the watchdog. Default: 30 000 (30 s).
	SilenceThresholdMS int64
}

// onSilenceOrDefault returns the effective OnSilence value.
func (c LineInConfig) onSilenceOrDefault() string {
	if c.OnSilence == "stop" {
		return "stop"
	}
	return "alert"
}

// silenceThresholdDBFS returns the effective silence threshold.
func (c LineInConfig) silenceThresholdDBFS() float64 {
	if c.SilenceThresholdDBFS != 0 {
		return c.SilenceThresholdDBFS
	}
	return -60.0
}

// silenceThresholdMS returns the effective silence duration threshold.
func (c LineInConfig) silenceThresholdMS() int64 {
	if c.SilenceThresholdMS > 0 {
		return c.SilenceThresholdMS
	}
	return 30_000
}

// ── InputDevice ───────────────────────────────────────────────────────────────

// InputDevice is the contract that an audio capture source must satisfy.
// FFmpegCapture is the production implementation; tests use stub implementations.
type InputDevice interface {
	// Open initialises the capture device with the given configuration.
	// Must be called before ReadFrames.
	Open(ctx context.Context, cfg LineInConfig) error

	// ReadFrames reads interleaved PCM float32 frames into dst.
	// Returns the number of frames read and any error.
	// Returns io.EOF when the stream ends (e.g. duration_ms reached or device closed).
	ReadFrames(ctx context.Context, dst []float32) (int, error)

	// Close releases all resources (kills subprocess, closes pipes).
	Close() error
}

// ── DeviceInfo ────────────────────────────────────────────────────────────────

// DeviceInfo describes a capture device available on the current system.
type DeviceInfo struct {
	// ID is the identifier to use in LineInConfig.DeviceID.
	ID string

	// Name is the human-readable device name shown in the UI.
	Name string
}

// ── Events (passed by the Manager to the caller via channel) ─────────────────

// EventType identifies the kind of event emitted by LineInManager.
type EventType string

const (
	EventStarted        EventType = "started"
	EventStopped        EventType = "stopped"
	EventSilenceAlert   EventType = "silence_alert"
	EventSilenceStop    EventType = "silence_stop"
	EventError          EventType = "error"
	EventLevelUpdate    EventType = "level"
)

// Event is a single notification emitted by LineInManager during a session.
type Event struct {
	Type EventType

	// ElapsedMS is populated on EventStopped.
	ElapsedMS int64

	// RMSDBFS is the instantaneous RMS level in dBFS, populated on EventLevelUpdate.
	RMSDBFS float64

	// SilenceDurationMS is populated on EventSilenceAlert / EventSilenceStop.
	SilenceDurationMS int64

	// Err holds the error on EventError.
	Err error
}
