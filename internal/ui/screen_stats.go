package ui

import (
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/dvet/keep-it-burning/internal/model"
	"github.com/dvet/keep-it-burning/internal/productivity"
	"github.com/dvet/keep-it-burning/internal/timer"
)

// statsViews são as visões do dashboard de produtividade: a geral e as que
// separam os dados de tarefas e de tempo.
var statsViews = []string{"Visão geral", "Tarefas", "Tempo"}

// statsScreen é o dashboard de produtividade: score, gráfico de pizza e os
// indicadores, com seletor de dia, semana ou mês e as visões separadas.
type statsScreen struct {
	back    widget.Clickable
	periods []widget.Clickable
	period  productivity.Period
	views   []widget.Clickable
	view    int

	// Intervalo livre da lista de concluídas: quando preenchido e aplicado,
	// vale no lugar do filtro dia/semana/mês.
	rangeFrom widget.Editor
	rangeTo   widget.Editor
	// Textos do quadro anterior, para a máscara de data dos campos De/Até.
	rangeFromLast string
	rangeToLast   string
	applyRange    widget.Clickable
	clearRange    widget.Clickable
	customFrom    time.Time
	customTo      time.Time
	rangeErr      string

	doneList widget.List
}

func (s *statsScreen) init(a *App) {
	s.period = productivity.PeriodDay
	s.periods = make([]widget.Clickable, len(productivity.Periods))
	s.views = make([]widget.Clickable, len(statsViews))
	s.rangeFrom.SingleLine = true
	s.rangeTo.SingleLine = true
	s.doneList.Axis = layout.Vertical
}

// applyCustomRange valida os campos De/Até e ativa o intervalo livre.
func (s *statsScreen) applyCustomRange() {
	from, err := parseDateOnly(s.rangeFrom.Text())
	if err != nil {
		s.rangeErr = "De: " + err.Error()
		return
	}
	to, err := parseDateOnly(s.rangeTo.Text())
	if err != nil {
		s.rangeErr = "Até: " + err.Error()
		return
	}
	if to.Before(from) {
		s.rangeErr = "O fim do intervalo não pode vir antes do começo."
		return
	}
	// O dia final entra inteiro: o intervalo é [from, to+24h).
	s.customFrom, s.customTo = from, to.Add(24*time.Hour)
	s.rangeErr = ""
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
	for i := range s.views {
		if s.views[i].Clicked(gtx) {
			s.view = i
		}
	}
	maskDateField(&s.rangeFrom, &s.rangeFromLast, 8)
	maskDateField(&s.rangeTo, &s.rangeToLast, 8)
	if s.applyRange.Clicked(gtx) {
		s.applyCustomRange()
	}
	if s.clearRange.Clicked(gtx) {
		s.customFrom, s.customTo = time.Time{}, time.Time{}
		s.rangeFrom.SetText("")
		s.rangeTo.SetText("")
		s.rangeFromLast, s.rangeToLast = "", ""
		s.rangeErr = ""
	}

	return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.th.panelFill(gtx, unit.Dp(18), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return s.header(gtx, a)
				}),
				layout.Rigid(spacerY(10).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return s.viewTabs(gtx, a)
				}),
				layout.Rigid(spacerY(14).Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					switch s.view {
					case 1:
						gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
						return s.tasksPanel(gtx, a)
					case 2:
						gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
						return s.timePanel(gtx, a)
					default:
						return s.overview(gtx, a)
					}
				}),
			)
		})
	})
}

// viewTabs é o seletor Visão geral / Tarefas / Tempo.
func (s *statsScreen) viewTabs(gtx layout.Context, a *App) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(statsViews)*2)
	for i, label := range statsViews {
		i, label := i, label
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			b := a.th.button(label)
			b.Size, b.PadX, b.PadY = unit.Sp(14), unit.Dp(16), unit.Dp(7)
			b.Radius = unit.Dp(10)
			if s.view == i {
				b.Bg = colorInk
				b.Fg = colorPaper
				b.Emphasis = true
			}
			return b.Layout(gtx, a.th, &s.views[i])
		}))
		children = append(children, layout.Rigid(spacerX(7).Layout))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// overview é a visão geral original: score, pizza e as médias.
