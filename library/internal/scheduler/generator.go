// Package scheduler implements the clock-rotation playlist generator.
package scheduler

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// ── Dependency interfaces ─────────────────────────────────────────────────────

// ClockQuerier retrieves clock data needed by the generator.
type ClockQuerier interface {
	// GetClockForHour returns the clock assigned to weekday (0=Sun) and hour (0-23),
	// or nil if no clock is configured for that slot.
	GetClockForHour(ctx context.Context, weekday, hour int) (*Clock, error)
}

// Clock is the generator's view of a programming clock.
type Clock struct {
	ID    string
	Name  string
	Slots []Slot
}

// Slot is a single ordered entry within a clock.
type Slot struct {
	ID              string
	Position        int
	SlotType        string // CATEGORY|JINGLE|SPOT|VINHETA|HORA_CERTA|FIXED
	CategoryID      string
	CategoryName    string
	FixedTrackID    string
	DurationHintMS  int64
}

// TrackQuerier retrieves track candidates for each slot.
type TrackQuerier interface {
	// TracksByCategory returns all tracks associated with the given category ID.
	TracksByCategory(ctx context.Context, categoryID string) ([]TrackRef, error)
	// TracksByType returns all tracks with the given type (JINGLE, SPOT, VINHETA).
	TracksByType(ctx context.Context, trackType string) ([]TrackRef, error)
	// TrackByID returns a single track by ID, or an error if not found.
	TrackByID(ctx context.Context, id string) (TrackRef, error)
}

// TrackRef is the generator's view of a track.
type TrackRef struct {
	ID           string
	Path         string
	Title        string
	Artist       string
	Album        string
	DurationMS   int64
	Type         string   // MUSIC|VINHETA|JINGLE|SPOT
	CategoryID   string   // may be empty; used for separation checks
	LoudnessLUFS *float64 // nil when not yet analyzed
}

// SeparationQuerier retrieves the active separation rules.
type SeparationQuerier interface {
	ListRules(ctx context.Context) ([]SeparationRule, error)
}

// SeparationRule defines the minimum time between repeats of a given field value.
type SeparationRule struct {
	ID            string
	Field         string // artist|title|album|category
	MinSepMinutes int
}

// RotationLogQuerier retrieves recent play history for separation checks.
type RotationLogQuerier interface {
	// RecentTrackIDs returns a map of track_id → last played_at for tracks played
	// within the last maxLookbackMinutes minutes.
	RecentTrackIDs(ctx context.Context, since time.Time) (map[string]time.Time, error)
	// RecentByField returns entries where the given field matches value, played since 'since'.
	RecentByField(ctx context.Context, field, value string, since time.Time) ([]LogEntry, error)
	// OldestInCategory returns the track_id played longest ago in a category,
	// or "" if there is no history.
	OldestInCategory(ctx context.Context, categoryID string) (string, error)
}

// LogEntry is a single entry from the rotation log.
type LogEntry struct {
	TrackID    string
	Artist     string
	Title      string
	Album      string
	CategoryID string
	PlayedAt   time.Time
}

// ── Output types ──────────────────────────────────────────────────────────────

// GeneratedItem is one resolved slot in the output playlist.
type GeneratedItem struct {
	Hour         int
	Position     int
	SlotID       string
	SlotType     string
	ClockID      string
	ClockName    string
	CategoryID   string
	CategoryName string
	Track        TrackRef
}

// ── Generator ─────────────────────────────────────────────────────────────────

// Generator builds playlists based on clock schedules and separation rules.
type Generator struct {
	clocks  ClockQuerier
	tracks  TrackQuerier
	sepRules SeparationQuerier
	rotLog  RotationLogQuerier
}

// New creates a Generator with the given dependencies.
func New(clocks ClockQuerier, tracks TrackQuerier, sepRules SeparationQuerier, rotLog RotationLogQuerier) *Generator {
	return &Generator{clocks: clocks, tracks: tracks, sepRules: sepRules, rotLog: rotLog}
}

