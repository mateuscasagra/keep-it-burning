package ui

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/dvet/keep-it-burning/internal/model"
)

// linkRow é um link em edição no formulário.
type linkRow struct {
	ref    widget.Editor
	remove widget.Clickable
}

func newLinkRow(ref string) *linkRow {
	r := &linkRow{}
	r.ref.SingleLine = true
	r.ref.SetText(ref)
	return r
}

// taskFormScreen é a tela de criar/editar tarefa.
type taskFormScreen struct {
	// editing guarda o ID da tarefa em edição; vazio significa tarefa nova.
	editing string

	title widget.Editor
	due   widget.Editor
	desc  widget.Editor

	priorityID   string
	priorityBtns []widget.Clickable

	categoryID   string
	categoryBtns []widget.Clickable

	links   []*linkRow
	addLink widget.Clickable

	files      []string
	fileRemove []widget.Clickable
	addFile    widget.Clickable
	// fileCh recebe o caminho escolhido no diálogo nativo, que roda em outra
	// goroutine; picking evita abrir dois diálogos ao mesmo tempo.
	fileCh  chan string
	picking bool

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
	s.fileCh = make(chan string, 1)
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
	s.categoryID = ""
	s.links = nil
	s.files = nil
	s.syncPriorityButtons(a)
	s.syncCategoryButtons(a)
}

// openEdit carrega uma tarefa existente no formulário.
func (s *taskFormScreen) openEdit(a *App, t model.Task) {
	s.editing = t.ID
	s.title.SetText(t.Title)
	s.due.SetText(formatDateInput(t.DueAt))
	s.desc.SetText(t.Description)
	s.today.Value = t.Today
	s.priorityID = t.PriorityID
	s.categoryID = t.CategoryID
	s.links = make([]*linkRow, 0, len(t.Links))
	for _, ref := range t.Links {
		s.links = append(s.links, newLinkRow(ref))
	}
	s.files = append([]string(nil), t.Files...)
	s.err = ""
	s.syncPriorityButtons(a)
	s.syncCategoryButtons(a)
}

// syncPriorityButtons garante um botão por prioridade configurada.
func (s *taskFormScreen) syncPriorityButtons(a *App) {
	n := len(a.state.SettingsFor(a.mode).Priorities)
	if len(s.priorityBtns) != n {
		s.priorityBtns = make([]widget.Clickable, n)
	}
}

// syncCategoryButtons garante um botão por categoria, mais o "Nenhuma" na
// posição zero — categoria é opcional.
func (s *taskFormScreen) syncCategoryButtons(a *App) {
	n := len(a.state.SettingsFor(a.mode).Categories) + 1
	if len(s.categoryBtns) != n {
		s.categoryBtns = make([]widget.Clickable, n)
	}
}

func (s *taskFormScreen) Layout(gtx layout.Context, a *App) layout.Dimensions {
	s.syncPriorityButtons(a)
	s.syncCategoryButtons(a)
	cfg := a.state.SettingsFor(a.mode)
	prios, cats := cfg.Priorities, cfg.Categories

	for i := range s.priorityBtns {
		if s.priorityBtns[i].Clicked(gtx) && i < len(prios) {
			s.priorityID = prios[i].ID
		}
	}
	for i := range s.categoryBtns {
		if !s.categoryBtns[i].Clicked(gtx) {
			continue
		}
		if i == 0 {
			s.categoryID = ""
		} else if i-1 < len(cats) {
			s.categoryID = cats[i-1].ID
		}
	}
	if s.cancel.Clicked(gtx) {
		a.goTo(screenDashboard)
	}
	if s.addLink.Clicked(gtx) {
		s.links = append(s.links, newLinkRow(""))
	}
	for i := len(s.links) - 1; i >= 0; i-- {
		if s.links[i].remove.Clicked(gtx) {
			s.links = append(s.links[:i], s.links[i+1:]...)
		}
	}
	// O caminho escolhido no diálogo chega pelo canal; a tela redesenha a
	// cada quadro, então basta drenar sem bloquear.
	select {
	case path := <-s.fileCh:
		s.picking = false
		if path != "" {
			s.files = append(s.files, path)
		}
	default:
	}
	if s.addFile.Clicked(gtx) && !s.picking {
		s.picking = true
		go s.pickFile(a)
	}
	if len(s.fileRemove) != len(s.files) {
		s.fileRemove = make([]widget.Clickable, len(s.files))
	}
	for i := len(s.files) - 1; i >= 0; i-- {
		if s.fileRemove[i].Clicked(gtx) {
			s.files = append(s.files[:i], s.files[i+1:]...)
		}
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
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					// O campo só aparece quando o modo tem categorias criadas
					// nas configurações; sem elas, a tarefa fica sem categoria.
					if len(cats) == 0 {
						return layout.Dimensions{}
					}
					return layout.Inset{Top: unit.Dp(12)}.Layout(gtx,
						func(gtx layout.Context) layout.Dimensions {
							return s.categoryField(gtx, a, cats)
						})
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
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Start}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return s.linksField(gtx, a)
						}),
						layout.Rigid(spacerX(16).Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return s.filesField(gtx, a)
						}),
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

