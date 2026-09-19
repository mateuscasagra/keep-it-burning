package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/dvet/keep-it-burning/internal/model"
	"github.com/dvet/keep-it-burning/internal/store"
)

// formHarness desenha o formulário fora de uma janela e injeta ponteiro nele.
// É a única forma de conferir de verdade o que o menu suspenso faz: olhar o
// desenho não diz se o clique chega na opção ou é engolido por quem está atrás.
type formHarness struct {
	t   *testing.T
	a   *App
	r   input.Router
	ops op.Ops
}

// newPendingHarness monta a mesma coisa apontando para a tela de tarefas, onde
// os menus precisam funcionar por cima da tabela.
func newPendingHarness(t *testing.T, subs int) *formHarness {
	t.Helper()
	h := newFormHarness(t, subs)
	h.a.pending.init(h.a)
	h.a.pending.open(h.a)
	h.a.screen = screenPending
	h.frame()
	return h
}

func newFormHarness(t *testing.T, subs int) *formHarness {
	t.Helper()
	st := model.NewState()
	cfg := st.SettingsFor(model.ModeWork)
	cfg.Categories = []model.Category{{ID: "c1", Title: "Reunião"}}
	for i := 1; i <= subs; i++ {
		cfg.Subcategories = append(cfg.Subcategories, model.Subcategory{
			ID:    "s" + itoa(i),
			Title: "Subcategoria " + itoa(i),
		})
	}
	st.SetSettings(model.ModeWork, cfg)

	a := &App{th: NewTheme(), state: st, mode: model.ModeWork, screen: screenTaskForm,
		startedAt: time.Now(), st: store.New("data.json")}
	a.form.init(a)
	a.form.openNew(a, screenDashboard)

	h := &formHarness{t: t, a: a}
	h.frame()
	return h
}

// frame desenha um quadro e registra os widgets para o roteador de eventos.
func (h *formHarness) frame() {
	h.ops.Reset()
	gtx := layout.Context{
		Ops:         &h.ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(fullSize),
		Now:         time.Now(),
		Source:      h.r.Source(),
	}
	h.a.layout(gtx)
	h.r.Frame(&h.ops)
}

func pt(p image.Point) f32.Point { return f32.Pt(float32(p.X), float32(p.Y)) }

// move leva o ponteiro até um ponto e desenha o quadro em que os widgets
// enxergam o movimento.
func (h *formHarness) move(p image.Point) {
	h.r.Queue(pointer.Event{Kind: pointer.Move, Position: pt(p), Source: pointer.Mouse})
	h.frame()
}

// scroll gira a roda do mouse sobre um ponto.
func (h *formHarness) scroll(p image.Point, dy float32) {
	h.r.Queue(pointer.Event{Kind: pointer.Scroll, Position: pt(p),
		Scroll: f32.Pt(0, dy), Source: pointer.Mouse})
	h.frame()
}

