package output_test

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
)

func TestFileOutput_ImplementsInterface(t *testing.T) {
	var _ output.OutputDevice = (*output.FileOutput)(nil)
}

func TestFileOutput_WritesValidWAV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.wav")

	fo := &output.FileOutput{Path: path}
	ctx := context.Background()

	if err := fo.Open(ctx, defaultCfg); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := fo.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Write 1000 stereo frames (2000 samples).
	frames := make([]float32, 1000*2)
	for i := range frames {
		frames[i] = float32(i) * 0.001
	}
	n, err := fo.Write(ctx, frames)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 1000 {
		t.Errorf("Write returned %d frames, want 1000", n)
	}

	if err := fo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Read and validate the WAV file.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Check RIFF header.
	if string(data[0:4]) != "RIFF" {
		t.Errorf("magic = %q, want RIFF", string(data[0:4]))
	}
	if string(data[8:12]) != "WAVE" {
		t.Errorf("format = %q, want WAVE", string(data[8:12]))
	}
	// fmt chunk.
	if string(data[12:16]) != "fmt " {
		t.Errorf("fmt chunk ID = %q", string(data[12:16]))
	}
	audioFormat := binary.LittleEndian.Uint16(data[20:22])
	if audioFormat != 3 { // IEEE float
		t.Errorf("AudioFormat = %d, want 3 (IEEE float)", audioFormat)
	}
	numChannels := binary.LittleEndian.Uint16(data[22:24])
	if numChannels != 2 {
		t.Errorf("NumChannels = %d, want 2", numChannels)
	}
	sampleRate := binary.LittleEndian.Uint32(data[24:28])
	if sampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", sampleRate)
	}
	bitsPerSample := binary.LittleEndian.Uint16(data[34:36])
	if bitsPerSample != 32 {
		t.Errorf("BitsPerSample = %d, want 32", bitsPerSample)
	}

	// data chunk.
	if string(data[38:42]) != "data" {
		t.Errorf("data chunk ID = %q", string(data[38:42]))
	}
	dataSize := binary.LittleEndian.Uint32(data[42:46])
	expectedDataSize := uint32(1000 * 2 * 4) // 1000 frames × 2 channels × 4 bytes
	if dataSize != expectedDataSize {
		t.Errorf("data chunk size = %d, want %d", dataSize, expectedDataSize)
	}

	// Validate the first few samples.
	audioData := data[46:]
	for i := 0; i < 10; i++ {
		bits := binary.LittleEndian.Uint32(audioData[i*4:])
		got := math.Float32frombits(bits)
		want := float32(i) * 0.001
		if math.Abs(float64(got-want)) > 1e-6 {
			t.Errorf("sample[%d] = %v, want %v", i, got, want)
		}
	}
}

func TestFileOutput_FramesWritten(t *testing.T) {
	dir := t.TempDir()
	fo := &output.FileOutput{Path: filepath.Join(dir, "out.wav")}
	ctx := context.Background()
	_ = fo.Open(ctx, defaultCfg)
	_ = fo.Start(ctx)

	_, _ = fo.Write(ctx, make([]float32, 500*2))
	_, _ = fo.Write(ctx, make([]float32, 300*2))

	if fo.FramesWritten() != 800 {
		t.Errorf("FramesWritten = %d, want 800", fo.FramesWritten())
	}
	_ = fo.Close()
}

func TestFileOutput_EmptyPath_Error(t *testing.T) {
	fo := &output.FileOutput{} // no path
	err := fo.Open(context.Background(), defaultCfg)
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}

func TestFileOutput_Info(t *testing.T) {
	dir := t.TempDir()
	fo := &output.FileOutput{Path: filepath.Join(dir, "out.wav")}
	_ = fo.Open(context.Background(), defaultCfg)

	info := fo.Info()
	if info.Driver != "file" {
		t.Errorf("Driver = %q, want file", info.Driver)
	}
	_ = fo.Close()
}
