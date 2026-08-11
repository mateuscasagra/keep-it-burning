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
)

// Cores da fogueira. As duas paletas são os extremos entre os quais a
// intensidade interpola: brasa apagada de um lado, fogo no talo do outro.
var (
	fireOuterCold = color.NRGBA{R: 0x7a, G: 0x2c, B: 0x18, A: 0xff}
	fireOuterHot  = color.NRGBA{R: 0xe8, G: 0x3b, B: 0x18, A: 0xff}
	fireMidCold   = color.NRGBA{R: 0x9c, G: 0x45, B: 0x1c, A: 0xff}
	fireMidHot    = color.NRGBA{R: 0xf9, G: 0x82, B: 0x0e, A: 0xff}
	fireInnerCold = color.NRGBA{R: 0xb8, G: 0x6a, B: 0x1e, A: 0xff}
	fireInnerHot  = color.NRGBA{R: 0xfd, G: 0xd8, B: 0x35, A: 0xff}

	logLight  = color.NRGBA{R: 0xc9, G: 0x9a, B: 0x6b, A: 0xff}
	logMid    = color.NRGBA{R: 0xa9, G: 0x77, B: 0x4b, A: 0xff}
	logDark   = color.NRGBA{R: 0x6d, G: 0x4a, B: 0x2f, A: 0xff}
	logBurnt  = color.NRGBA{R: 0x33, G: 0x2c, B: 0x27, A: 0xff}
	groundCol = color.NRGBA{R: 0xd7, G: 0xd1, B: 0xc7, A: 0xff}
)

// tongue descreve uma língua de fogo em coordenadas relativas ao tamanho do
// desenho, para que a fogueira funcione em qualquer escala.
type tongue struct {
	dx    float32 // deslocamento horizontal, em frações da largura do fogo
	w     float32 // largura, em frações da largura do fogo
	h     float32 // altura, em frações da altura máxima da chama
	phase float32 // defasagem da oscilação, para as línguas não baterem juntas
	speed float32 // velocidade da oscilação
	sway  float32 // amplitude do balanço da ponta
}

// As três camadas empilhadas dão o volume do desenho: vermelho por fora,
// laranja no meio, amarelo no núcleo. Em cada camada as línguas estão em
// ordem decrescente de tamanho — é essa ordem que define quais aparecem
// primeiro quando a produtividade sobe.
var (
	tonguesOuter = []tongue{
		{0.00, 0.62, 1.00, 0.0, 1.7, 0.05},
		{-0.20, 0.34, 0.70, 1.9, 2.3, 0.07},
		{0.21, 0.32, 0.66, 3.4, 2.1, 0.07},
		{-0.33, 0.20, 0.42, 0.8, 2.9, 0.09},
		{0.34, 0.19, 0.45, 2.6, 2.7, 0.09},
	}
	tonguesMid = []tongue{
		{0.00, 0.40, 0.80, 0.6, 2.0, 0.05},
		{-0.15, 0.22, 0.54, 2.2, 2.6, 0.07},
		{0.16, 0.21, 0.56, 4.1, 2.4, 0.07},
	}
	tonguesInner = []tongue{
		{0.00, 0.22, 0.62, 1.2, 2.4, 0.04},
		{-0.10, 0.13, 0.34, 3.0, 3.1, 0.06},
		{0.11, 0.12, 0.36, 0.4, 2.8, 0.06},
	}
)

// Fire desenha a fogueira animada. Intensity vai de 0 (brasa quase apagada) a
// 1 (fogueira no talo) e vem direto do score de produtividade; Time é o tempo
// decorrido em segundos, que move a animação.
type Fire struct {
	Intensity float64
	Time      float64
}

// Layout desenha a fogueira ocupando as restrições recebidas.
func (f Fire) Layout(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	// A fogueira é desenhada em proporção 5:4; sobrando espaço, ela é
	// centralizada em vez de esticada.
	w := float32(size.X)
	h := float32(size.Y)
	if w/h > 1.25 {
		w = h * 1.25
	} else {
		h = w / 1.25
	}
	offX := (float32(size.X) - w) / 2
	offY := (float32(size.Y) - h) / 2

	defer op.Affine(f32.Affine2D{}.Offset(f32.Pt(offX, offY))).Push(gtx.Ops).Pop()

	in := clamp01(f.Intensity)
	t := float32(f.Time)

	f.drawGround(gtx, w, h)
	f.drawGlow(gtx, w, h, in)
	f.drawFlames(gtx, w, h, in, t)
	f.drawLogs(gtx, w, h)
	f.drawEmbers(gtx, w, h, in, t)
	f.drawSparks(gtx, w, h, in, t)

	return layout.Dimensions{Size: size}
}