func (s *statsScreen) overview(gtx layout.Context, a *App) layout.Dimensions {
	rep := a.report(s.period)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1.2, func(gtx layout.Context) layout.Dimensions {
			return s.scoreRow(gtx, a, rep)
		}),
		layout.Rigid(spacerY(14).Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			return s.indicatorsPanel(gtx, a, rep)
		}),
	)
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

// tasksPanel é a visão só de tarefas: distribuição por prioridade, dificuldade,
// categoria e subcategoria, pontualidade, tempo de entrega e a lista do que foi
// concluído.
//
// O painel inteiro rola junto: o resumo é o primeiro item da lista e as
// concluídas vêm depois. Com o resumo fixo no topo, um modo com muitas
// categorias empurrava a lista de concluídas para fora do painel — ela ficava
// com altura zero e parecia que não havia nada concluído no período.
func (s *statsScreen) tasksPanel(gtx layout.Context, a *App) layout.Dimensions {
	ts := productivity.TaskStatsFor(a.state, a.mode, s.period, time.Now())
	from, to, custom := s.doneRange()
	tasks := productivity.DoneTasksIn(a.state, a.mode, from, to)
	scope := doneScope(s.period, from, to, custom)

	return a.th.panelFill(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return material.List(a.th.Theme, &s.doneList).Layout(gtx, len(tasks)+1,
					func(gtx layout.Context, i int) layout.Dimensions {
						if i == 0 {
							return s.tasksSummary(gtx, a, ts, len(tasks), scope)
						}
						return s.doneTaskRow(gtx, a, tasks[i-1])
					})
			}),
			layout.Rigid(spacerY(6).Layout),
			layout.Rigid(a.th.label(unit.Sp(11),
				"Entregues contam no "+labelForPeriod(s.period)+" selecionado; em aberto é o total atual do modo.",
				colorInkFaint).Layout),
		)
	})
}

// tasksSummary é o bloco que abre a visão: as tabelas, os prazos e o cabeçalho
// da lista de concluídas.
func (s *statsScreen) tasksSummary(gtx layout.Context, a *App, ts productivity.TaskStats,
	doneListLen int, scope string) layout.Dimensions {

	cfg := a.state.SettingsFor(a.mode)
	// Uma tabela por campo configurável. As que dependem de listas opcionais
	// viram um recado quando o modo ainda não tem nada cadastrado.
	table := func(title, firstCol, hint string, lines []productivity.PriorityLine, has bool) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if !has {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(a.th.heading(title).Layout),
					layout.Rigid(spacerY(10).Layout),
					layout.Rigid(a.th.small(hint).Layout),
				)
			}
			return taskTable(gtx, a.th, title, firstCol, lines, ts.DoneCount, ts.OpenCount)
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Start}.Layout(gtx,
				layout.Flexed(1, table("Por prioridade", "Prioridade", "", ts.Lines, true)),
				layout.Rigid(spacerX(20).Layout),
				layout.Flexed(1, table("Por dificuldade", "Dificuldade",
					"Crie dificuldades nas Configurações.", ts.Difficulties, len(cfg.Difficulties) > 0)),
				layout.Rigid(spacerX(20).Layout),
				layout.Flexed(1, table("Por categoria", "Categoria",
					"Crie categorias nas Configurações para fatiar as tarefas aqui.", ts.Categories, len(cfg.Categories) > 0)),
				layout.Rigid(spacerX(20).Layout),
				layout.Flexed(1, table("Por subcategoria", "Subcategoria",
					"Crie subcategorias nas Configurações para fatiar as tarefas aqui.", ts.Subcategories, len(cfg.Subcategories) > 0)),
			)
		}),
		layout.Rigid(spacerY(16).Layout),
		layout.Rigid(a.th.heading("Prazos e entrega").Layout),
		layout.Rigid(spacerY(10).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			v := "—"
			if ts.DoneWithDue > 0 {
				v = pct(ts.LateRatio) + " (" + itoa(ts.DoneLate) + " de " + itoa(ts.DoneWithDue) + " com prazo)"
			}
			return statLine(gtx, a.th, "Entregues fora do prazo", v)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return statLine(gtx, a.th, "Em aberto já vencidas", itoa(ts.OverdueOpen))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return statLine(gtx, a.th, "Em aberto sem data limite", itoa(ts.NoDueOpen))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return statLine(gtx, a.th, "Tempo médio de entrega (criação → conclusão)", formatDelivery(ts.AvgDelivery))
		}),
		layout.Rigid(spacerY(14).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.doneHeader(gtx, a)
		}),
		layout.Rigid(spacerY(6).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if doneListLen == 0 {
				return a.th.small("Nenhuma tarefa concluída " + scope + ".").Layout(gtx)
			}
			return a.th.small(itoa(doneListLen) + " concluída(s) " + scope).Layout(gtx)
		}),
		layout.Rigid(spacerY(6).Layout),
	)
}

