package ui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Formatos aceitos no campo de data limite, do mais completo ao mais simples.
// O usuário pode digitar só a data e o app assume o fim do dia como prazo.
var dateLayouts = []string{
	"02/01/2006 15:04",
	"02/01/2006 15h04",
	"02/01/2006",
	"2006-01-02 15:04",
	"2006-01-02",
}

// ErrEmptyDate indica campo de data vazio — não é erro de digitação, é ausência.
var ErrEmptyDate = errors.New("data vazia")

// parseDateTime interpreta o que o usuário digitou no campo de prazo.
// Aceita "05/02/2026", "05/02/2026 18:30" e também o formato ISO, e tolera
// a hora escrita antes da data ("18:30 05/02/2026"), que é como o mockup
// exibe os prazos.
func parseDateTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, ErrEmptyDate
	}

	// "18:30 05/02/2026" → reordena para o formato canônico.
	if parts := strings.Fields(s); len(parts) == 2 && strings.Contains(parts[0], ":") && strings.Contains(parts[1], "/") {
		s = parts[1] + " " + parts[0]
	}

	for _, layout := range dateLayouts {
		t, err := time.ParseInLocation(layout, s, time.Local)
		if err != nil {
			continue
		}
		// Só a data, sem hora: o prazo é o fim do dia, senão uma tarefa para
		// hoje já nasceria vencida à meia-noite.
		if !strings.ContainsAny(s, ":h") {
			t = t.Add(23*time.Hour + 59*time.Minute)
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("data inválida: use dd/mm/aaaa ou dd/mm/aaaa hh:mm")
}

// formatDateTime devolve "18:30 05/02/2026", como na lista de tarefas.
func formatDateTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("15:04 02/01/2006")
}

// formatDate devolve só "05/02/2026".
func formatDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("02/01/2006")
}

// formatDateInput devolve a data no formato que o campo de edição espera.
func formatDateInput(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("02/01/2006 15:04")
}

// parseDurationInput interpreta o tempo médio digitado na configuração.
// Aceita "8" e "8h" (horas), "8h30" e "8:30" (horas e minutos), "7,5" e "7.5"
// (horas fracionadas) e "90min" (minutos).
func parseDurationInput(s string) (time.Duration, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", ".")
	if s == "" {
		return 0, errors.New("informe um tempo")
	}

	// "90min" / "90m"
	if rest, ok := trimSuffixAny(s, "min", "m"); ok && !strings.Contains(rest, "h") {
		min, err := strconv.ParseFloat(rest, 64)
		if err != nil || min < 0 {
			return 0, errInvalidDuration()
		}
		return time.Duration(min * float64(time.Minute)), nil
	}

	// "8h30", "8:30", "8h"
	sep := strings.IndexAny(s, "h:")
	if sep >= 0 {
		hStr, mStr := s[:sep], strings.TrimSuffix(s[sep+1:], "min")
		hours, err := strconv.ParseFloat(hStr, 64)
		if err != nil || hours < 0 {
			return 0, errInvalidDuration()
		}
		if mStr == "" {
			return time.Duration(hours * float64(time.Hour)), nil
		}
		mins, err := strconv.ParseFloat(mStr, 64)
		if err != nil || mins < 0 || mins >= 60 {
			return 0, errInvalidDuration()
		}
		return time.Duration(hours*float64(time.Hour)) + time.Duration(mins*float64(time.Minute)), nil
	}

	// Número puro: horas.
	hours, err := strconv.ParseFloat(s, 64)
	if err != nil || hours < 0 {
		return 0, errInvalidDuration()
	}
	return time.Duration(hours * float64(time.Hour)), nil
}

func errInvalidDuration() error {
	return errors.New("tempo inválido: use 8, 8h30 ou 90min")
}

// trimSuffixAny remove o primeiro sufixo que casar e informa se removeu algum.
func trimSuffixAny(s string, suffixes ...string) (string, bool) {
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) && len(s) > len(suf) {
			return strings.TrimSuffix(s, suf), true
		}
	}
	return s, false
}

// formatDurationInput devolve a duração como o campo de configuração a exibe.
func formatDurationInput(d time.Duration) string {
	if d <= 0 {
		return "0h"
	}
	total := int(d.Minutes())
	h, m := total/60, total%60
	if m == 0 {
		return strconv.Itoa(h) + "h"
	}
	return fmt.Sprintf("%dh%02d", h, m)
}

// parsePriorityValue lê o peso de uma prioridade. Pesos precisam ser inteiros
// positivos: zero deixaria a tarefa invisível para o score.
func parsePriorityValue(s string) (int, error) {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, errors.New("o valor precisa ser um número inteiro")
	}
	if v < 1 {
		return 0, errors.New("o valor precisa ser 1 ou maior")
	}
	if v > 999 {
		return 0, errors.New("o valor máximo é 999")
	}
	return v, nil
}

// pct formata uma fração de 0 a 1 como porcentagem inteira.
func pct(f float64) string {
	return strconv.Itoa(int(f*100+0.5)) + "%"
}

// formatScore arredonda o score para exibição.
func formatScore(v float64) string {
	return strconv.Itoa(int(v + 0.5))
}

// formatAvg formata uma média de tarefas com uma casa decimal.
func formatAvg(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// truncate corta um texto comprido para caber na coluna, sem quebrar no meio
// de forma feia.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return strings.TrimRight(string(r[:max-1]), " ") + "…"
}