// drawGround desenha a sombra de chão sob a fogueira.
func (f Fire) drawGround(gtx layout.Context, w, h float32) {
	cx, cy := w*0.5, h*0.86
	rx, ry := w*0.40, h*0.075
	ellipse(gtx.Ops, cx, cy, rx, ry, groundCol)
	// Um segundo borrão levemente deslocado quebra a simetria e imita o
	// rabisco do desenho original.
	ellipse(gtx.Ops, cx-w*0.06, cy+h*0.02, rx*0.62, ry*0.6,
		color.NRGBA{R: 0xc4, G: 0xbd, B: 0xb2, A: 0xff})
}

// drawGlow desenha o brilho difuso ao redor do fogo, feito de elipses
// translúcidas sobrepostas — quanto mais forte o fogo, maior o halo.
func (f Fire) drawGlow(gtx layout.Context, w, h float32, in float64) {
	if in <= 0.05 {
		return
	}
	cx, cy := w*0.5, h*0.60
	// Várias elipses bem transparentes se acumulam num degradê suave; poucas
	// camadas opacas deixariam anéis visíveis em volta do fogo.
	const layers = 7
	spread := float32(0.26 + 0.30*in)
	for i := 0; i < layers; i++ {
		k := float32(layers-i) / layers
		rx := w * spread * k * 1.2
		ry := h * spread * k * 1.45
		c := fireOuterHot
		c.A = uint8(7 * in)
		if c.A == 0 {
			continue
		}
		ellipse(gtx.Ops, cx, cy, rx, ry, c)
	}
}

// drawFlames desenha as três camadas de chama.
func (f Fire) drawFlames(gtx layout.Context, w, h float32, in float64, t float32) {
	fireW := w * 0.40
	base := h * 0.82
	// Mesmo com produtividade zero sobra uma chaminha; ela some por completo
	// só visualmente, com as línguas encolhidas pelo fator de visibilidade.
	maxH := h * float32(0.24+0.60*in)
	cx := w * 0.5

	layers := []struct {
		tongues []tongue
		cold    color.NRGBA
		hot     color.NRGBA
	}{
		{tonguesOuter, fireOuterCold, fireOuterHot},
		{tonguesMid, fireMidCold, fireMidHot},
		{tonguesInner, fireInnerCold, fireInnerHot},
	}

	for _, l := range layers {
		col := lerpColor(l.cold, l.hot, in)
		for i, tg := range l.tongues {
			// Línguas maiores acendem primeiro; as pontas laterais só
			// aparecem quando a produtividade sobe.
			vis := clamp01((in - float64(i)*0.15) / 0.22)
			if i == 0 {
				// A língua central nunca some: mesmo com produtividade zero a
				// fogueira fica na brasa, com uma chaminha viva. Uma pilha de
				// lenha apagada não diria nada ao usuário.
				vis = 0.22 + 0.78*in
			}
			if vis <= 0.02 {
				continue
			}
			// A oscilação fica mais rápida e ampla com o fogo mais forte.
			speed := tg.speed * float32(0.7+0.6*in)
			osc := float32(math.Sin(float64(t*speed + tg.phase)))
			osc2 := float32(math.Sin(float64(t*speed*1.7 + tg.phase*1.3)))

			tw := fireW * tg.w * (0.85 + 0.15*osc2)
			th := maxH * tg.h * float32(vis) * (0.92 + 0.08*osc)
			sway := fireW * tg.sway * osc * float32(0.5+0.5*in)

			flame(gtx.Ops, cx+fireW*tg.dx, base, tw, th, sway, col)
		}
	}
}

