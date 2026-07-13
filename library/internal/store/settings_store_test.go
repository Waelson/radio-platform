package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Waelson/radio-library-service/internal/store"
)

func TestSettings_GetSet(t *testing.T) {
	ctx := context.Background()
	s := store.NewSettingsStore(openMemDB(t))

	if err := s.Set(ctx, "station.name", "Radio Exemplo FM"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	v, err := s.Get(ctx, "station.name")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v != "Radio Exemplo FM" {
		t.Errorf("value = %q, want Radio Exemplo FM", v)
	}
}

func TestSettings_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	s := store.NewSettingsStore(openMemDB(t))

	_, err := s.Get(ctx, "nonexistent.key")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestSettings_Set_Upsert(t *testing.T) {
	ctx := context.Background()
	s := store.NewSettingsStore(openMemDB(t))

	if err := s.Set(ctx, "station.name", "First"); err != nil {
		t.Fatalf("Set first: %v", err)
	}
	if err := s.Set(ctx, "station.name", "Second"); err != nil {
		t.Fatalf("Set second: %v", err)
	}

	v, _ := s.Get(ctx, "station.name")
	if v != "Second" {
		t.Errorf("value after upsert = %q, want Second", v)
	}
}

func TestSettings_List_DefaultKeys(t *testing.T) {
	ctx := context.Background()
	openMemDB(t) // trigger migrations — but we need the store with same db

	db := openMemDB(t)
	s := store.NewSettingsStore(db)

	rows, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	// Migration 007 inserts 11 default keys.
	if len(rows) < 11 {
		t.Errorf("expected ≥ 11 default settings rows, got %d", len(rows))
	}

	keys := make(map[string]string, len(rows))
	for _, r := range rows {
		keys[r.Key] = r.Value
	}

	expected := []string{
		"transmission_log.dir",
		"transmission_log.file_name_template",
		"transmission_log.poll_interval",
		"transmission_log.grace_period",
		"transmission_log.retention_days",
		"station.name",
		"station.cnpj",
		"station.frequency",
		"station.type",
		"station.city",
		"station.state",
	}
	for _, k := range expected {
		if _, ok := keys[k]; !ok {
			t.Errorf("missing default settings key %q", k)
		}
	}
}

func TestSettings_TransmissionLogDir_Default(t *testing.T) {
	ctx := context.Background()
	s := store.NewSettingsStore(openMemDB(t))

	dir, err := s.TransmissionLogDir(ctx)
	if err != nil {
		t.Fatalf("TransmissionLogDir: %v", err)
	}
	if dir == "" {
		t.Error("TransmissionLogDir must not be empty (has default)")
	}
}

func TestSettings_TransmissionLogPollInterval_Default(t *testing.T) {
	ctx := context.Background()
	s := store.NewSettingsStore(openMemDB(t))

	d, err := s.TransmissionLogPollInterval(ctx)
	if err != nil {
		t.Fatalf("PollInterval: %v", err)
	}
	if d <= 0 {
		t.Errorf("PollInterval = %v, want > 0", d)
	}
}

func TestSettings_TransmissionLogGracePeriod_Default(t *testing.T) {
	ctx := context.Background()
	s := store.NewSettingsStore(openMemDB(t))

	d, err := s.TransmissionLogGracePeriod(ctx)
	if err != nil {
		t.Fatalf("GracePeriod: %v", err)
	}
	if d <= 0 {
		t.Errorf("GracePeriod = %v, want > 0", d)
	}
}

func TestSettings_RetentionDaysOrDefault_MinSeven(t *testing.T) {
	ctx := context.Background()
	s := store.NewSettingsStore(openMemDB(t))

	// Set below minimum.
	s.Set(ctx, "transmission_log.retention_days", "3")
	if got := s.RetentionDaysOrDefault(ctx); got != 7 {
		t.Errorf("RetentionDaysOrDefault = %d, want 7 (min enforced)", got)
	}

	// Set above minimum.
	s.Set(ctx, "transmission_log.retention_days", "30")
	if got := s.RetentionDaysOrDefault(ctx); got != 30 {
		t.Errorf("RetentionDaysOrDefault = %d, want 30", got)
	}
}

func TestSettings_StationInfo(t *testing.T) {
	ctx := context.Background()
	s := store.NewSettingsStore(openMemDB(t))

	s.Set(ctx, "station.name", "Radio Test FM")
	s.Set(ctx, "station.cnpj", "12.345.678/0001-90")
	s.Set(ctx, "station.frequency", "98.5 MHz")
	s.Set(ctx, "station.type", "FM")
	s.Set(ctx, "station.city", "São Paulo")
	s.Set(ctx, "station.state", "SP")

	info := s.StationInfo(ctx)
	if info.Name != "Radio Test FM" {
		t.Errorf("Name = %q, want Radio Test FM", info.Name)
	}
	if info.CNPJ != "12.345.678/0001-90" {
		t.Errorf("CNPJ = %q", info.CNPJ)
	}
	if info.Frequency != "98.5 MHz" {
		t.Errorf("Frequency = %q", info.Frequency)
	}
	if info.Type != "FM" {
		t.Errorf("Type = %q", info.Type)
	}
	if info.City != "São Paulo" {
		t.Errorf("City = %q", info.City)
	}
	if info.State != "SP" {
		t.Errorf("State = %q", info.State)
	}
}

func TestSettings_List_UpdatedAt(t *testing.T) {
	ctx := context.Background()
	s := store.NewSettingsStore(openMemDB(t))

	s.Set(ctx, "station.name", "Test")
	rows, _ := s.List(ctx)

	for _, r := range rows {
		if r.Key == "station.name" {
			if r.UpdatedAt.IsZero() {
				t.Error("UpdatedAt should not be zero")
			}
			if r.UpdatedAt.After(time.Now().Add(time.Minute)) {
				t.Errorf("UpdatedAt %v is suspiciously in the future", r.UpdatedAt)
			}
			return
		}
	}
	t.Error("station.name not found in List result")
}
