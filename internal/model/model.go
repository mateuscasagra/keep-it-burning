// Package model define as entidades do Keep It Burning: tarefas, prioridades,
// sessões cronometradas e as configurações de cada modo (trabalho/estudo).
//
// O estado inteiro do app cabe em um único State, que é serializado em JSON
// pelo pacote store. Nada aqui depende de UI.
package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Mode separa o mundo de trabalho do mundo de estudo. Cada modo tem as próprias
// prioridades, metas de tempo e tarefas.
type Mode string

const (
	ModeWork  Mode = "trabalho"
	ModeStudy Mode = "estudo"
)

// Modes lista os modos válidos, na ordem em que aparecem na tela inicial.
var Modes = []Mode{ModeWork, ModeStudy}

// Label devolve o nome do modo como ele aparece na interface.
func (m Mode) Label() string {
	switch m {
	case ModeWork:
		return "Trabalho"
	case ModeStudy:
		return "Estudo"
	default:
		return string(m)
	}
}

// Valid informa se o modo é conhecido.
func (m Mode) Valid() bool {
	return m == ModeWork || m == ModeStudy
}

// Priority é um nível de prioridade configurável. Value é o peso: quanto maior,
// mais a tarefa pesa no score de produtividade e, por consequência, no fogo.
type Priority struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Value int    `json:"value"`
}

