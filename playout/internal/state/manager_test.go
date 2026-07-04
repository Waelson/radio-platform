package state_test

import (
	"sync"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/state"
)

func TestNewManager_InitialState(t *testing.T) {
	m := state.NewManager("studio-a")
	s := m.Snapshot()

	if s.EngineID != "studio-a" {
		t.Errorf("EngineID = %q, want %q", s.EngineID, "studio-a")
	}
	if s.State != state.StateStarting {
		t.Errorf("State = %s, want STARTING", s.State)
	}
	if s.Mode != state.ModeAuto {
		t.Errorf("Mode = %s, want AUTO", s.Mode)
	}
	if s.Panic {
		t.Error("Panic = true, want false")
	}
	if s.NowPlaying != nil {
		t.Error("NowPlaying should be nil initially")
	}
	if s.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero")
	}
}

func TestSetState_Transitions(t *testing.T) {
	m := state.NewManager("x")
	m.SetState(state.StateIdle)
	if m.Snapshot().State != state.StateIdle {
		t.Error("expected IDLE")
	}
	m.SetState(state.StatePlaying)
	if m.Snapshot().State != state.StatePlaying {
		t.Error("expected PLAYING")
	}
}

func TestSetState_PanicFlagging(t *testing.T) {
	m := state.NewManager("x")
	m.SetState(state.StatePanic)
	s := m.Snapshot()
	if !s.Panic {
		t.Error("Panic flag should be true in PANIC state")
	}
	m.SetState(state.StateIdle)
	s = m.Snapshot()
	if s.Panic {
		t.Error("Panic flag should be cleared when leaving PANIC")
	}
}

func TestSetError_SetsStateAndMessage(t *testing.T) {
	m := state.NewManager("x")
	m.SetError("decoder died")
	s := m.Snapshot()
	if s.State != state.StateError {
		t.Errorf("State = %s, want ERROR", s.State)
	}
	if s.ErrorMsg != "decoder died" {
		t.Errorf("ErrorMsg = %q", s.ErrorMsg)
	}
}

func TestClearError(t *testing.T) {
	m := state.NewManager("x")
	m.SetError("some error")
	m.ClearError()
	if m.Snapshot().ErrorMsg != "" {
		t.Error("ErrorMsg should be cleared")
	}
}

func TestSetNowPlaying_And_ClearNowPlaying(t *testing.T) {
	m := state.NewManager("x")
	np := &state.NowPlaying{
		QueueItemID: "qi_1",
		AssetID:     "a1",
		Title:       "Song",
		DurationMS:  180000,
	}
	m.SetNowPlaying(np)
	s := m.Snapshot()
	if s.NowPlaying == nil {
		t.Fatal("NowPlaying should not be nil after SetNowPlaying")
	}
	if s.NowPlaying.QueueItemID != "qi_1" {
		t.Errorf("QueueItemID = %q", s.NowPlaying.QueueItemID)
	}

	m.ClearNowPlaying()
	if m.Snapshot().NowPlaying != nil {
		t.Error("NowPlaying should be nil after ClearNowPlaying")
	}
}

func TestUpdateProgress(t *testing.T) {
	m := state.NewManager("x")
	m.SetNowPlaying(&state.NowPlaying{DurationMS: 100000})
	m.UpdateProgress(50000, 50.0)
	s := m.Snapshot()
	if s.NowPlaying.PositionMS != 50000 {
		t.Errorf("PositionMS = %d", s.NowPlaying.PositionMS)
	}
	if s.NowPlaying.Percent != 50.0 {
		t.Errorf("Percent = %f", s.NowPlaying.Percent)
	}
}

func TestUpdateProgress_NoOp_WhenNoNowPlaying(t *testing.T) {
	m := state.NewManager("x")
	// Should not panic when no item is playing.
	m.UpdateProgress(1000, 1.0)
}

func TestSetQueueInfo(t *testing.T) {
	m := state.NewManager("x")
	m.SetQueueInfo(5, "qi_next")
	s := m.Snapshot()
	if s.Queue.Size != 5 {
		t.Errorf("Queue.Size = %d, want 5", s.Queue.Size)
	}
	if s.Queue.NextItemID != "qi_next" {
		t.Errorf("Queue.NextItemID = %q", s.Queue.NextItemID)
	}
}

func TestRecordLastCommand(t *testing.T) {
	m := state.NewManager("x")
	before := time.Now().UTC().Add(-time.Millisecond)
	m.RecordLastCommand("PLAY", true)
	after := time.Now().UTC().Add(time.Millisecond)

	s := m.Snapshot()
	if s.LastCommand == nil {
		t.Fatal("LastCommand should not be nil")
	}
	if s.LastCommand.Command != "PLAY" {
		t.Errorf("Command = %q, want PLAY", s.LastCommand.Command)
	}
	if !s.LastCommand.Accepted {
		t.Error("Accepted should be true")
	}
	if s.LastCommand.At.Before(before) || s.LastCommand.At.After(after) {
		t.Errorf("At %v outside expected range", s.LastCommand.At)
	}
}

func TestSnapshot_DeepCopy_NowPlaying(t *testing.T) {
	m := state.NewManager("x")
	ti := &state.TransitionInfo{Type: "CROSSFADE", DurationMS: 8000}
	m.SetNowPlaying(&state.NowPlaying{
		QueueItemID: "qi_1",
		Transition:  ti,
	})

	s := m.Snapshot()
	// Mutate the snapshot's NowPlaying.
	s.NowPlaying.QueueItemID = "mutated"
	s.NowPlaying.Transition.Type = "MUTATED"

	// Internal state must be unchanged.
	s2 := m.Snapshot()
	if s2.NowPlaying.QueueItemID == "mutated" {
		t.Error("snapshot mutation leaked into manager state")
	}
	if s2.NowPlaying.Transition.Type == "MUTATED" {
		t.Error("TransitionInfo mutation leaked into manager state")
	}
}

func TestSnapshot_DeepCopy_LastCommand(t *testing.T) {
	m := state.NewManager("x")
	m.RecordLastCommand("SKIP", false)

	s := m.Snapshot()
	s.LastCommand.Command = "mutated"

	s2 := m.Snapshot()
	if s2.LastCommand.Command == "mutated" {
		t.Error("LastCommand mutation leaked into manager state")
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	m := state.NewManager("x")
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			m.SetState(state.StatePlaying)
			m.SetNowPlaying(&state.NowPlaying{QueueItemID: "qi_x"})
			m.UpdateProgress(1000, 0.5)
		}()
		go func() {
			defer wg.Done()
			_ = m.Snapshot()
		}()
	}
	wg.Wait()
}
