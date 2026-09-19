package ui

import (
	"errors"
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

// valueColumnWidth é a largura da coluna de peso das listas com valor.
const valueColumnWidth = unit.Dp(76)

// valueRow são os campos de um item com título e peso na tela de configuração.
// Prioridades e dificuldades usam a mesma grade.
type valueRow struct {
	id     string
	title  widget.Editor
	value  widget.Editor
	remove widget.Clickable
}

func newValueRow(id, title string, value int) *valueRow {
	r := &valueRow{id: id}
	r.title.SingleLine = true
	r.value.SingleLine = true
	r.title.SetText(title)
	r.value.SetText(itoa(value))
	return r
}

// titleRow são os campos de um item que só tem título: categorias e
// subcategorias, que classificam a tarefa sem pesar no score.
type titleRow struct {
	id     string
	title  widget.Editor
	remove widget.Clickable
}

func newTitleRow(id, title string) *titleRow {
	r := &titleRow{id: id}
	r.title.SingleLine = true
	r.title.SetText(title)
	return r
}

// settingsScreen edita as prioridades, as dificuldades, as categorias, as
// subcategorias e as metas de tempo do modo atual.
type settingsScreen struct {
	prios []*valueRow
	diffs []*valueRow
	cats  []*titleRow
	subs  []*titleRow

	daily  widget.Editor
	weekly widget.Editor

	addPrio widget.Clickable
	addDiff widget.Clickable
	addCat  widget.Clickable
	addSub  widget.Clickable

	save   widget.Clickable
	back   widget.Clickable
	toWork widget.Clickable
	toStud widget.Clickable

	prioList widget.List
	diffList widget.List
	catList  widget.List
	subList  widget.List
	err      string
}

func (s *settingsScreen) init(a *App) {
	s.daily.SingleLine = true
	s.weekly.SingleLine = true
	for _, l := range []*widget.List{&s.prioList, &s.diffList, &s.catList, &s.subList} {
		l.Axis = layout.Vertical
	}
}

// load traz para os campos a configuração atual do modo.
func (s *settingsScreen) load(a *App) {
	cfg := a.state.SettingsFor(a.mode)

	s.prios = make([]*valueRow, 0, len(cfg.Priorities))
	for _, p := range cfg.Priorities {
		s.prios = append(s.prios, newValueRow(p.ID, p.Title, p.Value))
	}
	s.diffs = make([]*valueRow, 0, len(cfg.Difficulties))
	for _, d := range cfg.Difficulties {
		s.diffs = append(s.diffs, newValueRow(d.ID, d.Title, d.Value))
	}
	s.cats = make([]*titleRow, 0, len(cfg.Categories))
	for _, c := range cfg.Categories {
		s.cats = append(s.cats, newTitleRow(c.ID, c.Title))
	}
	s.subs = make([]*titleRow, 0, len(cfg.Subcategories))
	for _, c := range cfg.Subcategories {
		s.subs = append(s.subs, newTitleRow(c.ID, c.Title))
	}
	s.daily.SetText(formatDurationInput(cfg.DailyTarget))
	s.weekly.SetText(formatDurationInput(cfg.WeeklyTarget))
	s.err = ""
}

func (s *settingsScreen) Layout(gtx layout.Context, a *App) layout.Dimensions {
	if s.back.Clicked(gtx) {
		a.goTo(screenDashboard)
	}
	if s.addPrio.Clicked(gtx) {
		s.prios = append(s.prios, newValueRow(model.NewID(), "", 1))
	}
	if s.addDiff.Clicked(gtx) {
		s.diffs = append(s.diffs, newValueRow(model.NewID(), "", 1))
	}
	if s.addCat.Clicked(gtx) {
		s.cats = append(s.cats, newTitleRow(model.NewID(), ""))
	}
	if s.addSub.Clicked(gtx) {
		s.subs = append(s.subs, newTitleRow(model.NewID(), ""))
	}
	if s.toWork.Clicked(gtx) && a.mode != model.ModeWork {
		a.setMode(model.ModeWork)
		s.load(a)
	}
	if s.toStud.Clicked(gtx) && a.mode != model.ModeStudy {
		a.setMode(model.ModeStudy)
		s.load(a)
	}
	s.prios = removeClicked(gtx, s.prios, func(r *valueRow) *widget.Clickable { return &r.remove })
	s.diffs = removeClicked(gtx, s.diffs, func(r *valueRow) *widget.Clickable { return &r.remove })
	s.cats = removeClicked(gtx, s.cats, func(r *titleRow) *widget.Clickable { return &r.remove })
	s.subs = removeClicked(gtx, s.subs, func(r *titleRow) *widget.Clickable { return &r.remove })
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
					return s.columns(gtx, a)
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

// removeClicked tira da lista as linhas cujo botão "remover" foi clicado.
// Percorre de trás para a frente para os índices não escorregarem.
func removeClicked[T any](gtx layout.Context, rows []T, btn func(T) *widget.Clickable) []T {
	for i := len(rows) - 1; i >= 0; i-- {
		if btn(rows[i]).Clicked(gtx) {
			rows = append(rows[:i], rows[i+1:]...)
		}
	}
	return rows
}

// columns distribui os painéis em três colunas: o que tem peso à esquerda, o
// que só classifica no meio, as metas de tempo à direita.
func (s *settingsScreen) columns(gtx layout.Context, a *App) layout.Dimensions {
	column := func(top, bottom layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
					return top(gtx)
				}),
				layout.Rigid(spacerY(14).Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
					return bottom(gtx)
				}),
			)
		}
	}
	return layout.Flex{Alignment: layout.Start}.Layout(gtx,
		layout.Flexed(1.15, column(
			func(gtx layout.Context) layout.Dimensions { return s.prioritiesPanel(gtx, a) },
			func(gtx layout.Context) layout.Dimensions { return s.difficultiesPanel(gtx, a) },
		)),
		layout.Rigid(spacerX(14).Layout),
		layout.Flexed(1, column(
			func(gtx layout.Context) layout.Dimensions { return s.categoriesPanel(gtx, a) },
			func(gtx layout.Context) layout.Dimensions { return s.subcategoriesPanel(gtx, a) },
		)),
		layout.Rigid(spacerX(14).Layout),
		layout.Flexed(0.95, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			return s.targetsPanel(gtx, a)
		}),
	)
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

