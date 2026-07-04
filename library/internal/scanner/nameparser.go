// Package scanner handles audio file discovery, metadata extraction and indexing.
package scanner

import (
	"strings"
)

// ParsedName holds the fields extracted from an audio file name.
type ParsedName struct {
	Category string
	Artist   string
	Album    string // present only when the name has 3 or more " - " segments
	Title    string
}

// Parse extracts Category, Artist, Album and Title from the base name of an
// audio file (without extension). Rules applied in order:
//
//  1. If the name starts with "[", everything between "[" and "]" becomes Category.
//     The remainder after "] " is the candidate string.
//  2. The candidate is split on " - " (space-dash-space):
//     - 1 segment  → Title only; Artist and Album are empty.
//     - 2 segments → Artist = seg[0], Title = seg[1]; Album is empty.
//     - 3+ segments → Artist = seg[0], Album = seg[1], Title = remaining joined with " - ".
//
// Example inputs (no extension):
//
//	"[Sertanejo] Roberto Carlos - Emoções - Detalhes" → {Category:"Sertanejo", Artist:"Roberto Carlos", Album:"Emoções", Title:"Detalhes"}
//	"[Sertanejo] Roberto Carlos - Detalhes"           → {Category:"Sertanejo", Artist:"Roberto Carlos", Album:"",       Title:"Detalhes"}
//	"[Entrada] Abertura Manhã"                        → {Category:"Entrada",   Artist:"",               Album:"",       Title:"Abertura Manhã"}
//	"Roberto Carlos - Detalhes"                       → {Category:"",          Artist:"Roberto Carlos",  Album:"",       Title:"Detalhes"}
//	"Detalhes"                                        → {Category:"",          Artist:"",               Album:"",       Title:"Detalhes"}
func Parse(baseName string) ParsedName {
	var p ParsedName
	candidate := strings.TrimSpace(baseName)

	// Step 1: extract [Category] prefix.
	if strings.HasPrefix(candidate, "[") {
		end := strings.Index(candidate, "]")
		if end > 1 {
			p.Category = strings.TrimSpace(candidate[1:end])
			candidate = strings.TrimSpace(candidate[end+1:])
		}
	}

	// Step 2: split by " - " into segments.
	const sep = " - "
	parts := strings.Split(candidate, sep)
	switch len(parts) {
	case 1:
		p.Title = strings.TrimSpace(parts[0])
	case 2:
		p.Artist = strings.TrimSpace(parts[0])
		p.Title = strings.TrimSpace(parts[1])
	default: // 3 or more
		p.Artist = strings.TrimSpace(parts[0])
		p.Album = strings.TrimSpace(parts[1])
		p.Title = strings.TrimSpace(strings.Join(parts[2:], sep))
	}

	return p
}
