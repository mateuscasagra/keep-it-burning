package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/dvet/keep-it-burning/internal/productivity"
	"github.com/dvet/keep-it-burning/internal/timer"
)

// statsScreen é o dashboard de produtividade: score, gráfico de pizza e os
// indicadores médios, com seletor de dia, semana ou mês.
type statsScreen struct {
	back    widget.Clickable
	periods []widget.Clickable
	period  productivity.Period
}

func (s *statsScreen) init(a *App) {
	s.period = productivity.PeriodDay
	s.periods = make([]widget.Clickable, len(productivity.Periods))
}

func (s *statsScreen) Layout(gtx layout.Context, a *App) layout.Dimensions {
	if s.back.Clicked(gtx) {
		a.goTo(screenDashboard)
	}
	for i := range s.periods {
		if s.periods[i].Clicked(gtx) {
			s.period = productivity.Periods[i]
		}
	}

	rep := a.report(s.period)

	return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.th.panelFill(gtx, unit.Dp(18), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return s.header(gtx, a)
				}),
				layout.Rigid(spacerY(16).Layout),
				layout.Flexed(1.2, func(gtx layout.Context) layout.Dimensions {
					return s.scoreRow(gtx, a, rep)
				}),
				layout.Rigid(spacerY(14).Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
					return s.indicatorsPanel(gtx, a, rep)
				}),
			)
		})
	})
}

func (s *statsScreen) header(gtx layout.Context, a *App) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			b := a.th.button("←")
			b.PadX, b.PadY = unit.Dp(13), unit.Dp(5)
			b.Size, b.Radius = unit.Sp(18), unit.Dp(9)
			return b.Layout(gtx, a.th, &s.back)
		}),
		layout.Rigid(spacerX(14).Layout),
		layout.Rigid(a.th.heading("Produtividade · " + a.mode.Label()).Layout),
		layout.Flexed(1, layout.Spacer{}.Layout),
	}
	for i, p := range productivity.Periods {
		i, p := i, p
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			b := a.th.button(p.Label())
			b.Size, b.PadX, b.PadY = unit.Sp(14), unit.Dp(16), unit.Dp(7)
			b.Radius = unit.Dp(10)
			if s.period == p {
				b.Bg = colorAccent
				b.Fg = colorPaper
				b.Emphasis = true
			}
			return b.Layout(gtx, a.th, &s.periods[i])
		}))
		children = append(children, layout.Rigid(spacerX(7).Layout))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// scoreRow põe o score grande à esquerda e o gráfico de pizza à direita.
func (s *statsScreen) scoreRow(gtx layout.Context, a *App, rep productivity.Report) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1.1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(a.th.label(unit.Sp(46), "SCORE", colorInk).Layout),
				layout.Rigid(spacerY(2).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
						layout.Rigid(a.th.label(unit.Sp(64), formatScore(rep.Score), colorAccent).Layout),
						layout.Rigid(spacerX(10).Layout),
						layout.Rigid(a.th.small("de 100 no "+labelForPeriod(s.period)).Layout),
					)
				}),
				layout.Rigid(spacerY(10).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return statLine(gtx, a.th, "Tarefas entregues (peso)", pct(rep.TaskRatio))
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return statLine(gtx, a.th, "Meta de tempo cumprida", pct(rep.TimeRatio))
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return statLine(gtx, a.th,
						"Tempo no período",
						timer.FormatHM(rep.Worked)+" de "+timer.FormatHM(rep.Target))
				}),
			)
		}),
		layout.Rigid(spacerX(16).Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = gtx.Constraints.Max
					return PieChart{Slices: rep.Slices}.Layout(gtx, a.th)
				}),
				layout.Rigid(spacerX(12).Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return pieLegend(gtx, a.th, rep.Slices)
				}),
			)
		}),
	)
}

// indicatorsPanel é a caixa de indicadores embaixo do score.
func (s *statsScreen) indicatorsPanel(gtx layout.Context, a *App, rep productivity.Report) layout.Dimensions {
	ind := rep.Indicators
	return a.th.panelFill(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Tempo médio de trabalho por semana", timer.FormatHM(ind.AvgTimePerWeek))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Tempo médio de trabalho por dia", timer.FormatHM(ind.AvgTimePerDay))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Média de tarefas entregues por dia", formatAvg(ind.AvgTasksPerDay))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Média de tarefas entregues por semana", formatAvg(ind.AvgTasksPerWeek))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Tarefas entregues no período", itoa(rep.DoneCount))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Tarefas ainda em aberto", itoa(rep.PendingCount))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Dias com atividade no período", itoa(ind.ActiveDays))
			}),
			layout.Flexed(1, layout.Spacer{}.Layout),
			layout.Rigid(a.th.label(unit.Sp(11),
				"As médias por dia usam apenas os dias em que houve atividade.",
				colorInkFaint).Layout),
		)
	})
}

// labelForPeriod devolve o período por extenso para a frase do score.
func labelForPeriod(p productivity.Period) string {
	switch p {
	case productivity.PeriodWeek:
		return "semana"
	case productivity.PeriodMonth:
		return "mês"
	default:
		return "dia"
	}
}
