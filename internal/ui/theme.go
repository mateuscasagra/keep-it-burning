package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// A paleta segue os mockups: papel branco, traço preto grosso e o fogo como
// único ponto de cor forte da interface.
var (
	colorInk        = color.NRGBA{R: 0x1c, G: 0x1b, B: 0x19, A: 0xff}
	colorInkSoft    = color.NRGBA{R: 0x5c, G: 0x57, B: 0x50, A: 0xff}
	colorInkFaint   = color.NRGBA{R: 0x9a, G: 0x94, B: 0x8b, A: 0xff}
	colorPaper      = color.NRGBA{R: 0xfd, G: 0xfc, B: 0xf9, A: 0xff}
	colorPanel      = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colorHover      = color.NRGBA{R: 0xf1, G: 0xee, B: 0xe7, A: 0xff}
	colorPressed    = color.NRGBA{R: 0xe4, G: 0xdf, B: 0xd4, A: 0xff}
	colorAccent     = color.NRGBA{R: 0xe0, G: 0x3c, B: 0x1f, A: 0xff}
	colorAccentSoft = color.NRGBA{R: 0xf5, G: 0x9e, B: 0x0b, A: 0xff}
	colorDanger     = color.NRGBA{R: 0xc0, G: 0x2a, B: 0x1c, A: 0xff}
	colorMuted      = color.NRGBA{R: 0xb5, G: 0xaf, B: 0xa5, A: 0xff}
)

// Espessuras de traço. O contorno grosso é o que dá o ar de rabisco.
const (
	strokeThin   = unit.Dp(1.5)
	strokeNormal = unit.Dp(2)
	strokeThick  = unit.Dp(3)
)

// Theme reúne o tema do material e os atalhos de estilo usados pelas telas.
type Theme struct {
	*material.Theme
}

// NewTheme monta o tema do aplicativo.
func NewTheme() *Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	th.Palette = material.Palette{
		Fg:         colorInk,
		Bg:         colorPaper,
		ContrastBg: colorAccent,
		ContrastFg: colorPaper,
	}
	th.TextSize = unit.Sp(15)
	return &Theme{Theme: th}
}

// label devolve um texto com tamanho e cor definidos.
func (t *Theme) label(size unit.Sp, txt string, c color.NRGBA) material.LabelStyle {
	l := material.Label(t.Theme, size, txt)
	l.Color = c
	return l
}

// body é o texto corrente.
func (t *Theme) body(txt string) material.LabelStyle {
	return t.label(unit.Sp(15), txt, colorInk)
}

// small é o texto secundário.
func (t *Theme) small(txt string) material.LabelStyle {
	return t.label(unit.Sp(13), txt, colorInkSoft)
}

// heading é um título de seção.
func (t *Theme) heading(txt string) material.LabelStyle {
	l := t.label(unit.Sp(18), txt, colorInk)
	l.Font.Weight = font.SemiBold
	return l
}

// rrect devolve o retângulo arredondado que cobre as restrições atuais.
func rrect(gtx layout.Context, size image.Point, radius unit.Dp) clip.RRect {
	r := gtx.Dp(radius)
	return clip.RRect{Rect: image.Rectangle{Max: size}, SE: r, SW: r, NE: r, NW: r}
}

// fillRRect pinta um retângulo arredondado.
func fillRRect(gtx layout.Context, size image.Point, radius unit.Dp, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, rrect(gtx, size, radius).Op(gtx.Ops))
}

// strokeRRect desenha só o contorno de um retângulo arredondado.
func strokeRRect(gtx layout.Context, size image.Point, radius unit.Dp, width unit.Dp, c color.NRGBA) {
	w := float32(gtx.Dp(width))
	// O traço é centrado no caminho; encolher o retângulo por meia largura
	// mantém a borda inteira dentro do widget em vez de vazar para fora.
	half := int(w / 2)
	rect := image.Rectangle{Min: image.Pt(half, half), Max: size.Sub(image.Pt(half, half))}
	if rect.Dx() <= 0 || rect.Dy() <= 0 {
		return
	}
	r := gtx.Dp(radius)
	rr := clip.RRect{Rect: rect, SE: r, SW: r, NE: r, NW: r}
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: rr.Path(gtx.Ops), Width: w}.Op())
}

