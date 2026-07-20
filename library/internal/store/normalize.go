package store

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// NormalizeCategory produces a canonical key for a category name so that
// variations in case, accents, special characters, and spacing do not create
// duplicate entries.
//
// Examples:
//   "Pop"        → "pop"
//   "  MPB  "    → "mpb"
//   "Sertanejo"  → "sertanejo"
//   "Rock & Roll" → "rock & roll"
//   "Clássico"   → "classico"
func NormalizeCategory(name string) string {
	// 1. NFD decomposition: split accented runes into base + combining marks.
	t := transform.Chain(
		norm.NFD,
		runes.Remove(runes.In(unicode.Mn)), // remove combining (diacritic) marks
		norm.NFC,
	)
	result, _, err := transform.String(t, name)
	if err != nil {
		result = name // fallback: use original if transform fails
	}

	// 2. Lowercase.
	result = strings.ToLower(result)

	// 3. Collapse internal whitespace and trim.
	fields := strings.Fields(result)
	result = strings.Join(fields, " ")

	return result
}
