package ui

import (
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
)

// dateRange é um filtro de intervalo: dois campos de data que recortam a lista
// pelo dia. Ele se aplica sozinho, sem botão de "aplicar" — um campo pela
// metade simplesmente não restringe nada, então a lista não pisca a cada tecla.
type dateRange struct {
	from widget.Editor
	to   widget.Editor
	// Textos do quadro anterior, para a máscara que insere as barras.
	fromLast string
	toLast   string
}

func (r *dateRange) init() {
	r.from.SingleLine = true
	r.to.SingleLine = true
}

// update aplica a máscara de data aos dois campos.
func (r *dateRange) update() {
	maskDateField(&r.from, &r.fromLast, 8)
	maskDateField(&r.to, &r.toLast, 8)
}

// reset limpa o intervalo.
func (r *dateRange) reset() {
	r.from.SetText("")
	r.to.SetText("")
	r.fromLast, r.toLast = "", ""
}

// bounds devolve os limites do intervalo já interpretados. O fim é exclusivo e
// inclui o dia inteiro digitado: quem escreve 17/09 espera ver o que aconteceu
// às 18h de 17/09.
func (r *dateRange) bounds() (from, to time.Time, hasFrom, hasTo bool) {
	return rangeBounds(r.from.Text(), r.to.Text())
}

// rangeBounds interpreta o par de datas digitado. Texto que ainda não forma uma
// data não vira limite nenhum.
func rangeBounds(fromTxt, toTxt string) (from, to time.Time, hasFrom, hasTo bool) {
	if t, err := parseDateOnly(fromTxt); err == nil {
		from, hasFrom = t, true
	}
	if t, err := parseDateOnly(toTxt); err == nil {
		to, hasTo = t.Add(24*time.Hour), true
	}
	return from, to, hasFrom, hasTo
}

// invalid informa se algum campo tem texto que ainda não forma uma data. Serve
// para avisar o usuário de que aquele pedaço do filtro não está valendo.
func (r *dateRange) invalid() bool {
	for _, ed := range []*widget.Editor{&r.from, &r.to} {
		if _, err := parseDateOnly(ed.Text()); err != nil && ed.Text() != "" {
			return true
		}
	}
	return false
}

// match informa se uma data cai no intervalo. Tarefa sem a data em questão
// fica de fora assim que o intervalo passa a valer: ela não cai em intervalo
// nenhum.
func (r *dateRange) match(t time.Time) bool {
	return matchRange(r.from.Text(), r.to.Text(), t)
}

// matchRange é a regra do intervalo, separada dos campos para poder ser
// conferida direto nos testes.
func matchRange(fromTxt, toTxt string, t time.Time) bool {
	from, to, hasFrom, hasTo := rangeBounds(fromTxt, toTxt)
	if !hasFrom && !hasTo {
		return true
	}
	if t.IsZero() {
		return false
	}
	if hasFrom && t.Before(from) {
		return false
	}
	if hasTo && !t.Before(to) {
		return false
	}
	return true
}

// dateRangeWidth é a largura de cada campo de data do intervalo.
const dateRangeWidth = unit.Dp(112)

// layout desenha o rótulo e os dois campos.
func (r *dateRange) layout(gtx layout.Context, th *Theme, label string) layout.Dimensions {
	field := func(ed *widget.Editor) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			w := gtx.Dp(dateRangeWidth)
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
			return th.editorBox(gtx, ed, "dd/mm/aaaa", unit.Dp(0))
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(th.small(label).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					// O aviso fica na linha do rótulo para a altura da faixa de
					// filtros não mudar enquanto o usuário digita.
					if !r.invalid() {
						return layout.Dimensions{}
					}
					return layout.Inset{Left: unit.Dp(6)}.Layout(gtx,
						th.label(unit.Sp(12), "· data incompleta", colorDanger).Layout)
				}),
			)
		}),
		layout.Rigid(spacerY(6).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				field(&r.from),
				layout.Rigid(spacerX(8).Layout),
				layout.Rigid(th.small("até").Layout),
				layout.Rigid(spacerX(8).Layout),
				field(&r.to),
			)
		}),
	)
}
