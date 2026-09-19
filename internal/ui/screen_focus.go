package ui

import (
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/dvet/keep-it-burning/internal/model"
	"github.com/dvet/keep-it-burning/internal/timer"
)

// focusScreen é a janela reduzida que fica aberta durante a sessão: a
// fogueira, o botão de pausa, o cronômetro e — fora da visualização mini — as
// tarefas do dia.
type focusScreen struct {
	expand widget.Clickable
	pause  widget.Clickable
	views  viewPicker
	rows   rowSet
	list   widget.List
}

func (s *focusScreen) init(a *App) {
	s.rows = rowSet{}
	s.list.Axis = layout.Vertical
}

func (s *focusScreen) Layout(gtx layout.Context, a *App) layout.Dimensions {
	s.views.update(gtx, a)
	if s.expand.Clicked(gtx) {
		a.setView(viewFull)
	}
	if s.pause.Clicked(gtx) {
		a.tmr.Toggle()
		// Ao pausar, o trecho recém-fechado já vai para o disco: uma queda de
		// energia depois disso não apaga o tempo trabalhado.
		a.flushSessions()
	}

	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return s.content(gtx, a)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return s.views.overlay(gtx, a, unit.Dp(58), unit.Dp(22))
		}),
	)
}

// content é o corpo da janela reduzida: fogueira, tarefas do dia e relógio.
func (s *focusScreen) content(gtx layout.Context, a *App) layout.Dimensions {
	running := a.tmr.Running()
	elapsed := a.tmr.Elapsed()
	intensity := a.dayIntensity()
	diaHoraAtual := time.Now()
	tasks := a.state.TodayTasks(a.mode, diaHoraAtual)
	alive := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		alive[t.ID] = true
	}
	s.rows.prune(alive)

	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.topBar(gtx, a)
		}),
		// A fogueira cede um pouco de espaço para a lista: com os grupos, ela
		// precisa de mais linhas para caber sem virar um espremido só.
		layout.Flexed(1.1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return Fire{Intensity: intensity, Time: a.animTime(gtx)}.Layout(gtx)
		}),
	}
	// As tarefas vão para a tela agrupadas por subcategoria, na ordem em que as
	// subcategorias estão configuradas.
	entries := groupBySubcategory(a.state, a.mode, tasks)

	children = append(children,
		layout.Rigid(spacerY(4).Layout),
		layout.Flexed(1.4, func(gtx layout.Context) layout.Dimensions {
			if len(tasks) == 0 {
				return a.th.small("Sem tarefas do dia.").Layout(gtx)
			}
			return material.List(a.th.Theme, &s.list).Layout(gtx, len(entries),
				func(gtx layout.Context, i int) layout.Dimensions {
					if entries[i].isHeader {
						return s.groupHeader(gtx, a, entries[i], i == 0)
					}
					return s.item(gtx, a, entries[i].task)
				})
		}),
		layout.Rigid(spacerY(8).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.footer(gtx, a, running, elapsed)
		}),
	)

	return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.th.panelFill(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

// topBar traz o modo, o seletor de visualização e o atalho de voltar ao
// tamanho cheio.
func (s *focusScreen) topBar(gtx layout.Context, a *App) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(a.th.label(unit.Sp(13), a.mode.Label(), colorInkSoft).Layout),
		layout.Flexed(1, layout.Spacer{}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.views.button(gtx, a, true)
		}),
		layout.Rigid(spacerX(6).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return expandButton(gtx, a.th, &s.expand)
		}),
	)
}

// focusEntry é uma linha da lista da janela reduzida: ou o cabeçalho de uma
// subcategoria, ou uma tarefa.
type focusEntry struct {
	isHeader bool
	// Preenchidos só no cabeçalho: o nome do grupo e quanto dele já saiu.
	header string
	done   int
	total  int

	task model.Task
}

// semSub é o rótulo do grupo das tarefas que ficaram sem subcategoria.
const semSub = "Sem subcategoria"

