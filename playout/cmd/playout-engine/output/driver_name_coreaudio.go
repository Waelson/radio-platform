//go:build coreaudio && !wasapi

package outfactory

// BuiltinDriverName returns "coreaudio" — the driver compiled into this binary.
func BuiltinDriverName() string { return "coreaudio" }
