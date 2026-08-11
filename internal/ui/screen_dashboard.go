package ui

import (
	"strconv"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/dvet/keep-it-burning/internal/model"
	"github.com/dvet/keep-it-burning/internal/productivity"
	"github.com/dvet/keep-it-burning/internal/timer"
)

// taskRow guarda os widgets de uma linha de tarefa. Como as linhas mudam a
// cada quadro, o estado de clique precisa viver fora do laço de desenho.
type taskRow struct {
	check widget.Clickable
	edit  widget.Clickable
	del   widget.Clickable
	today widget.Clickable
}

// rowSet é um conjunto de linhas indexado pelo ID da tarefa.
type rowSet map[string]*taskRow

// get devolve (criando se preciso) os widgets de uma tarefa.
func (r rowSet) get(id string) *taskRow {
	row, ok := r[id]
	if !ok {
		row = &taskRow{}
		r[id] = row
	}
	return row
}

// prune descarta os widgets de tarefas que saíram da lista, para o mapa não
// crescer indefinidamente ao longo do dia.
func (r rowSet) prune(alive map[string]bool) {
	for id := range r {
		if !alive[id] {
			delete(r, id)
		}
	}
}

// dashboardScreen é o menu principal: resumo do dia, fogueira, tarefas do dia
// e a lista de tarefas pendentes.
type dashboardScreen struct {
	back   widget.Clickable
	config widget.Clickable
	start  widget.Clickable
	more   widget.Clickable
	add    widget.Clickable

	todayRows   rowSet
	pendingRows rowSet

	todayList   widget.List
	pendingList widget.List
}

func (s *dashboardScreen) init(a *App) {
	s.todayRows = rowSet{}
	s.pendingRows = rowSet{}
	s.todayList.Axis = layout.Vertical
	s.pendingList.Axis = layout.Vertical
}

func (s *dashboardScreen) Layout(gtx layout.Context, a *App) layout.Dimensions {
	if s.back.Clicked(gtx) {
		a.goTo(screenHome)
	}
	if s.config.Clicked(gtx) {
		a.settings.load(a)
		a.goTo(screenSettings)
	}
	if s.more.Clicked(gtx) {
		a.goTo(screenStats)
	}
	if s.add.Clicked(gtx) {
		a.form.openNew(a)
		a.goTo(screenTaskForm)
	}
	if s.start.Clicked(gtx) {
		a.enterFocus()
	}

	rep := a.report(productivity.PeriodDay)

	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.th.panelFill(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return s.topBar(gtx, a)
				}),
				layout.Rigid(spacerY(12).Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return s.upperRow(gtx, a, rep)
				}),
				layout.Rigid(spacerY(12).Layout),
				layout.Flexed(1.05, func(gtx layout.Context) layout.Dimensions {
					return s.pendingPanel(gtx, a)
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

// topBar desenha a seta de voltar e o acesso às configurações.
func (s *dashboardScreen) topBar(gtx layout.Context, a *App) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			b := a.th.button("←")
			b.PadX, b.PadY = unit.Dp(13), unit.Dp(5)
			b.Size = unit.Sp(18)
			b.Radius = unit.Dp(9)
			return b.Layout(gtx, a.th, &s.back)
		}),
		layout.Rigid(spacerX(12).Layout),
		layout.Rigid(a.th.heading(a.mode.Label()).Layout),
		layout.Flexed(1, layout.Spacer{}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.th.button("Configurações").Layout(gtx, a.th, &s.config)
		}),
	)
}

// upperRow monta as três colunas do meio: resumo, fogueira e tarefas do dia.
func (s *dashboardScreen) upperRow(gtx layout.Context, a *App, rep productivity.Report) layout.Dimensions {
	return layout.Flex{Alignment: layout.Start}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			return s.summaryPanel(gtx, a, rep)
		}),
		layout.Rigid(spacerX(12).Layout),
		layout.Flexed(1.15, func(gtx layout.Context) layout.Dimensions {
			return s.firePanel(gtx, a, rep)
		}),
		layout.Rigid(spacerX(12).Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			return s.todayPanel(gtx, a)
		}),
	)
}

