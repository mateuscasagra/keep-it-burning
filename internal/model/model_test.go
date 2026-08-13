package model

import (
	"errors"
	"testing"
	"time"
)

func at(day, hour int) time.Time {
	return time.Date(2026, time.February, day, hour, 0, 0, 0, time.Local)
}

func TestNewStateTemOsDoisModosConfigurados(t *testing.T) {
	s := NewState()
	for _, m := range Modes {
		cfg := s.SettingsFor(m)
		if len(cfg.Priorities) != 3 {
			t.Errorf("modo %s: quero 3 prioridades padrão, tenho %d", m, len(cfg.Priorities))
		}
		if cfg.DailyTarget <= 0 || cfg.WeeklyTarget <= 0 {
			t.Errorf("modo %s: metas de tempo precisam ser positivas", m)
		}
	}
}

func TestAddTaskRecusaTituloVazio(t *testing.T) {
	s := NewState()
	if _, err := s.AddTask(Task{Mode: ModeWork, Title: "   "}); !errors.Is(err, ErrEmptyTitle) {
		t.Fatalf("quero ErrEmptyTitle, tenho %v", err)
	}
	if len(s.Tasks) != 0 {
		t.Fatal("tarefa inválida não pode ser gravada")
	}
}

func TestAddTaskPreencheIDEData(t *testing.T) {
	s := NewState()
	got, err := s.AddTask(Task{Mode: ModeWork, Title: "Integrar sistema com api", PriorityID: "alta"})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got.ID == "" {
		t.Error("o ID deveria ser gerado")
	}
	if got.CreatedAt.IsZero() {
		t.Error("a data de criação deveria ser preenchida")
	}
	if len(s.Tasks) != 1 {
		t.Fatalf("quero 1 tarefa no estado, tenho %d", len(s.Tasks))
	}
}

func TestUpdateTaskPreservaDataDeCriacao(t *testing.T) {
	s := NewState()
	orig, _ := s.AddTask(Task{
		ID: "t1", Mode: ModeWork, Title: "Original", PriorityID: "baixa", CreatedAt: at(5, 10),
	})

	edit := orig
	edit.Title = "Editada"
	edit.CreatedAt = at(20, 10) // tentativa de reescrever o histórico
	if err := s.UpdateTask(edit); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	saved, _ := s.Task("t1")
	if saved.Title != "Editada" {
		t.Errorf("título = %q, quero %q", saved.Title, "Editada")
	}
	if !saved.CreatedAt.Equal(at(5, 10)) {
		t.Errorf("data de criação = %v, ela não pode mudar", saved.CreatedAt)
	}
}

func TestUpdateTaskInexistente(t *testing.T) {
	s := NewState()
	if err := s.UpdateTask(Task{ID: "fantasma", Title: "x"}); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("quero ErrTaskNotFound, tenho %v", err)
	}
}

func TestDeleteTask(t *testing.T) {
	s := NewState()
	s.AddTask(Task{ID: "a", Mode: ModeWork, Title: "A"})
	s.AddTask(Task{ID: "b", Mode: ModeWork, Title: "B"})

	if err := s.DeleteTask("a"); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if _, ok := s.Task("a"); ok {
		t.Error("a tarefa deveria ter sumido")
	}
	if _, ok := s.Task("b"); !ok {
		t.Error("a outra tarefa não deveria ser afetada")
	}
	if err := s.DeleteTask("a"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("apagar duas vezes deveria dar ErrTaskNotFound, tenho %v", err)
	}
}

func TestSetDoneGravaEApagaOCarimbo(t *testing.T) {
	s := NewState()
	s.AddTask(Task{ID: "t1", Mode: ModeWork, Title: "Resolver bug"})

	if err := s.SetDone("t1", true, at(5, 14)); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	got, _ := s.Task("t1")
	if !got.Done || !got.DoneAt.Equal(at(5, 14)) {
		t.Fatalf("concluir deveria gravar Done e DoneAt, tenho %+v", got)
	}

	// Desmarcar precisa limpar o carimbo, senão o relatório do dia continuaria
	// contando a tarefa como entregue.
	if err := s.SetDone("t1", false, at(5, 15)); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	got, _ = s.Task("t1")
	if got.Done || !got.DoneAt.IsZero() {
		t.Fatalf("desmarcar deveria zerar Done e DoneAt, tenho %+v", got)
	}
}

func TestSetToday(t *testing.T) {
	s := NewState()
	s.AddTask(Task{ID: "t1", Mode: ModeWork, Title: "Resolver bug"})
	if err := s.SetToday("t1", true, time.Now()); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got, _ := s.Task("t1"); !got.Today {
		t.Error("a tarefa deveria estar marcada como tarefa do dia")
	}
}

