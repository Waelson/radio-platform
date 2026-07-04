package queue_test

import (
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/queue"
)

func TestQueueItem_EffectiveCueOut(t *testing.T) {
	t.Run("uses CueOutMS when set", func(t *testing.T) {
		item := queue.QueueItem{DurationMS: 240000, CueOutMS: 230000}
		if got := item.EffectiveCueOut(); got != 230000 {
			t.Errorf("EffectiveCueOut() = %d, want 230000", got)
		}
	})

	t.Run("falls back to DurationMS when CueOutMS is zero", func(t *testing.T) {
		item := queue.QueueItem{DurationMS: 240000, CueOutMS: 0}
		if got := item.EffectiveCueOut(); got != 240000 {
			t.Errorf("EffectiveCueOut() = %d, want 240000", got)
		}
	})
}

func TestQueueItem_EffectiveDuration(t *testing.T) {
	t.Run("full duration without cue points", func(t *testing.T) {
		item := queue.QueueItem{DurationMS: 240000}
		if got := item.EffectiveDuration(); got != 240000 {
			t.Errorf("EffectiveDuration() = %d, want 240000", got)
		}
	})

	t.Run("trimmed by cue in and cue out", func(t *testing.T) {
		item := queue.QueueItem{DurationMS: 240000, CueInMS: 5000, CueOutMS: 230000}
		if got := item.EffectiveDuration(); got != 225000 {
			t.Errorf("EffectiveDuration() = %d, want 225000", got)
		}
	})
}

func TestItemStatus_Constants(t *testing.T) {
	// Verify string values match the spec contract.
	cases := []struct {
		got  queue.ItemStatus
		want string
	}{
		{queue.ItemStatusQueued, "QUEUED"},
		{queue.ItemStatusPreloading, "PRELOADING"},
		{queue.ItemStatusPlaying, "PLAYING"},
		{queue.ItemStatusFadingOut, "FADING_OUT"},
		{queue.ItemStatusPlayed, "PLAYED"},
		{queue.ItemStatusSkipped, "SKIPPED"},
		{queue.ItemStatusFailed, "FAILED"},
		{queue.ItemStatusMissed, "MISSED"},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("ItemStatus constant = %q, want %q", c.got, c.want)
		}
	}
}

func TestItemResult_Constants(t *testing.T) {
	cases := []struct {
		got  queue.ItemResult
		want string
	}{
		{queue.ItemResultPlayed, "PLAYED"},
		{queue.ItemResultSkipped, "SKIPPED"},
		{queue.ItemResultFailed, "FAILED"},
		{queue.ItemResultMissed, "MISSED"},
		{queue.ItemResultInterruptedByPanic, "INTERRUPTED_BY_PANIC"},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("ItemResult constant = %q, want %q", c.got, c.want)
		}
	}
}
