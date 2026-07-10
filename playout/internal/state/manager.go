// Package state maintains a consistent, thread-safe snapshot of the engine's
// runtime state. It imports nothing from the project to stay at the base of
// the dependency graph.
package state

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// PlayerState represents the engine's current execution state.
type PlayerState string

const (
	StateStarting PlayerState = "STARTING"
	StateIdle     PlayerState = "IDLE"
	StatePlaying  PlayerState = "PLAYING"
	StatePaused   PlayerState = "PAUSED"
	StateAssist   PlayerState = "ASSIST"
	StatePanic    PlayerState = "PANIC"
	StateStopping PlayerState = "STOPPING"
	StateError    PlayerState = "ERROR"
)

// OperationalMode represents how the engine interprets automation decisions.
type OperationalMode string

const (
	ModeAuto   OperationalMode = "AUTO"
	ModeAssist OperationalMode = "ASSIST"
	ModePanic  OperationalMode = "PANIC"
)

// NowPlaying carries metadata and progress of the currently playing item.
type NowPlaying struct {
	QueueItemID string
	AssetID     string
	Path        string
	Title       string
	Artist      string
	Type        string
	DurationMS  int64
	PositionMS  int64
	Percent     float64
	Transition  *TransitionInfo

	// Break fields — non-empty when the item belongs to a commercial break.
	BreakID       string
	BreakTitle    string
	BreakPosition int // 1-based position within the break (same as BreakSeq)
	BreakTotal    int
	BreakRole     string // "open" | "spot" | "close"
}

// TransitionInfo describes the configured transition for the current item.
type TransitionInfo struct {
	Type       string
	DurationMS int64
}

// QueueInfo carries high-level queue metadata exposed in the status snapshot.
type QueueInfo struct {
	Size       int
	NextItemID string
}

// AudioHealth carries audio pipeline health metrics.
type AudioHealth struct {
	LevelDBFS     float64
	PeakDBFS      float64
	Silence       bool
	BufferPct     int
	UnderrunCount int64
}

// LastCommand records the result of the most recently processed command.
type LastCommand struct {
	Command  string
	Accepted bool
	At       time.Time
}

// Snapshot is a consistent, copyable view of the full engine state.
// It is safe to read after Snapshot() returns without any lock.
type Snapshot struct {
	EngineID      string
	State         PlayerState
	Mode          OperationalMode
	Panic         bool
	NowPlaying    *NowPlaying
	Queue         QueueInfo
	AudioHealth   AudioHealth
	LastCommand   *LastCommand
	StartedAt     time.Time
	ErrorMsg      string
	MainVolume    float32 `json:"main_volume"`
	PreviewVolume float32 `json:"preview_volume"`
	CartVolume    float32 `json:"cart_volume"`
	CartEnabled   bool    `json:"cart_enabled"`
}

// Manager maintains the engine state and exposes it via Snapshot().
// All callers that change state must use the provided mutator methods.
type Manager struct {
	mu         sync.RWMutex
	snap       Snapshot
	mainVol    atomic.Uint32 // float32 bits — read lock-free in the audio hot path
	previewVol atomic.Uint32 // float32 bits
	cartVol    atomic.Uint32 // float32 bits
}

// NewManager creates a Manager initialised in StateStarting with full volume (1.0).
func NewManager(engineID string) *Manager {
	m := &Manager{
		snap: Snapshot{
			EngineID:      engineID,
			State:         StateStarting,
			Mode:          ModeAuto,
			StartedAt:     time.Now().UTC(),
			MainVolume:    1.0,
			PreviewVolume: 1.0,
			CartVolume:    1.0,
		},
	}
	m.mainVol.Store(math.Float32bits(1.0))
	m.previewVol.Store(math.Float32bits(1.0))
	m.cartVol.Store(math.Float32bits(1.0))
	return m
}

// Snapshot returns a consistent copy of the current engine state.
func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.snap
	// deep-copy pointer fields so callers can't mutate internal state
	if m.snap.NowPlaying != nil {
		np := *m.snap.NowPlaying
		if m.snap.NowPlaying.Transition != nil {
			ti := *m.snap.NowPlaying.Transition
			np.Transition = &ti
		}
		s.NowPlaying = &np
	}
	if m.snap.LastCommand != nil {
		lc := *m.snap.LastCommand
		s.LastCommand = &lc
	}
	return s
}

// SetState transitions the engine to state s.
// Setting StatePanic also flips the Panic flag; any other state clears it.
func (m *Manager) SetState(s PlayerState) {
	m.mu.Lock()
	m.snap.State = s
	m.snap.Panic = s == StatePanic
	m.mu.Unlock()
}

