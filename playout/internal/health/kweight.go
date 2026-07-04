package health

// biquad is a second-order IIR filter (transposed direct form II).
// All arithmetic is float64; state (z1, z2) is updated per sample.
type biquad struct {
	b0, b1, b2 float64
	a1, a2     float64
	z1, z2     float64 // filter state
}

func (f *biquad) process(x float64) float64 {
	y := f.b0*x + f.z1
	f.z1 = f.b1*x - f.a1*y + f.z2
	f.z2 = f.b2*x - f.a2*y
	return y
}

func (f *biquad) reset() {
	f.z1 = 0
	f.z2 = 0
}

// kweightFilter is a two-stage K-weighting filter per EBU R128 / ITU-R BS.1770.
// Stage 1: high-shelf pre-filter (+4 dB above ~1.5 kHz).
// Stage 2: high-pass filter (−3 dB at ~38 Hz).
// Coefficients are fixed for 48 kHz sample rate.
// Instantiate one filter per audio channel.
type kweightFilter struct {
	stage1 biquad
	stage2 biquad
}

// newKweightFilter48k returns a K-weighting filter configured for 48 kHz.
func newKweightFilter48k() kweightFilter {
	return kweightFilter{
		stage1: biquad{
			b0: 1.53512485958697, b1: -2.69169618940638, b2: 1.19839281085285,
			a1: -1.69065929318241, a2: 0.73248077421585,
		},
		stage2: biquad{
			b0: 1.0, b1: -2.0, b2: 1.0,
			a1: -1.99004745483398, a2: 0.99007225036390,
		},
	}
}

func (f *kweightFilter) process(x float64) float64 {
	return f.stage2.process(f.stage1.process(x))
}

func (f *kweightFilter) reset() {
	f.stage1.reset()
	f.stage2.reset()
}