// panel desenha um cartão de papel com contorno preto e devolve o conteúdo
// dentro dele. É a caixa arredondada que aparece em todas as telas.
func (t *Theme) panel(gtx layout.Context, pad unit.Dp, w layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := layout.UniformInset(pad).Layout(gtx, w)
	call := macro.Stop()

	fillRRect(gtx, dims.Size, unit.Dp(14), colorPanel)
	strokeRRect(gtx, dims.Size, unit.Dp(14), strokeNormal, colorInk)
	call.Add(gtx.Ops)
	return dims
}

// panelFill é como panel, mas ocupa toda a altura disponível — usado nas
// colunas do dashboard, que precisam ter a mesma altura.
func (t *Theme) panelFill(gtx layout.Context, pad unit.Dp, w layout.Widget) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := layout.UniformInset(pad).Layout(gtx, w)
	call := macro.Stop()

	size := dims.Size
	if gtx.Constraints.Min.Y > size.Y {
		size.Y = gtx.Constraints.Min.Y
	}
	if gtx.Constraints.Min.X > size.X {
		size.X = gtx.Constraints.Min.X
	}
	fillRRect(gtx, size, unit.Dp(14), colorPanel)
	strokeRRect(gtx, size, unit.Dp(14), strokeNormal, colorInk)
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size, Baseline: dims.Baseline}
}

// ButtonStyle configura um botão desenhado à mão.
type ButtonStyle struct {
	Text     string
	Size     unit.Sp
	PadX     unit.Dp
	PadY     unit.Dp
	Radius   unit.Dp
	Border   unit.Dp
	Fg       color.NRGBA
	Bg       color.NRGBA
	Emphasis bool
}

// button devolve o estilo padrão de botão.
func (t *Theme) button(txt string) ButtonStyle {
	return ButtonStyle{
		Text:   txt,
		Size:   unit.Sp(16),
		PadX:   unit.Dp(22),
		PadY:   unit.Dp(11),
		Radius: unit.Dp(12),
		Border: strokeNormal,
		Fg:     colorInk,
		Bg:     colorPanel,
	}
}

// primary devolve o estilo do botão de ação principal (mais largo e forte).
func (t *Theme) primary(txt string) ButtonStyle {
	b := t.button(txt)
	b.Size = unit.Sp(20)
	b.PadX = unit.Dp(38)
	b.PadY = unit.Dp(14)
	b.Border = strokeThick
	b.Emphasis = true
	return b
}

// tiny devolve o estilo dos botões pequenos de ação nas listas.
func (t *Theme) tiny(txt string) ButtonStyle {
	b := t.button(txt)
	b.Size = unit.Sp(12)
	b.PadX = unit.Dp(9)
	b.PadY = unit.Dp(5)
	b.Radius = unit.Dp(8)
	b.Border = strokeThin
	return b
}

// Layout desenha o botão e trata o clique.
func (b ButtonStyle) Layout(gtx layout.Context, th *Theme, click *widget.Clickable) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		macro := op.Record(gtx.Ops)
		dims := layout.Inset{
			Top: b.PadY, Bottom: b.PadY, Left: b.PadX, Right: b.PadX,
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := th.label(b.Size, b.Text, b.Fg)
			lbl.Alignment = text.Middle
			if b.Emphasis {
				lbl.Font.Weight = font.SemiBold
			}
			return lbl.Layout(gtx)
		})
		call := macro.Stop()

		bg := b.Bg
		switch {
		case click.Pressed():
			bg = colorPressed
		case click.Hovered():
			bg = colorHover
		}
		fillRRect(gtx, dims.Size, b.Radius, bg)
		strokeRRect(gtx, dims.Size, b.Radius, b.Border, b.Fg)
		call.Add(gtx.Ops)
		return dims
	})
}

// checkboxShape desenha a caixinha quadrada das listas de tarefas. Marcada,
// ela ganha um tique desenhado à mão.
func checkboxShape(gtx layout.Context, checked bool) layout.Dimensions {
	return checkboxShapeBg(gtx, checked, colorPanel)
}

// checkboxShapeBg é a caixinha com cor de fundo explícita, para refletir os
// estados de hover e clique.
func checkboxShapeBg(gtx layout.Context, checked bool, bg color.NRGBA) layout.Dimensions {
	side := gtx.Dp(unit.Dp(22))
	size := image.Pt(side, side)

	fillRRect(gtx, size, unit.Dp(6), bg)
	strokeRRect(gtx, size, unit.Dp(6), strokeNormal, colorInk)

	if checked {
		var p clip.Path
		p.Begin(gtx.Ops)
		f := float32(side)
		p.MoveTo(ptf(f*0.22, f*0.52))
		p.LineTo(ptf(f*0.43, f*0.74))
		p.LineTo(ptf(f*0.80, f*0.24))
		w := float32(gtx.Dp(unit.Dp(2.5)))
		paint.FillShape(gtx.Ops, colorAccent, clip.Stroke{Path: p.End(), Width: w}.Op())
	}
	return layout.Dimensions{Size: size}
}

