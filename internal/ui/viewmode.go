package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// viewMode é a forma de visualização da janela: quanto da tela o app ocupa e
// o quanto ele mostra. É escolhida no seletor e não tem relação com o
// cronômetro — trabalhar e ver o app são coisas separadas.
type viewMode int

const (
	viewFull viewMode = iota
	viewFocus
)

// viewOption descreve uma forma de visualização no seletor: o nome, o tamanho
// da janela e o desenho que representa o espaço ocupado na tela.
type viewOption struct {
	mode  viewMode
	title string
	desc  string
	size  image.Point

	// fracX e fracY são a fatia da tela que o diagrama pinta (0 a 1). Menos que
	// a tela inteira é desenhado encostado no canto, que é onde a janela
	// reduzida costuma ficar.
	fracX, fracY float32
}

// viewOptions é a lista mostrada no seletor, da maior para a menor.
var viewOptions = []viewOption{
	{viewFull, "Completa", "Painel inteiro: indicadores e tarefas", fullSize, 1, 1},
	{viewFocus, "Foco", "Janela de canto com as tarefas do dia", focusSize, 0.34, 0.66},
}

// viewOptionFor devolve a opção de um modo; cai na primeira se o modo for
// desconhecido, para nunca deixar o app sem visualização válida.
func viewOptionFor(m viewMode) viewOption {
	for _, o := range viewOptions {
		if o.mode == m {
			return o
		}
	}
	return viewOptions[0]
}

// viewPicker é o botão de visualização e o menu que ele abre, no espírito do
// seletor de layout do Windows: cada opção é um diagrama da tela com a área
// que a janela vai ocupar pintada dentro.
type viewPicker struct {
	open bool

	btn   widget.Clickable
	scrim widget.Clickable
	opts  []widget.Clickable
}

// update trata os cliques do botão, das opções e do fundo. Roda antes do
// desenho, como o resto das telas.
func (p *viewPicker) update(gtx layout.Context, a *App) {
	if p.opts == nil {
		p.opts = make([]widget.Clickable, len(viewOptions))
	}
	if p.btn.Clicked(gtx) {
		p.open = !p.open
	}
	for i := range p.opts {
		if p.opts[i].Clicked(gtx) {
			p.open = false
			a.setView(viewOptions[i].mode)
		}
	}
	// Clicar fora fecha o menu, como em qualquer menu suspenso.
	if p.scrim.Clicked(gtx) {
		p.open = false
	}
}

// button desenha o gatilho do menu: o quadrinho dividido em painéis, que é o
// ícone universal de "escolher layout". Em compacto ele encolhe para caber na
// barra da janela reduzida.
func (p *viewPicker) button(gtx layout.Context, a *App, compact bool) layout.Dimensions {
	b := a.th.button("")
	b.PadX, b.PadY = unit.Dp(14), unit.Dp(9)
	w, h := unit.Dp(26), unit.Dp(19)
	if compact {
		b.PadX, b.PadY = unit.Dp(9), unit.Dp(5)
		b.Radius = unit.Dp(8)
		w, h = unit.Dp(22), unit.Dp(16)
	}
	if p.open {
		b.Bg = colorHover
	}
	return b.LayoutWith(gtx, a.th, &p.btn, func(gtx layout.Context) layout.Dimensions {
		return layoutIcon(gtx, w, h, func(gtx layout.Context, size image.Point) {
			paneGlyph(gtx, size)
		})
	})
}

// overlay desenha o menu aberto por cima da tela. Devolve dimensões vazias
// quando está fechado, para não roubar o clique de nada.
func (p *viewPicker) overlay(gtx layout.Context, a *App, top, right unit.Dp) layout.Dimensions {
	if !p.open {
		return layout.Dimensions{}
	}
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Stack{}.Layout(gtx,
		// O fundo invisível cobre a tela inteira só para capturar o clique de
		// fechar; o menu vem depois e fica por cima dele.
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return p.scrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.NE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: top, Right: right}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return p.menu(gtx, a)
				})
			})
		}),
	)
}