// click dá um toque completo em um ponto.
func (h *formHarness) click(p image.Point) {
	h.r.Queue(
		pointer.Event{Kind: pointer.Press, Position: pt(p), Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: pt(p), Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
	)
	h.frame()
}

// locate procura o ponto da tela que cai sobre um widget, perguntando ao
// próprio widget se o ponteiro está em cima dele. Assim o teste não depende de
// coordenadas chutadas: se o formulário mudar de arranjo, ele continua achando.
func (h *formHarness) locate(what string, hovered func() bool, ys []int, x0, x1, step int) image.Point {
	h.t.Helper()
	for _, y := range ys {
		for x := x0; x < x1; x += step {
			p := image.Pt(x, y)
			h.move(p)
			if hovered() {
				return p
			}
		}
	}
	h.t.Fatalf("não achei %s na tela", what)
	return image.Point{}
}

// choiceRowYs são as alturas prováveis da linha dos campos de escolha nas duas
// telas que a usam.
var choiceRowYs = []int{205, 195, 215, 225, 172, 160, 185}

func (h *formHarness) trigger(what string, f *selectField) image.Point {
	return h.locate(what, f.btn.Hovered, choiceRowYs, 40, fullSize.X-40, 10)
}

// option acha a linha do menu aberto. A sondagem entra 30px para dentro do
// campo: na borda esquerda ainda é a moldura do cartão, não a opção.
func (h *formHarness) option(f *selectField, i int, trigger image.Point) image.Point {
	h.t.Helper()
	ys := make([]int, 0, 100)
	for y := trigger.Y + 20; y < trigger.Y+320; y += 3 {
		ys = append(ys, y)
	}
	x := trigger.X + 30
	return h.locate("a opção da lista", f.opts[i].Hovered, ys, x, x+1, 1)
}

func TestSelectFieldAbreEscolheEFecha(t *testing.T) {
	h := newFormHarness(t, 12)
	sub := &h.a.form.subSel
	if sub.open {
		t.Fatal("o campo devia começar fechado")
	}

	trigger := h.trigger("o campo de subcategoria", sub)
	h.click(trigger)
	if !sub.open {
		t.Fatal("clicar no campo devia abrir o menu")
	}

	// A opção 0 é "Nenhuma" e a 1 é a subcategoria 1. O menu é desenhado por
	// cima do formulário: o clique precisa chegar nele, e não no campo que
	// ficou embaixo.
	h.click(h.option(sub, 1, trigger))

	if sub.open {
		t.Error("escolher uma opção devia fechar o menu")
	}
	if got := h.a.form.subcategoryID; got != "s1" {
		t.Errorf("subcategoria escolhida = %q, quero %q", got, "s1")
	}
}

func TestSelectFieldFechaComCliqueFora(t *testing.T) {
	h := newFormHarness(t, 5)
	sub := &h.a.form.subSel

	h.click(h.trigger("o campo de subcategoria", sub))
	if !sub.open {
		t.Fatal("o menu devia estar aberto")
	}
	// Um ponto vazio, bem longe da lista.
	h.click(image.Pt(fullSize.X/2, fullSize.Y-60))
	if sub.open {
		t.Error("clicar fora devia fechar o menu")
	}
}

func TestSelectFieldTrocaDeCampoComUmCliqueSo(t *testing.T) {
	h := newFormHarness(t, 5)
	sub, prio := &h.a.form.subSel, &h.a.form.prioSel

	subAt := h.trigger("o campo de subcategoria", sub)
	prioAt := h.trigger("o campo de prioridade", prio)

	h.click(subAt)
	if !sub.open {
		t.Fatal("o menu da subcategoria devia estar aberto")
	}
	// Com um menu aberto, o clique no campo vizinho precisa abrir o vizinho, e
	// não só fechar este — senão o usuário clica duas vezes para tudo.
	h.click(prioAt)
	if sub.open {
		t.Error("o menu antigo devia ter fechado")
	}
	if !prio.open {
		t.Error("o menu do campo clicado devia ter aberto")
	}
}

func TestSelectFieldFechaClicandoNoProprioCampo(t *testing.T) {
	h := newFormHarness(t, 5)
	sub := &h.a.form.subSel

	at := h.trigger("o campo de subcategoria", sub)
	h.click(at)
	if !sub.open {
		t.Fatal("o menu devia estar aberto")
	}
	h.click(at)
	if sub.open {
		t.Error("clicar de novo no campo devia fechar o menu")
	}
}

func TestSelectFieldFiltraNaTelaDeTarefas(t *testing.T) {
	h := newPendingHarness(t, 8)
	// Uma tarefa de cada subcategoria, para o filtro ter o que recortar.
	st := h.a.state
	for i, sub := range []string{"s1", "s2"} {
		st.Tasks = append(st.Tasks, model.Task{
			ID: "t" + itoa(i), Mode: model.ModeWork, Title: "Tarefa " + itoa(i),
			PriorityID: "alta", SubcategoryID: sub, CreatedAt: time.Now(),
		})
	}
	h.frame()

	sub := &h.a.pending.subSel
	h.click(h.trigger("o filtro de subcategoria", sub))
	if !sub.open {
		t.Fatal("o filtro devia abrir o menu")
	}

	// A lista é "Todas", "Sem subcategoria", "Subcategoria 1"... então a
	// terceira opção é a primeira subcategoria de verdade.
	h.click(h.option(sub, 2, h.trigger("o filtro de subcategoria", sub)))

	if got := h.a.pending.subcategoryID; got != "s1" {
		t.Fatalf("filtro escolhido = %q, quero %q", got, "s1")
	}
	// E o filtro precisa realmente recortar a lista.
	got := h.a.pending.filter(st.PendingTasks(model.ModeWork))
	if len(got) != 1 || got[0].SubcategoryID != "s1" {
		t.Errorf("a lista filtrada tem %d tarefas: %v", len(got), got)
	}
}
