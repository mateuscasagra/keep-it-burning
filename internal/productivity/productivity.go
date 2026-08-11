// Package productivity transforma tarefas e sessões em números: o score do
// período, a divisão por prioridade que alimenta o gráfico de pizza, os
// indicadores médios e — o que o usuário realmente olha — a intensidade da
// fogueira.
package productivity

import (
	"math"
	"sort"
	"time"

	"github.com/dvet/keep-it-burning/internal/dates"
	"github.com/dvet/keep-it-burning/internal/model"
)

// Period é a janela de tempo de um relatório.
type Period string

const (
	PeriodDay   Period = "dia"
	PeriodWeek  Period = "semana"
	PeriodMonth Period = "mes"
)

// Periods lista os períodos na ordem em que aparecem no dashboard.
var Periods = []Period{PeriodDay, PeriodWeek, PeriodMonth}

// Label devolve o nome do período como ele aparece na interface.
func (p Period) Label() string {
	switch p {
	case PeriodDay:
		return "Dia"
	case PeriodWeek:
		return "Semana"
	case PeriodMonth:
		return "Mês"
	default:
		return string(p)
	}
}

// Range devolve o intervalo [from, to) do período que contém ref.
func (p Period) Range(ref time.Time) (from, to time.Time) {
	switch p {
	case PeriodWeek:
		return dates.StartOfWeek(ref), dates.EndOfWeek(ref)
	case PeriodMonth:
		return dates.StartOfMonth(ref), dates.EndOfMonth(ref)
	default:
		return dates.StartOfDay(ref), dates.EndOfDay(ref)
	}
}

// Pesos do score. As tarefas entregues mandam mais que o relógio: ficar
// sentado na cadeira não é produtividade, mas ignorar o tempo tornaria fácil
// inflar o score fechando tarefas triviais.
const (
	taskWeight = 0.6
	timeWeight = 0.4
)

// Report é o resultado completo de um período.
type Report struct {
	Period Period
	From   time.Time
	To     time.Time

	// Score final de 0 a 100.
	Score float64

	// Componentes do score, de 0 a 1, úteis para explicar o número ao usuário.
	TaskRatio float64
	TimeRatio float64

	// Pesos de prioridade entregues e ainda em aberto no período.
	DoneWeight    float64
	PendingWeight float64

	DoneCount    int
	PendingCount int

	// Worked é o tempo cronometrado no período; Target é a meta correspondente.
	Worked time.Duration
	Target time.Duration

	// Slices alimenta o gráfico de pizza: uma fatia por prioridade entregue
	// mais uma fatia com tudo que ficou sem resolver.
	Slices []Slice

	Indicators Indicators
}

// Slice é uma fatia do gráfico de pizza.
type Slice struct {
	Label string
	// Value é o peso acumulado da fatia (soma dos valores de prioridade).
	Value float64
	// Count é quantas tarefas formam a fatia.
	Count int
	// Resolved distingue as fatias de prioridades entregues da fatia "Não
	// resolvidas", que é desenhada em cinza.
	Resolved bool
	// Fraction é a participação da fatia no total, de 0 a 1.
	Fraction float64
}

// Indicators são as médias exibidas embaixo do score.
type Indicators struct {
	AvgTimePerDay   time.Duration
	AvgTimePerWeek  time.Duration
	AvgTasksPerDay  float64
	AvgTasksPerWeek float64
	// ActiveDays é o número de dias do período com sessão cronometrada ou
	// tarefa entregue. É o divisor das médias "por dia": contar dias parados
	// afundaria a média de quem trabalha de segunda a sexta.
	ActiveDays int
	Weeks      int
}

// Partition separa as tarefas relevantes de um período em entregues e pendentes.
//
// Entregue: concluída com data de conclusão dentro de [from, to).
// Pendente: em aberto e cobrada no período — ou porque o prazo cai antes do
// fim do período (inclusive prazos já vencidos, que continuam pesando até
// serem resolvidos), ou porque foi marcada como tarefa do dia.
func Partition(tasks []model.Task, mode model.Mode, from, to time.Time) (done, pending []model.Task) {
	for _, t := range tasks {
		if t.Mode != mode {
			continue
		}
		switch {
		case t.Done:
			if dates.InRange(t.DoneAt, from, to) {
				done = append(done, t)
			}
		case t.Today || (t.HasDue() && t.DueAt.Before(to)):
			pending = append(pending, t)
		}
	}
	return done, pending
}

