package productivity

import (
	"math"
	"testing"
	"time"

	"github.com/dvet/keep-it-burning/internal/model"
)

// A semana de referência dos testes: 02/02/2026 é segunda, 05/02 é quinta.
func at(day, hour, min int) time.Time {
	return time.Date(2026, time.February, day, hour, min, 0, 0, time.Local)
}

// stateFixture monta um estado com metas redondas para facilitar a conferência
// das contas: 8h por dia, 40h por semana.
func stateFixture() *model.State {
	st := model.NewState()
	st.SetSettings(model.ModeWork, model.ModeSettings{
		Priorities:   model.DefaultPriorities(), // alta=5, media=3, baixa=1
		DailyTarget:  8 * time.Hour,
		WeeklyTarget: 40 * time.Hour,
	})
	return st
}

func nearly(got, want float64) bool { return math.Abs(got-want) < 1e-6 }

func TestPeriodRange(t *testing.T) {
	ref := at(5, 15, 0) // quinta
	tests := []struct {
		p          Period
		from, to   time.Time
		nomeDoCaso string
	}{
		{PeriodDay, at(5, 0, 0), at(6, 0, 0), "dia"},
		{PeriodWeek, at(2, 0, 0), at(9, 0, 0), "semana de segunda a segunda"},
		{PeriodMonth, time.Date(2026, time.February, 1, 0, 0, 0, 0, time.Local),
			time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), "mês"},
	}
	for _, tc := range tests {
		t.Run(tc.nomeDoCaso, func(t *testing.T) {
			from, to := tc.p.Range(ref)
			if !from.Equal(tc.from) || !to.Equal(tc.to) {
				t.Fatalf("Range = [%v, %v), quero [%v, %v)", from, to, tc.from, tc.to)
			}
		})
	}
}

func TestPartitionSeparaEntreguesEPendentes(t *testing.T) {
	from, to := at(5, 0, 0), at(6, 0, 0)
	tasks := []model.Task{
		{ID: "entregue-hoje", Mode: model.ModeWork, Done: true, DoneAt: at(5, 11, 0)},
		{ID: "entregue-ontem", Mode: model.ModeWork, Done: true, DoneAt: at(4, 11, 0)},
		{ID: "do-dia", Mode: model.ModeWork, Today: true},
		{ID: "vence-hoje", Mode: model.ModeWork, DueAt: at(5, 18, 0)},
		{ID: "atrasada", Mode: model.ModeWork, DueAt: at(3, 18, 0)},
		{ID: "vence-depois", Mode: model.ModeWork, DueAt: at(20, 18, 0)},
		{ID: "sem-prazo", Mode: model.ModeWork},
		{ID: "outro-modo", Mode: model.ModeStudy, Today: true},
	}

	done, pending := Partition(tasks, model.ModeWork, from, to)

	if len(done) != 1 || done[0].ID != "entregue-hoje" {
		t.Errorf("entregues = %v, quero só entregue-hoje", ids(done))
	}
	want := map[string]bool{"do-dia": true, "vence-hoje": true, "atrasada": true}
	if len(pending) != len(want) {
		t.Fatalf("pendentes = %v, quero %d itens", ids(pending), len(want))
	}
	for _, p := range pending {
		if !want[p.ID] {
			t.Errorf("pendente inesperado: %s", p.ID)
		}
	}
}

func ids(ts []model.Task) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}

func TestWorkedSomaSoDoModoEDoPeriodo(t *testing.T) {
	sessions := []model.Session{
		{Mode: model.ModeWork, Start: at(5, 9, 0), End: at(5, 11, 0)},   // 2h dentro
		{Mode: model.ModeWork, Start: at(5, 14, 0), End: at(5, 15, 30)}, // 1h30 dentro
		{Mode: model.ModeStudy, Start: at(5, 20, 0), End: at(5, 22, 0)}, // outro modo
		{Mode: model.ModeWork, Start: at(4, 9, 0), End: at(4, 17, 0)},   // outro dia
	}
	got := Worked(sessions, model.ModeWork, at(5, 0, 0), at(6, 0, 0))
	if want := 3*time.Hour + 30*time.Minute; got != want {
		t.Fatalf("Worked = %v, quero %v", got, want)
	}
}

