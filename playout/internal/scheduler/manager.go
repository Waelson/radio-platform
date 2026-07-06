package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/robfig/cron/v3"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// stateReader is the subset of *state.Manager used by the scheduler.
// Using an interface makes the Manager independently testable.
type stateReader interface {
	Snapshot() state.Snapshot
}

// clock abstracts time.Now() to allow deterministic unit tests.
type clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Config holds scheduler-specific settings derived from the YAML config.
type Config struct {
	// Timezone is an IANA timezone name used to evaluate cron expressions.
	// Empty string means the local system timezone.
	Timezone string

	// StorePath is the path to the JSON persistence file.
	// Empty string disables persistence (entries live in memory only).
	StorePath string

	// MissedThresholdMS is how many milliseconds late a FireAt entry can be
	// before it is marked as MISSED instead of fired. This prevents stale
	// one-shot entries from triggering after an engine restart.
	// Zero means no threshold (always fire, regardless of delay).
	MissedThresholdMS int
}

// Manager owns all scheduled entries and drives their firing logic.
// It is safe for concurrent use.
type Manager struct {
	mu      sync.RWMutex
	entries map[string]*Entry

	cr       *cron.Cron
	store    *FileStore
	cfg      Config
	cmdBus   *commands.Bus
	evtBus   *events.Bus
	stateMgr stateReader
	clk      clock
	log      *slog.Logger
}

// New creates a Manager and loads persisted entries from the store (if configured).
// Returns an error only when the timezone cannot be parsed.
func New(
	cfg Config,
	cmdBus *commands.Bus,
	evtBus *events.Bus,
	stateMgr stateReader,
	log *slog.Logger,
) (*Manager, error) {
	// Resolve timezone for cron evaluation.
	loc := time.Local
	if cfg.Timezone != "" {
		var err error
		loc, err = time.LoadLocation(cfg.Timezone)
		if err != nil {
			return nil, fmt.Errorf("scheduler: invalid timezone %q: %w", cfg.Timezone, err)
		}
	}

	m := &Manager{
		entries:  make(map[string]*Entry),
		cr:       cron.New(cron.WithLocation(loc)),
		cfg:      cfg,
		cmdBus:   cmdBus,
		evtBus:   evtBus,
		stateMgr: stateMgr,
		clk:      realClock{},
		log:      log,
	}

	// Attach store and restore persisted entries.
	if cfg.StorePath != "" {
		m.store = NewFileStore(cfg.StorePath)
		loaded, err := m.store.Load()
		if err != nil {
			log.Warn("scheduler: failed to load persisted entries; starting empty",
				"path", cfg.StorePath, "error", err)
		} else if len(loaded) > 0 {
			for _, e := range loaded {
				m.restoreEntry(e)
			}
			log.Info("scheduler: entries restored from store",
				"count", len(loaded), "path", cfg.StorePath)
		}
	}

	return m, nil
}

// withClock replaces the internal clock — used only in tests.
func (m *Manager) withClock(c clock) { m.clk = c }

// Run starts the cron scheduler and the 1-second FireAt ticker.
// It blocks until ctx is cancelled.
func (m *Manager) Run(ctx context.Context) {
	m.cr.Start()
	defer m.cr.Stop()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.tickFireAt()
		}
	}
}

// tickFireAt evaluates all one-shot (FireAt) entries against the current time.
func (m *Manager) tickFireAt() {
	now := m.clk.Now()
	threshold := time.Duration(m.cfg.MissedThresholdMS) * time.Millisecond

	m.mu.Lock()
	var toFire []*Entry
	var toMiss []*Entry

	for _, e := range m.entries {
		if !e.Enabled || e.FireAt == nil {
			continue
		}
		if !e.LastFiredAt.IsZero() {
			continue // already processed in a previous tick
		}
		if e.FireAt.After(now) {
			continue // target is in the future
		}

		delay := now.Sub(*e.FireAt)
		if threshold > 0 && delay > threshold {
			toMiss = append(toMiss, e)
		} else {
			toFire = append(toFire, e)
		}
	}

	// Mark all as processed before releasing the lock so a concurrent tick
	// (or the next tick within the same second) does not re-trigger.
	for _, e := range toFire {
		e.LastFiredAt = now
		e.Enabled = false // one-shots auto-disable
	}
	for _, e := range toMiss {
		e.LastFiredAt = now
		e.Enabled = false // missed entries are also disabled
	}
	m.mu.Unlock()

	for _, e := range toFire {
		m.fireEntry(e)
	}
	for _, e := range toMiss {
		delay := now.Sub(*e.FireAt)
		m.publishMissed(e, fmt.Sprintf(
			"fire_at missed by %dms (threshold=%dms)",
			delay.Milliseconds(), m.cfg.MissedThresholdMS,
		))
	}

	if len(toFire)+len(toMiss) > 0 {
		m.saveToStore()
	}
}

