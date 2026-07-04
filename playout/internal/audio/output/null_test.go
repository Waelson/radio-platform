package output_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
)

var defaultCfg = output.OutputConfig{
	DeviceID:     "default",
	SampleRate:   48000,
	Channels:     2,
	BufferFrames: 2048,
}

func TestNullOutput_ImplementsInterface(t *testing.T) {
	var _ output.OutputDevice = (*output.NullOutput)(nil)
}

func TestNullOutput_OpenStartStopClose(t *testing.T) {
	n := &output.NullOutput{}
	ctx := context.Background()

	if err := n.Open(ctx, defaultCfg); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := n.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := n.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := n.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestNullOutput_Write_CountsFrames(t *testing.T) {
	n := &output.NullOutput{}
	ctx := context.Background()
	_ = n.Open(ctx, defaultCfg)
	_ = n.Start(ctx)

	// 100 frames × 2 channels = 200 samples
	buf := make([]float32, 100*2)
	nFrames, err := n.Write(ctx, buf)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if nFrames != 100 {
		t.Errorf("Write returned %d frames, want 100", nFrames)
	}
	if n.FramesWritten() != 100 {
		t.Errorf("FramesWritten = %d, want 100", n.FramesWritten())
	}
}

func TestNullOutput_Write_AccumulatesFrames(t *testing.T) {
	n := &output.NullOutput{}
	ctx := context.Background()
	_ = n.Open(ctx, defaultCfg)
	_ = n.Start(ctx)

	buf := make([]float32, 2048*2) // 2048 frames
	for i := 0; i < 3; i++ {
		_, _ = n.Write(ctx, buf)
	}
	if n.FramesWritten() != 3*2048 {
		t.Errorf("FramesWritten = %d, want %d", n.FramesWritten(), 3*2048)
	}
}

func TestNullOutput_Reset_ClearsCounter(t *testing.T) {
	n := &output.NullOutput{}
	ctx := context.Background()
	_ = n.Open(ctx, defaultCfg)
	_ = n.Start(ctx)

	buf := make([]float32, 100*2)
	_, _ = n.Write(ctx, buf)
	n.Reset()

	if n.FramesWritten() != 0 {
		t.Errorf("FramesWritten after Reset = %d, want 0", n.FramesWritten())
	}
}

func TestNullOutput_ForceWriteErr(t *testing.T) {
	sentinel := errors.New("write failed")
	n := &output.NullOutput{ForceWriteErr: sentinel}
	ctx := context.Background()
	_ = n.Open(ctx, defaultCfg)
	_ = n.Start(ctx)

	_, err := n.Write(ctx, make([]float32, 10))
	if !errors.Is(err, sentinel) {
		t.Errorf("Write error = %v, want sentinel", err)
	}
}

func TestNullOutput_Info(t *testing.T) {
	n := &output.NullOutput{}
	_ = n.Open(context.Background(), defaultCfg)

	info := n.Info()
	if info.Driver != "null" {
		t.Errorf("Driver = %q, want null", info.Driver)
	}
	if info.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", info.SampleRate)
	}
}

func TestNullOutput_Realtime_SleeepsApproximately(t *testing.T) {
	n := &output.NullOutput{Realtime: true}
	ctx := context.Background()
	_ = n.Open(ctx, defaultCfg)
	_ = n.Start(ctx)

	// 480 frames at 48000 Hz = 10ms
	buf := make([]float32, 480*2)
	start := time.Now()
	_, err := n.Write(ctx, buf)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Allow generous bounds for CI environments.
	if elapsed < 5*time.Millisecond || elapsed > 200*time.Millisecond {
		t.Errorf("elapsed = %v, expected ~10ms", elapsed)
	}
}

func TestNullOutput_Realtime_ContextCancel(t *testing.T) {
	n := &output.NullOutput{Realtime: true}
	ctx, cancel := context.WithCancel(context.Background())
	_ = n.Open(ctx, defaultCfg)
	_ = n.Start(ctx)

	// Cancel immediately.
	cancel()

	// Large buffer that would take a long time at real time.
	buf := make([]float32, 48000*2) // 1 second
	start := time.Now()
	_, _ = n.Write(ctx, buf)
	elapsed := time.Since(start)

	if elapsed > 200*time.Millisecond {
		t.Errorf("context cancel not respected; elapsed = %v", elapsed)
	}
}

func TestDBToLinear(t *testing.T) {
	cases := []struct {
		db   float64
		want float64
		tol  float64
	}{
		{0, 1.0, 0.001},
		{-6, 0.5012, 0.001},
		{-20, 0.1, 0.001},
		{-144, 0.0, 0.001},
		{-200, 0.0, 0.001},
	}
	for _, tc := range cases {
		got := output.DBToLinear(tc.db)
		diff := got - tc.want
		if diff < 0 {
			diff = -diff
		}
		if diff > tc.tol {
			t.Errorf("DBToLinear(%v) = %v, want %v ±%v", tc.db, got, tc.want, tc.tol)
		}
	}
}