func TestWorkedRecortaSessaoQueAtravessaAMeiaNoite(t *testing.T) {
	// Vira o dia trabalhando das 22h às 2h: 2h contam para cada dia.
	sessions := []model.Session{{Mode: model.ModeWork, Start: at(5, 22, 0), End: at(6, 2, 0)}}

	if got, want := Worked(sessions, model.ModeWork, at(5, 0, 0), at(6, 0, 0)), 2*time.Hour; got != want {
		t.Errorf("dia 5 = %v, quero %v", got, want)
	}
	if got, want := Worked(sessions, model.ModeWork, at(6, 0, 0), at(7, 0, 0)), 2*time.Hour; got != want {
		t.Errorf("dia 6 = %v, quero %v", got, want)
	}
}

func TestTargetFor(t *testing.T) {
	cfg := model.ModeSettings{DailyTarget: 8 * time.Hour, WeeklyTarget: 40 * time.Hour}

	if got := TargetFor(cfg, PeriodDay, at(5, 0, 0), at(6, 0, 0)); got != 8*time.Hour {
		t.Errorf("meta do dia = %v, quero 8h", got)
	}
	if got := TargetFor(cfg, PeriodWeek, at(2, 0, 0), at(9, 0, 0)); got != 40*time.Hour {
		t.Errorf("meta da semana = %v, quero 40h", got)
	}
	// Fevereiro de 2026 tem 28 dias = 4 semanas exatas.
	from := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.Local)
	to := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	if got, want := TargetFor(cfg, PeriodMonth, from, to), 160*time.Hour; got != want {
		t.Errorf("meta do mês = %v, quero %v", got, want)
	}
}

func TestAnalyzeScoreCombinaTarefasETempo(t *testing.T) {
	st := stateFixture()
	// Entregue: uma alta (5). Pendente: uma baixa (1). Razão de tarefas = 5/6.
	st.AddTask(model.Task{ID: "a", Mode: model.ModeWork, Title: "Alta", PriorityID: "alta", Done: true, DoneAt: at(5, 10, 0)})
	st.AddTask(model.Task{ID: "b", Mode: model.ModeWork, Title: "Baixa", PriorityID: "baixa", Today: true})
	// Trabalhou 4h de uma meta de 8h: razão de tempo = 0,5.
	st.AddSession(model.Session{Mode: model.ModeWork, Start: at(5, 9, 0), End: at(5, 13, 0)})

	rep := Analyze(st, model.ModeWork, PeriodDay, at(5, 18, 0))

	if !nearly(rep.TaskRatio, 5.0/6.0) {
		t.Errorf("TaskRatio = %v, quero 5/6", rep.TaskRatio)
	}
	if !nearly(rep.TimeRatio, 0.5) {
		t.Errorf("TimeRatio = %v, quero 0.5", rep.TimeRatio)
	}
	want := 100 * (0.6*(5.0/6.0) + 0.4*0.5)
	if !nearly(rep.Score, want) {
		t.Errorf("Score = %v, quero %v", rep.Score, want)
	}
	if rep.DoneCount != 1 || rep.PendingCount != 1 {
		t.Errorf("contagens = %d entregues / %d pendentes, quero 1 e 1", rep.DoneCount, rep.PendingCount)
	}
}

func TestAnalyzeSemTarefasUsaSoOTempo(t *testing.T) {
	st := stateFixture()
	st.AddSession(model.Session{Mode: model.ModeWork, Start: at(5, 9, 0), End: at(5, 15, 0)}) // 6h de 8h

	rep := Analyze(st, model.ModeWork, PeriodDay, at(5, 18, 0))

	if want := 100 * 6.0 / 8.0; !nearly(rep.Score, want) {
		t.Fatalf("Score sem tarefas = %v, quero %v (só a meta de tempo)", rep.Score, want)
	}
}

