package ui

import (
	"image"
	"testing"
	"time"

	"github.com/dvet/keep-it-burning/internal/dates"
	"github.com/dvet/keep-it-burning/internal/model"
	"github.com/dvet/keep-it-burning/internal/productivity"
)

// newStatsHarness abre o relatório na visão de tarefas, com subs subcategorias
// configuradas e dones tarefas concluídas hoje.
func newStatsHarness(t *testing.T, subs, dones int) *formHarness {
	t.Helper()
	h := newFormHarness(t, subs)
	hoje := dates.StartOfDay(time.Now())
	for i := 1; i <= dones; i++ {
		h.a.state.Tasks = append(h.a.state.Tasks, model.Task{
			ID: "d" + itoa(i), Mode: model.ModeWork, Title: "Concluída " + itoa(i),
			PriorityID: "alta", CreatedAt: hoje,
			Done: true, DoneAt: hoje.Add(time.Duration(8+i) * time.Hour),
		})
	}
	h.a.stats.init(h.a)
	h.a.stats.view = 1
	h.a.screen = screenStats
	h.frame()
	return h
}

// A lista de concluídas precisa continuar alcançável mesmo quando as tabelas do
// topo ficam altas. Com o resumo fixo no topo do painel, um modo com muitas
// subcategorias empurrava a lista para fora e ela aparecia vazia.
func TestRelatorioAlcancaAsConcluidasComMuitasSubcategorias(t *testing.T) {
	const dones = 4
	h := newStatsHarness(t, 9, dones)

	// O dado precisa estar lá: se o relatório não enxerga as tarefas, o
	// problema é outro e o teste abaixo não diria nada.
	from, to := productivity.PeriodDay.Range(time.Now())
	if n := len(productivity.DoneTasksIn(h.a.state, model.ModeWork, from, to)); n != dones {
		t.Fatalf("o relatório enxergou %d concluídas, esperava %d", n, dones)
	}

	meio := image.Pt(fullSize.X/2, fullSize.Y/2)
	for i := 0; i < 20 && h.a.stats.doneList.Position.BeforeEnd; i++ {
		h.scroll(meio, 400)
	}

	pos := h.a.stats.doneList.Position
	if pos.BeforeEnd {
		t.Fatal("a lista do relatório não chegou ao fim nem depois de rolar")
	}
	// A lista tem o resumo mais uma linha por tarefa concluída; chegar ao fim
	// significa que as linhas das tarefas foram mesmo desenhadas.
	if got, want := pos.First+pos.Count, dones+1; got != want {
		t.Errorf("a lista terminou no item %d de %d — as concluídas ficaram de fora", got, want)
	}
}