// categoryField desenha as categorias como botões selecionáveis, com o
// "Nenhuma" na frente porque categoria é opcional.
func (s *taskFormScreen) categoryField(gtx layout.Context, a *App, cats []model.Category) layout.Dimensions {
	catBtn := func(i int, id, title string) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			b := a.th.button(title)
			b.Size, b.PadX, b.PadY = unit.Sp(13), unit.Dp(12), unit.Dp(7)
			b.Radius = unit.Dp(9)
			if id == s.categoryID {
				b.Bg = colorInk
				b.Fg = colorPaper
				b.Emphasis = true
			}
			return b.Layout(gtx, a.th, &s.categoryBtns[i])
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(a.th.small("Categoria").Layout),
		layout.Rigid(spacerY(6).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, (len(cats)+1)*2)
			children = append(children, catBtn(0, "", "Nenhuma"), layout.Rigid(spacerX(6).Layout))
			for i, c := range cats {
				children = append(children, catBtn(i+1, c.ID, c.Title), layout.Rigid(spacerX(6).Layout))
			}
			return layout.Flex{}.Layout(gtx, children...)
		}),
	)
}

// linksField edita a lista de links da tarefa.
func (s *taskFormScreen) linksField(gtx layout.Context, a *App) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(a.th.small("Links").Layout),
				layout.Rigid(spacerX(10).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.th.tiny("+ adicionar link").Layout(gtx, a.th, &s.addLink)
				}),
			)
		}),
	}
	for _, r := range s.links {
		r := r
		children = append(children,
			layout.Rigid(spacerY(6).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.th.editorBox(gtx, &r.ref, "https://exemplo.com", unit.Dp(0))
					}),
					layout.Rigid(spacerX(8).Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						b := a.th.tiny("remover")
						b.Fg = colorDanger
						return b.Layout(gtx, a.th, &r.remove)
					}),
				)
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// filesField lista os arquivos anexados; o "+ adicionar arquivo" abre o
// diálogo nativo do sistema para escolher.
func (s *taskFormScreen) filesField(gtx layout.Context, a *App) layout.Dimensions {
	label := "+ adicionar arquivo"
	if s.picking {
		label = "escolhendo…"
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(a.th.small("Arquivos").Layout),
				layout.Rigid(spacerX(10).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.th.tiny(label).Layout(gtx, a.th, &s.addFile)
				}),
			)
		}),
	}
	for i, path := range s.files {
		i, path := i, path
		children = append(children,
			layout.Rigid(spacerY(6).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, a.th.label(unit.Sp(14), truncate(filepath.Base(path), 34), colorInk).Layout),
					layout.Rigid(spacerX(8).Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if i >= len(s.fileRemove) {
							return layout.Dimensions{}
						}
						b := a.th.tiny("remover")
						b.Fg = colorDanger
						return b.Layout(gtx, a.th, &s.fileRemove[i])
					}),
				)
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// pickFile roda em goroutine própria: o diálogo nativo é bloqueante e não
// pode segurar o laço de desenho. O resultado volta pelo canal.
func (s *taskFormScreen) pickFile(a *App) {
	rc, err := a.expl.ChooseFile()
	path := ""
	if err == nil {
		path = fileDialogPath(rc)
		rc.Close()
	}
	s.fileCh <- path
	a.win.Invalidate()
}

// fileDialogPath extrai o caminho local do arquivo devolvido pelo diálogo.
func fileDialogPath(rc io.ReadCloser) string {
	switch f := rc.(type) {
	case *os.File:
		return f.Name()
	case interface{ URI() string }:
		u := strings.TrimPrefix(f.URI(), "file://")
		// URIs no Windows vêm como /C:/pasta/arquivo; o barra inicial sobra.
		if len(u) > 2 && u[0] == '/' && u[2] == ':' {
			u = u[1:]
		}
		return filepath.FromSlash(u)
	}
	return ""
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
		CategoryID:  s.categoryID,
		Links:       s.collectLinks(),
		Files:       append([]string(nil), s.files...),
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

// collectLinks junta os links preenchidos, descartando linhas vazias.
func (s *taskFormScreen) collectLinks() []string {
	var out []string
	for _, r := range s.links {
		if ref := strings.TrimSpace(r.ref.Text()); ref != "" {
			out = append(out, ref)
		}
	}
	return out
}

// checkboxStatic desenha uma caixinha sem clique próprio, para uso dentro de
// um widget.Bool que já captura o toque.
func checkboxStatic(gtx layout.Context, checked bool) layout.Dimensions {
	return checkboxShape(gtx, checked)
}
