package ui

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Decodificadores das imagens exibidas no preview de anexos.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/dvet/keep-it-burning/internal/model"
)

// taskViewScreen mostra os detalhes de uma tarefa, sem edição. Abre pelo
// botão "ver" das listas e guarda a tela de origem para voltar ao mesmo
// lugar — abrir e fechar a visualização não mexe no cronômetro.
type taskViewScreen struct {
	taskID   string
	returnTo screenID

	back  widget.Clickable
	check widget.Clickable
	list  widget.List
	// openBtns são os botões "abrir": primeiro os links, depois os arquivos.
	openBtns []widget.Clickable
	// imgs guarda as imagens decodificadas para o preview, por caminho.
	imgs map[string]*previewImage
}

// previewImage é uma imagem decodificada (ou a marca de que falhou).
type previewImage struct {
	op paint.ImageOp
	ok bool
}

// preview decodifica (uma única vez) a imagem do caminho para exibir na tela.
func (s *taskViewScreen) preview(path string) *previewImage {
	if s.imgs == nil {
		s.imgs = map[string]*previewImage{}
	}
	if p, ok := s.imgs[path]; ok {
		return p
	}
	p := &previewImage{}
	if f, err := os.Open(path); err == nil {
		if img, _, err := image.Decode(f); err == nil {
			p.op = paint.NewImageOp(img)
			p.ok = true
		}
		f.Close()
	}
	s.imgs[path] = p
	return p
}

// isImagePath informa se o arquivo é uma imagem que o app sabe exibir.
func isImagePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}

// isVideoPath informa se o arquivo é um vídeo (aberto no player do sistema).
func isVideoPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".webm":
		return true
	}
	return false
}

func (s *taskViewScreen) init(a *App) {
	s.list.Axis = layout.Vertical
}

// open aponta a tela para uma tarefa e registra de onde ela foi aberta.
func (s *taskViewScreen) open(id string, returnTo screenID) {
	s.taskID = id
	s.returnTo = returnTo
}

func (s *taskViewScreen) Layout(gtx layout.Context, a *App) layout.Dimensions {
	if s.back.Clicked(gtx) {
		a.goTo(s.returnTo)
	}
	if s.check.Clicked(gtx) {
		a.toggleDone(s.taskID)
	}

	t, ok := a.state.Task(s.taskID)
	if !ok {
		// A tarefa sumiu (excluída em outra tela); volta sem desenhar nada.
		a.goTo(s.returnTo)
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	refs := append(append([]string(nil), t.Links...), t.Files...)
	if len(s.openBtns) != len(refs) {
		s.openBtns = make([]widget.Clickable, len(refs))
	}
	for i := range s.openBtns {
		if s.openBtns[i].Clicked(gtx) {
			a.openAttachment(refs[i])
		}
	}

	prio, hasPrio := a.state.Priority(a.mode, t.PriorityID)
	prioLabel := "—"
	if hasPrio {
		prioLabel = prio.Title + " · " + itoa(prio.Value)
	}
	catLabel := "—"
	if cat, ok := a.state.Category(a.mode, t.CategoryID); ok {
		catLabel = cat.Title
	}

	dueColor := colorInk
	if t.Overdue(time.Now()) {
		dueColor = colorDanger
	}

	status := "Pendente"
	statusColor := colorAccentSoft
	if t.Done {
		status = "Concluída"
		statusColor = colorAccent
	}

	return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.th.panelFill(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							b := a.th.button("←")
							b.PadX, b.PadY = unit.Dp(13), unit.Dp(5)
							b.Size = unit.Sp(18)
							b.Radius = unit.Dp(9)
							return b.Layout(gtx, a.th, &s.back)
						}),
						layout.Rigid(spacerX(10).Layout),
						layout.Rigid(a.th.heading("Tarefa").Layout),
						layout.Flexed(1, layout.Spacer{}.Layout),
						layout.Rigid(a.th.label(unit.Sp(13), status, statusColor).Layout),
					)
				}),
				layout.Rigid(spacerY(12).Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return material.List(a.th.Theme, &s.list).Layout(gtx, 1,
						func(gtx layout.Context, _ int) layout.Dimensions {
							return s.details(gtx, a, t, prioLabel, catLabel, dueColor)
						})
				}),
				layout.Rigid(spacerY(10).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return checkbox(gtx, a.th, &s.check, t.Done)
						}),
						layout.Rigid(spacerX(9).Layout),
						layout.Rigid(a.th.body("Marcar como concluída").Layout),
					)
				}),
			)
		})
	})
}

