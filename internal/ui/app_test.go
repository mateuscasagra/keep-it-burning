package ui

import (
	"image"
	"testing"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/dvet/keep-it-burning/internal/model"
	"github.com/dvet/keep-it-burning/internal/timer"
)

// applyWindow aplica as opções em uma configuração de janela, como o Gio faria,
// e devolve o resultado. Config zerada equivale a "nada foi pedido".
func applyWindow(opts []app.Option) app.Config {
	var cfg app.Config
	for _, o := range opts {
		o(unit.Metric{PxPerDp: 1, PxPerSp: 1}, &cfg)
	}
	return cfg
}

func TestWindowOptionsFor(t *testing.T) {
	tests := []struct {
		name         string
		target, prev viewMode
		mode         app.WindowMode
		wantResize   bool
		wantSize     image.Point
	}{
		// O bug: com a janela maximizada, escolher trabalho ou estudo na tela
		// inicial reentrava na visualização completa e pedia o tamanho de
		// janela, tirando o app do maximizado.
		{"maximizada continua maximizada", viewFull, viewFull, app.Maximized, false, image.Point{}},
		{"tela cheia continua em tela cheia", viewFull, viewFull, app.Fullscreen, false, image.Point{}},
		// Mesmo em janela normal, reentrar no painel não é pedido de resize: o
		// tamanho que o usuário escolheu na mão precisa ser respeitado.
		{"janela mantém o tamanho do usuário", viewFull, viewFull, app.Windowed, false, image.Point{}},
		// Trocar de forma de visualização, aí sim, redimensiona.
		{"do foco para o painel", viewFull, viewFocus, app.Windowed, true, fullSize},
		{"do painel para o foco", viewFocus, viewFull, app.Windowed, true, focusSize},
		// A janela de canto precisa encolher mesmo saindo do maximizado.
		{"maximizada indo para o foco", viewFocus, viewFull, app.Maximized, true, focusSize},
	}
	for _, tc := range tests {
		opts := windowOptionsFor(tc.target, tc.prev, tc.mode)
		if got := len(opts) > 0; got != tc.wantResize {
			t.Errorf("%s: mexeu na janela = %v, quero %v", tc.name, got, tc.wantResize)
			continue
		}
		if !tc.wantResize {
			continue
		}
		cfg := applyWindow(opts)
		if cfg.Size != tc.wantSize {
			t.Errorf("%s: tamanho = %v, quero %v", tc.name, cfg.Size, tc.wantSize)
		}
		// app.Size sempre devolve a janela ao modo janela; é justamente por isso
		// que ele não pode ser chamado à toa.
		if cfg.Mode != app.Windowed {
			t.Errorf("%s: modo = %v, quero janela", tc.name, cfg.Mode)
		}
	}
}

// Prova de que o caminho da tela inicial não encosta na janela: com win nulo,
// qualquer tentativa de redimensionar derrubaria o teste.
func TestEntrarNoPainelNaoRedimensionaJanelaMaximizada(t *testing.T) {
	a := &App{
		state:   model.NewState(),
		mode:    model.ModeWork,
		screen:  screenHome,
		vmode:   viewFull,
		winMode: app.Maximized,
	}
	a.tmr = timer.New(a.mode)

	a.setView(viewFull)

	if a.screen != screenDashboard {
		t.Errorf("tela = %v, quero o dashboard", a.screen)
	}
	if a.vmode != viewFull {
		t.Errorf("visualização = %v, quero a completa", a.vmode)
	}
}
