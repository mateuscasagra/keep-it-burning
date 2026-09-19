package ui

import (
	"errors"
	"strings"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/dvet/keep-it-burning/internal/model"
)

// dueFilter é o recorte por prazo da tela expandida.
type dueFilter int

const (
	dueAny dueFilter = iota
	dueOverdue
	dueToday
	dueWeek
	dueNone
)

// dueFilters é a lista de recortes na ordem em que aparecem na tela. O recorte
// "Todos" tem id vazio, que é como o campo de escolha mostra "sem filtro".
var dueFilters = []struct {
	kind  dueFilter
	id    string
	label string
}{
	{dueAny, "", "Todos"},
	{dueOverdue, "vencidas", "Vencidas"},
	{dueToday, "hoje", "Vencem hoje"},
	{dueWeek, "semana", "Próximos 7 dias"},
	{dueNone, "sem-prazo", "Sem prazo"},
}

// dueOptions e dueFilterFor traduzem o recorte de prazo para o campo de escolha
// e de volta.
func dueOptions() []selectOption {
	out := make([]selectOption, len(dueFilters))
	for i, f := range dueFilters {
		out[i] = selectOption{id: f.id, title: f.label}
	}
	return out
}

func dueFilterFor(id string) dueFilter {
	for _, f := range dueFilters {
		if f.id == id {
			return f.kind
		}
	}
	return dueAny
}

func dueFilterID(kind dueFilter) string {
	for _, f := range dueFilters {
		if f.kind == kind {
			return f.id
		}
	}
	return ""
}

// match informa se a tarefa entra no recorte de prazo.
func (f dueFilter) match(t model.Task, now time.Time) bool {
	switch f {
	case dueOverdue:
		return t.Overdue(now)
	case dueToday:
		return t.HasDue() && sameDay(t.DueAt, now)
	case dueWeek:
		// A partir de agora até daqui a sete dias; o que já venceu tem recorte
		// próprio e não deve poluir a lista do que está por vir.
		return t.HasDue() && !t.DueAt.Before(now) && t.DueAt.Before(now.AddDate(0, 0, 7))
	case dueNone:
		return !t.HasDue()
	}
	return true
}

// todayFilter é o recorte pela marca "tarefa do dia".
type todayFilter int

const (
	todayAny todayFilter = iota
	todayOnly
	todayOut
)

// todayFilters é a lista de recortes na ordem em que aparecem na tela.
var todayFilters = []struct {
	kind  todayFilter
	id    string
	label string
}{
	{todayAny, "", "Todas"},
	{todayOnly, "so-do-dia", "Só do dia"},
	{todayOut, "fora-do-dia", "Fora do dia"},
}

func todayOptions() []selectOption {
	out := make([]selectOption, len(todayFilters))
	for i, f := range todayFilters {
		out[i] = selectOption{id: f.id, title: f.label}
	}
	return out
}

func todayFilterFor(id string) todayFilter {
	for _, f := range todayFilters {
		if f.id == id {
			return f.kind
		}
	}
	return todayAny
}

func todayFilterID(kind todayFilter) string {
	for _, f := range todayFilters {
		if f.kind == kind {
			return f.id
		}
	}
	return ""
}

// match informa se a tarefa entra no recorte da lista do dia.
func (f todayFilter) match(t model.Task) bool {
	switch f {
	case todayOnly:
		return t.Today
	case todayOut:
		return !t.Today
	}
	return true
}

// sameDay compara duas datas pelo dia do calendário local.
func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// noneFilter é o valor dos filtros "Sem categoria", "Sem subcategoria" e "Sem
// dificuldade". Começa com "@" justamente para não colidir com um ID
// configurado, que é sempre hexadecimal.
const noneFilter = "@sem-valor"