// groupBySubcategory organiza as tarefas do dia por subcategoria, na ordem em
// que elas estão configuradas, com as sem subcategoria por último. Com um grupo
// só a lista volta a ser plana: um cabeçalho que não separa nada de nada só
// gastaria as poucas linhas que a janela reduzida tem.
func groupBySubcategory(st *model.State, mode model.Mode, tasks []model.Task) []focusEntry {
	subs := st.SettingsFor(mode).Subcategories
	order := make(map[string]int, len(subs))
	for i, c := range subs {
		order[c.ID] = i
	}

	// A última posição é o grupo "sem subcategoria"; tarefa cuja subcategoria
	// foi apagada da configuração cai nele também.
	groups := make([][]model.Task, len(subs)+1)
	used := 0
	for _, t := range tasks {
		i, ok := order[t.SubcategoryID]
		if !ok {
			i = len(subs)
		}
		if len(groups[i]) == 0 {
			used++
		}
		groups[i] = append(groups[i], t)
	}

	entries := make([]focusEntry, 0, len(tasks)+used)
	if used <= 1 {
		for _, t := range tasks {
			entries = append(entries, focusEntry{task: t})
		}
		return entries
	}
	for i, g := range groups {
		if len(g) == 0 {
			continue
		}
		title := semSub
		if i < len(subs) {
			title = subs[i].Title
		}
		done := 0
		for _, t := range g {
			if t.Done {
				done++
			}
		}
		entries = append(entries, focusEntry{isHeader: true, header: title, done: done, total: len(g)})
		for _, t := range g {
			entries = append(entries, focusEntry{task: t})
		}
	}
	return entries
}

// groupHeader é a faixa que abre um grupo: o nome da subcategoria e o quanto
// dela já foi riscado.
func (s *focusScreen) groupHeader(gtx layout.Context, a *App, e focusEntry, first bool) layout.Dimensions {
	top := unit.Dp(10)
	if first {
		top = unit.Dp(0)
	}
	return layout.Inset{Top: top, Bottom: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, a.th.cell(unit.Sp(12), e.header, colorInkSoft).Layout),
						layout.Rigid(spacerX(6).Layout),
						layout.Rigid(a.th.label(unit.Sp(11),
							itoa(e.done)+"/"+itoa(e.total), colorInkFaint).Layout),
					)
				}),
				layout.Rigid(spacerY(3).Layout),
				layout.Rigid(separator),
			)
		})
}

// item é uma tarefa do dia na janela reduzida.
func (s *focusScreen) item(gtx layout.Context, a *App, t model.Task) layout.Dimensions {
	row := s.rows.get(t.ID)
	if row.check.Clicked(gtx) {
		a.toggleDone(t.ID)
	}
	if row.view.Clicked(gtx) {
		// Abrir a visualização não mexe no cronômetro: a sessão continua.
		a.view.open(t.ID, screenFocus)
		a.goTo(screenTaskView)
	}
	c := colorInk
	if t.Done {
		c = colorInkFaint
	}
	return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, a.th.label(unit.Sp(15), truncate(t.Title, 18), c).Layout),
				layout.Rigid(spacerX(8).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.th.tiny("ver").Layout(gtx, a.th, &row.view)
				}),
				layout.Rigid(spacerX(6).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return checkbox(gtx, a.th, &row.check, t.Done)
				}),
			)
		})
}

// footer traz o botão de pausa e o cronômetro.
func (s *focusScreen) footer(gtx layout.Context, a *App, running bool, elapsed time.Duration) layout.Dimensions {
	label := "Pausar"
	if !running {
		label = "Retomar"
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			b := a.th.button(label)
			b.Size, b.PadX, b.PadY = unit.Sp(16), unit.Dp(20), unit.Dp(10)
			if !running {
				b.Bg = colorAccent
				b.Fg = colorPaper
				b.Emphasis = true
			}
			return b.Layout(gtx, a.th, &s.pause)
		}),
		layout.Rigid(spacerX(12).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			c := colorInk
			if !running {
				c = colorInkFaint
			}
			return a.th.label(unit.Sp(24), timer.Format(elapsed), c).Layout(gtx)
		}),
	)
}
