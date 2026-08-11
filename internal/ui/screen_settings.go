package ui

import (
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/dvet/keep-it-burning/internal/model"
)

// removeColumnWidth fixa a largura da coluna do botão "remover", para que o
// cabeçalho e as linhas da tabela de prioridades usem a mesma grade.
const removeColumnWidth = unit.Dp(74)

// priorityRow são os campos de uma prioridade na tela de configuração.
type priorityRow struct {
	id     string
	title  widget.Editor
	value  widget.Editor
	remove widget.Clickable
}

// settingsScreen edita as prioridades e as metas de tempo do modo atual.
type settingsScreen struct {
	rows []*priorityRow

	daily  widget.Editor
	weekly widget.Editor

	add    widget.Clickable
	save   widget.Clickable
	back   widget.Clickable
	toWork widget.Clickable
	toStud widget.Clickable

	list widget.List
	err  string
}

func (s *settingsScreen) init(a *App) {
	s.daily.SingleLine = true
	s.weekly.SingleLine = true
	s.list.Axis = layout.Vertical
}

// load traz para os campos a configuração atual do modo.
func (s *settingsScreen) load(a *App) {
	cfg := a.state.SettingsFor(a.mode)

	s.rows = make([]*priorityRow, 0, len(cfg.Priorities))
	for _, p := range cfg.Priorities {
		s.rows = append(s.rows, newPriorityRow(p))
	}
	s.daily.SetText(formatDurationInput(cfg.DailyTarget))
	s.weekly.SetText(formatDurationInput(cfg.WeeklyTarget))
	s.err = ""
}

func newPriorityRow(p model.Priority) *priorityRow {
	r := &priorityRow{id: p.ID}
	r.title.SingleLine = true
	r.value.SingleLine = true
	r.title.SetText(p.Title)
	r.value.SetText(itoa(p.Value))
	return r
}

func (s *settingsScreen) Layout(gtx layout.Context, a *App) layout.Dimensions {
	if s.back.Clicked(gtx) {
		a.goTo(screenDashboard)
	}
	if s.add.Clicked(gtx) {
		s.rows = append(s.rows, newPriorityRow(model.Priority{ID: model.NewID(), Title: "", Value: 1}))
	}
	if s.toWork.Clicked(gtx) && a.mode != model.ModeWork {
		a.setMode(model.ModeWork)
		s.load(a)
	}
	if s.toStud.Clicked(gtx) && a.mode != model.ModeStudy {
		a.setMode(model.ModeStudy)
		s.load(a)
	}
	for i := len(s.rows) - 1; i >= 0; i-- {
		if s.rows[i].remove.Clicked(gtx) {
			s.rows = append(s.rows[:i], s.rows[i+1:]...)
		}
	}
	if s.save.Clicked(gtx) {
		if s.commit(a) {
			a.goTo(screenDashboard)
		}
	}

	return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.th.panelFill(gtx, unit.Dp(18), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return s.header(gtx, a)
				}),
				layout.Rigid(spacerY(16).Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Start}.Layout(gtx,
						layout.Flexed(1.3, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
							return s.prioritiesPanel(gtx, a)
						}),
						layout.Rigid(spacerX(16).Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
							return s.targetsPanel(gtx, a)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if s.err == "" {
						return layout.Dimensions{}
					}
					return layout.Inset{Top: unit.Dp(10)}.Layout(gtx,
						a.th.label(unit.Sp(13), s.err, colorDanger).Layout)
				}),
			)
		})
	})
}

// header traz o seletor de modo e os botões de voltar e salvar.
func (s *settingsScreen) header(gtx layout.Context, a *App) layout.Dimensions {
	modeBtn := func(m model.Mode, click *widget.Clickable) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			b := a.th.button(m.Label())
			b.Size, b.PadX, b.PadY = unit.Sp(15), unit.Dp(20), unit.Dp(8)
			if a.mode == m {
				b.Bg = colorAccent
				b.Fg = colorPaper
				b.Emphasis = true
			}
			return b.Layout(gtx, a.th, click)
		}
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			b := a.th.button("←")
			b.PadX, b.PadY = unit.Dp(13), unit.Dp(5)
			b.Size, b.Radius = unit.Sp(18), unit.Dp(9)
			return b.Layout(gtx, a.th, &s.back)
		}),
		layout.Rigid(spacerX(14).Layout),
		layout.Rigid(modeBtn(model.ModeWork, &s.toWork)),
		layout.Rigid(spacerX(8).Layout),
		layout.Rigid(modeBtn(model.ModeStudy, &s.toStud)),
		layout.Flexed(1, layout.Spacer{}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.th.button("Salvar").Layout(gtx, a.th, &s.save)
		}),
	)
}