// Add registers a new entry and returns its assigned ID.
// Returns an error if the CronExpr cannot be parsed.
func (m *Manager) Add(e Entry) (string, error) {
	e.ID = "sched_" + ulid.Make().String()
	if e.TriggerMode == "" {
		e.TriggerMode = TriggerAfterCurrent
	}
	e.CreatedAt = m.clk.Now()

	if e.CronExpr != "" {
		jobID, err := m.cr.AddFunc(e.CronExpr, m.makeCronJob(e.ID))
		if err != nil {
			return "", fmt.Errorf("scheduler: invalid cron expression %q: %w", e.CronExpr, err)
		}
		e.cronEntryID = jobID
	}

	m.mu.Lock()
	m.entries[e.ID] = &e
	m.mu.Unlock()

	m.evtBus.Publish(events.New(events.EvtScheduleEntryAdded, events.ScheduleEntryAddedPayload{
		EntryID:  e.ID,
		Name:     e.Name,
		CronExpr: e.CronExpr,
		OneShot:  e.FireAt != nil,
	}))
	m.log.Info("scheduler: entry added", "entry_id", e.ID, "name", e.Name)
	m.saveToStore()
	return e.ID, nil
}

// Remove deletes an entry by ID. Noop if the ID does not exist.
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	e, ok := m.entries[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	if e.cronEntryID != 0 {
		m.cr.Remove(e.cronEntryID)
	}
	delete(m.entries, id)
	m.mu.Unlock()

	m.evtBus.Publish(events.New(events.EvtScheduleEntryRemoved, events.ScheduleEntryRemovedPayload{
		EntryID: id,
	}))
	m.log.Info("scheduler: entry removed", "entry_id", id)
	m.saveToStore()
}

// Enable activates an existing entry. Returns false if the ID does not exist.
func (m *Manager) Enable(id string) bool {
	return m.setEnabled(id, true)
}

// Disable deactivates an existing entry without removing it. Returns false if
// the ID does not exist.
func (m *Manager) Disable(id string) bool {
	return m.setEnabled(id, false)
}

func (m *Manager) setEnabled(id string, enabled bool) bool {
	m.mu.Lock()
	e, ok := m.entries[id]
	if !ok {
		m.mu.Unlock()
		return false
	}
	e.Enabled = enabled
	m.mu.Unlock()

	m.evtBus.Publish(events.New(events.EvtScheduleEntryUpdated, events.ScheduleEntryUpdatedPayload{
		EntryID: id,
		Enabled: enabled,
	}))
	m.saveToStore()
	return true
}

// Get returns a copy of an entry by ID.
// The second return value is false when the ID does not exist.
func (m *Manager) Get(id string) (Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.entries[id]
	if !ok {
		return Entry{}, false
	}
	return *e, true
}