// panelHead é o título do painel com o botão de adicionar e a explicação.
func (s *settingsScreen) panelHead(gtx layout.Context, a *App, title, desc string, add *widget.Clickable) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(a.th.heading(title).Layout),
				layout.Flexed(1, layout.Spacer{}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					b := a.th.button("+ Adicionar")
					b.Size, b.PadX, b.PadY = unit.Sp(13), unit.Dp(13), unit.Dp(6)
					b.Radius = unit.Dp(9)
					return b.Layout(gtx, a.th, add)
				}),
			)
		}),
		layout.Rigid(spacerY(4).Layout),
		layout.Rigid(a.th.small(desc).Layout),
	)
}

// prioritiesPanel lista as prioridades editáveis.
func (s *settingsScreen) prioritiesPanel(gtx layout.Context, a *App) layout.Dimensions {
	return s.valuePanel(gtx, a, valuePanelArgs{
		title: "Prioridades",
		desc:  "O valor é o peso da prioridade: quanto maior, mais a tarefa alimenta o fogo.",
		hint:  "Alta",
		empty: "Sem prioridades. Adicione ao menos uma.",
		rows:  s.prios,
		add:   &s.addPrio,
		list:  &s.prioList,
	})
}

// difficultiesPanel lista as dificuldades editáveis.
func (s *settingsScreen) difficultiesPanel(gtx layout.Context, a *App) layout.Dimensions {
	return s.valuePanel(gtx, a, valuePanelArgs{
		title: "Dificuldades",
		desc:  "O valor multiplica o peso da prioridade: uma tarefa difícil entregue vale mais no score.",
		hint:  "Difícil",
		empty: "Sem dificuldades configuradas.",
		rows:  s.diffs,
		add:   &s.addDiff,
		list:  &s.diffList,
	})
}

