package store_test

import (
	"context"
	"testing"

	"github.com/Waelson/radio-library-service/internal/store"
)

func TestImportLog_StartFinish(t *testing.T) {
	ctx := context.Background()
	s := store.NewTransmissionImportLogStore(openMemDB(t))

	id, err := s.StartImport(ctx, "transmission_20260720_08.jsonl")
	if err != nil {
		t.Fatalf("StartImport: %v", err)
	}
	if id == "" {
		t.Fatal("StartImport returned empty id")
	}

	if err := s.FinishImport(ctx, id, 50, 48); err != nil {
		t.Fatalf("FinishImport: %v", err)
	}

	entries, err := s.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	e := entries[0]
	if e.Status != "success" {
		t.Errorf("Status = %q, want success", e.Status)
	}
	if e.RecordsTotal != 50 {
		t.Errorf("RecordsTotal = %d, want 50", e.RecordsTotal)
	}
	if e.RecordsImported != 48 {
		t.Errorf("RecordsImported = %d, want 48", e.RecordsImported)
	}
	if e.FinishedAt == nil {
		t.Error("FinishedAt must not be nil after FinishImport")
	}
	if e.FileName != "transmission_20260720_08.jsonl" {
		t.Errorf("FileName = %q, want transmission_20260720_08.jsonl", e.FileName)
	}
}

func TestImportLog_StartFail(t *testing.T) {
	ctx := context.Background()
	s := store.NewTransmissionImportLogStore(openMemDB(t))

	id, err := s.StartImport(ctx, "bad_file.jsonl")
	if err != nil {
		t.Fatalf("StartImport: %v", err)
	}

	if err := s.FailImport(ctx, id, 10, "unexpected EOF"); err != nil {
		t.Fatalf("FailImport: %v", err)
	}

	entries, err := s.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	e := entries[0]
	if e.Status != "failed" {
		t.Errorf("Status = %q, want failed", e.Status)
	}
	if e.ErrorMessage != "unexpected EOF" {
		t.Errorf("ErrorMessage = %q, want 'unexpected EOF'", e.ErrorMessage)
	}
	if e.RecordsTotal != 10 {
		t.Errorf("RecordsTotal = %d, want 10", e.RecordsTotal)
	}
	if e.FinishedAt == nil {
		t.Error("FinishedAt must not be nil after FailImport")
	}
}

func TestImportLog_RunningStatus(t *testing.T) {
	ctx := context.Background()
	s := store.NewTransmissionImportLogStore(openMemDB(t))

	_, err := s.StartImport(ctx, "running_file.jsonl")
	if err != nil {
		t.Fatalf("StartImport: %v", err)
	}

	entries, err := s.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	e := entries[0]
	if e.Status != "running" {
		t.Errorf("Status = %q, want running", e.Status)
	}
	if e.FinishedAt != nil {
		t.Error("FinishedAt must be nil while running")
	}
}

func TestImportLog_List_OrderedByStartedAtDesc(t *testing.T) {
	ctx := context.Background()
	s := store.NewTransmissionImportLogStore(openMemDB(t))

	id1, _ := s.StartImport(ctx, "file_1.jsonl")
	s.FinishImport(ctx, id1, 1, 1)

	id2, _ := s.StartImport(ctx, "file_2.jsonl")
	s.FinishImport(ctx, id2, 2, 2)

	entries, err := s.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(entries))
	}
	// Most recent first — file_2 should be first.
	if entries[0].FileName != "file_2.jsonl" {
		t.Errorf("first entry = %q, want file_2.jsonl (DESC order)", entries[0].FileName)
	}
}

func TestImportLog_List_Pagination(t *testing.T) {
	ctx := context.Background()
	s := store.NewTransmissionImportLogStore(openMemDB(t))

	for i := 0; i < 5; i++ {
		s.StartImport(ctx, "file.jsonl")
	}

	page1, err := s.List(ctx, 2, 0)
	if err != nil {
		t.Fatalf("List page1: %v", err)
	}
	if len(page1) != 2 {
		t.Errorf("page1 len = %d, want 2", len(page1))
	}

	page2, err := s.List(ctx, 2, 2)
	if err != nil {
		t.Fatalf("List page2: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("page2 len = %d, want 2", len(page2))
	}
}
