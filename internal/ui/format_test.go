package ui

import (
	"errors"
	"testing"
	"time"
)

func TestParseDateTime(t *testing.T) {
	tests := []struct {
		nome string
		in   string
		want time.Time
	}{
		{
			"data e hora",
			"05/02/2026 18:30",
			time.Date(2026, time.February, 5, 18, 30, 0, 0, time.Local),
		},
		{
			"hora antes da data, como a lista exibe",
			"18:30 05/02/2026",
			time.Date(2026, time.February, 5, 18, 30, 0, 0, time.Local),
		},
		{
			"so a data vira fim do dia",
			"05/02/2026",
			time.Date(2026, time.February, 5, 23, 59, 0, 0, time.Local),
		},
		{
			"formato iso",
			"2026-02-05 09:00",
			time.Date(2026, time.February, 5, 9, 0, 0, 0, time.Local),
		},
		{
			"espaços sobrando",
			"  05/02/2026 18:30  ",
			time.Date(2026, time.February, 5, 18, 30, 0, 0, time.Local),
		},
	}
	for _, tc := range tests {
		t.Run(tc.nome, func(t *testing.T) {
			got, err := parseDateTime(tc.in)
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("parseDateTime(%q) = %v, quero %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseDateTimeVazio(t *testing.T) {
	// Campo em branco significa "sem prazo", não erro de digitação — a tela de
	// tarefa precisa distinguir os dois casos.
	if _, err := parseDateTime("   "); !errors.Is(err, ErrEmptyDate) {
		t.Fatalf("quero ErrEmptyDate, tenho %v", err)
	}
}

func TestParseDateTimeInvalido(t *testing.T) {
	for _, in := range []string{"amanhã", "32/13/2026", "05-02", "abc"} {
		if _, err := parseDateTime(in); err == nil {
			t.Errorf("parseDateTime(%q) deveria falhar", in)
		} else if errors.Is(err, ErrEmptyDate) {
			t.Errorf("parseDateTime(%q) não é campo vazio", in)
		}
	}
}

func TestFormatDateTime(t *testing.T) {
	d := time.Date(2026, time.February, 5, 10, 0, 0, 0, time.Local)
	if got, want := formatDateTime(d), "10:00 05/02/2026"; got != want {
		t.Errorf("formatDateTime = %q, quero %q", got, want)
	}
	if got, want := formatDate(d), "05/02/2026"; got != want {
		t.Errorf("formatDate = %q, quero %q", got, want)
	}
	if got, want := formatDateTime(time.Time{}), "—"; got != want {
		t.Errorf("data zero = %q, quero %q", got, want)
	}
	if got := formatDateInput(time.Time{}); got != "" {
		t.Errorf("campo de edição com data zero deveria vir vazio, tenho %q", got)
	}
}

func TestParseDateTimeAceitaOQueFormatDateInputProduz(t *testing.T) {
	// Editar uma tarefa e salvar sem mexer no campo não pode alterar o prazo.
	original := time.Date(2026, time.February, 5, 18, 30, 0, 0, time.Local)
	got, err := parseDateTime(formatDateInput(original))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !got.Equal(original) {
		t.Fatalf("ida e volta mudou a data: %v -> %v", original, got)
	}
}

func TestParseDurationInput(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"8", 8 * time.Hour},
		{"8h", 8 * time.Hour},
		{"8h30", 8*time.Hour + 30*time.Minute},
		{"8:30", 8*time.Hour + 30*time.Minute},
		{"8h05", 8*time.Hour + 5*time.Minute},
		{"7.5", 7*time.Hour + 30*time.Minute},
		{"7,5", 7*time.Hour + 30*time.Minute},
		{"90min", 90 * time.Minute},
		{"45m", 45 * time.Minute},
		{" 40 h ", 40 * time.Hour},
		{"0", 0},
	}
	for _, tc := range tests {
		got, err := parseDurationInput(tc.in)
		if err != nil {
			t.Errorf("parseDurationInput(%q): erro inesperado %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseDurationInput(%q) = %v, quero %v", tc.in, got, tc.want)
		}
	}
}

func TestParseDurationInputInvalido(t *testing.T) {
	for _, in := range []string{"", "abc", "-3", "8h99", "8h-1", "oito horas"} {
		if got, err := parseDurationInput(in); err == nil {
			t.Errorf("parseDurationInput(%q) = %v, deveria falhar", in, got)
		}
	}
}

func TestFormatDurationInput(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{8 * time.Hour, "8h"},
		{8*time.Hour + 30*time.Minute, "8h30"},
		{8*time.Hour + 5*time.Minute, "8h05"},
		{40 * time.Hour, "40h"},
		{0, "0h"},
		{-time.Hour, "0h"},
	}
	for _, tc := range tests {
		if got := formatDurationInput(tc.in); got != tc.want {
			t.Errorf("formatDurationInput(%v) = %q, quero %q", tc.in, got, tc.want)
		}
	}
}

func TestDuracaoIdaEVolta(t *testing.T) {
	// Abrir a configuração e salvar sem editar não pode alterar as metas.
	for _, d := range []time.Duration{8 * time.Hour, 40 * time.Hour, 2*time.Hour + 30*time.Minute, 45 * time.Minute} {
		got, err := parseDurationInput(formatDurationInput(d))
		if err != nil {
			t.Fatalf("ida e volta de %v falhou: %v", d, err)
		}
		if got != d {
			t.Errorf("ida e volta de %v deu %v", d, got)
		}
	}
}

func TestParsePriorityValue(t *testing.T) {
	if got, err := parsePriorityValue(" 5 "); err != nil || got != 5 {
		t.Errorf("parsePriorityValue(\" 5 \") = %v, %v; quero 5, nil", got, err)
	}
	for _, in := range []string{"0", "-1", "abc", "", "1000", "2.5"} {
		if _, err := parsePriorityValue(in); err == nil {
			t.Errorf("parsePriorityValue(%q) deveria falhar", in)
		}
	}
}

func TestPct(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "0%"}, {0.5, "50%"}, {1, "100%"}, {0.333, "33%"}, {0.666, "67%"},
	}
	for _, tc := range tests {
		if got := pct(tc.in); got != tc.want {
			t.Errorf("pct(%v) = %q, quero %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatScore(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "0"}, {49.6, "50"}, {50, "50"}, {99.9, "100"},
	}
	for _, tc := range tests {
		if got := formatScore(tc.in); got != tc.want {
			t.Errorf("formatScore(%v) = %q, quero %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatAvg(t *testing.T) {
	if got, want := formatAvg(1.5), "1.5"; got != want {
		t.Errorf("formatAvg = %q, quero %q", got, want)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"curto", 10, "curto"},
		{"Integrar sistema com api", 12, "Integrar si…"},
		{"exato", 5, "exato"},
		// O "…" conta para o limite: 6 runas no total, não 6 mais o sinal.
		{"acentuação preservada", 6, "acent…"},
	}
	for _, tc := range tests {
		if got := truncate(tc.in, tc.max); got != tc.want {
			t.Errorf("truncate(%q, %d) = %q, quero %q", tc.in, tc.max, got, tc.want)
		}
	}
	// Corte em texto com acentos não pode partir um caractere ao meio.
	if got := truncate("ação", 3); len([]rune(got)) != 3 {
		t.Errorf("truncate com acentos = %q (%d runas), quero 3 runas", got, len([]rune(got)))
	}
}
