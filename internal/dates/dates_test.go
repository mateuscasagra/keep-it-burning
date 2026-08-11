package dates

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day, hour int) time.Time {
	return time.Date(y, m, day, hour, 0, 0, 0, time.Local)
}

func TestStartOfDay(t *testing.T) {
	got := StartOfDay(d(2026, time.February, 5, 17))
	want := d(2026, time.February, 5, 0)
	if !got.Equal(want) {
		t.Fatalf("StartOfDay = %v, quero %v", got, want)
	}
}

func TestEndOfDay(t *testing.T) {
	got := EndOfDay(d(2026, time.February, 5, 17))
	want := d(2026, time.February, 6, 0)
	if !got.Equal(want) {
		t.Fatalf("EndOfDay = %v, quero %v", got, want)
	}
}

func TestStartOfWeekComecaNaSegunda(t *testing.T) {
	// 05/02/2026 é uma quinta-feira; a segunda da semana é 02/02/2026.
	tests := []struct {
		nome string
		in   time.Time
		want time.Time
	}{
		{"quinta", d(2026, time.February, 5, 10), d(2026, time.February, 2, 0)},
		{"a propria segunda", d(2026, time.February, 2, 23), d(2026, time.February, 2, 0)},
		{"domingo fecha a semana", d(2026, time.February, 8, 12), d(2026, time.February, 2, 0)},
		{"segunda seguinte vira semana nova", d(2026, time.February, 9, 1), d(2026, time.February, 9, 0)},
	}
	for _, tc := range tests {
		t.Run(tc.nome, func(t *testing.T) {
			if got := StartOfWeek(tc.in); !got.Equal(tc.want) {
				t.Fatalf("StartOfWeek(%v) = %v, quero %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestEndOfWeek(t *testing.T) {
	got := EndOfWeek(d(2026, time.February, 5, 10))
	want := d(2026, time.February, 9, 0)
	if !got.Equal(want) {
		t.Fatalf("EndOfWeek = %v, quero %v", got, want)
	}
}

func TestMesesNaViradaDoAno(t *testing.T) {
	if got, want := StartOfMonth(d(2026, time.December, 20, 8)), d(2026, time.December, 1, 0); !got.Equal(want) {
		t.Fatalf("StartOfMonth = %v, quero %v", got, want)
	}
	if got, want := EndOfMonth(d(2026, time.December, 20, 8)), d(2027, time.January, 1, 0); !got.Equal(want) {
		t.Fatalf("EndOfMonth = %v, quero %v", got, want)
	}
}

func TestSameDay(t *testing.T) {
	if !SameDay(d(2026, time.February, 5, 0), d(2026, time.February, 5, 23)) {
		t.Fatal("mesmo dia em horas diferentes deveria bater")
	}
	if SameDay(d(2026, time.February, 5, 23), d(2026, time.February, 6, 0)) {
		t.Fatal("dias diferentes nao deveriam bater")
	}
}

func TestInRangeEhSemiAberto(t *testing.T) {
	from, to := d(2026, time.February, 2, 0), d(2026, time.February, 9, 0)
	if !InRange(from, from, to) {
		t.Fatal("o inicio faz parte do intervalo")
	}
	if InRange(to, from, to) {
		t.Fatal("o fim nao faz parte do intervalo")
	}
	if InRange(d(2026, time.February, 1, 23), from, to) {
		t.Fatal("antes do inicio nao faz parte")
	}
}

func TestDaysBetween(t *testing.T) {
	tests := []struct {
		nome     string
		from, to time.Time
		want     int
	}{
		{"uma semana", d(2026, time.February, 2, 0), d(2026, time.February, 9, 0), 7},
		{"mesmo dia", d(2026, time.February, 2, 0), d(2026, time.February, 3, 0), 1},
		{"intervalo vazio", d(2026, time.February, 2, 0), d(2026, time.February, 2, 0), 0},
		{"invertido nao é negativo", d(2026, time.February, 9, 0), d(2026, time.February, 2, 0), 0},
		{"fevereiro de 2026 tem 28 dias", d(2026, time.February, 1, 0), d(2026, time.March, 1, 0), 28},
	}
	for _, tc := range tests {
		t.Run(tc.nome, func(t *testing.T) {
			if got := DaysBetween(tc.from, tc.to); got != tc.want {
				t.Fatalf("DaysBetween = %d, quero %d", got, tc.want)
			}
		})
	}
}
