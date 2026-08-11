package ui

import (
	"errors"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/dvet/keep-it-burning/internal/model"
)

// taskFormScreen é a tela de criar/editar tarefa.
type taskFormScreen struct {
	// editing guarda o ID da tarefa em edição; vazio significa tarefa nova.
	editing string

	title widget.Editor
	due   widget.Editor
	desc  widget.Editor

	priorityID   string
	priorityBtns []widget.Clickable

	today widget.Bool

	save   widget.Clickable
	cancel widget.Clickable

	err string
}

func (s *taskFormScreen) init(a *App) {
	s.title.SingleLine = true
	s.title.Submit = true
	s.due.SingleLine = true
	s.desc.SingleLine = false
}

// openNew prepara o formulário para uma tarefa nova.
func (s *taskFormScreen) openNew(a *App) {
	s.editing = ""
	s.title.SetText("")
	s.due.SetText("")
	s.desc.SetText("")
	s.today.Value = false
	s.err = ""

	// A prioridade mais pesada vem pré-selecionada: é a escolha mais comum e
	// evita que a tarefa nasça sem peso nenhum no score.
	prios := a.state.SettingsFor(a.mode).Priorities
	s.priorityID = ""
	if len(prios) > 0 {
		s.priorityID = prios[0].ID
	}
	s.syncPriorityButtons(a)
}

// openEdit carrega uma tarefa existente no formulário.
func (s *taskFormScreen) openEdit(a *App, t model.Task) {
	s.editing = t.ID
	s.title.SetText(t.Title)
	s.due.SetText(formatDateInput(t.DueAt))
	s.desc.SetText(t.Description)
	s.today.Value = t.Today
	s.priorityID = t.PriorityID
	s.err = ""
	s.syncPriorityButtons(a)
}

// syncPriorityButtons garante um botão por prioridade configurada.
func (s *taskFormScreen) syncPriorityButtons(a *App) {
	n := len(a.state.SettingsFor(a.mode).Priorities)
	if len(s.priorityBtns) != n {
		s.priorityBtns = make([]widget.Clickable, n)
	}
}

func (s *taskFormScreen) Layout(gtx layout.Context, a *App) layout.Dimensions {
	s.syncPriorityButtons(a)
	prios := a.state.SettingsFor(a.mode).Priorities

	for i := range s.priorityBtns {
		if s.priorityBtns[i].Clicked(gtx) && i < len(prios) {
			s.priorityID = prios[i].ID
		}
	}
	if s.cancel.Clicked(gtx) {
		a.goTo(screenDashboard)
	}
	// Enter no título salva, como em qualquer formulário curto.
	submitted := false
	for {
		ev, ok := s.title.Update(gtx)
		if !ok {
			break
		}
		if _, isSubmit := ev.(widget.SubmitEvent); isSubmit {
			submitted = true
		}
	}
	if s.save.Clicked(gtx) || submitted {
		if s.commit(a) {
			a.goTo(screenDashboard)
		}
	}

	heading := "Nova tarefa"
	if s.editing != "" {
		heading = "Editar tarefa"
	}

	return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.th.panelFill(gtx, unit.Dp(18), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(a.th.heading(heading).Layout),
						layout.Flexed(1, layout.Spacer{}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.th.button("Salvar").Layout(gtx, a.th, &s.save)
						}),
						layout.Rigid(spacerX(8).Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							b := a.th.button("Cancelar")
							b.Fg = colorInkSoft
							return b.Layout(gtx, a.th, &s.cancel)
						}),
					)
				}),
				layout.Rigid(spacerY(16).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.th.field(gtx, "Nome da tarefa", &s.title, "Integrar sistema com api")
				}),
				layout.Rigid(spacerY(12).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx,
						layout.Flexed(1.4, func(gtx layout.Context) layout.Dimensions {
							return s.priorityField(gtx, a, prios)
						}),
						layout.Rigid(spacerX(16).Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.th.field(gtx, "Data para entrega", &s.due, "dd/mm/aaaa hh:mm")
						}),
					)
				}),
				layout.Rigid(spacerY(12).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return s.today.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return checkboxStatic(gtx, s.today.Value)
							})
						}),
						layout.Rigid(spacerX(9).Layout),
						layout.Rigid(a.th.body("Incluir nas tarefas do dia").Layout),
					)
				}),
				layout.Rigid(spacerY(14).Layout),
				layout.Rigid(a.th.small("Resumo da tarefa").Layout),
				layout.Rigid(spacerY(4).Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
					return a.th.editorBox(gtx, &s.desc, "Descreva rapidamente o que precisa ser feito", unit.Dp(90))
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if s.err == "" {
						return layout.Dimensions{}
					}
					return layout.Inset{Top: unit.Dp(8)}.Layout(gtx,
						a.th.label(unit.Sp(13), s.err, colorDanger).Layout)
				}),
			)
		})
	})
}

// priorityField desenha as prioridades como botões selecionáveis.
func (s *taskFormScreen) priorityField(gtx layout.Context, a *App, prios []model.Priority) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(a.th.small("Prioridade").Layout),
		layout.Rigid(spacerY(6).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(prios) == 0 {
				return a.th.small("Nenhuma prioridade configurada.").Layout(gtx)
			}
			children := make([]layout.FlexChild, 0, len(prios)*2)
			for i, p := range prios {
				i, p := i, p
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					b := a.th.button(p.Title + " · " + itoa(p.Value))
					b.Size, b.PadX, b.PadY = unit.Sp(13), unit.Dp(12), unit.Dp(7)
					b.Radius = unit.Dp(9)
					if p.ID == s.priorityID {
						b.Bg = colorAccent
						b.Fg = colorPaper
						b.Emphasis = true
					}
					return b.Layout(gtx, a.th, &s.priorityBtns[i])
				}))
				children = append(children, layout.Rigid(spacerX(6).Layout))
			}
			return layout.Flex{}.Layout(gtx, children...)
		}),
	)
}

// commit valida e grava o formulário. Devolve true se conseguiu salvar.
func (s *taskFormScreen) commit(a *App) bool {
	title := strings.TrimSpace(s.title.Text())
	if title == "" {
		s.err = "A tarefa precisa de um nome."
		return false
	}

	due, err := parseDateTime(s.due.Text())
	if err != nil && !errors.Is(err, ErrEmptyDate) {
		s.err = err.Error()
		return false
	}
	if errors.Is(err, ErrEmptyDate) {
		due = time.Time{} // sem prazo é um estado válido
	}

	task := model.Task{
		ID:          s.editing,
		Mode:        a.mode,
		Title:       title,
		Description: strings.TrimSpace(s.desc.Text()),
		PriorityID:  s.priorityID,
		DueAt:       due,
		Today:       s.today.Value,
	}

	if s.editing == "" {
		task.CreatedAt = time.Now()
		if _, err := a.state.AddTask(task); err != nil {
			s.err = err.Error()
			return false
		}
		a.setInfo("Tarefa criada.")
	} else {
		// Preserva o que o formulário não edita: conclusão e data dela.
		if old, ok := a.state.Task(s.editing); ok {
			task.Done = old.Done
			task.DoneAt = old.DoneAt
		}
		if err := a.state.UpdateTask(task); err != nil {
			s.err = err.Error()
			return false
		}
		a.setInfo("Tarefa atualizada.")
	}

	s.err = ""
	a.save()
	return true
}

// checkboxStatic desenha uma caixinha sem clique próprio, para uso dentro de
// um widget.Bool que já captura o toque.
func checkboxStatic(gtx layout.Context, checked bool) layout.Dimensions {
	return checkboxShape(gtx, checked)
}