// flame desenha uma língua de fogo: uma gota assimétrica cuja ponta balança.
func flame(ops *op.Ops, cx, base, w, h, sway float32, c color.NRGBA) {
	if h <= 0 || w <= 0 {
		return
	}
	half := w / 2
	tip := f32.Pt(cx+sway, base-h)

	// A silhueta é montada em quatro trechos: sobe pela esquerda até a
	// barriga, estrangula na cintura, curva até a ponta e desce pela direita.
	// É a cintura que separa uma língua de fogo de um triângulo.
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(f32.Pt(cx-half, base))
	p.CubeTo(
		f32.Pt(cx-half*1.14, base-h*0.14),
		f32.Pt(cx-half*1.08, base-h*0.44),
		f32.Pt(cx-half*0.36+sway*0.4, base-h*0.62),
	)
	p.CubeTo(
		f32.Pt(cx-half*0.16+sway*0.8, base-h*0.80),
		f32.Pt(cx-half*0.02+sway, base-h*0.93),
		tip,
	)
	p.CubeTo(
		f32.Pt(cx+half*0.06+sway, base-h*0.91),
		f32.Pt(cx+half*0.26+sway*0.8, base-h*0.78),
		f32.Pt(cx+half*0.38+sway*0.4, base-h*0.60),
	)
	p.CubeTo(
		f32.Pt(cx+half*1.08, base-h*0.42),
		f32.Pt(cx+half*1.14, base-h*0.14),
		f32.Pt(cx+half, base),
	)
	// Base levemente côncava, como no desenho.
	p.CubeTo(
		f32.Pt(cx+half*0.4, base-h*0.05),
		f32.Pt(cx-half*0.4, base-h*0.05),
		f32.Pt(cx-half, base),
	)
	p.Close()
	paint.FillShape(ops, c, clip.Outline{Path: p.End()}.Op())
}

// drawLogs desenha a pilha de lenha na frente da chama.
func (f Fire) drawLogs(gtx layout.Context, w, h float32) {
	stroke := float32(gtx.Dp(strokeThin))

	// Achas quase em pé no centro, carbonizadas pelo fogo. Elas ficam de pé
	// como uma barraquinha, e não cruzadas em X, para não roubarem a atenção
	// da chama que sobe atrás delas.
	woodLog(gtx.Ops, w*0.445, h*0.815, w*0.20, h*0.042, -1.28, logBurnt, stroke)
	woodLog(gtx.Ops, w*0.555, h*0.812, w*0.21, h*0.040, 1.30, logBurnt, stroke)
	woodLog(gtx.Ops, w*0.500, h*0.822, w*0.17, h*0.036, -1.49, logDark, stroke)

	// Toras laterais deitadas, com a ponta serrada virada para fora.
	woodLogWithEnd(gtx.Ops, w*0.33, h*0.850, w*0.24, h*0.085, -0.14, logMid, logLight, stroke, true)
	woodLogWithEnd(gtx.Ops, w*0.69, h*0.845, w*0.22, h*0.080, 0.16, logDark, logMid, stroke, false)
}

// woodLog desenha uma acha: um retângulo arredondado girado.
func woodLog(ops *op.Ops, cx, cy, length, thick, angle float32, fill color.NRGBA, stroke float32) {
	defer op.Affine(f32.Affine2D{}.Rotate(f32.Pt(cx, cy), angle)).Push(ops).Pop()
	rect := image.Rect(
		int(cx-length/2), int(cy-thick/2),
		int(cx+length/2), int(cy+thick/2),
	)
	if rect.Dx() <= 0 || rect.Dy() <= 0 {
		return
	}
	rr := clip.UniformRRect(rect, int(thick/2))
	paint.FillShape(ops, fill, rr.Op(ops))
	paint.FillShape(ops, colorInk, clip.Stroke{Path: rr.Path(ops), Width: stroke}.Op())
}

// woodLogWithEnd desenha uma tora com a ponta serrada visível, que é o detalhe
// que dá identidade ao desenho da fogueira.
func woodLogWithEnd(ops *op.Ops, cx, cy, length, thick, angle float32, fill, endFill color.NRGBA, stroke float32, endLeft bool) {
	woodLog(ops, cx, cy, length, thick, angle, fill, stroke)

	defer op.Affine(f32.Affine2D{}.Rotate(f32.Pt(cx, cy), angle)).Push(ops).Pop()
	ex := cx + length/2*0.86
	if endLeft {
		ex = cx - length/2*0.86
	}
	rx, ry := thick*0.30, thick*0.46
	ellipse(ops, ex, cy, rx, ry, endFill)
	ellipseStroke(ops, ex, cy, rx, ry, colorInk, stroke)
	// Anel interno: o miolo da madeira.
	ellipseStroke(ops, ex, cy, rx*0.45, ry*0.45, colorInk, stroke*0.8)
}