// checkbox é a caixinha clicável usada nas listas de tarefas.
func checkbox(gtx layout.Context, th *Theme, click *widget.Clickable, checked bool) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := colorPanel
		switch {
		case click.Pressed():
			bg = colorPressed
		case click.Hovered():
			bg = colorHover
		}
		return checkboxShapeBg(gtx, checked, bg)
	})
}

// expandButton desenha o botão de voltar ao tamanho cheio: a caixinha com a
// seta diagonal do rascunho. É desenhado à mão porque a fonte padrão não tem
// glifo para as setas de expandir.
func expandButton(gtx layout.Context, th *Theme, click *widget.Clickable) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		w := gtx.Dp(unit.Dp(34))
		h := gtx.Dp(unit.Dp(24))
		size := image.Pt(w, h)

		bg := colorPanel
		switch {
		case click.Pressed():
			bg = colorPressed
		case click.Hovered():
			bg = colorHover
		}
		fillRRect(gtx, size, unit.Dp(8), bg)
		strokeRRect(gtx, size, unit.Dp(8), strokeNormal, colorInk)

		fw, fh := float32(w), float32(h)
		stroke := float32(gtx.Dp(unit.Dp(1.6)))

		// Diagonal principal, do canto inferior esquerdo ao superior direito.
		var line clip.Path
		line.Begin(gtx.Ops)
		line.MoveTo(ptf(fw*0.30, fh*0.68))
		line.LineTo(ptf(fw*0.70, fh*0.32))
		paint.FillShape(gtx.Ops, colorInk, clip.Stroke{Path: line.End(), Width: stroke}.Op())

		// Uma farpa em cada ponta, apontando para fora.
		var heads clip.Path
		heads.Begin(gtx.Ops)
		heads.MoveTo(ptf(fw*0.30, fh*0.48))
		heads.LineTo(ptf(fw*0.30, fh*0.68))
		heads.LineTo(ptf(fw*0.52, fh*0.68))
		heads.MoveTo(ptf(fw*0.70, fh*0.52))
		heads.LineTo(ptf(fw*0.70, fh*0.32))
		heads.LineTo(ptf(fw*0.48, fh*0.32))
		paint.FillShape(gtx.Ops, colorInk, clip.Stroke{Path: heads.End(), Width: stroke}.Op())

		return layout.Dimensions{Size: size}
	})
}

// separator desenha uma linha horizontal fina.
func separator(gtx layout.Context) layout.Dimensions {
	h := gtx.Dp(unit.Dp(1))
	size := image.Pt(gtx.Constraints.Max.X, h)
	paint.FillShape(gtx.Ops, colorMuted, clip.Rect{Max: size}.Op())
	return layout.Dimensions{Size: size}
}

// spacerY devolve um espaçador vertical.
func spacerY(v unit.Dp) layout.Spacer { return layout.Spacer{Height: v} }

// spacerX devolve um espaçador horizontal.
func spacerX(v unit.Dp) layout.Spacer { return layout.Spacer{Width: v} }

// editorBox desenha um campo de texto dentro de uma caixa com contorno.
func (t *Theme) editorBox(gtx layout.Context, ed *widget.Editor, hint string, minHeight unit.Dp) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	inner := layout.UniformInset(unit.Dp(9)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = gtx.Dp(minHeight)
		e := material.Editor(t.Theme, ed, hint)
		e.Color = colorInk
		e.HintColor = colorInkFaint
		e.TextSize = unit.Sp(15)
		return e.Layout(gtx)
	})
	call := macro.Stop()

	size := inner.Size
	size.X = gtx.Constraints.Max.X
	fillRRect(gtx, size, unit.Dp(9), colorPanel)
	border := colorInkFaint
	if gtx.Focused(ed) {
		border = colorInk
	}
	strokeRRect(gtx, size, unit.Dp(9), strokeThin, border)
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

// field empilha um rótulo pequeno sobre um campo de texto.
func (t *Theme) field(gtx layout.Context, name string, ed *widget.Editor, hint string) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(t.small(name).Layout),
		layout.Rigid(spacerY(4).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return t.editorBox(gtx, ed, hint, unit.Dp(0))
		}),
	)
}
