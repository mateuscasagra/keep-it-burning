package ui

import "testing"

func TestMaskDigits(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"", 12, ""},
		{"0", 12, "0"},
		{"05", 12, "05"},
		{"050", 12, "05/0"},
		{"0502", 12, "05/02"},
		{"05022026", 12, "05/02/2026"},
		{"050220261", 12, "05/02/2026 1"},
		{"05022026183", 12, "05/02/2026 18:3"},
		{"050220261830", 12, "05/02/2026 18:30"},
		// Dígitos além do limite são ignorados.
		{"05022026183099", 12, "05/02/2026 18:30"},
		// Separadores já digitados não duplicam.
		{"05/02/2026 18:30", 12, "05/02/2026 18:30"},
		// Máscara só de data, dos filtros do relatório.
		{"05022026", 8, "05/02/2026"},
		{"0502202699", 8, "05/02/2026"},
	}
	for _, tc := range tests {
		if got := maskDigits(tc.in, tc.max); got != tc.want {
			t.Errorf("maskDigits(%q, %d) = %q, quero %q", tc.in, tc.max, got, tc.want)
		}
	}
}