// summaryPanel é o cartão "Produtividade do dia".
func (s *dashboardScreen) summaryPanel(gtx layout.Context, a *App, rep productivity.Report) layout.Dimensions {
	return a.th.panelFill(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(a.th.heading("Produtividade do dia").Layout),
			layout.Rigid(spacerY(10).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
					layout.Rigid(a.th.label(unit.Sp(38), formatScore(rep.Score), colorAccent).Layout),
					layout.Rigid(spacerX(6).Layout),
					layout.Rigid(a.th.small("/ 100").Layout),
				)
			}),
			layout.Rigid(spacerY(4).Layout),
			layout.Rigid(a.th.label(unit.Sp(13), productivity.StageOf(rep.Score).Label(), colorAccentSoft).Layout),
			layout.Rigid(spacerY(12).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Tarefas concluídas hoje", itoa(rep.DoneCount))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Tarefas em aberto", itoa(rep.PendingCount))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Tempo total trabalhado", timer.FormatHM(rep.Worked))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Meta do dia", timer.FormatHM(rep.Target))
			}),
			layout.Flexed(1, layout.Spacer{}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					b := a.th.button("Ver mais")
					b.Size, b.PadX, b.PadY = unit.Sp(13), unit.Dp(14), unit.Dp(7)
					b.Radius = unit.Dp(9)
					return b.Layout(gtx, a.th, &s.more)
				})
			}),
		)
	})
}

// statLine desenha um par rótulo/valor alinhado nas bordas.
func statLine(gtx layout.Context, th *Theme, label, value string) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
			layout.Flexed(1, th.small(label).Layout),
			layout.Rigid(spacerX(8).Layout),
			layout.Rigid(th.label(unit.Sp(14), value, colorInk).Layout),
		)
	})
}

// firePanel mostra a fogueira do dia e o botão Iniciar.
func (s *dashboardScreen) firePanel(gtx layout.Context, a *App, rep productivity.Report) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return Fire{
				Intensity: productivity.Intensity(rep.Score),
				Time:      a.animTime(gtx),
			}.Layout(gtx)
		}),
		layout.Rigid(spacerY(6).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.th.primary("Iniciar").Layout(gtx, a.th, &s.start)
		}),
	)
}

// todayPanel é o cartão "Tarefas do dia", com as caixinhas de conclusão.
func (s *dashboardScreen) todayPanel(gtx layout.Context, a *App) layout.Dimensions {
	tasks := a.state.TodayTasks(a.mode)

	alive := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		alive[t.ID] = true
	}
	s.todayRows.prune(alive)

	return a.th.panelFill(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(a.th.heading("Tarefas do dia").Layout),
			layout.Rigid(spacerY(10).Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if len(tasks) == 0 {
					return a.th.small("Marque tarefas como “tarefa do dia” na lista abaixo.").Layout(gtx)
				}
				return material.List(a.th.Theme, &s.todayList).Layout(gtx, len(tasks),
					func(gtx layout.Context, i int) layout.Dimensions {
						return s.todayItem(gtx, a, tasks[i])
					})
			}),
		)
	})
}

// todayItem desenha uma linha da lista de tarefas do dia.
func (s *dashboardScreen) todayItem(gtx layout.Context, a *App, t model.Task) layout.Dimensions {
	row := s.todayRows.get(t.ID)
	if row.check.Clicked(gtx) {
		a.toggleDone(t.ID)
	}
	txtColor := colorInk
	if t.Done {
		txtColor = colorInkFaint
	}
	return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3), Right: unit.Dp(4)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, a.th.label(unit.Sp(14), truncate(t.Title, 26), txtColor).Layout),
				layout.Rigid(spacerX(8).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return checkbox(gtx, a.th, &row.check, t.Done)
				}),
			)
		})
}

