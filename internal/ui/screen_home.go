package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/dvet/keep-it-burning/internal/model"
	"github.com/dvet/keep-it-burning/internal/productivity"
)

// homeScreen é a tela de abertura: a fogueira grande e a escolha entre
// trabalho e estudo.
type homeScreen struct {
	work  widget.Clickable
	study widget.Clickable
	close widget.Clickable
}

func (s *homeScreen) Layout(gtx layout.Context, a *App) layout.Dimensions {
	if s.close.Clicked(gtx) {
		a.closeWindow()
	}
	if s.work.Clicked(gtx) {
		a.setMode(model.ModeWork)
		a.goTo(screenDashboard)
	}
	if s.study.Clicked(gtx) {
		a.setMode(model.ModeStudy)
		a.goTo(screenDashboard)
	}

	score := a.overallDayScore()
	stage := productivity.StageOf(score)

	return layout.UniformInset(unit.Dp(22)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.th.panelFill(gtx, unit.Dp(18), func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{}.Layout(gtx,
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = gtx.Constraints.Max
					return s.content(gtx, a, score, stage)
				}),
				// O X fica solto no canto, como no rascunho. O Stacked precisa
				// ocupar a área toda, senão o alinhamento NE não tem contra o
				// que se alinhar e o botão cai no canto superior esquerdo.
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = gtx.Constraints.Max
					return layout.NE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						b := a.th.button("✕")
						b.PadX, b.PadY = unit.Dp(12), unit.Dp(6)
						b.Size = unit.Sp(17)
						b.Radius = unit.Dp(9)
						return b.Layout(gtx, a.th, &s.close)
					})
				}),
			)
		})
	})
}

func (s *homeScreen) content(gtx layout.Context, a *App, score float64, stage productivity.Stage) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := a.th.label(unit.Sp(28), "Keep It Burning", colorInk)
				return l.Layout(gtx)
			}),
			layout.Rigid(spacerY(4).Layout),
			layout.Rigid(a.th.small("Mantenha a chama acesa no trabalho e no estudo").Layout),
			layout.Rigid(spacerY(10).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// A fogueira é o elemento maior da tela; limitamos a área para
				// ela não empurrar os botões para fora em janelas baixas.
				gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(unit.Dp(280)))
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(350)))
				gtx.Constraints.Min = gtx.Constraints.Max
				return Fire{Intensity: productivity.Intensity(score), Time: a.animTime(gtx)}.Layout(gtx)
			}),
			layout.Rigid(spacerY(6).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(a.th.label(unit.Sp(14), stage.Label(), colorAccent).Layout),
					layout.Rigid(spacerX(8).Layout),
					layout.Rigid(a.th.small("· produtividade de hoje: "+formatScore(score)).Layout),
				)
			}),
			layout.Rigid(spacerY(18).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						b := a.th.button("Trabalho")
						b.Size, b.PadX, b.PadY = unit.Sp(18), unit.Dp(30), unit.Dp(13)
						return b.Layout(gtx, a.th, &s.work)
					}),
					layout.Rigid(spacerX(18).Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						b := a.th.button("Estudo")
						b.Size, b.PadX, b.PadY = unit.Sp(18), unit.Dp(30), unit.Dp(13)
						return b.Layout(gtx, a.th, &s.study)
					}),
				)
			}),
		)
	})
}