// pendingScreen é a tela cheia das tarefas pendentes: a mesma tabela do
// dashboard, mas com espaço para a busca e os filtros, que ficam sempre à
// vista — abrir a tela já deixa o cursor na busca.
type pendingScreen struct {
	back   widget.Clickable
	add    widget.Clickable
	clear  widget.Clickable
	wipe   widget.Clickable
	search widget.Editor
	// focusSearch pede o foco do campo de busca no próximo quadro; o comando
	// de foco só vale dentro de um layout.Context.
	focusSearch bool

	// Os filtros de lista são menus suspensos: em fila de botões, cada lista
	// nova empurrava a faixa de filtros para baixo e roubava a altura da tabela.
	priorityID string
	prioSel    selectField

	difficultyID string
	diffSel      selectField

	categoryID string
	catSel     selectField

	subcategoryID string
	subSel        selectField

	due    dueFilter
	dueSel selectField

	today    todayFilter
	todaySel selectField

	group selectGroup

	// Intervalos de data: um pela data de inclusão, outro pelo prazo.
	created  dateRange
	dueRange dateRange

	// donePrompt é a pergunta da data de conclusão, mostrada por cima da lista.
	donePrompt doneDatePrompt

	rows rowSet
	list widget.List
}

func (s *pendingScreen) init(a *App) {
	s.rows = rowSet{}
	s.donePrompt.init()
	s.search.SingleLine = true
	s.list.Axis = layout.Vertical
	s.group.init(&s.prioSel, &s.diffSel, &s.catSel, &s.subSel, &s.dueSel, &s.todaySel)
	s.created.init()
	s.dueRange.init()
}

// open zera busca e filtros e pede o foco do campo de busca: quem expande a
// lista quase sempre quer procurar alguma coisa.
func (s *pendingScreen) open(a *App) {
	s.reset()
	s.donePrompt.close()
	s.focusSearch = true
}

// reset limpa a busca e devolve os filtros ao estado "tudo".
func (s *pendingScreen) reset() {
	s.search.SetText("")
	s.priorityID = ""
	s.categoryID = ""
	s.subcategoryID = ""
	s.difficultyID = ""
	s.due = dueAny
	s.today = todayAny
	s.created.reset()
	s.dueRange.reset()
	s.group.closeAll()
}

func (s *pendingScreen) Layout(gtx layout.Context, a *App) layout.Dimensions {
	s.donePrompt.update(gtx, a)
	cfg := a.state.SettingsFor(a.mode)
	prioOpts := filterOptions("Todas", "", priorityOptions(cfg.Priorities))
	diffOpts := filterOptions("Todas", "Sem dificuldade", difficultyOptions(cfg.Difficulties)[1:])
	catOpts := filterOptions("Todas", "Sem categoria", categoryOptions(cfg.Categories)[1:])
	subOpts := filterOptions("Todas", "Sem subcategoria", subcategoryOptions(cfg.Subcategories)[1:])

	if id, ok := s.prioSel.update(gtx, prioOpts); ok {
		s.priorityID = id
	}
	if id, ok := s.diffSel.update(gtx, diffOpts); ok {
		s.difficultyID = id
	}
	if id, ok := s.catSel.update(gtx, catOpts); ok {
		s.categoryID = id
	}
	if id, ok := s.subSel.update(gtx, subOpts); ok {
		s.subcategoryID = id
	}
	if id, ok := s.dueSel.update(gtx, dueOptions()); ok {
		s.due = dueFilterFor(id)
	}
	if id, ok := s.todaySel.update(gtx, todayOptions()); ok {
		s.today = todayFilterFor(id)
	}
	s.group.update(gtx)
	s.created.update()
	s.dueRange.update()

	if s.focusSearch {
		s.focusSearch = false
		gtx.Execute(key.FocusCmd{Tag: &s.search})
	}
	if s.back.Clicked(gtx) {
		a.goTo(screenDashboard)
	}
	if s.add.Clicked(gtx) {
		a.form.openNew(a, screenPending)
		a.goTo(screenTaskForm)
	}
	if s.clear.Clicked(gtx) {
		s.reset()
		s.focusSearch = true
	}
	if s.wipe.Clicked(gtx) {
		s.search.SetText("")
		s.focusSearch = true
	}

	all := a.state.PendingTasks(a.mode)
	tasks := s.filter(all)

	alive := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		alive[t.ID] = true
	}
	s.rows.prune(alive)

	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return s.content(gtx, a, cfg, prioOpts, diffOpts, catOpts, subOpts, all, tasks)
		}),
		// O véu dos menus fica entre a tela e o menu, que é desenhado por último.
		layout.Stacked(s.group.scrimLayout),
		// A pergunta da data de conclusão é desenhada por último: o véu dela
		// cobre a lista e segura os cliques enquanto a resposta não vem.
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return s.donePrompt.overlay(gtx, a)
		}),
	)
}