func TestAnalyzeDiaZeradoDaScoreZero(t *testing.T) {
	st := stateFixture()
	rep := Analyze(st, model.ModeWork, PeriodDay, at(5, 18, 0))
	if rep.Score != 0 {
		t.Fatalf("Score de um dia vazio = %v, quero 0", rep.Score)
	}
	if len(rep.Slices) != 0 {
		t.Errorf("sem tarefas não deveria haver fatias, tenho %d", len(rep.Slices))
	}
}

func TestAnalyzeScoreMaximo(t *testing.T) {
	st := stateFixture()
	st.AddTask(model.Task{ID: "a", Mode: model.ModeWork, Title: "Alta", PriorityID: "alta", Done: true, DoneAt: at(5, 10, 0)})
	st.AddSession(model.Session{Mode: model.ModeWork, Start: at(5, 9, 0), End: at(5, 17, 0)}) // 8h cheias

	rep := Analyze(st, model.ModeWork, PeriodDay, at(5, 18, 0))
	if !nearly(rep.Score, 100) {
		t.Fatalf("tudo entregue e meta batida deveria dar 100, tenho %v", rep.Score)
	}
}

func TestTimeRatioNaoPassaDeUm(t *testing.T) {
	st := stateFixture()
	// 12h numa meta de 8h: hora extra não vira score acima de 100.
	st.AddSession(model.Session{Mode: model.ModeWork, Start: at(5, 8, 0), End: at(5, 20, 0)})

	rep := Analyze(st, model.ModeWork, PeriodDay, at(5, 18, 0))
	if !nearly(rep.TimeRatio, 1) {
		t.Errorf("TimeRatio = %v, quero 1 (teto)", rep.TimeRatio)
	}
	if rep.Score > 100 {
		t.Errorf("Score = %v, não pode passar de 100", rep.Score)
	}
}

func TestAnalyzeFatiasDoGraficoDePizza(t *testing.T) {
	st := stateFixture()
	st.AddTask(model.Task{ID: "a1", Mode: model.ModeWork, Title: "A1", PriorityID: "alta", Done: true, DoneAt: at(5, 10, 0)})
	st.AddTask(model.Task{ID: "a2", Mode: model.ModeWork, Title: "A2", PriorityID: "alta", Done: true, DoneAt: at(5, 11, 0)})
	st.AddTask(model.Task{ID: "b1", Mode: model.ModeWork, Title: "B1", PriorityID: "baixa", Done: true, DoneAt: at(5, 12, 0)})
	st.AddTask(model.Task{ID: "p1", Mode: model.ModeWork, Title: "P1", PriorityID: "media", Today: true})

	rep := Analyze(st, model.ModeWork, PeriodDay, at(5, 18, 0))

	// Alta = 10, Baixa = 1, Não resolvidas = 3. Total = 14.
	if len(rep.Slices) != 3 {
		t.Fatalf("quero 3 fatias, tenho %d: %+v", len(rep.Slices), rep.Slices)
	}
	if rep.Slices[0].Label != "Alta" || rep.Slices[0].Value != 10 || rep.Slices[0].Count != 2 {
		t.Errorf("primeira fatia = %+v, quero Alta/10/2", rep.Slices[0])
	}
	last := rep.Slices[len(rep.Slices)-1]
	if last.Label != "Não resolvidas" || last.Resolved || last.Value != 3 {
		t.Errorf("última fatia = %+v, quero Não resolvidas/3 e Resolved=false", last)
	}

	var soma float64
	for _, s := range rep.Slices {
		soma += s.Fraction
	}
	if !nearly(soma, 1) {
		t.Errorf("as frações somam %v, quero 1", soma)
	}
	if !nearly(rep.Slices[0].Fraction, 10.0/14.0) {
		t.Errorf("fração de Alta = %v, quero 10/14", rep.Slices[0].Fraction)
	}
}