// menu é o cartão com as opções.
func (p *viewPicker) menu(gtx layout.Context, a *App) layout.Dimensions {
	w := min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(292)))
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w

	children := make([]layout.FlexChild, 0, len(viewOptions)+2)
	children = append(children,
		layout.Rigid(a.th.label(unit.Sp(13), "Formas de visualização", colorInkSoft).Layout),
		layout.Rigid(spacerY(6).Layout),
	)
	for i, o := range viewOptions {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.option(gtx, a, i, o)
		}))
	}

	return a.th.panel(gtx, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// option desenha uma linha do menu: o diagrama da tela, o nome e a explicação.
func (p *viewPicker) option(gtx layout.Context, a *App, i int, o viewOption) layout.Dimensions {
	click := &p.opts[i]
	current := a.vmode == o.mode

	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		macro := op.Record(gtx.Ops)
		dims := layout.UniformInset(unit.Dp(7)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					// O destaque acompanha o mouse, como no seletor do Windows:
					// a área que a janela vai ocupar acende ao passar por cima.
					fill := colorMuted
					if current || click.Hovered() || click.Pressed() {
						fill = colorAccent
					}
					return layoutIcon(gtx, unit.Dp(64), unit.Dp(42), func(gtx layout.Context, size image.Point) {
						screenGlyph(gtx, size, o.fracX, o.fracY, fill)
					})
				}),
				layout.Rigid(spacerX(10).Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(a.th.label(unit.Sp(15), o.title, colorInk).Layout),
						layout.Rigid(a.th.label(unit.Sp(12), o.desc, colorInkSoft).Layout),
					)
				}),
			)
		})
		call := macro.Stop()

		bg := colorPanel
		switch {
		case click.Pressed():
			bg = colorPressed
		case click.Hovered():
			bg = colorHover
		case current:
			bg = colorHover
		}
		fillRRect(gtx, dims.Size, unit.Dp(9), bg)
		if current {
			strokeRRect(gtx, dims.Size, unit.Dp(9), strokeThin, colorInk)
		}
		call.Add(gtx.Ops)
		return dims
	})
}

// layoutIcon reserva uma área de tamanho fixo e chama o desenho dentro dela.
// Os glifos do seletor são desenhados à mão porque a fonte não tem esses
// símbolos.
func layoutIcon(gtx layout.Context, w, h unit.Dp, draw func(layout.Context, image.Point)) layout.Dimensions {
	size := image.Pt(gtx.Dp(w), gtx.Dp(h))
	draw(gtx, size)
	return layout.Dimensions{Size: size}
}

// screenGlyph desenha a moldura da tela com a área ocupada pela janela pintada
// dentro, como nos ícones de layout do Windows. Quando a janela não toma a
// tela toda, ela aparece encostada no canto inferior direito.
func screenGlyph(gtx layout.Context, size image.Point, fracX, fracY float32, fill color.NRGBA) {
	fillRRect(gtx, size, unit.Dp(5), colorPaper)
	strokeRRect(gtx, size, unit.Dp(5), strokeThin, colorInkFaint)

	pad := gtx.Dp(unit.Dp(5))
	inner := image.Rectangle{Min: image.Pt(pad, pad), Max: size.Sub(image.Pt(pad, pad))}
	if inner.Dx() <= 0 || inner.Dy() <= 0 {
		return
	}
	w := max(int(float32(inner.Dx())*fracX), gtx.Dp(unit.Dp(6)))
	h := max(int(float32(inner.Dy())*fracY), gtx.Dp(unit.Dp(6)))
	rect := image.Rectangle{
		Min: image.Pt(inner.Max.X-w, inner.Max.Y-h),
		Max: inner.Max,
	}
	if rect.Min.X < inner.Min.X {
		rect.Min.X = inner.Min.X
	}
	if rect.Min.Y < inner.Min.Y {
		rect.Min.Y = inner.Min.Y
	}
	r := gtx.Dp(unit.Dp(3))
	paint.FillShape(gtx.Ops, fill,
		clip.RRect{Rect: rect, SE: r, SW: r, NE: r, NW: r}.Op(gtx.Ops))
}

// paneGlyph desenha o ícone do botão: um retângulo dividido em painéis, o
// mesmo símbolo que o Windows usa para o seletor de layout.
func paneGlyph(gtx layout.Context, size image.Point) {
	strokeRRect(gtx, size, unit.Dp(4), strokeThin, colorInk)

	// A divisória vertical deixa o painel da esquerda maior, para o ícone ler
	// como "layouts" e não como uma caixa vazia.
	x := float32(size.X) * 0.58
	var div clip.Path
	div.Begin(gtx.Ops)
	div.MoveTo(ptf(x, float32(gtx.Dp(unit.Dp(2)))))
	div.LineTo(ptf(x, float32(size.Y-gtx.Dp(unit.Dp(2)))))
	// A divisória horizontal parte só o painel da direita em dois.
	y := float32(size.Y) * 0.5
	div.MoveTo(ptf(x, y))
	div.LineTo(ptf(float32(size.X-gtx.Dp(unit.Dp(2))), y))
	paint.FillShape(gtx.Ops, colorInk,
		clip.Stroke{Path: div.End(), Width: float32(gtx.Dp(unit.Dp(1.5)))}.Op())
}