// pendingPanel é a tabela inferior de tarefas pendentes.
func (s *dashboardScreen) pendingPanel(gtx layout.Context, a *App) layout.Dimensions {
	tasks := a.state.PendingTasks(a.mode)

	alive := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		alive[t.ID] = true
	}
	s.pendingRows.prune(alive)

	return a.th.panelFill(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(a.th.heading("Tarefas Pendentes").Layout),
					layout.Flexed(1, layout.Spacer{}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						b := a.th.button("+ Nova tarefa")
						b.Size, b.PadX, b.PadY = unit.Sp(14), unit.Dp(16), unit.Dp(7)
						b.Radius = unit.Dp(9)
						return b.Layout(gtx, a.th, &s.add)
					}),
				)
			}),
			layout.Rigid(spacerY(10).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return pendingHeader(gtx, a.th)
			}),
			layout.Rigid(spacerY(4).Layout),
			layout.Rigid(separator),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if len(tasks) == 0 {
					return layout.Inset{Top: unit.Dp(14)}.Layout(gtx,
						a.th.small("Nenhuma tarefa pendente. Use “+ Nova tarefa” para começar.").Layout)
				}
				return material.List(a.th.Theme, &s.pendingList).Layout(gtx, len(tasks),
					func(gtx layout.Context, i int) layout.Dimensions {
						return s.pendingItem(gtx, a, tasks[i])
					})
			}),
		)
	})
}

// Larguras relativas das colunas da tabela de pendentes.
const (
	colTitle    = 0.30
	colCreated  = 0.15
	colDue      = 0.15
	colPriority = 0.12
	colActions  = 0.28
)

func pendingHeader(gtx layout.Context, th *Theme) layout.Dimensions {
	head := func(txt string) layout.Widget {
		return th.label(unit.Sp(13), txt, colorInkSoft).Layout
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(colTitle, head("Tarefa")),
		layout.Flexed(colCreated, head("Data Incluída")),
		layout.Flexed(colDue, head("Data Limite")),
		layout.Flexed(colPriority, head("Prioridade")),
		layout.Flexed(colActions, head("Ações")),
	)
}

// pendingItem desenha uma linha da tabela com as ações da tarefa.
func (s *dashboardScreen) pendingItem(gtx layout.Context, a *App, t model.Task) layout.Dimensions {
	row := s.pendingRows.get(t.ID)

	if row.edit.Clicked(gtx) {
		a.form.openEdit(a, t)
		a.goTo(screenTaskForm)
	}
	if row.del.Clicked(gtx) {
		a.deleteTask(t.ID)
	}
	if row.today.Clicked(gtx) {
		a.toggleToday(t.ID)
	}

	prio, hasPrio := a.state.Priority(a.mode, t.PriorityID)
	prioLabel := "—"
	prioColor := colorInkFaint
	if hasPrio {
		prioLabel = prio.Title
		prioColor = colorInk
	}

	dueColor := colorInk
	if t.Overdue(time.Now()) {
		dueColor = colorDanger
	}

	todayLabel := "tarefa do dia"
	if t.Today {
		todayLabel = "tirar do dia"
	}

	return layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Right: unit.Dp(4)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(colTitle, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(a.th.label(unit.Sp(14), truncate(t.Title, 34), colorInk).Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if t.Description == "" {
								return layout.Dimensions{}
							}
							return a.th.label(unit.Sp(12), truncate(t.Description, 42), colorInkFaint).Layout(gtx)
						}),
					)
				}),
				layout.Flexed(colCreated, a.th.label(unit.Sp(13), formatDateTime(t.CreatedAt), colorInkSoft).Layout),
				layout.Flexed(colDue, a.th.label(unit.Sp(13), formatDateTime(t.DueAt), dueColor).Layout),
				layout.Flexed(colPriority, a.th.label(unit.Sp(13), prioLabel, prioColor).Layout),
				layout.Flexed(colActions, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.th.tiny("editar").Layout(gtx, a.th, &row.edit)
						}),
						layout.Rigid(spacerX(5).Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							b := a.th.tiny("excluir")
							b.Fg = colorDanger
							return b.Layout(gtx, a.th, &row.del)
						}),
						layout.Rigid(spacerX(5).Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							b := a.th.tiny(todayLabel)
							if t.Today {
								b.Bg = colorHover
							}
							return b.Layout(gtx, a.th, &row.today)
						}),
					)
				}),
			)
		})
}

// itoa é um atalho para converter contagens em texto nas telas.
func itoa(v int) string { return strconv.Itoa(v) }
