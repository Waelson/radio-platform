package store_test

import (
	"context"
	"testing"

	"github.com/Waelson/radio-library-service/internal/store"
)

func TestStreamingStore_CRUD(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	s := store.NewStreamingStore(db)

	// List empty
	targets, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("expected 0 targets, got %d", len(targets))
	}

	// Create
	enabled := true
	sendMeta := true
	reconnEnabled := true
	autoConnect := false

	in := store.StreamingTargetInput{
		Name:                       "Stream Principal",
		Enabled:                    &enabled,
		Type:                       "icecast",
		Host:                       "stream.example.com",
		Port:                       8000,
		Mount:                      "/ao-vivo",
		Password:                   "segredo123",
		Format:                     "mp3",
		BitrateKbps:                128,
		SampleRate:                 44100,
		Channels:                   2,
		SendMetadata:               &sendMeta,
		StationName:                "Rádio Exemplo",
		StationDescription:         "A melhor rádio",
		StationGenre:               "Pop",
		StationURL:                 "https://exemplo.com",
		ReconnectEnabled:           &reconnEnabled,
		ReconnectMaxRetries:        0,
		ReconnectInitialDelaySec:   2,
		ReconnectMaxDelaySec:       60,
		ReconnectBackoffMultiplier: 2.0,
		AutoConnect:                &autoConnect,
	}

	created, err := s.Create(ctx, in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if created.Name != "Stream Principal" {
		t.Errorf("Name: got %q, want %q", created.Name, "Stream Principal")
	}
	if created.Password != "segredo123" {
		t.Errorf("Password not stored correctly")
	}
	if !created.Enabled {
		t.Error("expected Enabled=true")
	}
	if created.BitrateKbps != 128 {
		t.Errorf("BitrateKbps: got %d, want 128", created.BitrateKbps)
	}

	// Get by ID
	got, err := s.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Host != "stream.example.com" {
		t.Errorf("Host: got %q, want %q", got.Host, "stream.example.com")
	}
	if got.ReconnectBackoffMultiplier != 2.0 {
		t.Errorf("BackoffMultiplier: got %v, want 2.0", got.ReconnectBackoffMultiplier)
	}

	// Get not found
	_, err = s.Get(ctx, "does-not-exist")
	if err != store.ErrNotFound {
		t.Errorf("Get missing: got %v, want ErrNotFound", err)
	}

	// Update
	newBitrate := true // reuse bool for enabled
	updIn := in
	updIn.Name = "Stream Atualizado"
	updIn.BitrateKbps = 192
	updIn.Enabled = &newBitrate
	updIn.Password = "" // empty password → keep existing

	updated, err := s.Update(ctx, created.ID, updIn)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Stream Atualizado" {
		t.Errorf("Updated Name: got %q, want %q", updated.Name, "Stream Atualizado")
	}
	if updated.BitrateKbps != 192 {
		t.Errorf("Updated BitrateKbps: got %d, want 192", updated.BitrateKbps)
	}
	// Password should be preserved when empty string is sent
	if updated.Password != "segredo123" {
		t.Errorf("Password changed unexpectedly: got %q", updated.Password)
	}

	// Update not found
	_, err = s.Update(ctx, "does-not-exist", updIn)
	if err != store.ErrNotFound {
		t.Errorf("Update missing: got %v, want ErrNotFound", err)
	}

	// Create a second target to verify List ordering
	in2 := in
	in2.Name = "AAA Stream"
	in2.Port = 8001
	second, err := s.Create(ctx, in2)
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}
	_ = second

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List: got %d targets, want 2", len(list))
	}
	// Ordered by name: "AAA Stream" should come first
	if list[0].Name != "AAA Stream" {
		t.Errorf("List order: first item got %q, want %q", list[0].Name, "AAA Stream")
	}

	// Delete
	if err := s.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = s.Get(ctx, created.ID)
	if err != store.ErrNotFound {
		t.Errorf("After delete Get: got %v, want ErrNotFound", err)
	}

	// Delete idempotent — no error when already gone
	if err := s.Delete(ctx, created.ID); err != nil {
		t.Errorf("Delete idempotent: got %v, want nil", err)
	}
}

func TestStreamingStore_Validation(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	s := store.NewStreamingStore(db)

	cases := []struct {
		name string
		in   store.StreamingTargetInput
	}{
		{
			name: "empty name",
			in:   store.StreamingTargetInput{Type: "icecast", Host: "h", Port: 8000},
		},
		{
			name: "invalid type",
			in:   store.StreamingTargetInput{Name: "x", Type: "rtmp", Host: "h", Port: 8000},
		},
		{
			name: "empty host",
			in:   store.StreamingTargetInput{Name: "x", Type: "icecast", Port: 8000},
		},
		{
			name: "port zero",
			in:   store.StreamingTargetInput{Name: "x", Type: "icecast", Host: "h", Port: 0},
		},
		{
			name: "port out of range",
			in:   store.StreamingTargetInput{Name: "x", Type: "icecast", Host: "h", Port: 99999},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.Create(ctx, tc.in)
			if err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

func TestStreamingStore_Defaults(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	s := store.NewStreamingStore(db)

	// Create with minimal fields — defaults should be applied
	in := store.StreamingTargetInput{
		Name: "Minimal",
		Type: "shoutcast_v1",
		Host: "h.example.com",
		Port: 8000,
	}
	t1, err := s.Create(ctx, in)
	if err != nil {
		t.Fatalf("Create minimal: %v", err)
	}
	if t1.Format != "mp3" {
		t.Errorf("default Format: got %q, want mp3", t1.Format)
	}
	if t1.BitrateKbps != 128 {
		t.Errorf("default BitrateKbps: got %d, want 128", t1.BitrateKbps)
	}
	if t1.SampleRate != 44100 {
		t.Errorf("default SampleRate: got %d, want 44100", t1.SampleRate)
	}
	if t1.Channels != 2 {
		t.Errorf("default Channels: got %d, want 2", t1.Channels)
	}
	if !t1.Enabled {
		t.Error("default Enabled: want true")
	}
	if !t1.SendMetadata {
		t.Error("default SendMetadata: want true")
	}
	if !t1.ReconnectEnabled {
		t.Error("default ReconnectEnabled: want true")
	}
	if t1.ReconnectInitialDelaySec != 2 {
		t.Errorf("default ReconnectInitialDelaySec: got %d, want 2", t1.ReconnectInitialDelaySec)
	}
	if t1.ReconnectMaxDelaySec != 60 {
		t.Errorf("default ReconnectMaxDelaySec: got %d, want 60", t1.ReconnectMaxDelaySec)
	}
	if t1.ReconnectBackoffMultiplier != 2.0 {
		t.Errorf("default BackoffMultiplier: got %v, want 2.0", t1.ReconnectBackoffMultiplier)
	}
}