// categoriesPanel lista as categorias editáveis do modo.
func (s *settingsScreen) categoriesPanel(gtx layout.Context, a *App) layout.Dimensions {
	return s.titlePanel(gtx, a, titlePanelArgs{
		title: "Categorias",
		desc:  "Classificam as tarefas e fatiam os relatórios (ex.: reunião, projeto).",
		hint:  "Reunião",
		empty: "Sem categorias. Elas são opcionais.",
		rows:  s.cats,
		add:   &s.addCat,
		list:  &s.catList,
	})
}

// subcategoriesPanel lista as subcategorias editáveis do modo.
func (s *settingsScreen) subcategoriesPanel(gtx layout.Context, a *App) layout.Dimensions {
	return s.titlePanel(gtx, a, titlePanelArgs{
		title: "Subcategorias",
		desc:  "Um recorte mais fino, escolhido junto da categoria e independente dela.",
		hint:  "Front-end",
		empty: "Sem subcategorias. Elas são opcionais.",
		rows:  s.subs,
		add:   &s.addSub,
		list:  &s.subList,
	})
}

// valuePanelArgs reúne o que muda entre o painel de prioridades e o de
// dificuldades — o resto da grade é o mesmo.
type valuePanelArgs struct {
	title string
	desc  string
	hint  string
	empty string
	rows  []*valueRow
	add   *widget.Clickable
	list  *widget.List
}

func (s *settingsScreen) valuePanel(gtx layout.Context, a *App, p valuePanelArgs) layout.Dimensions {
	return a.th.panelFill(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return s.panelHead(gtx, a, p.title, p.desc, p.add)
			}),
			layout.Rigid(spacerY(12).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, a.th.small("Título").Layout),
					layout.Rigid(spacerX(10).Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Dp(valueColumnWidth)
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
				if len(p.rows) == 0 {
					return a.th.small(p.empty).Layout(gtx)
				}
				return material.List(a.th.Theme, p.list).Layout(gtx, len(p.rows),
					func(gtx layout.Context, i int) layout.Dimensions {
						return s.valueRowLayout(gtx, a, p.rows[i], p.hint)
					})
			}),
		)
	})
}

func (s *settingsScreen) valueRowLayout(gtx layout.Context, a *App, r *valueRow, hint string) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(8), Right: unit.Dp(4)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.th.editorBox(gtx, &r.title, hint, unit.Dp(0))
				}),
				layout.Rigid(spacerX(10).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Dp(valueColumnWidth)
					gtx.Constraints.Max.X = gtx.Constraints.Min.X
					return a.th.editorBox(gtx, &r.value, "3", unit.Dp(0))
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

// titlePanelArgs reúne o que muda entre o painel de categorias e o de
// subcategorias.
type titlePanelArgs struct {
	title string
	desc  string
	hint  string
	empty string
	rows  []*titleRow
	add   *widget.Clickable
	list  *widget.List
}

func (s *settingsScreen) titlePanel(gtx layout.Context, a *App, p titlePanelArgs) layout.Dimensions {
	return a.th.panelFill(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return s.panelHead(gtx, a, p.title, p.desc, p.add)
			}),
			layout.Rigid(spacerY(12).Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if len(p.rows) == 0 {
					return a.th.small(p.empty).Layout(gtx)
				}
				return material.List(a.th.Theme, p.list).Layout(gtx, len(p.rows),
					func(gtx layout.Context, i int) layout.Dimensions {
						return s.titleRowLayout(gtx, a, p.rows[i], p.hint)
					})
			}),
		)
	})
}

func (s *settingsScreen) titleRowLayout(gtx layout.Context, a *App, r *titleRow, hint string) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(8), Right: unit.Dp(4)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.th.editorBox(gtx, &r.title, hint, unit.Dp(0))
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
			layout.Rigid(a.th.small("Como o score usa esses campos:").Layout),
			layout.Rigid(spacerY(3).Layout),
			layout.Rigid(a.th.label(unit.Sp(11),
				"O peso de cada tarefa é o valor da prioridade multiplicado pelo da "+
					"dificuldade. Tarefa sem dificuldade usa fator 1. Categoria e "+
					"subcategoria não pesam: elas só fatiam os relatórios.",
				colorInkFaint).Layout),
			layout.Flexed(1, layout.Spacer{}.Layout),
			layout.Rigid(a.th.small("Seus dados ficam em:").Layout),
			layout.Rigid(spacerY(3).Layout),
			layout.Rigid(a.th.label(unit.Sp(11), a.st.Path(), colorInkFaint).Layout),
		)
	})
}