// details é o corpo rolável da visualização: título, datas e o resumo.
func (s *taskViewScreen) details(gtx layout.Context, a *App, t model.Task, prioLabel, catLabel string, dueColor color.NRGBA) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(a.th.label(unit.Sp(19), t.Title, colorInk).Layout),
		layout.Rigid(spacerY(4).Layout),
		layout.Rigid(a.th.small(a.mode.Label()).Layout),
		layout.Rigid(spacerY(12).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return statLine(gtx, a.th, "Prioridade", prioLabel)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return statLine(gtx, a.th, "Categoria", catLabel)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return statLineColor(gtx, a.th, "Data incluída", formatDateTime(t.CreatedAt), colorInk)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return statLineColor(gtx, a.th, "Data limite", formatDateTime(t.DueAt), dueColor)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !t.Done {
				return layout.Dimensions{}
			}
			return statLineColor(gtx, a.th, "Concluída em", formatDateTime(t.DoneAt), colorInk)
		}),
		layout.Rigid(spacerY(10).Layout),
		layout.Rigid(separator),
		layout.Rigid(spacerY(10).Layout),
		layout.Rigid(a.th.small("Resumo da tarefa").Layout),
		layout.Rigid(spacerY(4).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			desc := t.Description
			c := colorInk
			if desc == "" {
				desc = "Sem resumo."
				c = colorInkFaint
			}
			return a.th.label(unit.Sp(14), desc, c).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(t.Links) == 0 {
				return layout.Dimensions{}
			}
			return s.linksSection(gtx, a, t.Links)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(t.Files) == 0 {
				return layout.Dimensions{}
			}
			return s.filesSection(gtx, a, t.Files, len(t.Links))
		}),
	)
}

// attachRow desenha uma linha de anexo: nome truncado e o botão "abrir".
func (s *taskViewScreen) attachRow(gtx layout.Context, a *App, name string, btn int) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, a.th.label(unit.Sp(13), truncate(name, 26), colorInk).Layout),
				layout.Rigid(spacerX(8).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if btn >= len(s.openBtns) {
						return layout.Dimensions{}
					}
					return a.th.tiny("abrir").Layout(gtx, a.th, &s.openBtns[btn])
				}),
			)
		})
}

// linksSection lista os links da tarefa.
func (s *taskViewScreen) linksSection(gtx layout.Context, a *App, links []string) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(spacerY(10).Layout),
		layout.Rigid(separator),
		layout.Rigid(spacerY(10).Layout),
		layout.Rigid(a.th.small("Links").Layout),
		layout.Rigid(spacerY(4).Layout),
	}
	for i, ref := range links {
		i, ref := i, ref
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.attachRow(gtx, a, ref, i)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// filesSection lista os arquivos: imagens ganham preview dentro do app;
// vídeos e outros formatos abrem no aplicativo padrão pelo "abrir".
func (s *taskViewScreen) filesSection(gtx layout.Context, a *App, files []string, btnBase int) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(spacerY(10).Layout),
		layout.Rigid(separator),
		layout.Rigid(spacerY(10).Layout),
		layout.Rigid(a.th.small("Arquivos").Layout),
		layout.Rigid(spacerY(4).Layout),
	}
	for i, path := range files {
		i, path := i, path
		name := filepath.Base(path)
		if isVideoPath(path) {
			name = name + " · vídeo"
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.attachRow(gtx, a, name, btnBase+i)
		}))
		if isImagePath(path) {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				p := s.preview(path)
				if !p.ok {
					return a.th.label(unit.Sp(12), "não foi possível carregar a imagem", colorInkFaint).Layout(gtx)
				}
				return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(6)}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(180))
						return widget.Image{Src: p.op, Fit: widget.Contain, Position: layout.NW}.Layout(gtx)
					})
			}))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// statLineColor é o statLine com cor própria no valor, para destacar prazos
// vencidos na visualização.
func statLineColor(gtx layout.Context, th *Theme, label, value string, c color.NRGBA) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
			layout.Flexed(1, th.small(label).Layout),
			layout.Rigid(spacerX(8).Layout),
			layout.Rigid(th.label(unit.Sp(14), value, c).Layout),
		)
	})
}
