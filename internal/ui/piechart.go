package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/dvet/keep-it-burning/internal/productivity"
)

// Paleta das fatias resolvidas, na ordem em que as prioridades aparecem (da
// mais pesada para a mais leve). As não resolvidas usam sempre o cinza.
var sliceColors = []color.NRGBA{
	{R: 0xe0, G: 0x3c, B: 0x1f, A: 0xff}, // vermelho
	{R: 0xf5, G: 0x9e, B: 0x0b, A: 0xff}, // laranja
	{R: 0xf6, G: 0xc7, B: 0x43, A: 0xff}, // amarelo
	{R: 0x8a, G: 0xb1, B: 0x7d, A: 0xff}, // verde
	{R: 0x6f, G: 0x9a, B: 0xb8, A: 0xff}, // azul
	{R: 0xa8, G: 0x8c, B: 0xc0, A: 0xff}, // roxo
}

var sliceUnresolved = color.NRGBA{R: 0xcf, G: 0xc9, B: 0xbf, A: 0xff}

// sliceColor devolve a cor da fatia i.
func sliceColor(i int, resolved bool) color.NRGBA {
	if !resolved {
		return sliceUnresolved
	}
	return sliceColors[i%len(sliceColors)]
}

// PieChart desenha a divisão das prioridades resolvidas e não resolvidas.
type PieChart struct {
	Slices []productivity.Slice
}

// Layout desenha o gráfico num quadrado centralizado nas restrições recebidas.
func (p PieChart) Layout(gtx layout.Context, th *Theme) layout.Dimensions {
	size := gtx.Constraints.Max
	side := size.X
	if size.Y < side {
		side = size.Y
	}
	if side <= 0 {
		return layout.Dimensions{Size: size}
	}

	cx := float32(size.X) / 2
	cy := float32(size.Y) / 2
	r := float32(side)/2 - float32(gtx.Dp(unit.Dp(2)))
	stroke := float32(gtx.Dp(strokeThin))

	// Sem dados o gráfico vira um círculo vazio, para o painel não "sumir".
	var total float64
	for _, s := range p.Slices {
		total += s.Value
	}
	if total <= 0 {
		ellipseStroke(gtx.Ops, cx, cy, r, r, colorInkFaint, stroke)
		return layout.Dimensions{Size: size}
	}

	// Começa no topo e gira no sentido horário, como no rascunho.
	angle := -math.Pi / 2
	colorIdx := 0
	for _, s := range p.Slices {
		if s.Value <= 0 {
			continue
		}
		sweep := 2 * math.Pi * (s.Value / total)
		c := sliceColor(colorIdx, s.Resolved)
		if s.Resolved {
			colorIdx++
		}
		wedge(gtx.Ops, cx, cy, r, float32(angle), float32(sweep), c)
		wedgeOutline(gtx.Ops, cx, cy, r, float32(angle), float32(sweep), stroke)
		angle += sweep
	}
	ellipseStroke(gtx.Ops, cx, cy, r, r, colorInk, stroke*1.4)

	return layout.Dimensions{Size: size}
}

// wedge pinta uma fatia como um leque de segmentos de reta. É preciso
// aproximar o arco porque clip.Path não tem primitiva de arco circular.
func wedge(ops *op.Ops, cx, cy, r, start, sweep float32, c color.NRGBA) {
	steps := arcSteps(sweep)
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(f32.Pt(cx, cy))
	for i := 0; i <= steps; i++ {
		a := start + sweep*float32(i)/float32(steps)
		p.LineTo(f32.Pt(cx+r*cos(a), cy+r*sin(a)))
	}
	p.Close()
	paint.FillShape(ops, c, clip.Outline{Path: p.End()}.Op())
}

// wedgeOutline desenha as duas linhas retas que separam a fatia das vizinhas.
func wedgeOutline(ops *op.Ops, cx, cy, r, start, sweep, width float32) {
	// Uma fatia que ocupa o círculo inteiro não tem divisória para desenhar.
	if sweep >= 2*math.Pi-0.001 {
		return
	}
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(f32.Pt(cx+r*cos(start), cy+r*sin(start)))
	p.LineTo(f32.Pt(cx, cy))
	p.LineTo(f32.Pt(cx+r*cos(start+sweep), cy+r*sin(start+sweep)))
	paint.FillShape(ops, colorInk, clip.Stroke{Path: p.End(), Width: width}.Op())
}

// arcSteps escolha quantos segmentos aproximam o arco: o bastante para a
// borda parecer curva sem gastar geometria à toa.
func arcSteps(sweep float32) int {
	steps := int(math.Abs(float64(sweep)) / 0.08)
	if steps < 2 {
		return 2
	}
	if steps > 96 {
		return 96
	}
	return steps
}

func cos(a float32) float32 { return float32(math.Cos(float64(a))) }
func sin(a float32) float32 { return float32(math.Sin(float64(a))) }

// pieLegend desenha a legenda do gráfico: um quadradinho colorido, o nome da
// fatia e a porcentagem.
func pieLegend(gtx layout.Context, th *Theme, slices []productivity.Slice) layout.Dimensions {
	if len(slices) == 0 {
		return th.small("Sem tarefas no período").Layout(gtx)
	}
	children := make([]layout.FlexChild, 0, len(slices)*2)
	colorIdx := 0
	for _, s := range slices {
		c := sliceColor(colorIdx, s.Resolved)
		if s.Resolved {
			colorIdx++
		}
		s := s
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					side := gtx.Dp(unit.Dp(11))
					sz := image.Pt(side, side)
					fillRRect(gtx, sz, unit.Dp(3), c)
					strokeRRect(gtx, sz, unit.Dp(3), unit.Dp(1), colorInk)
					return layout.Dimensions{Size: sz}
				}),
				layout.Rigid(spacerX(7).Layout),
				layout.Flexed(1, th.small(s.Label).Layout),
				layout.Rigid(spacerX(6).Layout),
				layout.Rigid(th.small(pct(s.Fraction)).Layout),
			)
		}))
		children = append(children, layout.Rigid(spacerY(5).Layout))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
