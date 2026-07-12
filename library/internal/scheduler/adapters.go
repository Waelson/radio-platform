package scheduler

import (
	"context"
	"time"

	"github.com/Waelson/radio-library-service/internal/store"
)

// ── Store adapters ────────────────────────────────────────────────────────────
// These types wrap the concrete *store.XxxStore structs to satisfy the
// Generator's dependency interfaces without creating an import cycle.

// ClockStoreAdapter wraps store.ClockStore to satisfy ClockQuerier.
type ClockStoreAdapter struct {
	S *store.ClockStore
}

func (a *ClockStoreAdapter) GetClockForHour(ctx context.Context, weekday, hour int) (*Clock, error) {
	clk, err := a.S.GetClockForHour(ctx, weekday, hour)
	if err != nil || clk == nil {
		return nil, err
	}
	slots := make([]Slot, len(clk.Slots))
	for i, s := range clk.Slots {
		slots[i] = Slot{
			ID:             s.ID,
			Position:       s.Position,
			SlotType:       s.SlotType,
			CategoryID:     s.CategoryID,
			CategoryName:   s.CategoryName,
			FixedTrackID:   s.FixedTrackID,
			DurationHintMS: s.DurationHintMS,
		}
	}
	return &Clock{ID: clk.ID, Name: clk.Name, Slots: slots}, nil
}

// TrackStoreAdapter wraps store.TrackStore to satisfy TrackQuerier.
type TrackStoreAdapter struct {
	S *store.TrackStore
	C *store.CategoryStore
}

func (a *TrackStoreAdapter) TracksByCategory(ctx context.Context, categoryID string) ([]TrackRef, error) {
	tracks, err := a.C.ListTracks(ctx, categoryID, 1000, 0)
	if err != nil {
		return nil, err
	}
	out := make([]TrackRef, len(tracks))
	for i, t := range tracks {
		out[i] = TrackRef{
			ID: t.ID, Path: t.Path, Title: t.Title,
			Artist: t.Artist, Album: t.Album, DurationMS: t.DurationMS,
			Type: t.Type,
		}
	}
	return out, nil
}

func (a *TrackStoreAdapter) TracksByType(ctx context.Context, trackType string) ([]TrackRef, error) {
	// Search is capped at 200; paginate to retrieve all tracks of this type.
	var all []TrackRef
	offset := 0
	const pageSize = 200
	for {
		tracks, err := a.S.Search(ctx, store.SearchQuery{Type: trackType, Limit: pageSize, Offset: offset})
		if err != nil {
			return nil, err
		}
		for _, t := range tracks {
			all = append(all, TrackRef{
				ID: t.ID, Path: t.Path, Title: t.Title,
				Artist: t.Artist, Album: t.Album, DurationMS: t.DurationMS,
				Type: t.Type,
			})
		}
		if len(tracks) < pageSize {
			break
		}
		offset += pageSize
	}
	return all, nil
}

func (a *TrackStoreAdapter) TrackByID(ctx context.Context, id string) (TrackRef, error) {
	t, err := a.S.FindByID(ctx, id)
	if err != nil {
		return TrackRef{}, err
	}
	return TrackRef{
		ID: t.ID, Path: t.Path, Title: t.Title,
		Artist: t.Artist, Album: t.Album, DurationMS: t.DurationMS,
		Type: t.Type,
	}, nil
}

// SeparationRuleStoreAdapter wraps store.SeparationRuleStore to satisfy SeparationQuerier.
type SeparationRuleStoreAdapter struct {
	S *store.SeparationRuleStore
}

func (a *SeparationRuleStoreAdapter) ListRules(ctx context.Context) ([]SeparationRule, error) {
	rules, err := a.S.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SeparationRule, len(rules))
	for i, r := range rules {
		out[i] = SeparationRule{ID: r.ID, Field: r.Field, MinSepMinutes: r.MinSepMinutes}
	}
	return out, nil
}

// RotationLogStoreAdapter wraps store.RotationLogStore to satisfy RotationLogQuerier.
type RotationLogStoreAdapter struct {
	S *store.RotationLogStore
}

func (a *RotationLogStoreAdapter) RecentTrackIDs(ctx context.Context, since time.Time) (map[string]time.Time, error) {
	return a.S.RecentTrackIDs(ctx, since)
}

func (a *RotationLogStoreAdapter) RecentByField(ctx context.Context, field, value string, since time.Time) ([]LogEntry, error) {
	entries, err := a.S.RecentByField(ctx, field, value, since)
	if err != nil {
		return nil, err
	}
	out := make([]LogEntry, len(entries))
	for i, e := range entries {
		out[i] = LogEntry{
			TrackID:    e.TrackID,
			Artist:     e.Artist,
			Title:      e.Title,
			Album:      e.Album,
			CategoryID: e.CategoryID,
			PlayedAt:   e.PlayedAt,
		}
	}
	return out, nil
}

func (a *RotationLogStoreAdapter) OldestInCategory(ctx context.Context, categoryID string) (string, error) {
	return a.S.OldestInCategory(ctx, categoryID)
}
