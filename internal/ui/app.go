// Package ui monta a interface do Keep It Burning em Gio: as telas, a
// fogueira animada e a ligação entre o que o usuário faz e o estado gravado
// em disco.
package ui

import (
	"image"
	"log"
	"time"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/dvet/keep-it-burning/internal/model"
	"github.com/dvet/keep-it-burning/internal/productivity"
	"github.com/dvet/keep-it-burning/internal/store"
	"github.com/dvet/keep-it-burning/internal/timer"
)

// screenID identifica a tela ativa.
type screenID int

const (
	screenHome screenID = iota
	screenDashboard
	screenTaskForm
	screenSettings
	screenStats
	screenFocus
)

// Tamanhos das janelas. A janela cheia é o dashboard; a reduzida é o modo
// foco, que fica de canto na tela enquanto o usuário trabalha.
var (
	fullSize  = image.Pt(1040, 700)
	focusSize = image.Pt(360, 470)
)

// Frequência da animação da fogueira. 30 quadros por segundo é suave o
// bastante para o fogo e barato para um app que fica aberto o dia todo.
const frameInterval = time.Second / 30

// App é o aplicativo inteiro: estado, persistência, cronômetro e telas.
type App struct {
	win *app.Window
	th  *Theme
	st  *store.Store

	state *model.State
	mode  model.Mode

	screen screenID
	tmr    *timer.Timer

	// startedAt é a referência da animação; o tempo do fogo é medido a partir
	// da abertura do app.
	startedAt time.Time

	// notice é a mensagem transitória mostrada no rodapé (erro ou confirmação).
	notice    string
	noticeErr bool
	noticeAt  time.Time

	home     homeScreen
	dash     dashboardScreen
	form     taskFormScreen
	settings settingsScreen
	stats    statsScreen
	focus    focusScreen
}

// New monta o aplicativo, carregando o estado gravado.
func New(win *app.Window, st *store.Store) (*App, error) {
	state, err := st.Load()
	if err != nil {
		return nil, err
	}
	a := &App{
		win:       win,
		th:        NewTheme(),
		st:        st,
		state:     state,
		mode:      model.ModeWork,
		screen:    screenHome,
		startedAt: time.Now(),
	}
	a.tmr = timer.New(a.mode)
	a.dash.init(a)
	a.form.init(a)
	a.settings.init(a)
	a.stats.init(a)
	a.focus.init(a)
	return a, nil
}

// Run roda o laço de eventos até a janela fechar.
func (a *App) Run() error {
	a.win.Option(app.Title("Keep It Burning"), app.Size(
		unit.Dp(fullSize.X), unit.Dp(fullSize.Y)),
		app.MinSize(unit.Dp(340), unit.Dp(430)),
	)

	var ops op.Ops
	for {
		switch e := a.win.Event().(type) {
		case app.DestroyEvent:
			a.shutdown()
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.layout(gtx)
			// A fogueira nunca fica parada, então cada quadro agenda o próximo.
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(frameInterval)})
			e.Frame(gtx.Ops)
		}
	}
}

// shutdown grava o que estiver pendente antes de fechar, para não perder o
// tempo cronometrado da sessão em andamento.
func (a *App) shutdown() {
	if sessions := a.tmr.Stop(); len(sessions) > 0 {
		for _, s := range sessions {
			a.state.AddSession(s)
		}
	}
	if err := a.st.Save(a.state); err != nil {
		log.Printf("keep-it-burning: não foi possível salvar ao sair: %v", err)
	}
}

