package ui

import (
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/dvet/keep-it-burning/internal/model"
)

// taskViewScreen mostra os detalhes de uma tarefa, sem edição. Abre pelo
// botão "ver" das listas e guarda a tela de origem para voltar ao mesmo
// lugar — abrir e fechar a visualização não mexe no cronômetro.
type taskViewScreen struct {
	taskID   string
	returnTo screenID

	back  widget.Clickable
	check widget.Clickable
	list  widget.List
}

func (s *taskViewScreen) init(a *App) {
	s.list.Axis = layout.Vertical
}

// open aponta a tela para uma tarefa e registra de onde ela foi aberta.
func (s *taskViewScreen) open(id string, returnTo screenID) {
	s.taskID = id
	s.returnTo = returnTo
}

func (s *taskViewScreen) Layout(gtx layout.Context, a *App) layout.Dimensions {
	if s.back.Clicked(gtx) {
		a.goTo(s.returnTo)
	}
	if s.check.Clicked(gtx) {
		a.toggleDone(s.taskID)
	}

	t, ok := a.state.Task(s.taskID)
	if !ok {
		// A tarefa sumiu (excluída em outra tela); volta sem desenhar nada.
		a.goTo(s.returnTo)
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	prio, hasPrio := a.state.Priority(a.mode, t.PriorityID)
	prioLabel := "—"
	if hasPrio {
		prioLabel = prio.Title + " · " + itoa(prio.Value)
	}
	catLabel := "—"
	if cat, ok := a.state.Category(a.mode, t.CategoryID); ok {
		catLabel = cat.Title
	}

	dueColor := colorInk
	if t.Overdue(time.Now()) {
		dueColor = colorDanger
	}

	status := "Pendente"
	statusColor := colorAccentSoft
	if t.Done {
		status = "Concluída"
		statusColor = colorAccent
	}

	return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.th.panelFill(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							b := a.th.button("←")
							b.PadX, b.PadY = unit.Dp(13), unit.Dp(5)
							b.Size = unit.Sp(18)
							b.Radius = unit.Dp(9)
							return b.Layout(gtx, a.th, &s.back)
						}),
						layout.Rigid(spacerX(10).Layout),
						layout.Rigid(a.th.heading("Tarefa").Layout),
						layout.Flexed(1, layout.Spacer{}.Layout),
						layout.Rigid(a.th.label(unit.Sp(13), status, statusColor).Layout),
					)
				}),
				layout.Rigid(spacerY(12).Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return material.List(a.th.Theme, &s.list).Layout(gtx, 1,
						func(gtx layout.Context, _ int) layout.Dimensions {
							return s.details(gtx, a, t, prioLabel, catLabel, dueColor)
						})
				}),
				layout.Rigid(spacerY(10).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return checkbox(gtx, a.th, &s.check, t.Done)
						}),
						layout.Rigid(spacerX(9).Layout),
						layout.Rigid(a.th.body("Marcar como concluída").Layout),
					)
				}),
			)
		})
	})
}

// details é o corpo rolável da visualização: título, datas e o resumo.
func (s *taskViewScreen) details(gtx layout.Context, a *App, t model.Task, prioLabel, catLabel string, dueColor color.NRGBA) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(a.th.label(unit.Sp(19), t.Title, colorInk).Layout),
		layout.Rigid(spacerY(4).Layout),
		layout.Rigid(a.th.small(a.mode.Label()).Layout),
		layout.Rigid(spacerY(12).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return statLine(gtx, a.th, "Prioridade", prioLabel)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return statLine(gtx, a.th, "Categoria", catLabel)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return statLineColor(gtx, a.th, "Data incluída", formatDateTime(t.CreatedAt), colorInk)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return statLineColor(gtx, a.th, "Data limite", formatDateTime(t.DueAt), dueColor)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !t.Done {
				return layout.Dimensions{}
			}
			return statLineColor(gtx, a.th, "Concluída em", formatDateTime(t.DoneAt), colorInk)
		}),
		layout.Rigid(spacerY(10).Layout),
		layout.Rigid(separator),
		layout.Rigid(spacerY(10).Layout),
		layout.Rigid(a.th.small("Resumo da tarefa").Layout),
		layout.Rigid(spacerY(4).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			desc := t.Description
			c := colorInk
			if desc == "" {
				desc = "Sem resumo."
				c = colorInkFaint
			}
			return a.th.label(unit.Sp(14), desc, c).Layout(gtx)
		}),
	)
}

// statLineColor é o statLine com cor própria no valor, para destacar prazos
// vencidos na visualização.
func statLineColor(gtx layout.Context, th *Theme, label, value string, c color.NRGBA) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
			layout.Flexed(1, th.small(label).Layout),
			layout.Rigid(spacerX(8).Layout),
			layout.Rigid(th.label(unit.Sp(14), value, c).Layout),
		)
	})
}
