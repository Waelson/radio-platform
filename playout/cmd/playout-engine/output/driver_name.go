//go:build !portaudio && !coreaudio && !wasapi

package outfactory

// BuiltinDriverName returns the name of the audio driver compiled into this binary.
// Binaries built without a driver tag use the null (silent) driver.
func BuiltinDriverName() string { return "null" }
