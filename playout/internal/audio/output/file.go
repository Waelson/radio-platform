package output

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

// FileOutput writes interleaved float32 PCM frames to a WAV file.
// It uses IEEE 754 float32 format (WAVE_FORMAT_IEEE_FLOAT = 3).
// Usage:
//
//	out := &FileOutput{Path: "/tmp/test.wav"}
//	out.Open(ctx, cfg)
//	out.Start(ctx)
//	out.Write(ctx, frames)
//	out.Close()  // patches the WAV header with final sizes
type FileOutput struct {
	// Path is the destination file path. Must be set before Open.
	Path string

	cfg          OutputConfig
	f            *os.File
	dataStart    int64  // byte offset where audio data begins
	samplesWrit  int64  // float32 samples written (includes all channels)
	framesWrit   int64  // frames (samplesWrit / channels)
}

var _ OutputDevice = (*FileOutput)(nil)

// wavHeaderSize is the byte size of the WAV header written by writeHeader.
// RIFF(12) + fmt(26, IEEE float) + data(8) = 46 bytes.
const wavHeaderSize = 46

func (fo *FileOutput) Open(_ context.Context, cfg OutputConfig) error {
	if fo.Path == "" {
		return fmt.Errorf("file output: path must be set before Open")
	}
	f, err := os.Create(fo.Path)
	if err != nil {
		return fmt.Errorf("file output: create %s: %w", fo.Path, err)
	}
	fo.f = f
	fo.cfg = cfg
	fo.samplesWrit = 0
	fo.framesWrit = 0

	// Write placeholder header; Close() will patch the sizes.
	return fo.writeHeader(0)
}

func (fo *FileOutput) Start(_ context.Context) error { return nil }

// Write appends interleaved float32 PCM frames to the file.
func (fo *FileOutput) Write(_ context.Context, frames []float32) (int, error) {
	if fo.f == nil {
		return 0, fmt.Errorf("file output: not open")
	}
	channels := fo.cfg.Channels
	if channels == 0 {
		channels = 2
	}
	if len(frames)%channels != 0 {
		return 0, fmt.Errorf("file output: frames length %d not divisible by channels %d",
			len(frames), channels)
	}

	// Encode each float32 sample as little-endian bytes.
	buf := make([]byte, len(frames)*4)
	for i, s := range frames {
		bits := math.Float32bits(s)
		binary.LittleEndian.PutUint32(buf[i*4:], bits)
	}
	if _, err := fo.f.Write(buf); err != nil {
		return 0, fmt.Errorf("file output: write: %w", err)
	}

	fo.samplesWrit += int64(len(frames))
	fo.framesWrit += int64(len(frames) / channels)
	return len(frames) / channels, nil
}

func (fo *FileOutput) Stop(_ context.Context) error { return nil }

// Close patches the WAV header with the final data size and closes the file.
func (fo *FileOutput) Close() error {
	if fo.f == nil {
		return nil
	}
	if err := fo.patchHeader(); err != nil {
		_ = fo.f.Close()
		fo.f = nil
		return err
	}
	err := fo.f.Close()
	fo.f = nil
	return err
}

func (fo *FileOutput) Info() OutputDeviceInfo {
	return OutputDeviceInfo{
		ID:         "file:" + fo.Path,
		Name:       "FileOutput",
		Driver:     "file",
		SampleRate: fo.cfg.SampleRate,
		Channels:   fo.cfg.Channels,
	}
}

// ListDevices returns a single pseudo-device representing the file output.
func (fo *FileOutput) ListDevices() ([]DeviceInfo, error) {
	return []DeviceInfo{{
		ID:                "file",
		Name:              "File Output",
		Driver:            "file",
		IsDefault:         true,
		MaxOutputChannels: 2,
		DefaultSampleRate: 48000,
	}}, nil
}

// FramesWritten returns the number of frames written so far.
func (fo *FileOutput) FramesWritten() int64 {
	return fo.framesWrit
}

// writeHeader writes a WAV header with the given data chunk size (0 for placeholder).
func (fo *FileOutput) writeHeader(dataSizeBytes uint32) error {
	sr := uint32(fo.cfg.SampleRate)
	ch := uint16(fo.cfg.Channels)
	if sr == 0 {
		sr = 48000
	}
	if ch == 0 {
		ch = 2
	}

	const bitsPerSample = 32 // float32
	blockAlign := ch * (bitsPerSample / 8)
	byteRate := sr * uint32(blockAlign)

	// Total RIFF chunk size = 4 ("WAVE") + 26 (fmt) + 8 (data hdr) + data
	riffSize := uint32(4 + 26 + 8 + dataSizeBytes)

	w := fo.f
	// RIFF chunk
	w.Write([]byte("RIFF"))
	binary.Write(w, binary.LittleEndian, riffSize)
	w.Write([]byte("WAVE"))

	// fmt chunk (18 bytes for IEEE float)
	w.Write([]byte("fmt "))
	binary.Write(w, binary.LittleEndian, uint32(18)) // chunk size
	binary.Write(w, binary.LittleEndian, uint16(3))  // WAVE_FORMAT_IEEE_FLOAT
	binary.Write(w, binary.LittleEndian, ch)
	binary.Write(w, binary.LittleEndian, sr)
	binary.Write(w, binary.LittleEndian, byteRate)
	binary.Write(w, binary.LittleEndian, blockAlign)
	binary.Write(w, binary.LittleEndian, uint16(bitsPerSample))
	binary.Write(w, binary.LittleEndian, uint16(0)) // extra param size

	// data chunk header
	w.Write([]byte("data"))
	binary.Write(w, binary.LittleEndian, dataSizeBytes)

	return nil
}

// patchHeader seeks to the beginning and rewrites the header with real sizes.
func (fo *FileOutput) patchHeader() error {
	dataSizeBytes := uint32(fo.samplesWrit * 4)
	if _, err := fo.f.Seek(0, 0); err != nil {
		return fmt.Errorf("file output: seek for header patch: %w", err)
	}
	return fo.writeHeader(dataSizeBytes)
}
