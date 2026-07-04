package scanner_test

import (
	"testing"

	"github.com/Waelson/radio-library-service/internal/scanner"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantCat string
		wantArt string
		wantAlb string
		wantTit string
	}{
		// Music: [Category] Artist - Title (2 segments, no album)
		{
			name:    "music full",
			input:   "[Sertanejo] Roberto Carlos - Detalhes",
			wantCat: "Sertanejo", wantArt: "Roberto Carlos", wantAlb: "", wantTit: "Detalhes",
		},
		{
			name:    "music pop",
			input:   "[Pop] Ana Gabriel - Tu Lo Decidiste",
			wantCat: "Pop", wantArt: "Ana Gabriel", wantAlb: "", wantTit: "Tu Lo Decidiste",
		},
		// Music with album: [Category] Artist - Album - Title (3 segments)
		{
			name:    "music with album",
			input:   "[Sertanejo] Roberto Carlos - Emoções - Detalhes",
			wantCat: "Sertanejo", wantArt: "Roberto Carlos", wantAlb: "Emoções", wantTit: "Detalhes",
		},
		{
			name:    "no category with album",
			input:   "The Beatles - Abbey Road - Come Together",
			wantCat: "", wantArt: "The Beatles", wantAlb: "Abbey Road", wantTit: "Come Together",
		},
		// Vinheta: [Category] Title (no artist, no album)
		{
			name:    "vinheta entrada",
			input:   "[Entrada] Abertura Manhã",
			wantCat: "Entrada", wantArt: "", wantAlb: "", wantTit: "Abertura Manhã",
		},
		{
			name:    "vinheta saida",
			input:   "[Saída] Encerramento Tarde",
			wantCat: "Saída", wantArt: "", wantAlb: "", wantTit: "Encerramento Tarde",
		},
		// Jingle: [Category] Radio - Title
		{
			name:    "jingle institucional",
			input:   "[Institucional] Rádio X - Você está na melhor",
			wantCat: "Institucional", wantArt: "Rádio X", wantAlb: "", wantTit: "Você está na melhor",
		},
		// Spot: [Category] Advertiser - Title
		{
			name:    "spot comercial",
			input:   "[Comercial] Farmácia XYZ - Promoção 30s",
			wantCat: "Comercial", wantArt: "Farmácia XYZ", wantAlb: "", wantTit: "Promoção 30s",
		},
		{
			name:    "spot patrocinio",
			input:   "[Patrocínio] Banco Y - Institucional",
			wantCat: "Patrocínio", wantArt: "Banco Y", wantAlb: "", wantTit: "Institucional",
		},
		// No category
		{
			name:    "no category with artist",
			input:   "Roberto Carlos - Detalhes",
			wantCat: "", wantArt: "Roberto Carlos", wantAlb: "", wantTit: "Detalhes",
		},
		// Bare title only
		{
			name:    "bare title",
			input:   "Detalhes",
			wantCat: "", wantArt: "", wantAlb: "", wantTit: "Detalhes",
		},
		// Category but no artist separator
		{
			name:    "category no artist",
			input:   "[Sertanejo] Detalhes",
			wantCat: "Sertanejo", wantArt: "", wantAlb: "", wantTit: "Detalhes",
		},
		// 3+ segments: artist - album - title (title keeps remaining dashes joined)
		{
			name:    "three segments with dashes in title",
			input:   "[Rock] AC-DC - Back in Black - Live",
			wantCat: "Rock", wantArt: "AC-DC", wantAlb: "Back in Black", wantTit: "Live",
		},
		{
			name:    "four segments",
			input:   "[Rock] AC-DC - Back in Black - Hells Bells - Remaster",
			wantCat: "Rock", wantArt: "AC-DC", wantAlb: "Back in Black", wantTit: "Hells Bells - Remaster",
		},
		// Edge: empty string
		{
			name:    "empty",
			input:   "",
			wantCat: "", wantArt: "", wantAlb: "", wantTit: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := scanner.Parse(tc.input)
			if got.Category != tc.wantCat {
				t.Errorf("Category = %q, want %q", got.Category, tc.wantCat)
			}
			if got.Artist != tc.wantArt {
				t.Errorf("Artist = %q, want %q", got.Artist, tc.wantArt)
			}
			if got.Album != tc.wantAlb {
				t.Errorf("Album = %q, want %q", got.Album, tc.wantAlb)
			}
			if got.Title != tc.wantTit {
				t.Errorf("Title = %q, want %q", got.Title, tc.wantTit)
			}
		})
	}
}