// content é a tela em si: barra de cima, busca, filtros e a tabela.
func (s *pendingScreen) content(gtx layout.Context, a *App, cfg model.ModeSettings,
	prioOpts, diffOpts, catOpts, subOpts []selectOption, all, tasks []model.Task) layout.Dimensions {

	cols := taskColsFor(cfg)
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.th.panelFill(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return s.topBar(gtx, a)
				}),
				layout.Rigid(spacerY(12).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return s.searchBar(gtx, a)
				}),
				layout.Rigid(spacerY(10).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return s.filters(gtx, a, cfg, prioOpts, diffOpts, catOpts, subOpts)
				}),
				layout.Rigid(spacerY(12).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(a.th.small("Mostrando "+itoa(len(tasks))+" de "+itoa(len(all))+" tarefas pendentes").Layout),
						layout.Flexed(1, layout.Spacer{}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.th.tiny("limpar filtros").Layout(gtx, a.th, &s.clear)
						}),
					)
				}),
				layout.Rigid(spacerY(8).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return cols.header(gtx, a.th, pendingActionsWidth, &s.list)
				}),
				layout.Rigid(spacerY(4).Layout),
				layout.Rigid(separator),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if len(tasks) == 0 {
						return layout.Inset{Top: unit.Dp(16)}.Layout(gtx,
							a.th.small(s.emptyMessage(len(all))).Layout)
					}
					return material.List(a.th.Theme, &s.list).Layout(gtx, len(tasks),
						func(gtx layout.Context, i int) layout.Dimensions {
							return s.item(gtx, a, tasks[i], cols)
						})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if a.notice == "" {
						return layout.Dimensions{}
					}
					return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, a.layoutNotice)
				}),
			)
		})
	})
}

// emptyMessage explica por que a lista está vazia: sem tarefas ou sem
// resultado para o filtro, que são coisas diferentes para quem está olhando.
func (s *pendingScreen) emptyMessage(total int) string {
	if total == 0 {
		return "Nenhuma tarefa pendente. Use “+ Nova tarefa” para começar."
	}
	return "Nenhuma tarefa pendente bate com a busca e os filtros atuais."
}

// topBar traz o voltar, o título e o atalho de criar tarefa.
func (s *pendingScreen) topBar(gtx layout.Context, a *App) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			b := a.th.button("←")
			b.PadX, b.PadY = unit.Dp(13), unit.Dp(5)
			b.Size = unit.Sp(18)
			b.Radius = unit.Dp(9)
			return b.Layout(gtx, a.th, &s.back)
		}),
		layout.Rigid(spacerX(12).Layout),
		layout.Rigid(a.th.heading("Tarefas Pendentes").Layout),
		layout.Rigid(spacerX(10).Layout),
		layout.Rigid(a.th.small("· "+a.mode.Label()).Layout),
		layout.Flexed(1, layout.Spacer{}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.th.button("+ Nova tarefa").Layout(gtx, a.th, &s.add)
		}),
	)
}

// searchBar é o campo de busca, sempre aberto no topo da tela.
func (s *pendingScreen) searchBar(gtx layout.Context, a *App) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.th.editorBox(gtx, &s.search, "Buscar por nome ou resumo da tarefa", unit.Dp(0))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if s.search.Text() == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.th.tiny("limpar").Layout(gtx, a.th, &s.wipe)
			})
		}),
	)
}

