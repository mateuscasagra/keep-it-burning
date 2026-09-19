package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// selectOption é uma opção de um campo de escolha. ID vazio é o "Nenhuma" das
// listas opcionais.
type selectOption struct {
	id    string
	title string
}

// selectField é um campo de escolha única em forma de menu suspenso: mostra o
// valor atual e abre a lista quando clicado. Substituiu as filas de botões do
// formulário, que cresciam para o lado e estouravam a largura da tela assim que
// o usuário criava algumas categorias a mais.
type selectField struct {
	open bool
	// justOpened marca o quadro em que o menu abriu, para a tela conseguir
	// fechar os outros campos — dois menus abertos ao mesmo tempo se
	// sobrepõem e confundem. justToggled marca qualquer clique no gatilho,
	// inclusive o que fecha: sem isso o véu fecharia o menu no mesmo quadro em
	// que o gatilho o reabre, e clicar no campo aberto não faria nada.
	justOpened  bool
	justToggled bool

	btn  widget.Clickable
	opts []widget.Clickable
	list widget.List
}

func (s *selectField) init() {
	s.list.Axis = layout.Vertical
}

// menuMaxHeight limita a altura do menu aberto; passando disso ele rola. É o
// que faz uma lista de trinta subcategorias caber na tela.
const menuMaxHeight = unit.Dp(240)

// update trata os cliques do gatilho e das opções. Devolve o ID escolhido e se
// houve escolha neste quadro.
func (s *selectField) update(gtx layout.Context, opts []selectOption) (string, bool) {
	s.opts = resizeClicks(s.opts, len(opts))
	s.justOpened, s.justToggled = false, false
	if s.btn.Clicked(gtx) {
		s.open = !s.open
		s.justOpened, s.justToggled = s.open, true
	}
	for i := range s.opts {
		if s.opts[i].Clicked(gtx) && i < len(opts) {
			s.open = false
			return opts[i].id, true
		}
	}
	return "", false
}

// layout desenha o rótulo e o gatilho. O menu aberto não entra no fluxo: ele é
// adiado para o fim do quadro, e por isso aparece por cima do que vem depois em
// vez de ficar escondido atrás.
func (s *selectField) layout(gtx layout.Context, th *Theme, label string, opts []selectOption, selected string) layout.Dimensions {
	// A opção de ID vazio é a "sem valor" — "Nenhuma" no formulário, "Todas"
	// nos filtros. Ela aparece apagada, para o campo preenchido se destacar.
	current, empty := "Nenhuma", true
	for _, o := range opts {
		if o.id == selected {
			current, empty = o.title, o.id == ""
			break
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.small(label).Layout),
		layout.Rigid(spacerY(6).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims := s.trigger(gtx, th, current, empty)
			if s.open {
				s.deferMenu(gtx, th, opts, selected, dims)
			}
			return dims
		}),
	)
}

// trigger é a caixa fechada do campo: valor atual à esquerda, setinha à direita.
func (s *selectField) trigger(gtx layout.Context, th *Theme, current string, empty bool) layout.Dimensions {
	return s.btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		macro := op.Record(gtx.Ops)
		inner := layout.Inset{Top: unit.Dp(9), Bottom: unit.Dp(9), Left: unit.Dp(10), Right: unit.Dp(10)}.
			Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				txt := colorInk
				if empty {
					txt = colorInkFaint
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, th.cell(unit.Sp(14), current, txt).Layout),
					layout.Rigid(spacerX(8).Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return caretDown(gtx, colorInkSoft)
					}),
				)
			})
		call := macro.Stop()

		size := inner.Size
		size.X = gtx.Constraints.Max.X
		bg := colorPanel
		switch {
		case s.btn.Pressed():
			bg = colorPressed
		case s.btn.Hovered() || s.open:
			bg = colorHover
		}
		border := colorInkFaint
		if s.open {
			border = colorInk
		}
		fillRRect(gtx, size, unit.Dp(9), bg)
		strokeRRect(gtx, size, unit.Dp(9), strokeThin, border)
		call.Add(gtx.Ops)
		return layout.Dimensions{Size: size}
	})
}