// overlap devolve o instante em que uma sessão passa a contar dentro de
// [from, to) e a duração do trecho que cai nesse intervalo. Uma sessão
// totalmente fora do período devolve duração zero.
//
// É o que impede que virar a madrugada trabalhando jogue as horas para o dia
// errado — ou, pior, que a sessão seja descartada por ter começado na véspera.
func overlap(s model.Session, from, to time.Time) (start time.Time, d time.Duration) {
	if s.Duration() <= 0 {
		return time.Time{}, 0
	}
	start, end := s.Start, s.End
	if start.Before(from) {
		start = from
	}
	if end.After(to) {
		end = to
	}
	if !end.After(start) {
		return time.Time{}, 0
	}
	return start, end.Sub(start)
}

// Worked soma a duração das sessões de um modo que caem no período. Sessões
// que atravessam a fronteira do período entram apenas com a parte de dentro.
func Worked(sessions []model.Session, mode model.Mode, from, to time.Time) time.Duration {
	var total time.Duration
	for _, s := range sessions {
		if s.Mode != mode {
			continue
		}
		if _, d := overlap(s, from, to); d > 0 {
			total += d
		}
	}
	return total
}

// TargetFor devolve a meta de tempo do período. O mês é derivado da meta
// semanal proporcionalmente ao número de dias.
func TargetFor(cfg model.ModeSettings, p Period, from, to time.Time) time.Duration {
	switch p {
	case PeriodWeek:
		return cfg.WeeklyTarget
	case PeriodMonth:
		days := dates.DaysBetween(from, to)
		return time.Duration(float64(cfg.WeeklyTarget) * float64(days) / 7)
	default:
		return cfg.DailyTarget
	}
}

// Analyze produz o relatório de um modo em um período, tomando ref como
// referência de "agora".
func Analyze(st *model.State, mode model.Mode, p Period, ref time.Time) Report {
	from, to := p.Range(ref)
	cfg := st.SettingsFor(mode)

	done, pending := Partition(st.Tasks, mode, from, to)
	worked := Worked(st.Sessions, mode, from, to)
	target := TargetFor(cfg, p, from, to)

	rep := Report{
		Period:       p,
		From:         from,
		To:           to,
		DoneCount:    len(done),
		PendingCount: len(pending),
		Worked:       worked,
		Target:       target,
	}

	for _, t := range done {
		rep.DoneWeight += float64(st.PriorityValue(mode, t.PriorityID))
	}
	for _, t := range pending {
		rep.PendingWeight += float64(st.PriorityValue(mode, t.PriorityID))
	}

	total := rep.DoneWeight + rep.PendingWeight
	if total > 0 {
		rep.TaskRatio = rep.DoneWeight / total
	}
	if target > 0 {
		rep.TimeRatio = math.Min(float64(worked)/float64(target), 1)
	}

	// Sem nenhuma tarefa no período o score vira puramente o cumprimento da
	// meta de tempo — caso contrário quem só cronometra ficaria travado em 40.
	if total > 0 {
		rep.Score = 100 * (taskWeight*rep.TaskRatio + timeWeight*rep.TimeRatio)
	} else {
		rep.Score = 100 * rep.TimeRatio
	}

	rep.Slices = buildSlices(st, mode, done, rep.PendingWeight, len(pending))
	rep.Indicators = buildIndicators(st, mode, done, from, to)
	return rep
}