func TestAnalyzeSemPendentesNaoCriaFatiaCinza(t *testing.T) {
	st := stateFixture()
	st.AddTask(model.Task{ID: "a", Mode: model.ModeWork, Title: "A", PriorityID: "alta", Done: true, DoneAt: at(5, 10, 0)})

	rep := Analyze(st, model.ModeWork, PeriodDay, at(5, 18, 0))
	for _, s := range rep.Slices {
		if !s.Resolved {
			t.Fatalf("não deveria existir fatia de não resolvidas: %+v", s)
		}
	}
}

func TestIndicadoresUsamDiasAtivos(t *testing.T) {
	st := stateFixture()
	// Segunda e terça da semana de 02/02: 4h + 6h. Quarta a domingo parados.
	st.AddSession(model.Session{Mode: model.ModeWork, Start: at(2, 9, 0), End: at(2, 13, 0)})
	st.AddSession(model.Session{Mode: model.ModeWork, Start: at(3, 9, 0), End: at(3, 15, 0)})
	st.AddTask(model.Task{ID: "a", Mode: model.ModeWork, Title: "A", PriorityID: "alta", Done: true, DoneAt: at(2, 12, 0)})
	st.AddTask(model.Task{ID: "b", Mode: model.ModeWork, Title: "B", PriorityID: "media", Done: true, DoneAt: at(3, 12, 0)})
	st.AddTask(model.Task{ID: "c", Mode: model.ModeWork, Title: "C", PriorityID: "baixa", Done: true, DoneAt: at(3, 14, 0)})

	ind := Analyze(st, model.ModeWork, PeriodWeek, at(5, 18, 0)).Indicators

	if ind.ActiveDays != 2 {
		t.Fatalf("ActiveDays = %d, quero 2", ind.ActiveDays)
	}
	if ind.Weeks != 1 {
		t.Fatalf("Weeks = %d, quero 1", ind.Weeks)
	}
	if want := 5 * time.Hour; ind.AvgTimePerDay != want { // 10h / 2 dias
		t.Errorf("AvgTimePerDay = %v, quero %v", ind.AvgTimePerDay, want)
	}
	if want := 10 * time.Hour; ind.AvgTimePerWeek != want {
		t.Errorf("AvgTimePerWeek = %v, quero %v", ind.AvgTimePerWeek, want)
	}
	if !nearly(ind.AvgTasksPerDay, 1.5) { // 3 tarefas / 2 dias
		t.Errorf("AvgTasksPerDay = %v, quero 1.5", ind.AvgTasksPerDay)
	}
	if !nearly(ind.AvgTasksPerWeek, 3) {
		t.Errorf("AvgTasksPerWeek = %v, quero 3", ind.AvgTasksPerWeek)
	}
}

func TestIndicadoresContamSessaoQueViraAMeiaNoite(t *testing.T) {
	st := stateFixture()
	// Vira a madrugada trabalhando: começa às 21h do dia 4 e para às 00h40 do
	// dia 5. Do ponto de vista do dia 5, foram 40 minutos trabalhados.
	st.AddSession(model.Session{Mode: model.ModeWork, Start: at(4, 21, 0), End: at(5, 0, 40)})

	rep := Analyze(st, model.ModeWork, PeriodDay, at(5, 12, 0))

	if want := 40 * time.Minute; rep.Worked != want {
		t.Fatalf("Worked = %v, quero %v", rep.Worked, want)
	}
	// O indicador precisa bater com o total do período: contar só o instante
	// de início jogaria a sessão inteira fora.
	if got := rep.Indicators.AvgTimePerDay; got != 40*time.Minute {
		t.Errorf("AvgTimePerDay = %v, quero 40min", got)
	}
	if rep.Indicators.ActiveDays != 1 {
		t.Errorf("ActiveDays = %d, quero 1", rep.Indicators.ActiveDays)
	}
}