func TestAddSessionIgnoraDuracaoZero(t *testing.T) {
	s := NewState()
	s.AddSession(Session{Mode: ModeWork, Start: at(5, 9), End: at(5, 9)})
	s.AddSession(Session{Mode: ModeWork, Start: at(5, 12), End: at(5, 9)}) // invertida
	if len(s.Sessions) != 0 {
		t.Fatalf("sessões vazias ou invertidas não entram, tenho %d", len(s.Sessions))
	}

	s.AddSession(Session{Mode: ModeWork, Start: at(5, 9), End: at(5, 11)})
	if len(s.Sessions) != 1 {
		t.Fatalf("quero 1 sessão, tenho %d", len(s.Sessions))
	}
	if s.Sessions[0].ID == "" {
		t.Error("a sessão deveria receber um ID")
	}
	if got := s.Sessions[0].Duration(); got != 2*time.Hour {
		t.Errorf("duração = %v, quero 2h", got)
	}
}

func TestPriorityValueDePrioridadeRemovida(t *testing.T) {
	s := NewState()
	if got := s.PriorityValue(ModeWork, "alta"); got != 5 {
		t.Errorf("peso de alta = %d, quero 5", got)
	}
	// Prioridade que o usuário apagou da configuração mas que tarefas antigas
	// ainda referenciam: precisa valer zero em vez de quebrar.
	if got := s.PriorityValue(ModeWork, "inexistente"); got != 0 {
		t.Errorf("peso de prioridade removida = %d, quero 0", got)
	}
}

func TestPendingTasksOrdenaPorPesoEPrazo(t *testing.T) {
	s := NewState()
	s.AddTask(Task{ID: "baixa", Mode: ModeWork, Title: "Baixa", PriorityID: "baixa", DueAt: at(6, 7)})
	s.AddTask(Task{ID: "alta-tarde", Mode: ModeWork, Title: "Alta tarde", PriorityID: "alta", DueAt: at(9, 7)})
	s.AddTask(Task{ID: "alta-cedo", Mode: ModeWork, Title: "Alta cedo", PriorityID: "alta", DueAt: at(6, 7)})
	s.AddTask(Task{ID: "media", Mode: ModeWork, Title: "Média", PriorityID: "media"})
	s.AddTask(Task{ID: "outro-modo", Mode: ModeStudy, Title: "Estudo", PriorityID: "alta"})

	got := s.PendingTasks(ModeWork)
	want := []string{"alta-cedo", "alta-tarde", "media", "baixa"}
	if len(got) != len(want) {
		t.Fatalf("quero %d pendentes de trabalho, tenho %d", len(want), len(got))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("posição %d = %s, quero %s", i, got[i].ID, id)
		}
	}
}

func TestPendingTasksIgnoraConcluidas(t *testing.T) {
	s := NewState()
	s.AddTask(Task{ID: "feita", Mode: ModeWork, Title: "Feita", PriorityID: "alta", Done: true, DoneAt: at(5, 12)})
	s.AddTask(Task{ID: "aberta", Mode: ModeWork, Title: "Aberta", PriorityID: "baixa"})

	got := s.PendingTasks(ModeWork)
	if len(got) != 1 || got[0].ID != "aberta" {
		t.Fatalf("quero só a tarefa aberta, tenho %+v", got)
	}
}

func TestTodayTasksMostraConcluidasPorUltimo(t *testing.T) {
	s := NewState()
	s.AddTask(Task{ID: "feita", Mode: ModeWork, Title: "Feita", PriorityID: "alta", Today: true, Done: true, DoneAt: at(5, 12)})
	s.AddTask(Task{ID: "aberta", Mode: ModeWork, Title: "Aberta", PriorityID: "baixa", Today: true})
	s.AddTask(Task{ID: "fora", Mode: ModeWork, Title: "Fora", PriorityID: "alta"})

	got := s.TodayTasks(ModeWork, time.Now())
	if len(got) != 2 {
		t.Fatalf("quero 2 tarefas do dia, tenho %d", len(got))
	}
	if got[0].ID != "aberta" || got[1].ID != "feita" {
		t.Errorf("ordem = %s,%s; quero aberta antes de feita", got[0].ID, got[1].ID)
	}
}

