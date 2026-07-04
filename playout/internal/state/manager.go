// Package state maintains a consistent, thread-safe snapshot of the engine's
// runtime state. It imports nothing from the project to stay at the base of
// the dependency graph.
package state

import (
	"sync"
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
	EngineID    string
	State       PlayerState
	Mode        OperationalMode
	Panic       bool
	NowPlaying  *NowPlaying
	Queue       QueueInfo
	AudioHealth AudioHealth
	LastCommand *LastCommand
	StartedAt   time.Time
	ErrorMsg    string
}

// Manager maintains the engine state and exposes it via Snapshot().
// All callers that change state must use the provided mutator methods.
type Manager struct {
	mu  sync.RWMutex
	snap Snapshot
}

// NewManager creates a Manager initialised in StateStarting.
func NewManager(engineID string) *Manager {
	return &Manager{
		snap: Snapshot{
			EngineID:  engineID,
			State:     StateStarting,
			Mode:      ModeAuto,
			StartedAt: time.Now().UTC(),
		},
	}
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
