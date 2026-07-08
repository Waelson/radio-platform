//go:build wasapi

package outfactory

// BuiltinDriverName returns "wasapi" — the driver compiled into this binary.
func BuiltinDriverName() string { return "wasapi" }
