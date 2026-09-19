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
	// returnTo é a tela que abriu o formulário. Salvar ou cancelar volta para
	// lá, e não sempre para o dashboard: quem veio da lista expandida perderia
	// a busca e os filtros no caminho.
	returnTo screenID

	title widget.Editor
	due   widget.Editor
	// dueLast é o texto do prazo no quadro anterior, usado pela máscara que
	// formata a data enquanto o usuário digita.
	dueLast string
	desc    widget.Editor

	// Os quatro campos de escolha são menus suspensos: em fila de botões, uma
	// lista um pouco maior já não cabia na largura da tela.
	priorityID string
	prioSel    selectField

	difficultyID string
	diffSel      selectField

	categoryID string
	catSel     selectField

	subcategoryID string
	subSel        selectField

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

	// group cuida dos quatro menus juntos: um aberto por vez e clique fora
	// fecha.
	group selectGroup

	save   widget.Clickable
	cancel widget.Clickable

	err string
}

func (s *taskFormScreen) init(a *App) {
	s.group.init(&s.prioSel, &s.diffSel, &s.catSel, &s.subSel)
	s.title.SingleLine = true
	s.title.Submit = true
	s.due.SingleLine = true
	s.desc.SingleLine = false
	s.fileCh = make(chan string, 1)
}

// openNew prepara o formulário para uma tarefa nova.
func (s *taskFormScreen) openNew(a *App, returnTo screenID) {
	s.editing = ""
	s.returnTo = returnTo
	s.title.SetText("")
	s.due.SetText("")
	s.dueLast = ""
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
	// Dificuldade, categoria e subcategoria nascem vazias: são opcionais, e sem
	// dificuldade a tarefa pesa o fator neutro.
	s.difficultyID = ""
	s.categoryID = ""
	s.subcategoryID = ""
	s.links = nil
	s.files = nil
	s.closeSelects()
}

// openEdit carrega uma tarefa existente no formulário.
func (s *taskFormScreen) openEdit(a *App, t model.Task, returnTo screenID) {
	s.editing = t.ID
	s.returnTo = returnTo
	s.title.SetText(t.Title)
	s.due.SetText(formatDateInput(t.DueAt))
	s.dueLast = s.due.Text()
	s.desc.SetText(t.Description)
	s.today.Value = t.Today
	s.priorityID = t.PriorityID
	s.difficultyID = t.DifficultyID
	s.categoryID = t.CategoryID
	s.subcategoryID = t.SubcategoryID
	s.links = make([]*linkRow, 0, len(t.Links))
	for _, ref := range t.Links {
		s.links = append(s.links, newLinkRow(ref))
	}
	s.files = append([]string(nil), t.Files...)
	s.err = ""
	s.closeSelects()
}

// closeSelects fecha os menus abertos — abrir o formulário de novo com um menu
// pendurado seria uma surpresa desagradável.
func (s *taskFormScreen) closeSelects() {
	s.group.closeAll()
}

// resizeClicks devolve uma fatia com n botões, reaproveitando a atual quando o
// tamanho já bate — recriar a cada quadro perderia o estado do clique.
func resizeClicks(clicks []widget.Clickable, n int) []widget.Clickable {
	if len(clicks) == n {
		return clicks
	}
	return make([]widget.Clickable, n)
}

func (s *taskFormScreen) Layout(gtx layout.Context, a *App) layout.Dimensions {
	cfg := a.state.SettingsFor(a.mode)
	prioOpts := priorityOptions(cfg.Priorities)
	diffOpts := difficultyOptions(cfg.Difficulties)
	catOpts := categoryOptions(cfg.Categories)
	subOpts := subcategoryOptions(cfg.Subcategories)

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
	s.group.update(gtx)

	if s.cancel.Clicked(gtx) {
		a.goTo(s.returnTo)
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
	// A data de entrega ganha os separadores enquanto é digitada:
	// "05022026" vira "05/02/2026" sem o usuário digitar as barras.
	maskDateField(&s.due, &s.dueLast, 12)

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
			a.goTo(s.returnTo)
		}
	}

	heading := "Nova tarefa"
	if s.editing != "" {
		heading = "Editar tarefa"
	}

	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return s.content(gtx, a, heading, prioOpts, diffOpts, catOpts, subOpts)
		}),
		// O véu fica entre o formulário e o menu, que é desenhado por último.
		layout.Stacked(s.group.scrimLayout),
	)
}

