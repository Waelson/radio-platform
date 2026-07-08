//go:build !portaudio && !coreaudio && !wasapi

package outfactory

import (
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/config"
)

func TestNewOutputDevice_ReturnsNullWithoutBuildTag(t *testing.T) {
	dev, err := NewOutputDevice(&config.Config{})
	if err != nil {
		t.Fatalf("NewOutputDevice: %v", err)
	}
	if _, ok := dev.(*output.NullOutput); !ok {
		t.Errorf("expected *output.NullOutput, got %T", dev)
	}
}

func TestNewPreviewOutputDevice_ReturnsNullWithoutBuildTag(t *testing.T) {
	dev, err := NewPreviewOutputDevice(&config.Config{})
	if err != nil {
		t.Fatalf("NewPreviewOutputDevice: %v", err)
	}
	if _, ok := dev.(*output.NullOutput); !ok {
		t.Errorf("expected *output.NullOutput, got %T", dev)
	}
}

func TestBuiltinDriverName_ReturnsNullWithoutBuildTag(t *testing.T) {
	if got := BuiltinDriverName(); got != "null" {
		t.Errorf("BuiltinDriverName() = %q, want %q", got, "null")
	}
}
