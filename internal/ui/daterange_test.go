package ui

import (
	"testing"
	"time"

	"github.com/dvet/keep-it-burning/internal/model"
)

func day(d int) time.Time { return time.Date(2026, time.September, d, 10, 0, 0, 0, time.Local) }

func TestMatchRange(t *testing.T) {
	tests := []struct {
		name     string
		from, to string
		when     time.Time
		want     bool
	}{
		{"sem intervalo aceita tudo", "", "", day(5), true},
		{"sem intervalo aceita até data vazia", "", "", time.Time{}, true},
		{"dentro do intervalo", "10/09/2026", "20/09/2026", day(15), true},
		{"antes do começo", "10/09/2026", "20/09/2026", day(9), false},
		{"depois do fim", "10/09/2026", "20/09/2026", day(21), false},
		// O dia digitado entra inteiro: 20/09 às 10h ainda é 20/09.
		{"último dia entra inteiro", "10/09/2026", "20/09/2026", day(20), true},
		{"primeiro dia entra inteiro", "10/09/2026", "", day(10), true},
		{"só o começo", "15/09/2026", "", day(30), true},
		{"só o fim", "", "15/09/2026", day(30), false},
		// Data pela metade não restringe: a lista não pode piscar a cada tecla.
		{"data incompleta não filtra", "15/09", "", day(1), true},
		// Tarefa sem a data em questão não cai em intervalo nenhum.
		{"sem data fica de fora", "10/09/2026", "", time.Time{}, false},
	}
	for _, tc := range tests {
		if got := matchRange(tc.from, tc.to, tc.when); got != tc.want {
			t.Errorf("%s: matchRange(%q, %q, %v) = %v, quero %v",
				tc.name, tc.from, tc.to, tc.when.Format("02/01/2006"), got, tc.want)
		}
	}
}

// O filtro da tela precisa usar a data certa em cada intervalo: um pela
// inclusão, outro pelo prazo.
func TestPendingFiltraPorIntervaloDeDatas(t *testing.T) {
	var s pendingScreen
	s.init(nil)

	tasks := []model.Task{
		{ID: "antiga", Title: "Antiga", CreatedAt: day(1), DueAt: day(30)},
		{ID: "nova", Title: "Nova", CreatedAt: day(20), DueAt: day(25)},
		{ID: "sem-prazo", Title: "Sem prazo", CreatedAt: day(20)},
	}

	if got := len(s.filter(tasks)); got != 3 {
		t.Fatalf("sem filtro devia passar tudo, passaram %d", got)
	}

	s.created.from.SetText("15/09/2026")
	got := s.filter(tasks)
	if len(got) != 2 || got[0].ID != "nova" {
		t.Errorf("filtro por inclusão devolveu %v", ids(got))
	}

	s.created.reset()
	s.dueRange.from.SetText("01/09/2026")
	s.dueRange.to.SetText("26/09/2026")
	got = s.filter(tasks)
	if len(got) != 1 || got[0].ID != "nova" {
		t.Errorf("filtro por prazo devolveu %v — a sem prazo não devia entrar", ids(got))
	}
}

func ids(tasks []model.Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.ID
	}
	return out
}