// content é o formulário em si.
func (s *taskFormScreen) content(gtx layout.Context, a *App, heading string,
	prioOpts, diffOpts, catOpts, subOpts []selectOption) layout.Dimensions {

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
					return s.choicesRow(gtx, a, prioOpts, diffOpts, catOpts, subOpts)
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

// choicesRow põe os campos de escolha e o prazo numa linha só. Cada campo é um
// menu suspenso: a lista pode crescer à vontade nas configurações sem empurrar
// nada para fora da tela.
func (s *taskFormScreen) choicesRow(gtx layout.Context, a *App,
	prioOpts, diffOpts, catOpts, subOpts []selectOption) layout.Dimensions {

	sel := func(f *selectField, label string, opts []selectOption, selected string) layout.FlexChild {
		return layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return f.layout(gtx, a.th, label, opts, selected)
		})
	}
	children := []layout.FlexChild{
		sel(&s.prioSel, "Prioridade", prioOpts, s.priorityID),
	}
	// Um campo sem nada configurado não vira um menu vazio: ele simplesmente
	// não aparece, como já acontecia com a categoria.
	if len(diffOpts) > 1 {
		children = append(children, layout.Rigid(spacerX(12).Layout),
			sel(&s.diffSel, "Dificuldade", diffOpts, s.difficultyID))
	}
	if len(catOpts) > 1 {
		children = append(children, layout.Rigid(spacerX(12).Layout),
			sel(&s.catSel, "Categoria", catOpts, s.categoryID))
	}
	if len(subOpts) > 1 {
		children = append(children, layout.Rigid(spacerX(12).Layout),
			sel(&s.subSel, "Subcategoria", subOpts, s.subcategoryID))
	}
	children = append(children,
		layout.Rigid(spacerX(12).Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.th.field(gtx, "Data para entrega", &s.due, "dd/mm/aaaa hh:mm")
		}),
	)
	return layout.Flex{Alignment: layout.Start}.Layout(gtx, children...)
}

// priorityOptions monta as opções do campo de prioridade. Ela é obrigatória,
// então não tem "Nenhuma"; o valor aparece junto porque é o peso da tarefa.
func priorityOptions(prios []model.Priority) []selectOption {
	out := make([]selectOption, 0, len(prios))
	for _, p := range prios {
		out = append(out, selectOption{id: p.ID, title: p.Title + " · " + itoa(p.Value)})
	}
	return out
}

// difficultyOptions monta as opções do campo de dificuldade, com o "Nenhuma" na
// frente — sem dificuldade escolhida a tarefa usa o fator neutro.
func difficultyOptions(diffs []model.Difficulty) []selectOption {
	out := make([]selectOption, 0, len(diffs)+1)
	out = append(out, selectOption{title: "Nenhuma"})
	for _, d := range diffs {
		out = append(out, selectOption{id: d.ID, title: d.Title + " · " + itoa(d.Value)})
	}
	return out
}

func categoryOptions(cats []model.Category) []selectOption {
	out := make([]selectOption, 0, len(cats)+1)
	out = append(out, selectOption{title: "Nenhuma"})
	for _, c := range cats {
		out = append(out, selectOption{id: c.ID, title: c.Title})
	}
	return out
}

func subcategoryOptions(subs []model.Subcategory) []selectOption {
	out := make([]selectOption, 0, len(subs)+1)
	out = append(out, selectOption{title: "Nenhuma"})
	for _, c := range subs {
		out = append(out, selectOption{id: c.ID, title: c.Title})
	}
	return out
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
		ID:            s.editing,
		Mode:          a.mode,
		Title:         title,
		Description:   strings.TrimSpace(s.desc.Text()),
		PriorityID:    s.priorityID,
		DifficultyID:  s.difficultyID,
		CategoryID:    s.categoryID,
		SubcategoryID: s.subcategoryID,
		Links:         s.collectLinks(),
		Files:         append([]string(nil), s.files...),
		DueAt:         due,
		Today:         s.today.Value,
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
