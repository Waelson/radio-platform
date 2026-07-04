package health

import (
	"math"
	"sync"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/events"
)

const (
	// lufsMomentaryFrames is the EBU R128 momentary window: 400 ms at 48 kHz.
	lufsMomentaryMS = 400
	// clipThresholdDBFS: peaks at or above this level are flagged as clip.
	clipThresholdDBFS = -0.1
)

// vuState accumulates professional audio metrics for the VU meter.
// push() is called from the audio hot path (Monitor.Push); snapshot() is
// called from the Run goroutine ticker. Both paths hold mu.
type vuState struct {
	mu sync.Mutex

	channels int
	sr       int

	// Per-channel RMS accumulators (50 ms window, aligned with health monitor).
	rmsFrameCount  int
	rmsFrameWindow int // frames per 50 ms
	chanSumSq      [2]float64
	chanPeakLinear [2]float64

	// Per-channel K-weighted accumulators for LUFS momentary (400 ms window).
	kFilters      [2]kweightFilter
	kSumSq        [2]float64
	kFrameCount   int
	kFrameWindow  int // frames per 400 ms

	// Per-channel K-weighted accumulators for LUFS integrated (lifetime).
	intSumSq    [2]float64
	intFrames   int64

	// Peak hold state (linear amplitude).
	peakHoldLinear float64
	peakHoldDB     float64
	peakHoldExpiry time.Time
	peakHoldMS     int

	// Last flushed snapshot values (read by snapshot()).
	lastRMS      [2]float64 // dBFS per channel
	lastPeak     [2]float64 // dBFS per channel
	lastPeakHold float64    // dBFS
	lastLUFSMom  float64    // LUFS
	lastLUFSInt  float64    // LUFS
}

func newVUState(channels, sampleRate, peakHoldMS int) *vuState {
	if channels < 1 {
		channels = 1
	}
	if channels > 2 {
		channels = 2
	}
	v := &vuState{
		channels:      channels,
		sr:            sampleRate,
		rmsFrameWindow: sampleRate * windowDurationMS / 1000,
		kFrameWindow:  sampleRate * lufsMomentaryMS / 1000,
		peakHoldMS:    peakHoldMS,
		lastLUFSMom:   -144.0,
		lastLUFSInt:   -144.0,
		lastPeakHold:  -120.0,
	}
	for c := 0; c < channels; c++ {
		v.kFilters[c] = newKweightFilter48k()
		v.lastRMS[c] = -120.0
		v.lastPeak[c] = -120.0
	}
	return v
}

// push ingests interleaved PCM samples. Called from Monitor.Push — must not
// allocate and must only do scalar arithmetic inside the lock.
func (v *vuState) push(samples []float32) {
	if len(samples) == 0 {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()

	ch := v.channels
	for i := 0; i+ch <= len(samples); i += ch {
		for c := 0; c < ch; c++ {
			s := float64(samples[i+c])
			// RMS per channel.
			v.chanSumSq[c] += s * s
			abs := s
			if abs < 0 {
				abs = -abs
			}
			if abs > v.chanPeakLinear[c] {
				v.chanPeakLinear[c] = abs
			}
			// K-weighted for LUFS.
			ks := v.kFilters[c].process(s)
			v.kSumSq[c] += ks * ks
			v.intSumSq[c] += ks * ks
		}
		v.rmsFrameCount++
		v.kFrameCount++
		v.intFrames++

		if v.rmsFrameCount >= v.rmsFrameWindow {
			v.flushRMS()
		}
		if v.kFrameCount >= v.kFrameWindow {
			v.flushLUFSMomentary()
		}
	}
}

// flushRMS computes and stores RMS/peak values for the completed 50 ms window.
// Must be called with v.mu held.
func (v *vuState) flushRMS() {
	n := float64(v.rmsFrameCount)
	now := time.Now()
	ch := v.channels
	var maxPeakLinear float64
	for c := 0; c < ch; c++ {
		rms := math.Sqrt(v.chanSumSq[c] / n)
		v.lastRMS[c] = linearToDBFS(rms)
		pk := v.chanPeakLinear[c]
		v.lastPeak[c] = linearToDBFS(pk)
		if pk > maxPeakLinear {
			maxPeakLinear = pk
		}
		v.chanSumSq[c] = 0
		v.chanPeakLinear[c] = 0
	}
	// Update peak hold.
	if maxPeakLinear > v.peakHoldLinear || now.After(v.peakHoldExpiry) {
		v.peakHoldLinear = maxPeakLinear
		v.peakHoldDB = linearToDBFS(maxPeakLinear)
		v.peakHoldExpiry = now.Add(time.Duration(v.peakHoldMS) * time.Millisecond)
	}
	v.lastPeakHold = v.peakHoldDB
	v.rmsFrameCount = 0
}

// flushLUFSMomentary computes and stores LUFS momentary for the 400 ms window.
// Must be called with v.mu held.
func (v *vuState) flushLUFSMomentary() {
	n := float64(v.kFrameCount)
	ch := v.channels
	var meanSq float64
	for c := 0; c < ch; c++ {
		meanSq += v.kSumSq[c] / n
		v.kSumSq[c] = 0
	}
	meanSq /= float64(ch)
	v.lastLUFSMom = lufsFromMeanSquare(meanSq)

	// LUFS integrated: recompute from lifetime accumulators.
	if v.intFrames > 0 {
		var intMeanSq float64
		for c := 0; c < ch; c++ {
			intMeanSq += v.intSumSq[c] / float64(v.intFrames)
		}
		intMeanSq /= float64(ch)
		v.lastLUFSInt = lufsFromMeanSquare(intMeanSq)
	}
	v.kFrameCount = 0
}

// snapshot returns a VUMeterPayload with the latest computed values.
// Called from the Run goroutine ticker — safe to call concurrently with push().
func (v *vuState) snapshot() events.VUMeterPayload {
	v.mu.Lock()
	defer v.mu.Unlock()

	ch := v.channels
	channels := make([]events.VUChannelPayload, ch)
	var maxPeakDB float64 = -120.0
	var rmsSum float64
	for c := 0; c < ch; c++ {
		channels[c] = events.VUChannelPayload{
			RMSDbfs:  v.lastRMS[c],
			PeakDbfs: v.lastPeak[c],
		}
		if v.lastPeak[c] > maxPeakDB {
			maxPeakDB = v.lastPeak[c]
		}
		rmsSum += v.lastRMS[c]
	}
	rmsAvg := rmsSum / float64(ch)

	return events.VUMeterPayload{
		RMSDbfs:        rmsAvg,
		PeakDbfs:       maxPeakDB,
		PeakHoldDbfs:   v.lastPeakHold,
		LUFSMomentary:  v.lastLUFSMom,
		LUFSIntegrated: v.lastLUFSInt,
		Clip:           maxPeakDB >= clipThresholdDBFS,
		Channels:       channels,
	}
}

// lufsFromMeanSquare converts a mean square (K-weighted) to LUFS per EBU R128.
func lufsFromMeanSquare(meanSq float64) float64 {
	if meanSq <= 0 {
		return -144.0
	}
	return -0.691 + 10*math.Log10(meanSq)
}
