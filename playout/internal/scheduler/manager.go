package scheduler

import (
	"context"
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

// Manager owns all scheduled entries and drives their firing logic.
// It is safe for concurrent use.
type Manager struct {
	mu      sync.RWMutex
	entries map[string]*Entry

	cr       *cron.Cron
	cmdBus   *commands.Bus
	evtBus   *events.Bus
	stateMgr stateReader
	clk      clock
	log      *slog.Logger
}

// New creates a ready-to-use Manager. Call Run(ctx) to start the tick goroutine.
func New(
	cmdBus *commands.Bus,
	evtBus *events.Bus,
	stateMgr stateReader,
	log *slog.Logger,
) *Manager {
	return &Manager{
		entries:  make(map[string]*Entry),
		cr:       cron.New(),
		cmdBus:   cmdBus,
		evtBus:   evtBus,
		stateMgr: stateMgr,
		clk:      realClock{},
		log:      log,
	}
}

// withClock replaces the clock — used by tests only.
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

// tickFireAt evaluates all one-shot (FireAt) entries.
func (m *Manager) tickFireAt() {
	now := m.clk.Now()

	m.mu.Lock()
	var toFire []*Entry
	for _, e := range m.entries {
		if !e.Enabled || e.FireAt == nil {
			continue
		}
		// Fire if the target time is now or in the past (within this tick window).
		if !e.FireAt.After(now) && e.lastFiredAt.IsZero() {
			toFire = append(toFire, e)
		}
	}
	// Mark as fired (disable one-shots) before releasing the lock so
	// a subsequent tick within the same second doesn't re-trigger.
	for _, e := range toFire {
		e.lastFiredAt = now
		e.Enabled = false // one-shots auto-disable after firing
	}
	m.mu.Unlock()

	for _, e := range toFire {
		m.fireEntry(e)
	}
}

// Add registers a new entry and returns its assigned ID.
// Returns an error if the entry is invalid or a CronExpr cannot be parsed.
func (m *Manager) Add(e Entry) (string, error) {
	e.ID = "sched_" + ulid.Make().String()
	if e.TriggerMode == "" {
		e.TriggerMode = TriggerAfterCurrent
	}

	if e.CronExpr != "" {
		jobID, err := m.cr.AddFunc(e.CronExpr, func() {
			m.mu.RLock()
			entry, ok := m.entries[e.ID]
			m.mu.RUnlock()
			if !ok || !entry.Enabled {
				return
			}
			m.mu.Lock()
			entry.lastFiredAt = m.clk.Now()
			m.mu.Unlock()
			m.fireEntry(entry)
		})
		if err != nil {
			return "", err
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
	return e.ID, nil
}

// Remove deletes an entry. Noop if the ID does not exist.
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
}

// Enable activates an existing entry.
func (m *Manager) Enable(id string) bool {
	return m.setEnabled(id, true)
}

// Disable deactivates an existing entry without removing it.
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
	return true
}

// Get returns a copy of an entry by ID. The second return value is false if
// the entry does not exist.
func (m *Manager) Get(id string) (Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.entries[id]
	if !ok {
		return Entry{}, false
	}
	return *e, true
}

// List returns a snapshot of all registered entries.
func (m *Manager) List() []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Entry, 0, len(m.entries))
	for _, e := range m.entries {
		out = append(out, *e)
	}
	return out
}