// layout desenha a tela ativa sobre o fundo de papel.
func (a *App) layout(gtx layout.Context) layout.Dimensions {
	paint.Fill(gtx.Ops, colorPaper)

	switch a.screen {
	case screenHome:
		return a.home.Layout(gtx, a)
	case screenDashboard:
		return a.dash.Layout(gtx, a)
	case screenTaskForm:
		return a.form.Layout(gtx, a)
	case screenSettings:
		return a.settings.Layout(gtx, a)
	case screenStats:
		return a.stats.Layout(gtx, a)
	case screenFocus:
		return a.focus.Layout(gtx, a)
	default:
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}

// animTime devolve os segundos decorridos desde a abertura, que movem a
// animação da fogueira.
func (a *App) animTime(gtx layout.Context) float64 {
	return gtx.Now.Sub(a.startedAt).Seconds()
}

// report devolve o relatório do modo atual num período.
func (a *App) report(p productivity.Period) productivity.Report {
	return productivity.Analyze(a.state, a.mode, p, time.Now())
}

// dayIntensity devolve a intensidade do fogo referente ao dia de hoje no modo
// atual. É o número que o dashboard e o modo foco usam.
func (a *App) dayIntensity() float64 {
	return productivity.Intensity(a.report(productivity.PeriodDay).Score)
}

// overallDayScore é o score usado na tela inicial, onde ainda não há modo
// escolhido: vale o melhor dos dois lados. Quem só trabalha não deveria ver a
// fogueira pela metade por não ter estudado.
func (a *App) overallDayScore() float64 {
	now := time.Now()
	best := 0.0
	for _, m := range model.Modes {
		if s := productivity.Analyze(a.state, m, productivity.PeriodDay, now).Score; s > best {
			best = s
		}
	}
	return best
}

// setMode troca o modo ativo e ajusta o cronômetro.
func (a *App) setMode(m model.Mode) {
	if a.mode == m {
		return
	}
	a.flushSessions()
	a.mode = m
	a.tmr = timer.New(m)
}

// goTo troca a tela ativa.
func (a *App) goTo(s screenID) {
	a.screen = s
}

// enterFocus reduz a janela e começa a contar o tempo.
func (a *App) enterFocus() {
	a.screen = screenFocus
	a.tmr.Start()
	a.win.Option(app.Size(unit.Dp(focusSize.X), unit.Dp(focusSize.Y)))
}

// leaveFocus volta ao dashboard, grava o tempo cronometrado e restaura o
// tamanho da janela.
func (a *App) leaveFocus() {
	for _, s := range a.tmr.Stop() {
		a.state.AddSession(s)
	}
	a.save()
	a.screen = screenDashboard
	a.win.Option(app.Size(unit.Dp(fullSize.X), unit.Dp(fullSize.Y)))
}

// flushSessions grava em disco os trechos de tempo já fechados sem interromper
// a contagem em andamento.
func (a *App) flushSessions() {
	drained := a.tmr.Drain()
	if len(drained) == 0 {
		return
	}
	for _, s := range drained {
		a.state.AddSession(s)
	}
	a.save()
}

// save persiste o estado e avisa o usuário se falhar — perder tarefas em
// silêncio seria pior do que uma mensagem no rodapé.
func (a *App) save() {
	if err := a.st.Save(a.state); err != nil {
		a.setError("Não foi possível salvar: " + err.Error())
		log.Printf("keep-it-burning: erro ao salvar: %v", err)
	}
}

// setError mostra uma mensagem de erro no rodapé.
func (a *App) setError(msg string) {
	a.notice, a.noticeErr, a.noticeAt = msg, true, time.Now()
}

// setInfo mostra uma confirmação no rodapé.
func (a *App) setInfo(msg string) {
	a.notice, a.noticeErr, a.noticeAt = msg, false, time.Now()
}

// noticeTTL é quanto tempo a mensagem do rodapé fica na tela.
const noticeTTL = 4 * time.Second

// layoutNotice desenha a mensagem transitória, se ainda estiver válida.
func (a *App) layoutNotice(gtx layout.Context) layout.Dimensions {
	if a.notice == "" {
		return layout.Dimensions{}
	}
	if time.Since(a.noticeAt) > noticeTTL {
		a.notice = ""
		return layout.Dimensions{}
	}
	c := colorInkSoft
	if a.noticeErr {
		c = colorDanger
	}
	return a.th.label(unit.Sp(13), a.notice, c).Layout(gtx)
}

// closeWindow fecha o aplicativo pelo botão X da tela inicial.
func (a *App) closeWindow() {
	a.win.Perform(system.ActionClose)
}

// toggleDone marca ou desmarca a conclusão de uma tarefa e salva na hora — é
// a ação mais frequente do app e não pode depender de o usuário sair da tela.
func (a *App) toggleDone(id string) {
	t, ok := a.state.Task(id)
	if !ok {
		return
	}
	if err := a.state.SetDone(id, !t.Done, time.Now()); err != nil {
		a.setError(err.Error())
		return
	}
	a.save()
}

// toggleToday inclui ou tira a tarefa da lista do dia.
func (a *App) toggleToday(id string) {
	t, ok := a.state.Task(id)
	if !ok {
		return
	}
	if err := a.state.SetToday(id, !t.Today); err != nil {
		a.setError(err.Error())
		return
	}
	a.save()
	if t.Today {
		a.setInfo("Tarefa tirada da lista do dia.")
	} else {
		a.setInfo("Tarefa adicionada às tarefas do dia.")
	}
}

// deleteTask remove uma tarefa.
func (a *App) deleteTask(id string) {
	t, _ := a.state.Task(id)
	if err := a.state.DeleteTask(id); err != nil {
		a.setError(err.Error())
		return
	}
	a.save()
	a.setInfo("Tarefa “" + truncate(t.Title, 30) + "” excluída.")
}
