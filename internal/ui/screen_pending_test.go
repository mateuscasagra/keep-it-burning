package ui

import (
	"testing"
	"time"

	"github.com/dvet/keep-it-burning/internal/model"
)

func TestParseDoneDate(t *testing.T) {
	now := time.Date(2026, 9, 16, 14, 30, 0, 0, time.Local)
	at := func(d, h, m int) time.Time {
		return time.Date(2026, 9, d, h, m, 0, 0, time.Local)
	}

	tests := []struct {
		name string
		in   string
		want time.Time
		err  bool
	}{
		{"data e hora", "15/09/2026 09:20", at(15, 9, 20), false},
		// Só a data de um dia passado: vale o fim daquele dia.
		{"dia anterior sem hora", "15/09/2026", at(15, 23, 59), false},
		// Hoje sem hora viraria 23:59, que ainda não chegou: vale agora.
		{"hoje sem hora", "16/09/2026", now, false},
		{"hoje mais cedo", "16/09/2026 08:00", at(16, 8, 0), false},
		{"campo vazio", "", time.Time{}, true},
		{"data inválida", "32/13/2026", time.Time{}, true},
		{"no futuro", "17/09/2026 10:00", time.Time{}, true},
	}
	for _, tc := range tests {
		got, err := parseDoneDate(tc.in, now)
		if tc.err {
			if err == nil {
				t.Errorf("%s: parseDoneDate(%q) devia falhar, devolveu %v", tc.name, tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: parseDoneDate(%q) falhou: %v", tc.name, tc.in, err)
			continue
		}
		if !got.Equal(tc.want) {
			t.Errorf("%s: parseDoneDate(%q) = %v, quero %v", tc.name, tc.in, got, tc.want)
		}
	}
}

// Com todas as colunas na tela os pesos precisam somar 1; é assim que a grade
// fica previsível. Omitir uma opcional redistribui o peso dela, o que o Gio faz
// sozinho — mas só funciona se a base for 1.
func TestTaskColumnsSumToOne(t *testing.T) {
	sum := colTitle + colCategory + colSub + colDiff + colCreated + colDue + colPriority
	if diff := sum - 1; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("soma das colunas = %v, quero 1", sum)
	}
}

// As colunas opcionais acompanham o que o modo tem configurado: sem categorias
// cadastradas, a coluna de categoria não deve roubar espaço das outras.
func TestTaskColsFollowSettings(t *testing.T) {
	empty := taskColsFor(model.ModeSettings{})
	if empty.cat || empty.sub || empty.diff {
		t.Errorf("modo sem listas configuradas não devia mostrar colunas opcionais: %+v", empty)
	}
	full := taskColsFor(model.ModeSettings{
		Categories:    []model.Category{{ID: "c"}},
		Subcategories: []model.Subcategory{{ID: "s"}},
		Difficulties:  model.DefaultDifficulties(),
	})
	if !full.cat || !full.sub || !full.diff {
		t.Errorf("modo com as três listas devia mostrar as três colunas: %+v", full)
	}
}
