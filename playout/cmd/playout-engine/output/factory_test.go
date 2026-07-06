//go:build !portaudio && !coreaudio && !wasapi

package outfactory

import (
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/config"
)

func TestNewOutputDevice_Null(t *testing.T) {
	cfg := &config.Config{}
	cfg.Audio.Output.Driver = "null"
	dev, err := NewOutputDevice(cfg)
	if err != nil {
		t.Fatalf("NewOutputDevice(null): %v", err)
	}
	if _, ok := dev.(*output.NullOutput); !ok {
		t.Errorf("expected *output.NullOutput, got %T", dev)
	}
}

func TestNewOutputDevice_File(t *testing.T) {
	cfg := &config.Config{}
	cfg.Audio.Output.Driver = "file"
	dev, err := NewOutputDevice(cfg)
	if err != nil {
		t.Fatalf("NewOutputDevice(file): %v", err)
	}
	if _, ok := dev.(*output.FileOutput); !ok {
		t.Errorf("expected *output.FileOutput, got %T", dev)
	}
}

func TestNewOutputDevice_PortAudioWithoutTag(t *testing.T) {
	cfg := &config.Config{}
	cfg.Audio.Output.Driver = "portaudio"
	_, err := NewOutputDevice(cfg)
	if err == nil {
		t.Fatal("expected error when requesting portaudio without build tag")
	}
}

func TestNewOutputDevice_Unknown(t *testing.T) {
	cfg := &config.Config{}
	cfg.Audio.Output.Driver = "alsa"
	_, err := NewOutputDevice(cfg)
	if err == nil {
		t.Fatal("expected error for unknown driver")
	}
}