func TestIndicadoresBatemComOTotalDoPeriodo(t *testing.T) {
	st := stateFixture()
	st.AddSession(model.Session{Mode: model.ModeWork, Start: at(1, 22, 0), End: at(2, 3, 0)}) // 3h dentro da semana
	st.AddSession(model.Session{Mode: model.ModeWork, Start: at(3, 9, 0), End: at(3, 13, 0)}) // 4h
	st.AddSession(model.Session{Mode: model.ModeWork, Start: at(8, 20, 0), End: at(9, 1, 0)}) // 4h dentro, 1h fora

	rep := Analyze(st, model.ModeWork, PeriodWeek, at(5, 12, 0))

	// A soma dos indicadores por semana tem que ser exatamente o tempo do
	// período; qualquer diferença é sessão perdida no recorte.
	if got := rep.Indicators.AvgTimePerWeek; got != rep.Worked {
		t.Fatalf("AvgTimePerWeek = %v, mas o período tem %v", got, rep.Worked)
	}
	if want := 11 * time.Hour; rep.Worked != want {
		t.Errorf("Worked = %v, quero %v", rep.Worked, want)
	}
	if rep.Indicators.ActiveDays != 3 {
		t.Errorf("ActiveDays = %d, quero 3", rep.Indicators.ActiveDays)
	}
}

func TestIndicadoresPeriodoVazioNaoDivideporZero(t *testing.T) {
	st := stateFixture()
	ind := Analyze(st, model.ModeWork, PeriodMonth, at(5, 18, 0)).Indicators

	if ind.ActiveDays != 0 || ind.AvgTimePerDay != 0 || ind.AvgTasksPerDay != 0 {
		t.Fatalf("período sem atividade deveria zerar as médias, tenho %+v", ind)
	}
	if ind.Weeks != 4 { // fevereiro de 2026: 28 dias
		t.Errorf("Weeks = %d, quero 4", ind.Weeks)
	}
}

func TestTarefaComPrioridadeApagadaNaoQuebraOScore(t *testing.T) {
	st := stateFixture()
	st.AddTask(model.Task{ID: "x", Mode: model.ModeWork, Title: "Órfã", PriorityID: "sumiu", Done: true, DoneAt: at(5, 10, 0)})

	rep := Analyze(st, model.ModeWork, PeriodDay, at(5, 18, 0))
	if rep.DoneCount != 1 {
		t.Errorf("a tarefa órfã ainda conta como entregue, DoneCount = %d", rep.DoneCount)
	}
	if len(rep.Slices) != 1 || rep.Slices[0].Label != "Sem prioridade" {
		t.Errorf("fatias = %+v, quero uma fatia 'Sem prioridade'", rep.Slices)
	}
}

func TestAnalyzeSemanaAgregaOsDias(t *testing.T) {
	st := stateFixture()
	for _, dia := range []int{2, 3, 4} {
		st.AddSession(model.Session{Mode: model.ModeWork, Start: at(dia, 9, 0), End: at(dia, 17, 0)})
	}
	rep := Analyze(st, model.ModeWork, PeriodWeek, at(5, 12, 0))

	if want := 24 * time.Hour; rep.Worked != want {
		t.Errorf("Worked = %v, quero %v", rep.Worked, want)
	}
	if rep.Target != 40*time.Hour {
		t.Errorf("Target = %v, quero 40h", rep.Target)
	}
	if !nearly(rep.TimeRatio, 24.0/40.0) {
		t.Errorf("TimeRatio = %v, quero 0.6", rep.TimeRatio)
	}
}

func TestIntensity(t *testing.T) {
	tests := []struct{ score, want float64 }{
		{0, 0}, {50, 0.5}, {100, 1},
		{-10, 0}, // score negativo não existe, mas a UI não pode receber lixo
		{150, 1}, // teto
	}
	for _, tc := range tests {
		if got := Intensity(tc.score); !nearly(got, tc.want) {
			t.Errorf("Intensity(%v) = %v, quero %v", tc.score, got, tc.want)
		}
	}
}

func TestStageOf(t *testing.T) {
	tests := []struct {
		score float64
		want  Stage
	}{
		{0, StageEmbers}, {19.9, StageEmbers},
		{20, StageLow}, {39.9, StageLow},
		{40, StageSteady}, {64.9, StageSteady},
		{65, StageStrong}, {84.9, StageStrong},
		{85, StageBlazing}, {100, StageBlazing},
	}
	for _, tc := range tests {
		if got := StageOf(tc.score); got != tc.want {
			t.Errorf("StageOf(%v) = %v, quero %v", tc.score, got, tc.want)
		}
	}
	for s := StageEmbers; s <= StageBlazing; s++ {
		if s.Label() == "" {
			t.Errorf("estágio %d ficou sem rótulo", s)
		}
	}
}