// filterOptions monta as opções de um filtro de lista: "Todas" na frente e,
// quando o campo é opcional na tarefa, o "Sem ..." logo depois.
func filterOptions(all, none string, items []selectOption) []selectOption {
	out := make([]selectOption, 0, len(items)+2)
	out = append(out, selectOption{title: all})
	if none != "" {
		out = append(out, selectOption{id: noneFilter, title: none})
	}
	return append(out, items...)
}

// filters desenha a faixa de filtros: os campos de escolha numa linha e os
// intervalos de data na outra.
func (s *pendingScreen) filters(gtx layout.Context, a *App, cfg model.ModeSettings,
	prioOpts, diffOpts, catOpts, subOpts []selectOption) layout.Dimensions {

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.selectRow(gtx, a, cfg, prioOpts, diffOpts, catOpts, subOpts)
		}),
		layout.Rigid(spacerY(12).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.rangeRow(gtx, a)
		}),
	)
}

// selectRow põe os filtros de lista lado a lado. Os que dependem de listas
// configuráveis só aparecem quando o modo tem itens cadastrados.
func (s *pendingScreen) selectRow(gtx layout.Context, a *App, cfg model.ModeSettings,
	prioOpts, diffOpts, catOpts, subOpts []selectOption) layout.Dimensions {

	sel := func(f *selectField, label string, opts []selectOption, selected string) layout.FlexChild {
		return layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return f.layout(gtx, a.th, label, opts, selected)
		})
	}
	gap := layout.Rigid(spacerX(12).Layout)

	children := []layout.FlexChild{sel(&s.prioSel, "Prioridade", prioOpts, s.priorityID)}
	if len(cfg.Difficulties) > 0 {
		children = append(children, gap, sel(&s.diffSel, "Dificuldade", diffOpts, s.difficultyID))
	}
	if len(cfg.Categories) > 0 {
		children = append(children, gap, sel(&s.catSel, "Categoria", catOpts, s.categoryID))
	}
	if len(cfg.Subcategories) > 0 {
		children = append(children, gap, sel(&s.subSel, "Subcategoria", subOpts, s.subcategoryID))
	}
	children = append(children,
		gap, sel(&s.dueSel, "Prazo", dueOptions(), dueFilterID(s.due)),
		gap, sel(&s.todaySel, "Tarefas do dia", todayOptions(), todayFilterID(s.today)),
	)
	return layout.Flex{Alignment: layout.Start}.Layout(gtx, children...)
}

// rangeRow põe os dois intervalos de data lado a lado.
func (s *pendingScreen) rangeRow(gtx layout.Context, a *App) layout.Dimensions {
	return layout.Flex{Alignment: layout.Start}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.created.layout(gtx, a.th, "Incluída entre")
		}),
		layout.Rigid(spacerX(28).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.dueRange.layout(gtx, a.th, "Data limite entre")
		}),
		layout.Flexed(1, layout.Spacer{}.Layout),
	)
}

