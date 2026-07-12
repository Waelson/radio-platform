package store_test

import (
	"context"
	"testing"

	"github.com/Waelson/radio-library-service/internal/store"
)

func TestClock_CreateAndGet(t *testing.T) {
	cls := store.NewClockStore(openMemDB(t))
	ctx := context.Background()

	clk, err := cls.Create(ctx, "Manhã Adulto")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if clk.ID == "" {
		t.Error("ID must be set")
	}
	if clk.Name != "Manhã Adulto" {
		t.Errorf("Name = %q", clk.Name)
	}
	if clk.Slots != nil && len(clk.Slots) != 0 {
		t.Error("new clock should have no slots")
	}

	got, err := cls.Get(ctx, clk.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != clk.ID {
		t.Errorf("ID mismatch")
	}
}

func TestClock_List(t *testing.T) {
	cls := store.NewClockStore(openMemDB(t))
	ctx := context.Background()

	for _, name := range []string{"Tarde", "Manhã", "Noite"} {
		if _, err := cls.Create(ctx, name); err != nil {
			t.Fatalf("Create %q: %v", name, err)
		}
	}

	clocks, err := cls.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(clocks) != 3 {
		t.Errorf("expected 3, got %d", len(clocks))
	}
	if clocks[0].Name != "Manhã" {
		t.Errorf("first should be Manhã (alphabetical), got %q", clocks[0].Name)
	}
}

func TestClock_Update(t *testing.T) {
	cls := store.NewClockStore(openMemDB(t))
	ctx := context.Background()

	clk, _ := cls.Create(ctx, "Old")
	if err := cls.Update(ctx, clk.ID, "New"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := cls.Get(ctx, clk.ID)
	if got.Name != "New" {
		t.Errorf("Name after update = %q", got.Name)
	}
}

func TestClock_Delete(t *testing.T) {
	cls := store.NewClockStore(openMemDB(t))
	ctx := context.Background()

	clk, _ := cls.Create(ctx, "ToDelete")
	if err := cls.Delete(ctx, clk.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := cls.Get(ctx, clk.ID); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestClock_DeleteBlockedBySchedule(t *testing.T) {
	cls := store.NewClockStore(openMemDB(t))
	ctx := context.Background()

	clk, _ := cls.Create(ctx, "Scheduled")
	if err := cls.SetGridCells(ctx, []store.ScheduleCell{{Weekday: 1, Hour: 8, ClockID: clk.ID}}); err != nil {
		t.Fatalf("SetGridCells: %v", err)
	}

	if err := cls.Delete(ctx, clk.ID); err == nil {
		t.Error("expected error deleting clock referenced by schedule")
	}
}

func TestClockSlot_AddAndReorder(t *testing.T) {
	db := openMemDB(t)
	cs := store.NewCategoryStore(db)
	cls := store.NewClockStore(db)
	ctx := context.Background()

	cat, _ := cs.Create(ctx, "MPB", "", "#000")
	clk, _ := cls.Create(ctx, "Manhã")

	s1, err := cls.AddSlot(ctx, clk.ID, store.ClockSlot{SlotType: "CATEGORY", CategoryID: cat.ID})
	if err != nil {
		t.Fatalf("AddSlot 1: %v", err)
	}
	s2, err := cls.AddSlot(ctx, clk.ID, store.ClockSlot{SlotType: "JINGLE"})
	if err != nil {
		t.Fatalf("AddSlot 2: %v", err)
	}
	s3, err := cls.AddSlot(ctx, clk.ID, store.ClockSlot{SlotType: "SPOT"})
	if err != nil {
		t.Fatalf("AddSlot 3: %v", err)
	}

	// Verify positions 1,2,3.
	got, _ := cls.Get(ctx, clk.ID)
	if len(got.Slots) != 3 {
		t.Fatalf("expected 3 slots, got %d", len(got.Slots))
	}
	if got.Slots[0].Position != 1 || got.Slots[1].Position != 2 || got.Slots[2].Position != 3 {
		t.Errorf("positions wrong: %v", got.Slots)
	}

	// Reorder: s3, s1, s2
	if err := cls.ReorderSlots(ctx, clk.ID, []string{s3.ID, s1.ID, s2.ID}); err != nil {
		t.Fatalf("ReorderSlots: %v", err)
	}
	got, _ = cls.Get(ctx, clk.ID)
	if got.Slots[0].ID != s3.ID || got.Slots[1].ID != s1.ID || got.Slots[2].ID != s2.ID {
		t.Errorf("order after reorder wrong: %v", got.Slots)
	}
}

func TestClockSlot_Delete(t *testing.T) {
	cls := store.NewClockStore(openMemDB(t))
	ctx := context.Background()

	clk, _ := cls.Create(ctx, "Tarde")
	s1, _ := cls.AddSlot(ctx, clk.ID, store.ClockSlot{SlotType: "JINGLE"})
	s2, _ := cls.AddSlot(ctx, clk.ID, store.ClockSlot{SlotType: "SPOT"})
	_ = s2

	if err := cls.DeleteSlot(ctx, s1.ID); err != nil {
		t.Fatalf("DeleteSlot: %v", err)
	}

	got, _ := cls.Get(ctx, clk.ID)
	if len(got.Slots) != 1 {
		t.Errorf("expected 1 slot after delete, got %d", len(got.Slots))
	}
	// Remaining slot should have position 1.
	if got.Slots[0].Position != 1 {
		t.Errorf("position after compact = %d, want 1", got.Slots[0].Position)
	}
}

func TestClockGrid_SetAndGet(t *testing.T) {
	cls := store.NewClockStore(openMemDB(t))
	ctx := context.Background()

	clk, _ := cls.Create(ctx, "Manhã")

	cells := []store.ScheduleCell{
		{Weekday: 1, Hour: 8, ClockID: clk.ID},
		{Weekday: 1, Hour: 9, ClockID: clk.ID},
		{Weekday: 6, Hour: 8, ClockID: clk.ID},
	}
	if err := cls.SetGridCells(ctx, cells); err != nil {
		t.Fatalf("SetGridCells: %v", err)
	}

	got, err := cls.GetGrid(ctx)
	if err != nil {
		t.Fatalf("GetGrid: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 cells, got %d", len(got))
	}

	// Test GetClockForHour.
	c, err := cls.GetClockForHour(ctx, 1, 8)
	if err != nil {
		t.Fatalf("GetClockForHour: %v", err)
	}
	if c == nil || c.ID != clk.ID {
		t.Errorf("GetClockForHour returned wrong clock: %v", c)
	}

	// Empty cell.
	c2, err := cls.GetClockForHour(ctx, 0, 0)
	if err != nil {
		t.Fatalf("GetClockForHour empty: %v", err)
	}
	if c2 != nil {
		t.Errorf("expected nil for unset cell, got %v", c2)
	}
}

func TestClockGrid_ClearCell(t *testing.T) {
	cls := store.NewClockStore(openMemDB(t))
	ctx := context.Background()

	clk, _ := cls.Create(ctx, "Noite")
	if err := cls.SetGridCells(ctx, []store.ScheduleCell{{Weekday: 0, Hour: 22, ClockID: clk.ID}}); err != nil {
		t.Fatalf("SetGridCells: %v", err)
	}

	// Clear the cell.
	if err := cls.SetGridCells(ctx, []store.ScheduleCell{{Weekday: 0, Hour: 22, ClockID: ""}}); err != nil {
		t.Fatalf("SetGridCells clear: %v", err)
	}

	c, _ := cls.GetClockForHour(ctx, 0, 22)
	if c != nil {
		t.Errorf("expected nil after clearing cell, got %v", c)
	}
}
