package ui

import (
	"testing"

	"github.com/dvet/keep-it-burning/internal/model"
)

// stateComSubs monta um modo com duas subcategorias configuradas.
func stateComSubs() *model.State {
	st := model.NewState()
	cfg := st.SettingsFor(model.ModeWork)
	cfg.Subcategories = []model.Subcategory{
		{ID: "front", Title: "Front-end"},
		{ID: "infra", Title: "Infra"},
	}
	st.SetSettings(model.ModeWork, cfg)
	return st
}

func task(id, sub string, done bool) model.Task {
	return model.Task{ID: id, Mode: model.ModeWork, Title: id, SubcategoryID: sub, Today: true, Done: done}
}

// resumo descreve a lista montada, para os erros dizerem o que apareceu.
func resumo(entries []focusEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		if e.isHeader {
			out[i] = "# " + e.header + " " + itoa(e.done) + "/" + itoa(e.total)
			continue
		}
		out[i] = e.task.ID
	}
	return out
}

func TestGroupBySubcategoriaSeparaEContaOsGrupos(t *testing.T) {
	st := stateComSubs()
	tasks := []model.Task{
		task("a", "front", true),
		task("b", "infra", false),
		task("c", "", false),
		task("d", "front", false),
	}

	got := resumo(groupBySubcategory(st, model.ModeWork, tasks))
	// A ordem dos grupos é a da configuração; o "sem subcategoria" fica por
	// último. Dentro do grupo a ordem recebida é preservada.
	want := []string{
		"# Front-end 1/2", "a", "d",
		"# Infra 0/1", "b",
		"# " + semSub + " 0/1", "c",
	}
	if len(got) != len(want) {
		t.Fatalf("lista = %v, quero %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lista = %v, quero %v", got, want)
		}
	}
}

func TestGroupBySubcategoriaNaoCriaCabecalhoInutil(t *testing.T) {
	st := stateComSubs()
	// Todas na mesma subcategoria: um cabeçalho só não separa nada.
	umGrupo := []model.Task{task("a", "front", false), task("b", "front", false)}
	if got := resumo(groupBySubcategory(st, model.ModeWork, umGrupo)); len(got) != 2 {
		t.Errorf("com um grupo só a lista devia ser plana, veio %v", got)
	}
	// Nenhuma classificada, que é o caso de quem não usa subcategorias.
	semNada := []model.Task{task("a", "", false), task("b", "", false)}
	if got := resumo(groupBySubcategory(st, model.ModeWork, semNada)); len(got) != 2 {
		t.Errorf("sem subcategoria nenhuma a lista devia ser plana, veio %v", got)
	}
}

func TestGroupBySubcategoriaAcolheSubcategoriaApagada(t *testing.T) {
	st := stateComSubs()
	// A tarefa aponta para uma subcategoria que não existe mais na
	// configuração: ela precisa cair no grupo "sem", e não sumir da tela.
	tasks := []model.Task{task("a", "front", false), task("b", "apagada", false)}

	got := resumo(groupBySubcategory(st, model.ModeWork, tasks))
	want := []string{"# Front-end 0/1", "a", "# " + semSub + " 0/1", "b"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("lista = %v, quero %v", got, want)
		}
	}
}