// prioritiesPanel lista as prioridades editáveis.
func (s *settingsScreen) prioritiesPanel(gtx layout.Context, a *App) layout.Dimensions {
	return a.th.panelFill(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(a.th.heading("Prioridades").Layout),
					layout.Flexed(1, layout.Spacer{}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						b := a.th.button("+ Adicionar")
						b.Size, b.PadX, b.PadY = unit.Sp(13), unit.Dp(13), unit.Dp(6)
						b.Radius = unit.Dp(9)
						return b.Layout(gtx, a.th, &s.add)
					}),
				)
			}),
			layout.Rigid(spacerY(4).Layout),
			layout.Rigid(a.th.small("O valor é o peso da prioridade: quanto maior, mais a tarefa alimenta o fogo.").Layout),
			layout.Rigid(spacerY(12).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, a.th.small("Título").Layout),
					layout.Rigid(spacerX(10).Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Dp(unit.Dp(86))
						return a.th.small("Valor").Layout(gtx)
					}),
					layout.Rigid(spacerX(10).Layout),
					// Espaço reservado para a coluna do botão "remover", para
					// que os títulos fiquem em cima dos campos certos.
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Dp(removeColumnWidth)
						return layout.Dimensions{Size: gtx.Constraints.Min}
					}),
				)
			}),
			layout.Rigid(spacerY(6).Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if len(s.rows) == 0 {
					return a.th.small("Sem prioridades. Adicione ao menos uma.").Layout(gtx)
				}
				return material.List(a.th.Theme, &s.list).Layout(gtx, len(s.rows),
					func(gtx layout.Context, i int) layout.Dimensions {
						return s.priorityRowLayout(gtx, a, s.rows[i])
					})
			}),
		)
	})
}

func (s *settingsScreen) priorityRowLayout(gtx layout.Context, a *App, r *priorityRow) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(8), Right: unit.Dp(4)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.th.editorBox(gtx, &r.title, "Alta", unit.Dp(0))
				}),
				layout.Rigid(spacerX(10).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Dp(unit.Dp(86))
					gtx.Constraints.Max.X = gtx.Constraints.Min.X
					return a.th.editorBox(gtx, &r.value, "5", unit.Dp(0))
				}),
				layout.Rigid(spacerX(10).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Dp(removeColumnWidth)
					b := a.th.tiny("remover")
					b.Fg = colorDanger
					return b.Layout(gtx, a.th, &r.remove)
				}),
			)
		})
}

// targetsPanel edita o tempo médio diário e semanal.
func (s *settingsScreen) targetsPanel(gtx layout.Context, a *App) layout.Dimensions {
	return a.th.panelFill(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(a.th.heading("Tempo médio").Layout),
			layout.Rigid(spacerY(4).Layout),
			layout.Rigid(a.th.small("Metas de tempo do modo "+strings.ToLower(a.mode.Label())+". Aceita 8, 8h30 ou 90min.").Layout),
			layout.Rigid(spacerY(14).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.th.field(gtx, "Diário", &s.daily, "8h")
			}),
			layout.Rigid(spacerY(12).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.th.field(gtx, "Semanal", &s.weekly, "40h")
			}),
			layout.Rigid(spacerY(18).Layout),
			layout.Rigid(separator),
			layout.Rigid(spacerY(12).Layout),
			layout.Rigid(a.th.small("Seus dados ficam em:").Layout),
			layout.Rigid(spacerY(3).Layout),
			layout.Rigid(a.th.label(unit.Sp(11), a.st.Path(), colorInkFaint).Layout),
		)
	})
}

// commit valida e grava as configurações. Devolve true se conseguiu salvar.
func (s *settingsScreen) commit(a *App) bool {
	prios := make([]model.Priority, 0, len(s.rows))
	seen := map[string]bool{}
	for _, r := range s.rows {
		title := strings.TrimSpace(r.title.Text())
		if title == "" {
			s.err = "Toda prioridade precisa de um título."
			return false
		}
		key := strings.ToLower(title)
		if seen[key] {
			s.err = "Há duas prioridades com o título “" + title + "”."
			return false
		}
		seen[key] = true

		value, err := parsePriorityValue(r.value.Text())
		if err != nil {
			s.err = "Prioridade “" + title + "”: " + err.Error()
			return false
		}
		prios = append(prios, model.Priority{ID: r.id, Title: title, Value: value})
	}
	if len(prios) == 0 {
		s.err = "Configure ao menos uma prioridade."
		return false
	}

	daily, err := parseDurationInput(s.daily.Text())
	if err != nil {
		s.err = "Tempo diário: " + err.Error()
		return false
	}
	weekly, err := parseDurationInput(s.weekly.Text())
	if err != nil {
		s.err = "Tempo semanal: " + err.Error()
		return false
	}
	if daily <= 0 || weekly <= 0 {
		s.err = "As metas de tempo precisam ser maiores que zero."
		return false
	}
	if weekly < daily {
		s.err = "A meta semanal não pode ser menor que a diária."
		return false
	}

	a.state.SetSettings(a.mode, model.ModeSettings{
		Priorities:   prios,
		DailyTarget:  daily,
		WeeklyTarget: weekly,
	})
	s.err = ""
	a.save()
	a.setInfo("Configurações salvas.")
	return true
}