// SetMode changes the operational mode.
func (m *Manager) SetMode(mode OperationalMode) {
	m.mu.Lock()
	m.snap.Mode = mode
	m.mu.Unlock()
}

// SetNowPlaying replaces the currently-playing item information.
func (m *Manager) SetNowPlaying(np *NowPlaying) {
	m.mu.Lock()
	m.snap.NowPlaying = np
	m.mu.Unlock()
}

// ClearNowPlaying removes the now-playing information (e.g. on stop or idle).
func (m *Manager) ClearNowPlaying() {
	m.mu.Lock()
	m.snap.NowPlaying = nil
	m.mu.Unlock()
}

// UpdateProgress updates playback position and percentage for the current item.
// It is a no-op when no item is playing.
func (m *Manager) UpdateProgress(posMS int64, percent float64) {
	m.mu.Lock()
	if m.snap.NowPlaying != nil {
		m.snap.NowPlaying.PositionMS = posMS
		m.snap.NowPlaying.Percent = percent
	}
	m.mu.Unlock()
}

// SetQueueInfo updates the queue size and next-item ID shown in status.
func (m *Manager) SetQueueInfo(size int, nextItemID string) {
	m.mu.Lock()
	m.snap.Queue = QueueInfo{Size: size, NextItemID: nextItemID}
	m.mu.Unlock()
}

// UpdateAudioHealth replaces current audio health metrics.
func (m *Manager) UpdateAudioHealth(h AudioHealth) {
	m.mu.Lock()
	m.snap.AudioHealth = h
	m.mu.Unlock()
}

// RecordLastCommand stores the result of the most recently dispatched command.
func (m *Manager) RecordLastCommand(cmdType string, accepted bool) {
	m.mu.Lock()
	m.snap.LastCommand = &LastCommand{
		Command:  cmdType,
		Accepted: accepted,
		At:       time.Now().UTC(),
	}
	m.mu.Unlock()
}

// SetError records an error message and forces the state to StateError.
func (m *Manager) SetError(msg string) {
	m.mu.Lock()
	m.snap.State = StateError
	m.snap.ErrorMsg = msg
	m.mu.Unlock()
}

// ClearError removes the error message (used after a successful RESET).
func (m *Manager) ClearError() {
	m.mu.Lock()
	m.snap.ErrorMsg = ""
	m.mu.Unlock()
}

// SetMainVolume sets the main output volume (0.0–1.0).
// The atomic field is written first so the audio hot path never blocks.
func (m *Manager) SetMainVolume(v float32) {
	v = clamp01(v)
	m.mainVol.Store(math.Float32bits(v))
	m.mu.Lock()
	m.snap.MainVolume = v
	m.mu.Unlock()
}

// MainVolume returns the current main output volume.
// Lock-free — safe to call from the audio hot path.
func (m *Manager) MainVolume() float32 {
	return math.Float32frombits(m.mainVol.Load())
}

// SetPreviewVolume sets the preview/CUE output volume (0.0–1.0).
func (m *Manager) SetPreviewVolume(v float32) {
	v = clamp01(v)
	m.previewVol.Store(math.Float32bits(v))
	m.mu.Lock()
	m.snap.PreviewVolume = v
	m.mu.Unlock()
}

// PreviewVolume returns the current preview/CUE output volume.
// Lock-free — safe to call from the audio hot path.
func (m *Manager) PreviewVolume() float32 {
	return math.Float32frombits(m.previewVol.Load())
}

// SetCartVolume sets the cart output volume (0.0–1.0).
func (m *Manager) SetCartVolume(v float32) {
	v = clamp01(v)
	m.cartVol.Store(math.Float32bits(v))
	m.mu.Lock()
	m.snap.CartVolume = v
	m.mu.Unlock()
}

// CartVolume returns the current cart output volume.
// Lock-free — safe to call from the audio hot path.
func (m *Manager) CartVolume() float32 {
	return math.Float32frombits(m.cartVol.Load())
}

// SetCartEnabled records whether the cart player is enabled.
func (m *Manager) SetCartEnabled(enabled bool) {
	m.mu.Lock()
	m.snap.CartEnabled = enabled
	m.mu.Unlock()
}

// CartVolAtomicPtr returns a pointer to the underlying atomic cart volume field.
// Use this to share a lock-free volume reference with the cart player's audio hot path.
func (m *Manager) CartVolAtomicPtr() *atomic.Uint32 {
	return &m.cartVol
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
