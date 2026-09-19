package ui

import (
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/dvet/keep-it-burning/internal/model"
)

// Larguras relativas das colunas das tabelas de tarefas — a do dashboard e a
// da tela expandida usam a mesma grade. A coluna de ações não entra aqui: ela
// é fixa em Dp, porque guarda botões de largura conhecida e encolher junto com
// a janela só faria os botões vazarem por cima da coluna vizinha.
//
// Os pesos só precisam somar 1 quando todas as colunas aparecem: omitir uma
// opcional redistribui o peso dela entre as outras. Como o cabeçalho e as
// linhas montam a lista pela mesma regra, a grade continua alinhada.
const (
	colTitle    = 0.25
	colCategory = 0.12
	colSub      = 0.12
	colDiff     = 0.11
	colCreated  = 0.15
	colDue      = 0.15
	colPriority = 0.10
)

// cellGutter é o respiro no fim de cada célula. Sem ele um título comprido
// encosta no valor da coluna seguinte e as duas viram uma frase só.
const cellGutter = unit.Dp(10)

// gutter reserva o respiro dentro da célula: a coluna continua com a mesma
// largura, o texto é que para um pouco antes.
func gutter(w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Right: cellGutter}.Layout(gtx, w)
	}
}

// taskRowInset é a margem das linhas e do cabeçalho das tabelas de tarefas.
var taskRowInset = layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Right: unit.Dp(4)}

// taskCols diz quais colunas opcionais a tabela mostra. As que dependem de
// listas configuráveis somem quando o modo não tem nenhuma cadastrada: uma
// coluna inteira de traços só rouba espaço das que têm conteúdo.
type taskCols struct {
	cat  bool
	sub  bool
	diff bool
}

func taskColsFor(cfg model.ModeSettings) taskCols {
	return taskCols{
		cat:  len(cfg.Categories) > 0,
		sub:  len(cfg.Subcategories) > 0,
		diff: len(cfg.Difficulties) > 0,
	}
}

// actionsColumn devolve a largura da coluna de ações em pixels. Em janela
// estreita ela cede espaço — um terço da linha é o limite — para as colunas de
// dados não sumirem.
func actionsColumn(gtx layout.Context, w unit.Dp) int {
	return min(gtx.Dp(w), gtx.Constraints.Max.X/3)
}

// header desenha os títulos das colunas. A margem repete a das linhas,
// inclusive a faixa que a barra de rolagem da lista ocupa: é ela que faz o
// título cair exatamente em cima do valor.
func (c taskCols) header(gtx layout.Context, th *Theme, actionsW unit.Dp, list *widget.List) layout.Dimensions {
	head := func(txt string) layout.Widget {
		return gutter(th.cell(unit.Sp(13), txt, colorInkSoft).Layout)
	}
	inset := layout.Inset{Right: taskRowInset.Right + th.scrollGutter(list)}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Flexed(colTitle, head("Tarefa"))}
		if c.cat {
			children = append(children, layout.Flexed(colCategory, head("Categoria")))
		}
		if c.sub {
			children = append(children, layout.Flexed(colSub, head("Subcategoria")))
		}
		if c.diff {
			children = append(children, layout.Flexed(colDiff, head("Dificuldade")))
		}
		children = append(children,
			layout.Flexed(colCreated, head("Data Incluída")),
			layout.Flexed(colDue, head("Data Limite")),
			layout.Flexed(colPriority, head("Prioridade")),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// Min e Max juntos: com a largura só no mínimo, o respiro da
				// célula sobraria por fora e a coluna ficaria maior aqui do que
				// nas linhas, empurrando todas as outras para a esquerda.
				w := actionsColumn(gtx, actionsW)
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
				return head("Ações")(gtx)
			}),
		)
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

// row desenha as colunas de dados de uma tarefa; actions é a coluna de ações,
// que muda de tela para tela.
func (c taskCols) row(gtx layout.Context, a *App, t model.Task, actionsW unit.Dp, actions layout.Widget) layout.Dimensions {
	cell := func(txt string, col color.NRGBA) layout.Widget {
		return gutter(a.th.cell(unit.Sp(13), txt, col).Layout)
	}
	// Campo não preenchido vira um traço apagado: a linha continua legível sem
	// fingir que a tarefa tem uma classificação que ela não tem.
	classify := func(title string, ok bool) layout.Widget {
		if !ok {
			return cell("—", colorInkFaint)
		}
		return cell(title, colorInkSoft)
	}

	prioLabel, prioColor := "—", colorInkFaint
	if p, ok := a.state.Priority(a.mode, t.PriorityID); ok {
		prioLabel, prioColor = p.Title, colorInk
	}
	cat, hasCat := a.state.Category(a.mode, t.CategoryID)
	sub, hasSub := a.state.Subcategory(a.mode, t.SubcategoryID)
	diff, hasDiff := a.state.Difficulty(a.mode, t.DifficultyID)
	dueColor := colorInk
	if t.Overdue(time.Now()) {
		dueColor = colorDanger
	}

	children := []layout.FlexChild{
		layout.Flexed(colTitle, gutter(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(a.th.cell(unit.Sp(14), t.Title, colorInk).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if t.Description == "" {
						return layout.Dimensions{}
					}
					return a.th.cell(unit.Sp(12), t.Description, colorInkFaint).Layout(gtx)
				}),
			)
		})),
	}
	if c.cat {
		children = append(children, layout.Flexed(colCategory, classify(cat.Title, hasCat)))
	}
	if c.sub {
		children = append(children, layout.Flexed(colSub, classify(sub.Title, hasSub)))
	}
	if c.diff {
		children = append(children, layout.Flexed(colDiff, classify(diff.Title, hasDiff)))
	}
	children = append(children,
		layout.Flexed(colCreated, cell(formatDateTime(t.CreatedAt), colorInkSoft)),
		layout.Flexed(colDue, cell(formatDateTime(t.DueAt), dueColor)),
		layout.Flexed(colPriority, cell(prioLabel, prioColor)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// A coluna de ações tem largura exata nos dois lados da tabela; é
			// ela que define quanto sobra para as colunas de dados.
			w := actionsColumn(gtx, actionsW)
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
			return actions(gtx)
		}),
	)

	return taskRowInset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

// actionButtons põe os botões de ação de uma linha lado a lado.
func actionButtons(gtx layout.Context, btns ...layout.Widget) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(btns)*2)
	for i, b := range btns {
		if i > 0 {
			children = append(children, layout.Rigid(spacerX(5).Layout))
		}
		children = append(children, layout.Rigid(b))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}