// filter aplica a busca e os filtros sobre as pendentes já ordenadas.
func (s *pendingScreen) filter(tasks []model.Task) []model.Task {
	now := time.Now()
	q := strings.ToLower(strings.TrimSpace(s.search.Text()))

	out := make([]model.Task, 0, len(tasks))
	for _, t := range tasks {
		if s.priorityID != "" && t.PriorityID != s.priorityID {
			continue
		}
		if !matchesOptional(s.categoryID, t.CategoryID) {
			continue
		}
		if !matchesOptional(s.subcategoryID, t.SubcategoryID) {
			continue
		}
		if !matchesOptional(s.difficultyID, t.DifficultyID) {
			continue
		}
		if !s.due.match(t, now) {
			continue
		}
		if !s.created.match(t.CreatedAt) {
			continue
		}
		if !s.dueRange.match(t.DueAt) {
			continue
		}
		if !s.today.match(t) {
			continue
		}
		if q != "" && !matchesQuery(t, q) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// matchesOptional aplica um filtro de lista configurável: vazio aceita tudo,
// noneFilter só aceita tarefa sem o campo preenchido, e um ID exige aquele
// valor exato.
func matchesOptional(filter, value string) bool {
	switch filter {
	case "":
		return true
	case noneFilter:
		return value == ""
	default:
		return value == filter
	}
}

// matchesQuery procura o termo no nome e no resumo da tarefa.
func matchesQuery(t model.Task, q string) bool {
	return strings.Contains(strings.ToLower(t.Title), q) ||
		strings.Contains(strings.ToLower(t.Description), q)
}

// pendingActionsWidth é a largura da coluna de ações da tela expandida, que
// tem um botão a mais que a do dashboard: o de concluir.
const pendingActionsWidth = unit.Dp(318)

// item é uma linha da tabela expandida.
func (s *pendingScreen) item(gtx layout.Context, a *App, t model.Task, cols taskCols) layout.Dimensions {
	row := s.rows.get(t.ID)

	if row.check.Clicked(gtx) {
		// Tarefa do dia é marcada no próprio dia, então vale a hora de agora.
		// As outras costumam ser riscadas da lista dias depois de prontas, e aí
		// a data precisa ser perguntada — é ela que os relatórios usam.
		if t.Today {
			a.completeTask(t.ID, time.Now())
		} else {
			s.donePrompt.open(t)
		}
	}
	if row.edit.Clicked(gtx) {
		a.form.openEdit(a, t, screenPending)
		a.goTo(screenTaskForm)
	}
	if row.del.Clicked(gtx) {
		a.deleteTask(t.ID)
	}
	if row.today.Clicked(gtx) {
		a.toggleToday(t.ID)
	}

	todayLabel := "tarefa do dia"
	if t.Today {
		todayLabel = "tirar do dia"
	}

	return cols.row(gtx, a, t, pendingActionsWidth, func(gtx layout.Context) layout.Dimensions {
		return actionButtons(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return a.th.tiny("concluir").Layout(gtx, a.th, &row.check)
			},
			func(gtx layout.Context) layout.Dimensions {
				return a.th.tiny("editar").Layout(gtx, a.th, &row.edit)
			},
			func(gtx layout.Context) layout.Dimensions {
				b := a.th.tiny("excluir")
				b.Fg = colorDanger
				return b.Layout(gtx, a.th, &row.del)
			},
			func(gtx layout.Context) layout.Dimensions {
				b := a.th.tiny(todayLabel)
				if t.Today {
					b.Bg = colorHover
				}
				return b.Layout(gtx, a.th, &row.today)
			},
		)
	})
}

// doneDatePrompt pergunta em que momento a tarefa foi concluída. Ela só aparece
// para tarefas que não estão na lista do dia: essas costumam ser marcadas dias
// depois de prontas, e carimbar a hora do clique jogaria o trabalho para o
// relatório do dia errado.
type doneDatePrompt struct {
	taskID string
	title  string

	date widget.Editor
	// last é o texto do quadro anterior, usado pela máscara que formata a data
	// enquanto o usuário digita.
	last string
	err  string

	confirm widget.Clickable
	cancel  widget.Clickable
	scrim   widget.Clickable
	// body engole os cliques que caem na caixa. Sem ele um clique no texto da
	// pergunta atravessaria até o véu, que fecha tudo.
	body widget.Clickable

	// focus pede o foco do campo no próximo quadro; o comando de foco só vale
	// dentro de um layout.Context.
	focus bool
}

func (p *doneDatePrompt) init() {
	p.date.SingleLine = true
	p.date.Submit = true
}

// open abre a pergunta já preenchida com o momento de agora, que é a resposta
// mais comum: quem acabou de terminar a tarefa só confirma.
func (p *doneDatePrompt) open(t model.Task) {
	p.taskID = t.ID
	p.title = t.Title
	p.date.SetText(formatDateInput(time.Now()))
	p.last = p.date.Text()
	p.err = ""
	p.focus = true
}

// close fecha a pergunta sem concluir nada.
func (p *doneDatePrompt) close() {
	p.taskID = ""
	p.err = ""
}

// isOpen informa se a pergunta está na tela.
func (p *doneDatePrompt) isOpen() bool { return p.taskID != "" }

// update trata a digitação e os botões. Confirmar conclui a tarefa na data
// informada; cancelar e o clique fora apenas fecham.
func (p *doneDatePrompt) update(gtx layout.Context, a *App) {
	if !p.isOpen() {
		return
	}
	// "16092026" vira "16/09/2026" sem o usuário digitar as barras.
	maskDateField(&p.date, &p.last, 12)

	// Enter confirma, como em qualquer caixa de diálogo de um campo só.
	submitted := false
	for {
		ev, ok := p.date.Update(gtx)
		if !ok {
			break
		}
		if _, isSubmit := ev.(widget.SubmitEvent); isSubmit {
			submitted = true
		}
	}
	// O clique na própria caixa não faz nada, mas precisa ser consumido.
	p.body.Clicked(gtx)
	if p.cancel.Clicked(gtx) || p.scrim.Clicked(gtx) {
		p.close()
		return
	}
	if !p.confirm.Clicked(gtx) && !submitted {
		return
	}
	at, err := parseDoneDate(p.date.Text(), time.Now())
	if err != nil {
		p.err = err.Error()
		return
	}
	id := p.taskID
	p.close()
	a.completeTask(id, at)
}

// parseDoneDate lê a data de conclusão digitada. Conclusão no futuro não
// existe: a única exceção é a data de hoje sem hora, que parseDateTime empurra
// para o fim do dia e aqui volta a ser o momento de agora.
func parseDoneDate(txt string, now time.Time) (time.Time, error) {
	at, err := parseDateTime(txt)
	switch {
	case errors.Is(err, ErrEmptyDate):
		return time.Time{}, errors.New("informe a data em que a tarefa foi concluída")
	case err != nil:
		return time.Time{}, err
	}
	if at.After(now) {
		if sameDay(at, now) {
			return now, nil
		}
		return time.Time{}, errors.New("a data de conclusão não pode estar no futuro")
	}
	return at, nil
}

// overlay desenha a pergunta por cima da lista, com um véu que escurece o
// fundo e segura os cliques da tela de trás.
func (p *doneDatePrompt) overlay(gtx layout.Context, a *App) layout.Dimensions {
	if !p.isOpen() {
		return layout.Dimensions{}
	}
	if p.focus {
		p.focus = false
		gtx.Execute(key.FocusCmd{Tag: &p.date})
	}
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return p.scrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Max
				paint.FillShape(gtx.Ops, colorScrim, clip.Rect{Max: size}.Op())
				return layout.Dimensions{Size: size}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return p.body.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return p.card(gtx, a)
				})
			})
		}),
	)
}

