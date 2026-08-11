// Package dates concentra o cálculo de fronteiras de dia, semana e mês.
// Tudo é feito no fuso horário local do usuário, que é o que ele enxerga
// quando olha para o relógio da máquina.
package dates

import "time"

// StartOfDay devolve a meia-noite do dia de t.
func StartOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// EndOfDay devolve a meia-noite do dia seguinte ao de t (limite exclusivo).
func EndOfDay(t time.Time) time.Time {
	return StartOfDay(t).AddDate(0, 0, 1)
}

// StartOfWeek devolve a segunda-feira da semana de t. A semana brasileira de
// trabalho começa na segunda, então é essa a convenção usada no app inteiro.
func StartOfWeek(t time.Time) time.Time {
	day := StartOfDay(t)
	// time.Weekday() vai de domingo (0) a sábado (6); queremos segunda como 0.
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}

// EndOfWeek devolve a segunda-feira seguinte (limite exclusivo).
func EndOfWeek(t time.Time) time.Time {
	return StartOfWeek(t).AddDate(0, 0, 7)
}

// StartOfMonth devolve o primeiro instante do mês de t.
func StartOfMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}

// EndOfMonth devolve o primeiro instante do mês seguinte (limite exclusivo).
func EndOfMonth(t time.Time) time.Time {
	return StartOfMonth(t).AddDate(0, 1, 0)
}

// SameDay informa se a e b caem no mesmo dia do calendário.
func SameDay(a, b time.Time) bool {
	return StartOfDay(a).Equal(StartOfDay(b))
}

// InRange informa se t está em [from, to).
func InRange(t, from, to time.Time) bool {
	return !t.Before(from) && t.Before(to)
}

// DaysBetween conta os dias de calendário em [from, to). Nunca devolve negativo.
func DaysBetween(from, to time.Time) int {
	if !to.After(from) {
		return 0
	}
	days := 0
	for d := StartOfDay(from); d.Before(to); d = d.AddDate(0, 0, 1) {
		days++
	}
	return days
}
