// Package horacerta resolves the current time to a sequence of pre-recorded
// audio files that announce the hour and minute (Brazilian "hora certa" style).
//
// File naming convention (configurable via patterns):
//
//	Hours:   HRS{HH}.mp3  — e.g. HRS14.mp3 for 14h
//	Minutes: MIN{MM}.mp3  — e.g. MIN35.mp3 for :35
//	         (MIN00 is optional; when absent, only the hour file is played at XX:00)
package horacerta

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/audio"
	"github.com/Waelson/radio-playout-engine/internal/audio/decoder"
)

// Config holds the hora certa feature configuration.
type Config struct {
	// HoursDir is the directory containing hour announcement files.
	HoursDir string `yaml:"hours_dir"`

	// MinutesDir is the directory containing minute announcement files.
	MinutesDir string `yaml:"minutes_dir"`

	// HourPattern is the filename pattern for hour files.
	// {HH} is replaced with the zero-padded hour (00–23).
	// Default: "HRS{HH}.mp3"
	HourPattern string `yaml:"hour_pattern"`

	// MinutePattern is the filename pattern for minute files.
	// {MM} is replaced with the zero-padded minute (00–59).
	// Default: "MIN{MM}.mp3"
	MinutePattern string `yaml:"minute_pattern"`

	// GainDB is the default gain applied to hora certa audio (0 = unity).
	// Individual queue items may override this via their own GainDB field.
	GainDB float64 `yaml:"gain_db"`
}

// Resolver resolves the current time to a sequence of audio file paths.
type Resolver struct {
	cfg Config
}

// NewResolver creates a Resolver from cfg, applying default patterns where omitted.
func NewResolver(cfg Config) *Resolver {
	if cfg.HourPattern == "" {
		cfg.HourPattern = "HRS{HH}.mp3"
	}
	if cfg.MinutePattern == "" {
		cfg.MinutePattern = "MIN{MM}.mp3"
	}
	return &Resolver{cfg: cfg}
}

// Resolve returns the ordered list of audio file paths for time t.
// At XX:00 (minute zero) only the hour file is returned if no MIN00 file exists,
// so the announcement is "são três horas" without a redundant "e zero minutos".
func (r *Resolver) Resolve(t time.Time) ([]string, error) {
	hh := fmt.Sprintf("%02d", t.Hour())
	mm := fmt.Sprintf("%02d", t.Minute())

	hPath := filepath.Join(r.cfg.HoursDir,
		strings.ReplaceAll(r.cfg.HourPattern, "{HH}", hh))

	if _, err := os.Stat(hPath); err != nil {
		return nil, fmt.Errorf("hora certa: hour file not found %q", hPath)
	}

	paths := []string{hPath}

	mPath := filepath.Join(r.cfg.MinutesDir,
		strings.ReplaceAll(r.cfg.MinutePattern, "{MM}", mm))

	if _, err := os.Stat(mPath); err == nil {
		paths = append(paths, mPath)
	} else if t.Minute() != 0 {
		// Minute file missing and it is not XX:00 — report the error.
		return nil, fmt.Errorf("hora certa: minute file not found %q", mPath)
	}
	// If minute == 0 and MIN00 doesn't exist, play hour file only (no error).

	return paths, nil
}

// EffectiveGainDB returns the gain to apply for a hora certa item.
// itemGainDB overrides the config default when non-zero.
func (r *Resolver) EffectiveGainDB(itemGainDB float64) float64 {
	if itemGainDB != 0 {
		return itemGainDB
	}
	return r.cfg.GainDB
}

// OpenChain opens the given paths in sequence and returns a single PCMStream
// that reads them one after the other, applying gainDB to every sample.
func (r *Resolver) OpenChain(ctx context.Context, dec decoder.Decoder, paths []string, gainDB float64) (decoder.PCMStream, error) {
	streams := make([]decoder.PCMStream, 0, len(paths))
	for _, p := range paths {
		s, err := dec.Open(ctx, decoder.Source{Path: p})
		if err != nil {
			for _, already := range streams {
				_ = already.Close()
			}
			return nil, fmt.Errorf("hora certa: open %q: %w", p, err)
		}
		streams = append(streams, s)
	}
	return &chainStream{
		streams:     streams,
		gainLinear:  math.Pow(10, gainDB/20),
	}, nil
}

// --- chainStream -------------------------------------------------------

// chainStream implements decoder.PCMStream by reading from a sequence of
// streams one after the other, applying a linear gain to all samples.
type chainStream struct {
	streams    []decoder.PCMStream
	idx        int
	gainLinear float64
}

func (c *chainStream) ReadFrames(ctx context.Context, dst []float32) (int, error) {
	channels := c.Format().Channels
	for c.idx < len(c.streams) {
		n, err := c.streams[c.idx].ReadFrames(ctx, dst)
		if n > 0 {
			if c.gainLinear != 1.0 {
				for i := 0; i < n*channels; i++ {
					dst[i] *= float32(c.gainLinear)
				}
			}
			if err == io.EOF && c.idx < len(c.streams)-1 {
				// More streams to come: suppress EOF, advance index.
				c.idx++
				return n, nil
			}
			return n, err
		}
		if err == io.EOF {
			c.idx++
			continue
		}
		if err != nil {
			return 0, err
		}
	}
	return 0, io.EOF
}

func (c *chainStream) Close() error {
	var last error
	for _, s := range c.streams {
		if err := s.Close(); err != nil {
			last = err
		}
	}
	return last
}

func (c *chainStream) Format() audio.AudioFormat {
	if len(c.streams) > 0 {
		return c.streams[0].Format()
	}
	return audio.DefaultFormat
}