func TestPeriodLabel(t *testing.T) {
	want := map[Period]string{PeriodDay: "Dia", PeriodWeek: "Semana", PeriodMonth: "Mês"}
	for p, w := range want {
		if got := p.Label(); got != w {
			t.Errorf("Label(%v) = %q, quero %q", p, got, w)
		}
	}
}

func TestTaskStatsForSeparaPrioridadesEPontualidade(t *testing.T) {
	st := stateFixture()
	cfg := st.SettingsFor(model.ModeWork)
	cfg.Categories = []model.Category{{ID: "reuniao", Title: "Reunião"}}
	st.SetSettings(model.ModeWork, cfg)
	st.Tasks = []model.Task{
		// Entregues no dia 5: uma alta no prazo (categoria Reunião), uma média
		// atrasada sem categoria.
		{ID: "alta-ok", Mode: model.ModeWork, PriorityID: "alta", CategoryID: "reuniao", Done: true,
			CreatedAt: at(3, 9, 0), DueAt: at(5, 18, 0), DoneAt: at(5, 12, 0)},
		{ID: "media-atrasada", Mode: model.ModeWork, PriorityID: "media", Done: true,
			CreatedAt: at(4, 9, 0), DueAt: at(4, 18, 0), DoneAt: at(5, 10, 0)},
		// Entregue fora do período: não conta nas entregues.
		{ID: "fora-do-dia", Mode: model.ModeWork, PriorityID: "alta", Done: true, DoneAt: at(2, 10, 0)},
		// Em aberto: uma baixa vencida, uma alta sem prazo, uma média futura.
		{ID: "baixa-vencida", Mode: model.ModeWork, PriorityID: "baixa", DueAt: at(4, 18, 0)},
		{ID: "alta-sem-prazo", Mode: model.ModeWork, PriorityID: "alta"},
		{ID: "media-futura", Mode: model.ModeWork, PriorityID: "media", DueAt: at(20, 18, 0)},
		// Outro modo: ignorada.
		{ID: "estudo", Mode: model.ModeStudy, PriorityID: "alta"},
	}

	ts := TaskStatsFor(st, model.ModeWork, PeriodDay, at(5, 15, 0))

	if ts.DoneCount != 2 || ts.OpenCount != 3 {
		t.Fatalf("done=%d open=%d, quero 2 e 3", ts.DoneCount, ts.OpenCount)
	}
	want := map[string][2]int{"Alta": {1, 1}, "Média": {1, 1}, "Baixa": {0, 1}}
	for _, l := range ts.Lines {
		w, ok := want[l.Label]
		if !ok {
			t.Errorf("linha inesperada: %q", l.Label)
			continue
		}
		if l.Done != w[0] || l.Open != w[1] {
			t.Errorf("%s: done=%d open=%d, quero %d e %d", l.Label, l.Done, l.Open, w[0], w[1])
		}
	}
	if ts.DoneWithDue != 2 || ts.DoneLate != 1 || !nearly(ts.LateRatio, 0.5) {
		t.Errorf("pontualidade: comPrazo=%d atrasadas=%d ratio=%v, quero 2, 1 e 0.5",
			ts.DoneWithDue, ts.DoneLate, ts.LateRatio)
	}
	if ts.OverdueOpen != 1 || ts.NoDueOpen != 1 {
		t.Errorf("abertas: vencidas=%d semPrazo=%d, quero 1 e 1", ts.OverdueOpen, ts.NoDueOpen)
	}
	// Entregas: alta levou 51h, média levou 25h → média de 38h.
	if want := 38 * time.Hour; ts.AvgDelivery != want {
		t.Errorf("AvgDelivery = %v, quero %v", ts.AvgDelivery, want)
	}
	// Categorias: Reunião tem a entrega alta-ok; o resto cai em Sem categoria.
	wantCats := map[string][2]int{"Reunião": {1, 0}, "Sem categoria": {1, 3}}
	if len(ts.Categories) != len(wantCats) {
		t.Fatalf("categorias = %d linhas, quero %d", len(ts.Categories), len(wantCats))
	}
	for _, l := range ts.Categories {
		w, ok := wantCats[l.Label]
		if !ok {
			t.Errorf("categoria inesperada: %q", l.Label)
			continue
		}
		if l.Done != w[0] || l.Open != w[1] {
			t.Errorf("%s: done=%d open=%d, quero %d e %d", l.Label, l.Done, l.Open, w[0], w[1])
		}
	}
}