// card é a caixa da pergunta.
func (p *doneDatePrompt) card(gtx layout.Context, a *App) layout.Dimensions {
	w := min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(380)))
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
	return a.th.panel(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(a.th.heading("Quando a tarefa foi concluída?").Layout),
			layout.Rigid(spacerY(6).Layout),
			layout.Rigid(a.th.small("“"+truncate(p.title, 44)+"” não está nas tarefas do dia, "+
				"então a data da conclusão precisa ser informada.").Layout),
			layout.Rigid(spacerY(12).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.th.field(gtx, "Data da conclusão", &p.date, "dd/mm/aaaa hh:mm")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.err == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Top: unit.Dp(6)}.Layout(gtx,
					a.th.label(unit.Sp(13), p.err, colorDanger).Layout)
			}),
			layout.Rigid(spacerY(16).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, layout.Spacer{}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						b := a.th.button("Cancelar")
						b.Size, b.PadX, b.PadY = unit.Sp(14), unit.Dp(16), unit.Dp(8)
						b.Radius = unit.Dp(9)
						b.Fg = colorInkSoft
						return b.Layout(gtx, a.th, &p.cancel)
					}),
					layout.Rigid(spacerX(8).Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						b := a.th.button("Concluir")
						b.Size, b.PadX, b.PadY = unit.Sp(14), unit.Dp(16), unit.Dp(8)
						b.Radius = unit.Dp(9)
						b.Emphasis = true
						return b.Layout(gtx, a.th, &p.confirm)
					}),
				)
			}),
		)
	})
}