// Task é uma tarefa de trabalho ou estudo.
type Task struct {
	ID          string    `json:"id"`
	Mode        Mode      `json:"mode"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	PriorityID  string    `json:"priorityId"`
	CreatedAt   time.Time `json:"createdAt"`
	DueAt       time.Time `json:"dueAt"`
	// Today marca a tarefa como "tarefa do dia": ela aparece no painel lateral
	// do dashboard e na janela reduzida durante a sessão.
	Today   bool      `json:"today"`
	TodayAt time.Time `json:"todayat"`
	Done    bool      `json:"done"`
	DoneAt  time.Time `json:"doneAt"`
}

// HasDue informa se a tarefa tem data limite definida.
func (t Task) HasDue() bool { return !t.DueAt.IsZero() }

// Overdue informa se a tarefa está pendente e já passou da data limite.
func (t Task) Overdue(now time.Time) bool {
	return !t.Done && t.HasDue() && t.DueAt.Before(now)
}

// Session é um intervalo cronometrado de trabalho ou estudo. O tempo em pausa
// não vira sessão: cada trecho contínuo é gravado separadamente.
type Session struct {
	ID    string    `json:"id"`
	Mode  Mode      `json:"mode"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Duration devolve a duração da sessão, ou zero se ela ainda não terminou.
func (s Session) Duration() time.Duration {
	if s.End.Before(s.Start) {
		return 0
	}
	return s.End.Sub(s.Start)
}

// ModeSettings guarda as prioridades e as metas de tempo de um modo.
// DailyTarget e WeeklyTarget são o "tempo médio diário/semanal" da tela de
// configuração: são as metas contra as quais o tempo cronometrado é comparado.
type ModeSettings struct {
	Priorities   []Priority    `json:"priorities"`
	DailyTarget  time.Duration `json:"dailyTarget"`
	WeeklyTarget time.Duration `json:"weeklyTarget"`
}

// State é o estado completo e persistido do aplicativo.
type State struct {
	Tasks    []Task                `json:"tasks"`
	Sessions []Session             `json:"sessions"`
	Settings map[Mode]ModeSettings `json:"settings"`
}

// NewID gera um identificador aleatório para tarefas, prioridades e sessões.
func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand só falha em situações catastróficas; um fallback baseado
		// no relógio ainda mantém o app utilizável.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// DefaultPriorities devolve o conjunto inicial de prioridades de um modo novo.
func DefaultPriorities() []Priority {
	return []Priority{
		{ID: "alta", Title: "Alta", Value: 5},
		{ID: "media", Title: "Média", Value: 3},
		{ID: "baixa", Title: "Baixa", Value: 1},
	}
}

// NewState devolve um estado zerado com as configurações padrão dos dois modos.
func NewState() *State {
	return &State{
		Tasks:    []Task{},
		Sessions: []Session{},
		Settings: map[Mode]ModeSettings{
			ModeWork: {
				Priorities:   DefaultPriorities(),
				DailyTarget:  8 * time.Hour,
				WeeklyTarget: 40 * time.Hour,
			},
			ModeStudy: {
				Priorities:   DefaultPriorities(),
				DailyTarget:  2 * time.Hour,
				WeeklyTarget: 10 * time.Hour,
			},
		},
	}
}

// Normalize conserta um estado carregado de disco: preenche mapas nulos,
// recria configurações faltantes e descarta entradas inconsistentes. Isso
// mantém o app funcional mesmo se o JSON foi editado à mão ou vem de uma
// versão anterior.
func (s *State) Normalize() {
	if s.Tasks == nil {
		s.Tasks = []Task{}
	}
	if s.Sessions == nil {
		s.Sessions = []Session{}
	}
	if s.Settings == nil {
		s.Settings = map[Mode]ModeSettings{}
	}
	defaults := NewState()
	for _, m := range Modes {
		cfg, ok := s.Settings[m]
		if !ok {
			s.Settings[m] = defaults.Settings[m]
			continue
		}
		if len(cfg.Priorities) == 0 {
			cfg.Priorities = DefaultPriorities()
		}
		if cfg.DailyTarget <= 0 {
			cfg.DailyTarget = defaults.Settings[m].DailyTarget
		}
		if cfg.WeeklyTarget <= 0 {
			cfg.WeeklyTarget = defaults.Settings[m].WeeklyTarget
		}
		s.Settings[m] = cfg
	}
	// Uma tarefa marcada como concluída sem data de conclusão quebraria os
	// relatórios por período; nesse caso usamos a data de criação.
	for i := range s.Tasks {
		if s.Tasks[i].Done && s.Tasks[i].DoneAt.IsZero() {
			s.Tasks[i].DoneAt = s.Tasks[i].CreatedAt
		}
		if !s.Tasks[i].Done {
			s.Tasks[i].DoneAt = time.Time{}
		}
	}
}

// SettingsFor devolve as configurações do modo, criando o padrão se faltarem.
func (s *State) SettingsFor(m Mode) ModeSettings {
	if cfg, ok := s.Settings[m]; ok && len(cfg.Priorities) > 0 {
		return cfg
	}
	return NewState().Settings[m]
}

// SetSettings grava as configurações de um modo.
func (s *State) SetSettings(m Mode, cfg ModeSettings) {
	if s.Settings == nil {
		s.Settings = map[Mode]ModeSettings{}
	}
	s.Settings[m] = cfg
}

// Priority procura uma prioridade pelo ID dentro de um modo.
func (s *State) Priority(m Mode, id string) (Priority, bool) {
	for _, p := range s.SettingsFor(m).Priorities {
		if p.ID == id {
			return p, true
		}
	}
	return Priority{}, false
}

// PriorityValue devolve o peso de uma prioridade. Uma prioridade removida da
// configuração mas ainda referenciada por tarefas antigas vale zero.
func (s *State) PriorityValue(m Mode, id string) int {
	if p, ok := s.Priority(m, id); ok {
		return p.Value
	}
	return 0
}

// ErrTaskNotFound é devolvido quando um ID de tarefa não existe.
var ErrTaskNotFound = errors.New("tarefa não encontrada")

// ErrEmptyTitle é devolvido ao tentar salvar uma tarefa sem título.
var ErrEmptyTitle = errors.New("a tarefa precisa de um título")

// AddTask insere uma tarefa nova e devolve a versão gravada (com ID e data de
// criação preenchidos).
func (s *State) AddTask(t Task) (Task, error) {
	if strings.TrimSpace(t.Title) == "" {
		return Task{}, ErrEmptyTitle
	}
	if t.ID == "" {
		t.ID = NewID()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	if !t.Done {
		t.DoneAt = time.Time{}
	}
	s.Tasks = append(s.Tasks, t)
	return t, nil
}

// UpdateTask substitui os campos editáveis de uma tarefa existente.
func (s *State) UpdateTask(t Task) error {
	if strings.TrimSpace(t.Title) == "" {
		return ErrEmptyTitle
	}
	for i := range s.Tasks {
		if s.Tasks[i].ID != t.ID {
			continue
		}
		// A data de criação é imutável: ela é o histórico da tarefa.
		t.CreatedAt = s.Tasks[i].CreatedAt
		if !t.Done {
			t.DoneAt = time.Time{}
		}
		s.Tasks[i] = t
		return nil
	}
	return ErrTaskNotFound
}

// DeleteTask remove uma tarefa pelo ID.
func (s *State) DeleteTask(id string) error {
	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			s.Tasks = append(s.Tasks[:i], s.Tasks[i+1:]...)
			return nil
		}
	}
	return ErrTaskNotFound
}