func TestTimeStatsForResumeSessoes(t *testing.T) {
	st := stateFixture()
	st.Sessions = []model.Session{
		{ID: "a", Mode: model.ModeWork, Start: at(5, 9, 0), End: at(5, 12, 0)},  // 3h
		{ID: "b", Mode: model.ModeWork, Start: at(5, 14, 0), End: at(5, 15, 0)}, // 1h
		{ID: "c", Mode: model.ModeWork, Start: at(4, 9, 0), End: at(4, 10, 0)},  // fora do dia
		{ID: "d", Mode: model.ModeStudy, Start: at(5, 9, 0), End: at(5, 10, 0)}, // outro modo
	}

	ts := TimeStatsFor(st, model.ModeWork, PeriodDay, at(5, 15, 0))

	if ts.Worked != 4*time.Hour || ts.Target != 8*time.Hour {
		t.Fatalf("worked=%v target=%v, quero 4h e 8h", ts.Worked, ts.Target)
	}
	if ts.Sessions != 2 || ts.AvgSession != 2*time.Hour || ts.Longest != 3*time.Hour {
		t.Errorf("sessões=%d média=%v maisLonga=%v, quero 2, 2h e 3h", ts.Sessions, ts.AvgSession, ts.Longest)
	}
	if ts.ActiveDays != 1 || ts.AvgPerDay != 4*time.Hour {
		t.Errorf("diasAtivos=%d médiaDia=%v, quero 1 e 4h", ts.ActiveDays, ts.AvgPerDay)
	}
	if !ts.BestDay.Equal(at(5, 0, 0)) || ts.BestDayTime != 4*time.Hour {
		t.Errorf("melhor dia = %v (%v), quero 05/02 com 4h", ts.BestDay, ts.BestDayTime)
	}
}

func TestDoneTasksInFiltraEOrdena(t *testing.T) {
	st := stateFixture()
	st.Tasks = []model.Task{
		{ID: "cedo", Mode: model.ModeWork, Done: true, DoneAt: at(3, 9, 0)},
		{ID: "tarde", Mode: model.ModeWork, Done: true, DoneAt: at(5, 17, 0)},
		{ID: "meio", Mode: model.ModeWork, Done: true, DoneAt: at(4, 12, 0)},
		{ID: "fora", Mode: model.ModeWork, Done: true, DoneAt: at(10, 9, 0)},
		{ID: "aberta", Mode: model.ModeWork},
		{ID: "estudo", Mode: model.ModeStudy, Done: true, DoneAt: at(4, 9, 0)},
	}

	got := DoneTasksIn(st, model.ModeWork, at(3, 0, 0), at(6, 0, 0))

	want := []string{"tarde", "meio", "cedo"} // mais recentes primeiro
	if len(got) != len(want) {
		t.Fatalf("concluídas = %v, quero %v", ids(got), want)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("posição %d = %s, quero %s", i, got[i].ID, id)
		}
	}
}

// difficultyFixture é o stateFixture com a escala de dificuldade configurada:
// facil=1, media=2, dificil=3.
func difficultyFixture() *model.State {
	st := stateFixture()
	cfg := st.SettingsFor(model.ModeWork)
	cfg.Difficulties = model.DefaultDifficulties()
	st.SetSettings(model.ModeWork, cfg)
	return st
}