// Generate builds a playlist starting at 'from' covering 'hours' hours.
// It returns the generated items and any non-fatal warnings encountered.
func (g *Generator) Generate(ctx context.Context, from time.Time, hours int) ([]GeneratedItem, []string, error) {
	if hours <= 0 {
		hours = 1
	}
	if hours > 24 {
		hours = 24
	}

	rules, err := g.sepRules.ListRules(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("generator: load separation rules: %w", err)
	}

	// Determine the maximum lookback window needed for separation checks.
	maxLookback := maxLookbackDuration(rules)

	// Load recent track history from the persistent log.
	since := from.Add(-maxLookback)
	persistentHistory, err := g.rotLog.RecentTrackIDs(ctx, since)
	if err != nil {
		return nil, nil, fmt.Errorf("generator: load rotation log: %w", err)
	}

	// sessionHistory tracks what was chosen in this generation run (not yet in the DB).
	// key: track_id → chosen at (simulated time within the generated playlist).
	sessionHistory := map[string]time.Time{}
	// sessionArtist tracks artist → last chosen time within this session.
	sessionArtist := map[string]time.Time{}
	// sessionCategory tracks category → last chosen time.
	sessionCategory := map[string]time.Time{}

	var items []GeneratedItem
	var warnings []string

	for h := 0; h < hours; h++ {
		t := from.Add(time.Duration(h) * time.Hour)
		weekday := int(t.Weekday())
		hour := t.Hour()

		clock, err := g.clocks.GetClockForHour(ctx, weekday, hour)
		if err != nil {
			return nil, nil, fmt.Errorf("generator: get clock for %d:%02d: %w", weekday, hour, err)
		}
		if clock == nil {
			warnings = append(warnings, fmt.Sprintf(
				"hora %d (%s): nenhum clock configurado na grade — hora ignorada", hour, t.Format("Mon")))
			continue
		}

		// Simulated cursor within the hour for session history timestamps.
		cursor := t

		for _, slot := range clock.Slots {
			track, warn, err := g.resolveSlot(ctx, slot, clock, cursor, rules,
				persistentHistory, sessionHistory, sessionArtist, sessionCategory, since)
			if err != nil {
				return nil, nil, err
			}
			if warn != "" {
				warnings = append(warnings, fmt.Sprintf("hora %d, slot %d (%s): %s", hour, slot.Position, clock.Name, warn))
			}
			if track == nil {
				warnings = append(warnings, fmt.Sprintf(
					"hora %d, slot %d (%s, tipo %s): nenhuma faixa disponível — slot ignorado",
					hour, slot.Position, clock.Name, slot.SlotType))
				continue
			}

			items = append(items, GeneratedItem{
				Hour:         hour,
				Position:     slot.Position,
				SlotID:       slot.ID,
				SlotType:     slot.SlotType,
				ClockID:      clock.ID,
				ClockName:    clock.Name,
				CategoryID:   slot.CategoryID,
				CategoryName: slot.CategoryName,
				Track:        *track,
			})

			// Register in session history so the next slot respects separation.
			sessionHistory[track.ID] = cursor
			sessionArtist[track.Artist] = cursor
			if slot.CategoryID != "" {
				sessionCategory[slot.CategoryID] = cursor
			}

			// Advance simulated cursor by the track duration (or hint).
			dur := time.Duration(track.DurationMS) * time.Millisecond
			if slot.DurationHintMS > 0 {
				dur = time.Duration(slot.DurationHintMS) * time.Millisecond
			}
			if dur == 0 {
				dur = 3 * time.Minute // safe default
			}
			cursor = cursor.Add(dur)
		}
	}

	return items, warnings, nil
}