// doneScope descreve em palavras o intervalo da lista de concluídas.
func doneScope(p productivity.Period, from, to time.Time, custom bool) string {
	if custom {
		return "de " + formatDate(from) + " até " + formatDate(to.Add(-24*time.Hour))
	}
	return "no " + labelForPeriod(p)
}

// doneRange devolve o intervalo da lista de concluídas: o intervalo livre
// quando aplicado, senão o do filtro dia/semana/mês.
func (s *statsScreen) doneRange() (from, to time.Time, custom bool) {
	if !s.customFrom.IsZero() {
		return s.customFrom, s.customTo, true
	}
	from, to = s.period.Range(time.Now())
	return from, to, false
}

// doneHeader é o título da lista de concluídas com os campos De/Até do
// intervalo livre.
func (s *statsScreen) doneHeader(gtx layout.Context, a *App) layout.Dimensions {
	_, _, custom := s.doneRange()
	dateField := func(ed *widget.Editor, hint string) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(unit.Dp(108))
			gtx.Constraints.Max.X = gtx.Constraints.Min.X
			return a.th.editorBox(gtx, ed, hint, unit.Dp(0))
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(a.th.heading("Tarefas concluídas").Layout),
				layout.Flexed(1, layout.Spacer{}.Layout),
				layout.Rigid(a.th.small("De").Layout),
				layout.Rigid(spacerX(6).Layout),
				dateField(&s.rangeFrom, "dd/mm/aaaa"),
				layout.Rigid(spacerX(8).Layout),
				layout.Rigid(a.th.small("Até").Layout),
				layout.Rigid(spacerX(6).Layout),
				dateField(&s.rangeTo, "dd/mm/aaaa"),
				layout.Rigid(spacerX(8).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					b := a.th.tiny("aplicar")
					if custom {
						b.Bg = colorInk
						b.Fg = colorPaper
					}
					return b.Layout(gtx, a.th, &s.applyRange)
				}),
				layout.Rigid(spacerX(5).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !custom && s.rangeErr == "" {
						return layout.Dimensions{}
					}
					return a.th.tiny("limpar").Layout(gtx, a.th, &s.clearRange)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if s.rangeErr == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(4)}.Layout(gtx,
				a.th.label(unit.Sp(12), s.rangeErr, colorDanger).Layout)
		}),
	)
}

// doneTaskRow é uma linha da lista: quando entregou, o título e a categoria.
func (s *statsScreen) doneTaskRow(gtx layout.Context, a *App, t model.Task) layout.Dimensions {
	catLabel := ""
	if cat, ok := a.state.Category(a.mode, t.CategoryID); ok {
		catLabel = cat.Title
	}
	return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3), Right: unit.Dp(4)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(a.th.label(unit.Sp(13), formatDateTime(t.DoneAt), colorInkSoft).Layout),
				layout.Rigid(spacerX(10).Layout),
				layout.Flexed(1, a.th.label(unit.Sp(14), truncate(t.Title, 48), colorInk).Layout),
				layout.Rigid(spacerX(10).Layout),
				layout.Rigid(a.th.label(unit.Sp(12), catLabel, colorInkFaint).Layout),
			)
		})
}