func TestAnalyzeDificuldadeMultiplicaOPeso(t *testing.T) {
	st := difficultyFixture()
	// Entregue: uma média (3) difícil (×3) = 9.
	st.AddTask(model.Task{ID: "a", Mode: model.ModeWork, Title: "Difícil", PriorityID: "media",
		DifficultyID: "dificil", Done: true, DoneAt: at(5, 10, 0)})
	// Pendente: uma média (3) fácil (×1) = 3. Razão de tarefas = 9/12.
	st.AddTask(model.Task{ID: "b", Mode: model.ModeWork, Title: "Fácil", PriorityID: "media",
		DifficultyID: "facil", Today: true})

	rep := Analyze(st, model.ModeWork, PeriodDay, at(5, 18, 0))

	if !nearly(rep.DoneWeight, 9) || !nearly(rep.PendingWeight, 3) {
		t.Errorf("pesos = %v entregue / %v pendente, quero 9 e 3", rep.DoneWeight, rep.PendingWeight)
	}
	if !nearly(rep.TaskRatio, 0.75) {
		t.Errorf("TaskRatio = %v, quero 0.75", rep.TaskRatio)
	}
	// Sem a dificuldade as duas tarefas teriam o mesmo peso e a razão seria
	// 0,5: é exatamente essa diferença que o campo precisa produzir.
	if nearly(rep.TaskRatio, 0.5) {
		t.Error("a dificuldade não mudou nada no score")
	}
}

func TestAnalyzeSemDificuldadeMantemOPesoAntigo(t *testing.T) {
	st := difficultyFixture()
	// Mesma conta do score clássico, com a escala configurada mas nenhuma
	// tarefa usando: o fator neutro precisa manter a razão em 5/6.
	st.AddTask(model.Task{ID: "a", Mode: model.ModeWork, Title: "Alta", PriorityID: "alta", Done: true, DoneAt: at(5, 10, 0)})
	st.AddTask(model.Task{ID: "b", Mode: model.ModeWork, Title: "Baixa", PriorityID: "baixa", Today: true})

	rep := Analyze(st, model.ModeWork, PeriodDay, at(5, 18, 0))

	if !nearly(rep.TaskRatio, 5.0/6.0) {
		t.Errorf("TaskRatio = %v, quero 5/6", rep.TaskRatio)
	}
}

func TestTaskStatsFatiaPorSubcategoriaEDificuldade(t *testing.T) {
	st := difficultyFixture()
	cfg := st.SettingsFor(model.ModeWork)
	cfg.Subcategories = []model.Subcategory{{ID: "front", Title: "Front-end"}}
	st.SetSettings(model.ModeWork, cfg)

	st.AddTask(model.Task{ID: "a", Mode: model.ModeWork, Title: "Entregue", PriorityID: "alta",
		DifficultyID: "dificil", SubcategoryID: "front", Done: true, DoneAt: at(5, 10, 0)})
	st.AddTask(model.Task{ID: "b", Mode: model.ModeWork, Title: "Aberta", PriorityID: "alta", Today: true})

	ts := TaskStatsFor(st, model.ModeWork, PeriodDay, at(5, 18, 0))

	// Uma linha por item configurado, mais a linha de sobra criada pela tarefa
	// que não preencheu o campo.
	if len(ts.Subcategories) != 2 || ts.Subcategories[0].Label != "Front-end" || ts.Subcategories[0].Done != 1 {
		t.Errorf("subcategorias = %+v", ts.Subcategories)
	}
	if last := ts.Subcategories[len(ts.Subcategories)-1]; last.Label != "Sem subcategoria" || last.Open != 1 {
		t.Errorf("linha de sobra das subcategorias = %+v", last)
	}
	if len(ts.Difficulties) != 4 {
		t.Errorf("dificuldades = %+v, quero as três configuradas mais a sobra", ts.Difficulties)
	}
	if ts.Difficulties[2].Label != "Difícil" || ts.Difficulties[2].Done != 1 {
		t.Errorf("linha de difícil = %+v", ts.Difficulties[2])
	}
}