// buildSlices monta as fatias do gráfico: uma por prioridade entregue, na
// ordem de peso decrescente, mais a fatia cinza das não resolvidas.
func buildSlices(st *model.State, mode model.Mode, done []model.Task, pendingWeight float64, pendingCount int) []Slice {
	byPriority := map[string]*Slice{}
	for _, t := range done {
		p, ok := st.Priority(mode, t.PriorityID)
		label := p.Title
		value := float64(p.Value)
		if !ok {
			label = "Sem prioridade"
			value = 0
		}
		s, exists := byPriority[label]
		if !exists {
			s = &Slice{Label: label, Resolved: true}
			byPriority[label] = s
		}
		s.Value += value
		s.Count++
	}

	slices := make([]Slice, 0, len(byPriority)+1)
	for _, s := range byPriority {
		slices = append(slices, *s)
	}
	sort.Slice(slices, func(i, j int) bool {
		if slices[i].Value != slices[j].Value {
			return slices[i].Value > slices[j].Value
		}
		return slices[i].Label < slices[j].Label
	})
	if pendingCount > 0 {
		slices = append(slices, Slice{
			Label:    "Não resolvidas",
			Value:    pendingWeight,
			Count:    pendingCount,
			Resolved: false,
		})
	}

	var total float64
	for _, s := range slices {
		total += s.Value
	}
	if total > 0 {
		for i := range slices {
			slices[i].Fraction = slices[i].Value / total
		}
	}
	return slices
}

// buildIndicators calcula as médias do período.
func buildIndicators(st *model.State, mode model.Mode, done []model.Task, from, to time.Time) Indicators {
	activeDays := map[time.Time]bool{}
	var worked time.Duration
	for _, s := range st.Sessions {
		if s.Mode != mode {
			continue
		}
		// Usa o mesmo recorte de Worked: uma sessão que começou ontem e
		// terminou hoje conta as horas de hoje, e o dia ativo é o de hoje.
		start, d := overlap(s, from, to)
		if d <= 0 {
			continue
		}
		activeDays[dates.StartOfDay(start)] = true
		worked += d
	}
	for _, t := range done {
		activeDays[dates.StartOfDay(t.DoneAt)] = true
	}

	ind := Indicators{ActiveDays: len(activeDays)}

	// A semana conta como unidade inteira: um período de 1 a 7 dias é uma
	// semana, de 8 a 14 são duas, e assim por diante.
	days := dates.DaysBetween(from, to)
	ind.Weeks = int(math.Ceil(float64(days) / 7))
	if ind.Weeks < 1 {
		ind.Weeks = 1
	}

	if ind.ActiveDays > 0 {
		ind.AvgTimePerDay = worked / time.Duration(ind.ActiveDays)
		ind.AvgTasksPerDay = float64(len(done)) / float64(ind.ActiveDays)
	}
	ind.AvgTimePerWeek = worked / time.Duration(ind.Weeks)
	ind.AvgTasksPerWeek = float64(len(done)) / float64(ind.Weeks)
	return ind
}

// Intensity converte um score de 0 a 100 na intensidade do fogo, de 0 a 1.
// É o que liga a produtividade à animação da fogueira.
func Intensity(score float64) float64 {
	return math.Max(0, math.Min(score/100, 1))
}

// Stage é o estágio visual da fogueira. Serve para escolher paleta e texto;
// a animação em si usa a intensidade contínua.
type Stage int

const (
	StageEmbers  Stage = iota // brasa: quase apagada
	StageLow                  // chama fraca
	StageSteady               // fogo firme
	StageStrong               // fogo forte
	StageBlazing              // fogueira no talo
)

// StageOf classifica um score em um estágio.
func StageOf(score float64) Stage {
	switch {
	case score < 20:
		return StageEmbers
	case score < 40:
		return StageLow
	case score < 65:
		return StageSteady
	case score < 85:
		return StageStrong
	default:
		return StageBlazing
	}
}

// Label devolve a descrição do estágio mostrada ao usuário.
func (s Stage) Label() string {
	switch s {
	case StageEmbers:
		return "Quase apagando"
	case StageLow:
		return "Chama fraca"
	case StageSteady:
		return "Fogo aceso"
	case StageStrong:
		return "Fogo forte"
	default:
		return "Fogueira no talo"
	}
}