// Update replaces an existing entry, preserving its ID, CreatedAt and LastFiredAt.
// The old cron job is cancelled and a new one registered if CronExpr changed.
// Returns an error if the ID does not exist or the new CronExpr is invalid.
func (m *Manager) Update(id string, e Entry) error {
	// Parse and register the new cron job before acquiring the lock, so that
	// an invalid expression fails fast without mutating state.
	var newJobID cron.EntryID
	if e.CronExpr != "" {
		var err error
		newJobID, err = m.cr.AddFunc(e.CronExpr, m.makeCronJob(id))
		if err != nil {
			return fmt.Errorf("scheduler: invalid cron expression %q: %w", e.CronExpr, err)
		}
	}

	m.mu.Lock()
	old, ok := m.entries[id]
	if !ok {
		m.mu.Unlock()
		if newJobID != 0 {
			m.cr.Remove(newJobID) // roll back the job we already added
		}
		return fmt.Errorf("scheduler: entry %q not found", id)
	}
	if old.cronEntryID != 0 {
		m.cr.Remove(old.cronEntryID)
	}
	e.ID = id
	e.CreatedAt = old.CreatedAt
	e.LastFiredAt = old.LastFiredAt
	e.cronEntryID = newJobID
	if e.TriggerMode == "" {
		e.TriggerMode = TriggerAfterCurrent
	}
	m.entries[id] = &e
	m.mu.Unlock()

	m.evtBus.Publish(events.New(events.EvtScheduleEntryUpdated, events.ScheduleEntryUpdatedPayload{
		EntryID: id,
		Enabled: e.Enabled,
	}))
	m.log.Info("scheduler: entry updated", "entry_id", id, "name", e.Name)
	m.saveToStore()
	return nil
}

// NextFireAt returns the next evaluation time for entry id, or nil when the
// entry does not exist, is disabled, is a one-shot already fired, or has no
// cron schedule registered yet.
func (m *Manager) NextFireAt(id string) *time.Time {
	m.mu.RLock()
	e, ok := m.entries[id]
	m.mu.RUnlock()
	if !ok || !e.Enabled {
		return nil
	}
	if e.FireAt != nil && e.LastFiredAt.IsZero() {
		return e.FireAt
	}
	if e.cronEntryID == 0 {
		return nil
	}
	next := m.cr.Entry(e.cronEntryID).Next
	if next.IsZero() {
		return nil
	}
	t := next
	return &t
}

// List returns a snapshot of all registered entries (in arbitrary order).
func (m *Manager) List() []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Entry, 0, len(m.entries))
	for _, e := range m.entries {
		out = append(out, *e)
	}
	return out
}

// --- internal helpers --------------------------------------------------------

// makeCronJob returns the func() registered with robfig/cron for a given entry ID.
// The closure looks up the live Entry pointer so mutations (Enable/Disable) are
// respected without re-registering the job.
func (m *Manager) makeCronJob(entryID string) func() {
	return func() {
		m.mu.RLock()
		e, ok := m.entries[entryID]
		m.mu.RUnlock()
		if !ok || !e.Enabled {
			return
		}
		m.mu.Lock()
		e.LastFiredAt = m.clk.Now()
		m.mu.Unlock()

		m.fireEntry(e)
		m.saveToStore()
	}
}

// restoreEntry re-registers an entry loaded from the store without publishing
// events or writing back to the store.
func (m *Manager) restoreEntry(e Entry) {
	// One-shot already fired or missed — add to map as disabled, don't re-register cron.
	if e.FireAt != nil && !e.LastFiredAt.IsZero() {
		e.Enabled = false
		m.entries[e.ID] = &e
		return
	}

	if e.CronExpr != "" && e.Enabled {
		jobID, err := m.cr.AddFunc(e.CronExpr, m.makeCronJob(e.ID))
		if err != nil {
			m.log.Warn("scheduler: restore: invalid cron, disabling entry",
				"entry_id", e.ID, "cron", e.CronExpr, "error", err)
			e.Enabled = false
		} else {
			e.cronEntryID = jobID
		}
	}

	m.entries[e.ID] = &e
}

// saveToStore writes the current entries to the FileStore.
// Logs a warning on error; never returns an error to avoid blocking callers.
func (m *Manager) saveToStore() {
	if m.store == nil {
		return
	}
	m.mu.RLock()
	snap := make([]Entry, 0, len(m.entries))
	for _, e := range m.entries {
		snap = append(snap, *e)
	}
	m.mu.RUnlock()

	if err := m.store.Save(snap); err != nil {
		m.log.Warn("scheduler: failed to persist entries", "error", err)
	}
}