// drawEmbers desenha as brasas entre as achas. Elas continuam vivas mesmo com
// produtividade zero — a fogueira nunca apaga de vez, só fica na brasa.
func (f Fire) drawEmbers(gtx layout.Context, w, h float32, in float64, t float32) {
	spots := []struct{ x, y, r, phase float32 }{
		{0.44, 0.845, 0.016, 0.0},
		{0.53, 0.860, 0.013, 2.1},
		{0.49, 0.828, 0.010, 4.0},
		{0.58, 0.836, 0.011, 1.2},
	}
	hot := lerpColor(color.NRGBA{R: 0x8a, G: 0x2b, B: 0x12, A: 0xff}, fireInnerHot, 0.35+0.65*in)
	for _, s := range spots {
		pulse := 0.75 + 0.25*math.Sin(float64(t*2.2+s.phase))
		c := hot
		c.A = uint8(clamp01(0.55+0.45*in) * pulse * 255)
		ellipse(gtx.Ops, w*s.x, h*s.y, w*s.r, h*s.r*1.1, c)
	}
}

// drawSparks desenha as fagulhas que sobem. A quantidade acompanha a
// produtividade: dia fraco, nenhuma fagulha; dia forte, o ar cheio delas.
func (f Fire) drawSparks(gtx layout.Context, w, h float32, in float64, t float32) {
	n := int(in * 9)
	if n == 0 {
		return
	}
	base := h * 0.78
	for i := 0; i < n; i++ {
		// Cada fagulha tem um ciclo próprio; o módulo faz ela renascer na base
		// assim que chega ao topo.
		fi := float64(i)
		speed := 0.22 + 0.10*math.Mod(fi*0.37, 1)
		prog := math.Mod(float64(t)*speed+fi*0.618, 1)

		riseH := h * float32(0.30+0.32*in)
		y := base - riseH*float32(prog)
		drift := math.Sin(float64(t)*1.4+fi*2.2) * float64(w) * 0.06 * prog
		x := w*float32(0.5) + float32(math.Mod(fi*0.53, 1)-0.5)*w*0.28 + float32(drift)

		// A fagulha nasce forte e some conforme sobe.
		alpha := uint8((1 - prog) * (1 - prog) * 235 * in)
		if alpha < 6 {
			continue
		}
		c := fireInnerHot
		if i%3 == 0 {
			c = fireMidHot
		}
		c.A = alpha
		r := w * float32(0.006+0.005*(1-prog))
		ellipse(gtx.Ops, x, y, r, r*1.4, c)
	}
}

// ellipse pinta uma elipse cheia.
func ellipse(ops *op.Ops, cx, cy, rx, ry float32, c color.NRGBA) {
	if rx <= 0 || ry <= 0 {
		return
	}
	rect := image.Rect(int(cx-rx), int(cy-ry), int(cx+rx), int(cy+ry))
	if rect.Dx() <= 0 || rect.Dy() <= 0 {
		return
	}
	paint.FillShape(ops, c, clip.Ellipse(rect).Op(ops))
}

// ellipseStroke desenha só o contorno de uma elipse.
func ellipseStroke(ops *op.Ops, cx, cy, rx, ry float32, c color.NRGBA, width float32) {
	if rx <= 0 || ry <= 0 || width <= 0 {
		return
	}
	rect := image.Rect(int(cx-rx), int(cy-ry), int(cx+rx), int(cy+ry))
	if rect.Dx() <= 0 || rect.Dy() <= 0 {
		return
	}
	paint.FillShape(ops, c, clip.Stroke{Path: clip.Ellipse(rect).Path(ops), Width: width}.Op())
}

// ptf é um atalho para criar pontos em ponto flutuante.
func ptf(x, y float32) f32.Point { return f32.Pt(x, y) }

// clamp01 limita um valor ao intervalo [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// lerpColor interpola duas cores. t fora de [0,1] é grampeado.
func lerpColor(a, b color.NRGBA, t float64) color.NRGBA {
	t = clamp01(t)
	mix := func(x, y uint8) uint8 {
		return uint8(float64(x) + (float64(y)-float64(x))*t)
	}
	return color.NRGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: mix(a.A, b.A)}
}
