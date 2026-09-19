// Package ui monta a interface do Keep It Burning em Gio: as telas, a
// fogueira animada e a ligação entre o que o usuário faz e o estado gravado
// em disco.
package ui

import (
	"image"
	"log"
	"os/exec"
	"path/filepath"
	"time"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/x/explorer"

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
	screenTaskView
	screenPending
)

// Tamanhos das janelas, um por forma de visualização. A cheia é o dashboard;
// a reduzida e a mini ficam de canto na tela enquanto o usuário trabalha.
var (
	// A janela cheia precisa desta largura para a tabela de tarefas caber sem
	// espremer as colunas de categoria, subcategoria e dificuldade.
	fullSize  = image.Pt(1240, 700)
	focusSize = image.Pt(360, 470)

	// minSize é o limite mínimo da janela. Fica um pouco abaixo da menor
	// visualização, senão o Windows recusa o encolhimento e a janela reduzida
	// abre maior do que devia.
	minSize = image.Pt(320, 420)
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
	// vmode é a forma de visualização escolhida no seletor; é independente do
	// cronômetro, que corre igual em qualquer uma delas.
	vmode viewMode
	// winMode acompanha o estado da janela no sistema (janela, maximizada,
	// tela cheia). Sem ele o app não tem como saber que já está ocupando a
	// tela toda e acabaria encolhendo a janela do usuário.
	winMode app.WindowMode
	tmr     *timer.Timer

	// expl abre os diálogos nativos de escolher arquivo.
	expl *explorer.Explorer

	// startedAt é a referência da animação; o tempo do fogo é medido a partir
	// da abertura do app.
	startedAt time.Time

	// notice é a mensagem transitória mostrada no rodapé (erro ou confirmação).
	notice    string
	noticeErr bool
	noticeAt  time.Time

	// updateCh recebe o resultado da recompilação disparada pelo botão
	// Atualizar; nil quando não há atualização em andamento.
	updateCh chan updateResult
	updating bool
	// handedOff marca que o estado já foi gravado e o app novo assumiu. Daí em
	// diante esta instância não pode mais gravar: ela só tem uma cópia velha do
	// estado, e sobrescrever o disco apagaria o que o app novo já mexeu.
	handedOff bool

	home     homeScreen
	dash     dashboardScreen
	form     taskFormScreen
	settings settingsScreen
	stats    statsScreen
	focus    focusScreen
	view     taskViewScreen
	pending  pendingScreen
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
		vmode:     viewFull,
		startedAt: time.Now(),
	}
	a.tmr = timer.New(a.mode)
	a.expl = explorer.NewExplorer(win)
	a.dash.init(a)
	a.form.init(a)
	a.settings.init(a)
	a.stats.init(a)
	a.focus.init(a)
	a.view.init(a)
	a.pending.init(a)
	return a, nil
}

