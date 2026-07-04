package audio_test

import (
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/audio"
)

func TestDefaultFormat(t *testing.T) {
	f := audio.DefaultFormat
	if f.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", f.SampleRate)
	}
	if f.Channels != 2 {
		t.Errorf("Channels = %d, want 2", f.Channels)
	}
}

func TestSamplesPerFrame(t *testing.T) {
	f := audio.AudioFormat{SampleRate: 48000, Channels: 2}
	if f.SamplesPerFrame() != 2 {
		t.Errorf("SamplesPerFrame = %d, want 2", f.SamplesPerFrame())
	}
}

func TestBytesPerFrame(t *testing.T) {
	f := audio.AudioFormat{SampleRate: 48000, Channels: 2}
	if f.BytesPerFrame() != 8 { // 2 channels × 4 bytes
		t.Errorf("BytesPerFrame = %d, want 8", f.BytesPerFrame())
	}
}

func TestFramesPerMs(t *testing.T) {
	f := audio.AudioFormat{SampleRate: 48000, Channels: 2}
	if frames := f.FramesPerMs(1000); frames != 48000 {
		t.Errorf("FramesPerMs(1000) = %d, want 48000", frames)
	}
	if frames := f.FramesPerMs(500); frames != 24000 {
		t.Errorf("FramesPerMs(500) = %d, want 24000", frames)
	}
}

func TestMsFromFrames(t *testing.T) {
	f := audio.AudioFormat{SampleRate: 48000, Channels: 2}
	if ms := f.MsFromFrames(48000); ms != 1000 {
		t.Errorf("MsFromFrames(48000) = %d, want 1000", ms)
	}
	if ms := f.MsFromFrames(0); ms != 0 {
		t.Errorf("MsFromFrames(0) = %d, want 0", ms)
	}
}

func TestMsFromFrames_ZeroSampleRate(t *testing.T) {
	f := audio.AudioFormat{SampleRate: 0, Channels: 2}
	if ms := f.MsFromFrames(1000); ms != 0 {
		t.Errorf("MsFromFrames with zero SampleRate = %d, want 0", ms)
	}
}