func TestNormalizeRecuperaEstadoIncompleto(t *testing.T) {
	s := &State{
		Tasks: []Task{
			{ID: "sem-carimbo", Title: "Feita sem DoneAt", Done: true, CreatedAt: at(5, 9)},
			{ID: "aberta-com-carimbo", Title: "Aberta", Done: false, DoneAt: at(5, 9)},
		},
	}
	s.Normalize()

	if s.Sessions == nil || s.Settings == nil {
		t.Fatal("Normalize deveria preencher as coleções nulas")
	}
	for _, m := range Modes {
		if len(s.SettingsFor(m).Priorities) == 0 {
			t.Errorf("modo %s ficou sem prioridades", m)
		}
	}
	if got, _ := s.Task("sem-carimbo"); !got.DoneAt.Equal(at(5, 9)) {
		t.Errorf("tarefa concluída sem DoneAt deveria herdar CreatedAt, tenho %v", got.DoneAt)
	}
	if got, _ := s.Task("aberta-com-carimbo"); !got.DoneAt.IsZero() {
		t.Errorf("tarefa aberta não pode ter DoneAt, tenho %v", got.DoneAt)
	}
}

func TestNormalizeCorrigeMetasInvalidas(t *testing.T) {
	s := &State{Settings: map[Mode]ModeSettings{
		ModeWork: {Priorities: nil, DailyTarget: 0, WeeklyTarget: -1},
	}}
	s.Normalize()

	cfg := s.SettingsFor(ModeWork)
	if len(cfg.Priorities) == 0 {
		t.Error("prioridades vazias deveriam voltar ao padrão")
	}
	if cfg.DailyTarget <= 0 || cfg.WeeklyTarget <= 0 {
		t.Errorf("metas inválidas deveriam voltar ao padrão, tenho %v/%v", cfg.DailyTarget, cfg.WeeklyTarget)
	}
}

func TestTasksByModeSeparaOsDoisMundos(t *testing.T) {
	s := NewState()
	s.AddTask(Task{ID: "w1", Mode: ModeWork, Title: "Trabalho 1"})
	s.AddTask(Task{ID: "w2", Mode: ModeWork, Title: "Trabalho 2", Done: true, DoneAt: at(5, 12)})
	s.AddTask(Task{ID: "e1", Mode: ModeStudy, Title: "Estudo 1"})

	if got := s.TasksByMode(ModeWork); len(got) != 2 {
		t.Errorf("trabalho = %d tarefas, quero 2", len(got))
	}
	if got := s.TasksByMode(ModeStudy); len(got) != 1 || got[0].ID != "e1" {
		t.Errorf("estudo = %+v, quero só e1", got)
	}
}

func TestSetDoneESetTodayEmTarefaInexistente(t *testing.T) {
	s := NewState()
	if err := s.SetDone("fantasma", true, at(5, 12)); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("SetDone: quero ErrTaskNotFound, tenho %v", err)
	}
	if err := s.SetToday("fantasma", true, time.Now()); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("SetToday: quero ErrTaskNotFound, tenho %v", err)
	}
}

func TestPriorityEncontraEIgnora(t *testing.T) {
	s := NewState()
	p, ok := s.Priority(ModeWork, "media")
	if !ok || p.Title != "Média" || p.Value != 3 {
		t.Errorf("Priority(media) = %+v, %v; quero Média/3", p, ok)
	}
	if _, ok := s.Priority(ModeWork, "sumiu"); ok {
		t.Error("prioridade inexistente não deveria ser encontrada")
	}
}

func TestNewIDNaoRepete(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		id := NewID()
		if id == "" {
			t.Fatal("NewID devolveu string vazia")
		}
		if seen[id] {
			t.Fatalf("NewID repetiu o identificador %q", id)
		}
		seen[id] = true
	}
}

func TestOverdue(t *testing.T) {
	pendente := Task{DueAt: at(5, 12)}
	if !pendente.Overdue(at(6, 9)) {
		t.Error("tarefa pendente com prazo vencido está atrasada")
	}
	if pendente.Overdue(at(5, 9)) {
		t.Error("antes do prazo não está atrasada")
	}
	concluida := Task{DueAt: at(5, 12), Done: true}
	if concluida.Overdue(at(6, 9)) {
		t.Error("tarefa concluída nunca está atrasada")
	}
	semPrazo := Task{}
	if semPrazo.Overdue(at(6, 9)) {
		t.Error("tarefa sem prazo nunca está atrasada")
	}
}

func TestModeLabelEValid(t *testing.T) {
	if ModeWork.Label() != "Trabalho" || ModeStudy.Label() != "Estudo" {
		t.Error("rótulos dos modos errados")
	}
	if !ModeWork.Valid() || !ModeStudy.Valid() || Mode("outro").Valid() {
		t.Error("Valid deveria aceitar só trabalho e estudo")
	}
}
