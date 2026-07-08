//go:build portaudio && !wasapi

package outfactory

// BuiltinDriverName returns "portaudio" — the driver compiled into this binary.
func BuiltinDriverName() string { return "portaudio" }
