package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dvet/keep-it-burning/internal/model"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	return New(filepath.Join(t.TempDir(), "dados", FileName))
}

func TestLoadArquivoInexistenteDaEstadoNovo(t *testing.T) {
	st, err := tempStore(t).Load()
	if err != nil {
		t.Fatalf("o primeiro uso do app não é erro: %v", err)
	}
	if len(st.Tasks) != 0 {
		t.Errorf("estado novo deveria vir sem tarefas, tenho %d", len(st.Tasks))
	}
	for _, m := range model.Modes {
		if len(st.SettingsFor(m).Priorities) == 0 {
			t.Errorf("modo %s deveria vir com prioridades padrão", m)
		}
	}
}

func TestSaveELoadPreservamOEstado(t *testing.T) {
	s := tempStore(t)

	original := model.NewState()
	due := time.Date(2026, time.February, 6, 7, 0, 0, 0, time.Local)
	created := time.Date(2026, time.February, 5, 10, 0, 0, 0, time.Local)
	original.AddTask(model.Task{
		ID: "t1", Mode: model.ModeWork, Title: "Integrar sistema com api",
		Description: "Subir o endpoint novo", PriorityID: "alta",
		CreatedAt: created, DueAt: due, Today: true,
	})
	original.AddSession(model.Session{
		ID: "s1", Mode: model.ModeWork,
		Start: created, End: created.Add(90 * time.Minute),
	})
	original.SetSettings(model.ModeStudy, model.ModeSettings{
		Priorities:   []model.Priority{{ID: "p1", Title: "Prova", Value: 9}},
		DailyTarget:  3 * time.Hour,
		WeeklyTarget: 15 * time.Hour,
	})

	if err := s.Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	task, ok := loaded.Task("t1")
	if !ok {
		t.Fatal("a tarefa não voltou do disco")
	}
	if task.Title != "Integrar sistema com api" || task.Description != "Subir o endpoint novo" {
		t.Errorf("texto da tarefa não bate: %+v", task)
	}
	if !task.DueAt.Equal(due) || !task.CreatedAt.Equal(created) {
		t.Errorf("datas não bateram: criada %v, limite %v", task.CreatedAt, task.DueAt)
	}
	if !task.Today {
		t.Error("a marca de tarefa do dia se perdeu")
	}
	if len(loaded.Sessions) != 1 || loaded.Sessions[0].Duration() != 90*time.Minute {
		t.Errorf("sessões não bateram: %+v", loaded.Sessions)
	}
	cfg := loaded.SettingsFor(model.ModeStudy)
	if len(cfg.Priorities) != 1 || cfg.Priorities[0].Title != "Prova" || cfg.Priorities[0].Value != 9 {
		t.Errorf("prioridades personalizadas não voltaram: %+v", cfg.Priorities)
	}
	if cfg.DailyTarget != 3*time.Hour || cfg.WeeklyTarget != 15*time.Hour {
		t.Errorf("metas não bateram: %v / %v", cfg.DailyTarget, cfg.WeeklyTarget)
	}
}

func TestSaveCriaODiretorio(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c", FileName)
	if err := New(path).Save(model.NewState()); err != nil {
		t.Fatalf("Save deveria criar a árvore de diretórios: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("arquivo não foi criado: %v", err)
	}
}

func TestSaveSobrescreveSemDeixarLixo(t *testing.T) {
	s := tempStore(t)

	st := model.NewState()
	st.AddTask(model.Task{ID: "t1", Mode: model.ModeWork, Title: "Primeira"})
	if err := s.Save(st); err != nil {
		t.Fatalf("primeiro Save: %v", err)
	}

	st.DeleteTask("t1")
	st.AddTask(model.Task{ID: "t2", Mode: model.ModeWork, Title: "Segunda"})
	if err := s.Save(st); err != nil {
		t.Fatalf("segundo Save: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := loaded.Task("t1"); ok {
		t.Error("a tarefa apagada voltou")
	}
	if _, ok := loaded.Task("t2"); !ok {
		t.Error("a tarefa nova não foi gravada")
	}

	// Nenhum arquivo temporário pode sobrar ao lado do arquivo de dados.
	entries, err := os.ReadDir(filepath.Dir(s.Path()))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != FileName {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("diretório deveria conter só %s, tenho %v", FileName, names)
	}
}

func TestLoadJSONInvalidoDaErro(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("{isso nao e json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(path).Load(); err == nil {
		t.Fatal("JSON quebrado deveria devolver erro em vez de estado silenciosamente vazio")
	}
}

func TestLoadNormalizaEstadoAntigo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	// Arquivo de uma versão antiga: sem sessões e sem configurações.
	antigo := `{"tasks":[{"id":"t1","mode":"trabalho","title":"Velha","done":true,"createdAt":"2026-02-05T10:00:00-03:00"}]}`
	if err := os.WriteFile(path, []byte(antigo), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := New(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, m := range model.Modes {
		cfg := st.SettingsFor(m)
		if len(cfg.Priorities) == 0 || cfg.DailyTarget <= 0 {
			t.Errorf("modo %s deveria ganhar configuração padrão, tenho %+v", m, cfg)
		}
	}
	if task, _ := st.Task("t1"); task.DoneAt.IsZero() {
		t.Error("tarefa concluída sem DoneAt deveria ser normalizada")
	}
}

func TestSaveEstadoNuloDaErro(t *testing.T) {
	if err := tempStore(t).Save(nil); err == nil {
		t.Fatal("Save(nil) deveria falhar")
	}
}

func TestSaveEmCaminhoImpossivelDaErro(t *testing.T) {
	// O caminho aponta para dentro de um arquivo comum, então nem o diretório
	// pode ser criado. O erro precisa chegar ao chamador para o app conseguir
	// avisar o usuário em vez de perder as tarefas em silêncio.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "bloqueio")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := New(filepath.Join(blocker, "sub", FileName))
	if err := s.Save(model.NewState()); err == nil {
		t.Fatal("Save deveria falhar quando o diretório não pode ser criado")
	}
}

func TestLoadDeDiretorioDaErro(t *testing.T) {
	// Um diretório no lugar do arquivo não é "primeiro uso": é configuração
	// quebrada, e precisa aparecer como erro.
	dir := t.TempDir()
	if _, err := New(dir).Load(); err == nil {
		t.Fatal("Load de um diretório deveria falhar")
	}
}

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if filepath.Base(path) != FileName {
		t.Errorf("o caminho deveria terminar em %s, tenho %s", FileName, path)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("o caminho deveria ser absoluto, tenho %s", path)
	}
}