// Task procura uma tarefa pelo ID.
func (s *State) Task(id string) (Task, bool) {
	for _, t := range s.Tasks {
		if t.ID == id {
			return t, true
		}
	}
	return Task{}, false
}

// SetDone marca ou desmarca a conclusão de uma tarefa, cuidando do carimbo de
// data que os relatórios usam para saber em que dia ela foi entregue.
func (s *State) SetDone(id string, done bool, at time.Time) error {
	for i := range s.Tasks {
		if s.Tasks[i].ID != id {
			continue
		}
		s.Tasks[i].Done = done
		if done {
			s.Tasks[i].DoneAt = at
		} else {
			s.Tasks[i].DoneAt = time.Time{}
		}
		return nil
	}
	return ErrTaskNotFound
}

// SetToday marca ou desmarca a tarefa como tarefa do dia.
func (s *State) SetToday(id string, today bool, at time.Time) error {
	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			s.Tasks[i].Today = today
			if !today {
				s.Tasks[i].TodayAt = time.Time{}
			} else {
				s.Tasks[i].TodayAt = at
			}
			return nil
		}
	}
	return ErrTaskNotFound
}

// AddSession grava uma sessão cronometrada. Sessões de duração zero são
// ignoradas: elas só poluiriam os relatórios.
func (s *State) AddSession(sess Session) {
	if sess.Duration() <= 0 {
		return
	}
	if sess.ID == "" {
		sess.ID = NewID()
	}
	s.Sessions = append(s.Sessions, sess)
}

// TasksByMode devolve as tarefas de um modo, das mais recentes para as mais
// antigas dentro de cada nível de urgência.
func (s *State) TasksByMode(m Mode) []Task {
	out := make([]Task, 0, len(s.Tasks))
	for _, t := range s.Tasks {
		if t.Mode == m {
			out = append(out, t)
		}
	}
	return out
}

// PendingTasks devolve as tarefas não concluídas de um modo, ordenadas por
// peso de prioridade (maior primeiro) e, em caso de empate, pela data limite
// mais próxima. É a ordem da lista "Tarefas Pendentes" do dashboard.
func (s *State) PendingTasks(m Mode) []Task {
	out := make([]Task, 0, len(s.Tasks))
	for _, t := range s.Tasks {
		if t.Mode == m && !t.Done {
			out = append(out, t)
		}
	}
	s.sortByUrgency(m, out)
	return out
}

// TodayTasks devolve as tarefas marcadas como tarefa do dia, incluindo as já
// concluídas — o usuário precisa ver o que já riscou da lista.
func (s *State) TodayTasks(m Mode, diaHoraAtual time.Time) []Task {
	out := make([]Task, 0, len(s.Tasks))
	for _, t := range s.Tasks {
		if t.Mode == m && t.Today {
			diff24Horas := diaHoraAtual.Sub(t.DoneAt)
			if diff24Horas.Hours() < 24 || t.DoneAt.IsZero() {
				out = append(out, t)
			}

		}
	}
	s.sortByUrgency(m, out)
	return out
}

// sortByUrgency ordena in-place: pendentes antes de concluídas, depois maior
// peso, depois data limite mais próxima, e por fim título para dar estabilidade.
func (s *State) sortByUrgency(m Mode, tasks []Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		a, b := tasks[i], tasks[j]
		if a.Done != b.Done {
			return !a.Done
		}
		av, bv := s.PriorityValue(m, a.PriorityID), s.PriorityValue(m, b.PriorityID)
		if av != bv {
			return av > bv
		}
		if a.HasDue() != b.HasDue() {
			return a.HasDue()
		}
		if a.HasDue() && !a.DueAt.Equal(b.DueAt) {
			return a.DueAt.Before(b.DueAt)
		}
		return a.Title < b.Title
	})
}
