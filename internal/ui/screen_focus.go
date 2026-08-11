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

// focusScreen é a janela reduzida que fica aberta durante a sessão: só a
// fogueira, as tarefas do dia, o botão de pausa e o cronômetro.
type focusScreen struct {
	expand widget.Clickable
	pause  widget.Clickable
	rows   rowSet
	list   widget.List
}

func (s *focusScreen) init(a *App) {
	s.rows = rowSet{}
	s.list.Axis = layout.Vertical
}

func (s *focusScreen) Layout(gtx layout.Context, a *App) layout.Dimensions {
	if s.expand.Clicked(gtx) {
		a.leaveFocus()
	}
	if s.pause.Clicked(gtx) {
		a.tmr.Toggle()
		// Ao pausar, o trecho recém-fechado já vai para o disco: uma queda de
		// energia depois disso não apaga o tempo trabalhado.
		a.flushSessions()
	}

	running := a.tmr.Running()
	elapsed := a.tmr.Elapsed()
	intensity := a.dayIntensity()

	tasks := a.state.TodayTasks(a.mode)
	alive := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		alive[t.ID] = true
	}
	s.rows.prune(alive)

	return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.th.panelFill(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(a.th.label(unit.Sp(13), a.mode.Label(), colorInkSoft).Layout),
						layout.Flexed(1, layout.Spacer{}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return expandButton(gtx, a.th, &s.expand)
						}),
					)
				}),
				layout.Flexed(1.3, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = gtx.Constraints.Max
					return Fire{Intensity: intensity, Time: a.animTime(gtx)}.Layout(gtx)
				}),
				layout.Rigid(spacerY(4).Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if len(tasks) == 0 {
						return a.th.small("Sem tarefas do dia.").Layout(gtx)
					}
					return material.List(a.th.Theme, &s.list).Layout(gtx, len(tasks),
						func(gtx layout.Context, i int) layout.Dimensions {
							return s.item(gtx, a, tasks[i])
						})
				}),
				layout.Rigid(spacerY(8).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return s.footer(gtx, a, running, elapsed)
				}),
			)
		})
	})
}

// item é uma tarefa do dia na janela reduzida.
func (s *focusScreen) item(gtx layout.Context, a *App, t model.Task) layout.Dimensions {
	row := s.rows.get(t.ID)
	if row.check.Clicked(gtx) {
		a.toggleDone(t.ID)
	}
	c := colorInk
	if t.Done {
		c = colorInkFaint
	}
	return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, a.th.label(unit.Sp(15), truncate(t.Title, 22), c).Layout),
				layout.Rigid(spacerX(8).Layout),
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