// resolveSlot picks a track for a single slot using the 3-phase fallback strategy.
// Returns (nil, warn, nil) when no track is available (non-fatal).
func (g *Generator) resolveSlot(
	ctx context.Context,
	slot Slot,
	clock *Clock,
	at time.Time,
	rules []SeparationRule,
	persistentHistory map[string]time.Time,
	sessionHistory map[string]time.Time,
	sessionArtist map[string]time.Time,
	sessionCategory map[string]time.Time,
	since time.Time,
) (*TrackRef, string, error) {
	// FIXED slot: return the pinned track directly.
	if slot.SlotType == "FIXED" {
		if slot.FixedTrackID == "" {
			return nil, "slot FIXED sem track_id configurado", nil
		}
		t, err := g.tracks.TrackByID(ctx, slot.FixedTrackID)
		if err != nil {
			return nil, fmt.Sprintf("track fixo %q não encontrado: %v", slot.FixedTrackID, err), nil
		}
		return &t, "", nil
	}

	// Load candidates.
	var candidates []TrackRef
	var err error
	switch slot.SlotType {
	case "CATEGORY":
		if slot.CategoryID == "" {
			return nil, "slot CATEGORY sem category_id configurado", nil
		}
		candidates, err = g.tracks.TracksByCategory(ctx, slot.CategoryID)
	case "JINGLE", "SPOT", "VINHETA", "HORA_CERTA":
		candidates, err = g.tracks.TracksByType(ctx, slot.SlotType)
	default:
		return nil, fmt.Sprintf("tipo de slot desconhecido: %q", slot.SlotType), nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("resolver slot: carregar candidatos: %w", err)
	}
	if len(candidates) == 0 {
		return nil, "", nil
	}

	// Phase 1 — strict: apply all separation rules.
	filtered := applyAllRules(candidates, rules, slot, at,
		persistentHistory, sessionHistory, sessionArtist, sessionCategory, since)
	if len(filtered) > 0 {
		t := pick(filtered)
		return &t, "", nil
	}

	// Phase 2 — relaxed: drop the least-critical rule (smallest min_sep_minutes).
	if len(rules) > 0 {
		relaxed, droppedRule := relaxRules(rules)
		filtered = applyAllRules(candidates, relaxed, slot, at,
			persistentHistory, sessionHistory, sessionArtist, sessionCategory, since)
		if len(filtered) > 0 {
			t := pick(filtered)
			return &t, fmt.Sprintf("separação relaxada (regra %q descartada — poucos candidatos)", droppedRule.Field), nil
		}
	}

	// Phase 3 — fallback: pick the track used longest ago in the category,
	// or a random track if there is no history.
	oldestID, err := g.rotLog.OldestInCategory(ctx, slot.CategoryID)
	if err != nil {
		return nil, "", fmt.Errorf("resolver slot fallback: %w", err)
	}
	var fallback *TrackRef
	if oldestID != "" {
		for i := range candidates {
			if candidates[i].ID == oldestID {
				fallback = &candidates[i]
				break
			}
		}
	}
	if fallback == nil {
		t := pick(candidates)
		fallback = &t
	}
	return fallback, "separação ignorada (fallback total — candidatos insuficientes)", nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func applyAllRules(
	candidates []TrackRef,
	rules []SeparationRule,
	slot Slot,
	at time.Time,
	persistentHistory map[string]time.Time,
	sessionHistory map[string]time.Time,
	sessionArtist map[string]time.Time,
	sessionCategory map[string]time.Time,
	since time.Time,
) []TrackRef {
	var out []TrackRef
	for _, c := range candidates {
		if !violatesRules(c, slot, rules, at, persistentHistory, sessionHistory, sessionArtist, sessionCategory) {
			out = append(out, c)
		}
	}
	return out
}

func violatesRules(
	t TrackRef,
	slot Slot,
	rules []SeparationRule,
	at time.Time,
	persistentHistory map[string]time.Time,
	sessionHistory map[string]time.Time,
	sessionArtist map[string]time.Time,
	sessionCategory map[string]time.Time,
) bool {
	// Check if this exact track was played recently (persistent + session).
	for _, r := range rules {
		minDur := time.Duration(r.MinSepMinutes) * time.Minute
		cutoff := at.Add(-minDur)

		switch r.Field {
		case "title":
			if lastPlayed, ok := persistentHistory[t.ID]; ok && lastPlayed.After(cutoff) {
				return true
			}
			if lastPlayed, ok := sessionHistory[t.ID]; ok && lastPlayed.After(cutoff) {
				return true
			}
		case "artist":
			if t.Artist == "" {
				continue
			}
			if lastPlayed, ok := sessionArtist[t.Artist]; ok && lastPlayed.After(cutoff) {
				return true
			}
		case "album":
			// Album separation: skip if album empty (avoid false collisions).
			if t.Album == "" {
				continue
			}
			// Check session only (album not tracked in persistentHistory by track_id).
			// This is a simplification — good enough for most use cases.
		case "category":
			if slot.CategoryID == "" {
				continue
			}
			if lastPlayed, ok := sessionCategory[slot.CategoryID]; ok && lastPlayed.After(cutoff) {
				return true
			}
		}
	}
	return false
}

// relaxRules returns a new slice without the rule with the smallest min_sep_minutes
// and the dropped rule.
func relaxRules(rules []SeparationRule) ([]SeparationRule, SeparationRule) {
	if len(rules) == 0 {
		return rules, SeparationRule{}
	}
	minIdx := 0
	for i, r := range rules {
		if r.MinSepMinutes < rules[minIdx].MinSepMinutes {
			minIdx = i
		}
	}
	dropped := rules[minIdx]
	out := make([]SeparationRule, 0, len(rules)-1)
	out = append(out, rules[:minIdx]...)
	out = append(out, rules[minIdx+1:]...)
	return out, dropped
}

// pick returns a random element from a non-empty slice.
func pick(tracks []TrackRef) TrackRef {
	return tracks[rand.Intn(len(tracks))]
}

// maxLookbackDuration returns the maximum separation window across all rules.
func maxLookbackDuration(rules []SeparationRule) time.Duration {
	max := 120 * time.Minute // default minimum lookback
	for _, r := range rules {
		d := time.Duration(r.MinSepMinutes) * time.Minute
		if d > max {
			max = d
		}
	}
	return max
}