// deferMenu desenha o menu logo abaixo do gatilho, mas no fim do quadro:
// op.Defer guarda a posição atual e executa o desenho depois de todo o resto,
// então o menu fica por cima do formulário e recebe os cliques primeiro.
func (s *selectField) deferMenu(gtx layout.Context, th *Theme, opts []selectOption, selected string, trigger layout.Dimensions) {
	macro := op.Record(gtx.Ops)
	off := op.Offset(image.Pt(0, trigger.Size.Y+gtx.Dp(unit.Dp(4)))).Push(gtx.Ops)

	mgtx := gtx
	mgtx.Constraints.Min.X, mgtx.Constraints.Max.X = trigger.Size.X, trigger.Size.X
	mgtx.Constraints.Min.Y, mgtx.Constraints.Max.Y = 0, gtx.Dp(menuMaxHeight)
	s.menu(mgtx, th, opts, selected)

	off.Pop()
	op.Defer(gtx.Ops, macro.Stop())
}

// menu é o cartão com as opções.
func (s *selectField) menu(gtx layout.Context, th *Theme, opts []selectOption, selected string) layout.Dimensions {
	return th.panel(gtx, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return material.List(th.Theme, &s.list).Layout(gtx, len(opts),
			func(gtx layout.Context, i int) layout.Dimensions {
				return s.option(gtx, th, i, opts[i], opts[i].id == selected)
			})
	})
}

// option é uma linha do menu.
func (s *selectField) option(gtx layout.Context, th *Theme, i int, o selectOption, current bool) layout.Dimensions {
	if i >= len(s.opts) {
		return layout.Dimensions{}
	}
	click := &s.opts[i]
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		macro := op.Record(gtx.Ops)
		dims := layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(8), Right: unit.Dp(8)}.
			Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				c := colorInk
				if o.id == "" {
					c = colorInkSoft
				}
				lbl := th.cell(unit.Sp(14), o.title, c)
				if current {
					lbl.Font.Weight = font.SemiBold
				}
				return lbl.Layout(gtx)
			})
		call := macro.Stop()

		bg := colorPanel
		switch {
		case click.Pressed():
			bg = colorPressed
		case click.Hovered(), current:
			bg = colorHover
		}
		fillRRect(gtx, dims.Size, unit.Dp(7), bg)
		call.Add(gtx.Ops)
		return dims
	})
}

// caretDown desenha a setinha do campo fechado. É desenhada à mão porque a
// fonte padrão não tem um glifo decente para ela.
func caretDown(gtx layout.Context, c color.NRGBA) layout.Dimensions {
	w, h := gtx.Dp(unit.Dp(9)), gtx.Dp(unit.Dp(5))
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(ptf(0, 0))
	p.LineTo(ptf(float32(w), 0))
	p.LineTo(ptf(float32(w)/2, float32(h)))
	p.Close()
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// selectGroup coordena os menus suspensos de uma tela: garante que só um fique
// aberto por vez e escuta o clique fora, que fecha todos.
type selectGroup struct {
	fields []*selectField
	scrim  widget.Clickable
}

// init registra os campos do grupo, na ordem em que aparecem na tela.
func (g *selectGroup) init(fields ...*selectField) {
	g.fields = fields
	for _, f := range fields {
		f.init()
	}
}

// update roda depois do update de cada campo: fecha os outros quando um abre e
// trata o clique fora.
func (g *selectGroup) update(gtx layout.Context) {
	toggled := false
	for _, f := range g.fields {
		if f.justOpened {
			for _, other := range g.fields {
				if other != f {
					other.open = false
				}
			}
		}
		toggled = toggled || f.justToggled
	}
	// O clique no gatilho de um campo já foi decidido pelo próprio campo; se o
	// véu também fechasse, clicar num campo aberto não faria nada.
	if g.scrim.Clicked(gtx) && !toggled {
		g.closeAll()
	}
}

func (g *selectGroup) closeAll() {
	for _, f := range g.fields {
		f.open = false
	}
}

func (g *selectGroup) anyOpen() bool {
	for _, f := range g.fields {
		if f.open {
			return true
		}
	}
	return false
}

// scrimLayout é o véu que cobre a tela enquanto há menu aberto. Ele deixa o
// clique passar adiante: quem clicou em outro campo quer aquele campo, não quer
// só fechar este e clicar de novo.
func (g *selectGroup) scrimLayout(gtx layout.Context) layout.Dimensions {
	if !g.anyOpen() {
		return layout.Dimensions{}
	}
	gtx.Constraints.Min = gtx.Constraints.Max
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	return g.scrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
}