// Run roda o laço de eventos até a janela fechar.
func (a *App) Run() error {
	a.win.Option(app.Title("Keep It Burning"), app.Size(
		unit.Dp(fullSize.X), unit.Dp(fullSize.Y)),
		app.MinSize(unit.Dp(minSize.X), unit.Dp(minSize.Y)),
	)

	var ops op.Ops
	for {
		evt := a.win.Event()
		// O explorer precisa ver os eventos da janela para ancorar os
		// diálogos nativos de arquivo.
		a.expl.ListenEvents(evt)
		switch e := evt.(type) {
		case app.ConfigEvent:
			a.winMode = e.Config.Mode
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
	// Depois de entregar o lugar ao app novo não há nada pendente para gravar:
	// o estado já foi para o disco e o cronômetro já foi parado.
	if a.handedOff {
		return
	}
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
	a.pollUpdate()

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
	case screenTaskView:
		return a.view.Layout(gtx, a)
	case screenPending:
		return a.pending.Layout(gtx, a)
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

// setView troca a forma de visualização: redimensiona a janela e vai para a
// tela correspondente. Nunca mexe no cronômetro — mudar de visualização não
// começa nem interrompe a sessão; só os trechos já fechados vão para o disco.
func (a *App) setView(m viewMode) {
	opt := viewOptionFor(m)
	prev := a.vmode
	a.vmode = opt.mode
	a.flushSessions()
	if opt.mode == viewFull {
		a.screen = screenDashboard
	} else {
		a.screen = screenFocus
	}
	if opts := windowOptionsFor(opt.mode, prev, a.winMode); len(opts) > 0 {
		a.win.Option(opts...)
	}
}

// windowOptionsFor decide como a janela precisa ser ajustada ao entrar em uma
// forma de visualização. Devolve nada quando não há o que mexer — e é aí que
// está a graça: app.Size sempre devolve a janela ao modo "janela", então pedir
// o tamanho à toa desmaximiza quem só queria entrar no painel.
func windowOptionsFor(target, prev viewMode, mode app.WindowMode) []app.Option {
	if target == prev {
		// A forma não mudou: escolher trabalho ou estudo na tela inicial não é
		// um pedido para redimensionar nada.
		return nil
	}
	if target == viewFull && mode != app.Windowed {
		// Maximizada ou em tela cheia, a janela já ocupa o que a visualização
		// completa quer — e mais.
		return nil
	}
	opt := viewOptionFor(target)
	return []app.Option{app.Size(unit.Dp(opt.size.X), unit.Dp(opt.size.Y))}
}

// startTimer começa a contar o tempo sem mudar a visualização: o botão
// Iniciar só liga o cronômetro, a janela fica como está.
func (a *App) startTimer() {
	a.tmr.Start()
}

// pauseTimer pausa a contagem e grava o trecho recém-fechado em disco.
func (a *App) pauseTimer() {
	a.tmr.Pause()
	a.flushSessions()
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

// openAttachment abre um anexo da tarefa e avisa no rodapé se falhar.
func (a *App) openAttachment(ref string) {
	if err := openExternal(ref); err != nil {
		a.setError("Não foi possível abrir: " + err.Error())
	}
}

// startUpdate dispara a recompilação em segundo plano; o resultado chega pelo
// canal e é tratado em pollUpdate, para a interface não congelar no build.
func (a *App) startUpdate() {
	if a.updating {
		return
	}
	root, err := projectRoot()
	if err != nil {
		a.setError(err.Error())
		return
	}
	a.updating = true
	a.setInfo("Recompilando o app…")
	ch := make(chan updateResult, 1)
	a.updateCh = ch
	go func() {
		exe, err := rebuild(root)
		ch <- updateResult{exe: exe, err: err}
	}()
}

// pollUpdate confere a cada quadro se a recompilação terminou. No sucesso,
// salva o estado, lança o binário novo e fecha esta instância.
func (a *App) pollUpdate() {
	if a.updateCh == nil {
		return
	}
	select {
	case res := <-a.updateCh:
		a.updateCh = nil
		a.updating = false
		if res.err != nil {
			a.setError(res.err.Error())
			return
		}
		// O estado vai para o disco antes de o app novo abrir e carregá-lo.
		for _, s := range a.tmr.Stop() {
			a.state.AddSession(s)
		}
		a.save()
		cmd := exec.Command(res.exe)
		cmd.Dir = filepath.Dir(res.exe)
		if err := cmd.Start(); err != nil {
			a.setError("Build ok, mas não consegui reabrir o app: " + err.Error())
			return
		}
		// A partir daqui quem manda no arquivo é o app novo.
		a.handedOff = true
		a.closeWindow()
	default:
	}
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

// completeTask marca a tarefa como concluída num momento escolhido, que nem
// sempre é agora: uma tarefa feita ontem e riscada da lista hoje precisa pesar
// no relatório de ontem.
func (a *App) completeTask(id string, at time.Time) {
	t, ok := a.state.Task(id)
	if !ok {
		return
	}
	if err := a.state.SetDone(id, true, at); err != nil {
		a.setError(err.Error())
		return
	}
	a.save()
	// Concluir tira a tarefa da lista de pendentes; sem o aviso o sumiço
	// pareceria um erro da tela.
	a.setInfo("Tarefa “" + truncate(t.Title, 30) + "” concluída em " + formatDateTime(at) + ".")
}

// toggleToday inclui ou tira a tarefa da lista do dia.
func (a *App) toggleToday(id string) {
	t, ok := a.state.Task(id)
	if !ok {
		return
	}
	if err := a.state.SetToday(id, !t.Today, time.Now()); err != nil {
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