// commit valida e grava as configurações. Devolve true se conseguiu salvar.
func (s *settingsScreen) commit(a *App) bool {
	prios := make([]model.Priority, 0, len(s.prios))
	for _, r := range s.prios {
		title, value, err := valueRowValues(r, "prioridade")
		if err != nil {
			s.err = err.Error()
			return false
		}
		prios = append(prios, model.Priority{ID: r.id, Title: title, Value: value})
	}
	if len(prios) == 0 {
		s.err = "Configure ao menos uma prioridade."
		return false
	}
	if err := checkDuplicates(titlesOf(prios, func(p model.Priority) string { return p.Title }), "prioridades"); err != nil {
		s.err = err.Error()
		return false
	}

	diffs := make([]model.Difficulty, 0, len(s.diffs))
	for _, r := range s.diffs {
		title, value, err := valueRowValues(r, "dificuldade")
		if err != nil {
			s.err = err.Error()
			return false
		}
		diffs = append(diffs, model.Difficulty{ID: r.id, Title: title, Value: value})
	}
	if err := checkDuplicates(titlesOf(diffs, func(d model.Difficulty) string { return d.Title }), "dificuldades"); err != nil {
		s.err = err.Error()
		return false
	}

	// Categorias e subcategorias são opcionais, mas as que existirem precisam
	// de título, e único dentro da própria lista.
	cats := make([]model.Category, 0, len(s.cats))
	for _, r := range s.cats {
		title, err := titleRowValue(r, "categoria")
		if err != nil {
			s.err = err.Error()
			return false
		}
		cats = append(cats, model.Category{ID: r.id, Title: title})
	}
	if err := checkDuplicates(titlesOf(cats, func(c model.Category) string { return c.Title }), "categorias"); err != nil {
		s.err = err.Error()
		return false
	}

	subs := make([]model.Subcategory, 0, len(s.subs))
	for _, r := range s.subs {
		title, err := titleRowValue(r, "subcategoria")
		if err != nil {
			s.err = err.Error()
			return false
		}
		subs = append(subs, model.Subcategory{ID: r.id, Title: title})
	}
	if err := checkDuplicates(titlesOf(subs, func(c model.Subcategory) string { return c.Title }), "subcategorias"); err != nil {
		s.err = err.Error()
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
		Priorities:    prios,
		Categories:    cats,
		Subcategories: subs,
		Difficulties:  diffs,
		DailyTarget:   daily,
		WeeklyTarget:  weekly,
	})
	s.err = ""
	a.save()
	a.setInfo("Configurações salvas.")
	return true
}

// valueRowValues lê o título e o peso de uma linha, com o nome do campo na
// mensagem de erro para o usuário saber qual lista corrigir.
func valueRowValues(r *valueRow, what string) (string, int, error) {
	title := strings.TrimSpace(r.title.Text())
	if title == "" {
		return "", 0, errors.New("Toda " + what + " precisa de um título.")
	}
	value, err := parsePriorityValue(r.value.Text())
	if err != nil {
		return "", 0, errors.New(capitalize(what) + " “" + title + "”: " + err.Error())
	}
	return title, value, nil
}

func titleRowValue(r *titleRow, what string) (string, error) {
	title := strings.TrimSpace(r.title.Text())
	if title == "" {
		return "", errors.New("Toda " + what + " precisa de um título.")
	}
	return title, nil
}

// titlesOf extrai os títulos de uma lista já validada.
func titlesOf[T any](items []T, title func(T) string) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = title(it)
	}
	return out
}

// checkDuplicates recusa dois itens com o mesmo título na mesma lista: dois
// nomes iguais tornariam impossível saber o que foi escolhido na tarefa.
func checkDuplicates(titles []string, what string) error {
	seen := make(map[string]bool, len(titles))
	for _, t := range titles {
		key := strings.ToLower(t)
		if seen[key] {
			return errors.New("Há duas " + what + " com o título “" + t + "”.")
		}
		seen[key] = true
	}
	return nil
}

// capitalize põe a primeira letra em maiúscula, para a mensagem de erro
// começar como frase.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