// taskTable desenha uma tabela entregues/em aberto com linha de total; serve
// tanto para prioridades quanto para categorias.
func taskTable(gtx layout.Context, th *Theme, title, firstCol string, lines []productivity.PriorityLine, doneTotal, openTotal int) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(th.heading(title).Layout),
		layout.Rigid(spacerY(10).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return taskTableRow(gtx, th, firstCol, "Entregues", "Em aberto", colorInkSoft)
		}),
		layout.Rigid(spacerY(4).Layout),
		layout.Rigid(separator),
		layout.Rigid(spacerY(4).Layout),
	}
	for _, line := range lines {
		line := line
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return taskTableRow(gtx, th, line.Label, itoa(line.Done), itoa(line.Open), colorInk)
		}))
	}
	children = append(children,
		layout.Rigid(spacerY(4).Layout),
		layout.Rigid(separator),
		layout.Rigid(spacerY(4).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return taskTableRow(gtx, th, "Total", itoa(doneTotal), itoa(openTotal), colorInk)
		}),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// taskTableRow desenha uma linha da tabela de prioridades em três colunas.
func taskTableRow(gtx layout.Context, th *Theme, label, done, open string, c color.NRGBA) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// O respiro no fim de cada coluna evita que o rótulo encoste no número
		// da coluna seguinte quando as tabelas ficam estreitas.
		col := func(txt string) layout.Widget {
			return func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, th.cell(unit.Sp(13), txt, c).Layout)
			}
		}
		// As colunas de número precisam caber "Entregues" e "Em aberto" por
		// extenso; são quatro tabelas lado a lado, então sobra pouco para cada.
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
			layout.Flexed(0.44, col(label)),
			layout.Flexed(0.28, col(done)),
			layout.Flexed(0.28, col(open)),
		)
	})
}

// timePanel é a visão só de tempo: totais, sessões e o dia mais carregado.
func (s *statsScreen) timePanel(gtx layout.Context, a *App) layout.Dimensions {
	ts := productivity.TimeStatsFor(a.state, a.mode, s.period, time.Now())

	metaPct := "—"
	if ts.Target > 0 {
		ratio := float64(ts.Worked) / float64(ts.Target)
		if ratio > 1 {
			ratio = 1
		}
		metaPct = pct(ratio)
	}
	bestDay := "—"
	if ts.ActiveDays > 0 {
		bestDay = formatDate(ts.BestDay) + " · " + timer.FormatHM(ts.BestDayTime)
	}

	return a.th.panelFill(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(a.th.heading("Tempo no "+labelForPeriod(s.period)).Layout),
			layout.Rigid(spacerY(10).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Tempo trabalhado", timer.FormatHM(ts.Worked)+" de "+timer.FormatHM(ts.Target))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Meta cumprida", metaPct)
			}),
			layout.Rigid(spacerY(12).Layout),
			layout.Rigid(a.th.heading("Sessões").Layout),
			layout.Rigid(spacerY(10).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Sessões cronometradas", itoa(ts.Sessions))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Duração média por sessão", timer.FormatHM(ts.AvgSession))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Sessão mais longa", timer.FormatHM(ts.Longest))
			}),
			layout.Rigid(spacerY(12).Layout),
			layout.Rigid(a.th.heading("Ritmo").Layout),
			layout.Rigid(spacerY(10).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Dias com atividade", itoa(ts.ActiveDays))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Tempo médio por dia ativo", timer.FormatHM(ts.AvgPerDay))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Tempo médio por semana", timer.FormatHM(ts.AvgPerWeek))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return statLine(gtx, a.th, "Dia com mais tempo", bestDay)
			}),
			layout.Flexed(1, layout.Spacer{}.Layout),
			layout.Rigid(a.th.label(unit.Sp(11),
				"Sessões que cruzam a fronteira do período contam só a parte de dentro.",
				colorInkFaint).Layout),
		)
	})
}

// formatDelivery formata o tempo médio de entrega, que pode passar de dias.
func formatDelivery(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	if d >= 48*time.Hour {
		days := int(d.Hours()) / 24
		rest := d - time.Duration(days)*24*time.Hour
		if rest < time.Minute {
			return itoa(days) + "d"
		}
		return itoa(days) + "d " + timer.FormatHM(rest)
	}
	return timer.FormatHM(d)
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
