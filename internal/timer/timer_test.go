package timer

import (
	"sync"
	"testing"
	"time"

	"github.com/dvet/keep-it-burning/internal/model"
)

// fakeClock é um relógio controlado pelo teste: o tempo só anda quando
// mandamos. Sem isso os testes de cronômetro virariam sleeps lentos e instáveis.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, time.February, 5, 9, 0, 0, 0, time.Local)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestTimer() (*Timer, *fakeClock) {
	c := newClock()
	t := New(model.ModeWork)
	t.SetClock(c.Now)
	return t, c
}

func TestTimerComecaZeradoEParado(t *testing.T) {
	tm, _ := newTestTimer()
	if tm.Running() {
		t.Error("o cronômetro deveria começar parado")
	}
	if tm.Elapsed() != 0 {
		t.Errorf("Elapsed inicial = %v, quero 0", tm.Elapsed())
	}
	if tm.Mode() != model.ModeWork {
		t.Errorf("Mode = %v, quero trabalho", tm.Mode())
	}
}

func TestElapsedContaEnquantoRoda(t *testing.T) {
	tm, clk := newTestTimer()
	tm.Start()

	clk.Advance(90 * time.Second)
	if got := tm.Elapsed(); got != 90*time.Second {
		t.Fatalf("Elapsed = %v, quero 1m30s", got)
	}
	if !tm.Running() {
		t.Error("deveria estar rodando")
	}
}

func TestPauseCongelaOTempo(t *testing.T) {
	tm, clk := newTestTimer()
	tm.Start()
	clk.Advance(5 * time.Minute)
	tm.Pause()

	// Tempo passa com o cronômetro pausado: não pode entrar na conta.
	clk.Advance(30 * time.Minute)

	if got := tm.Elapsed(); got != 5*time.Minute {
		t.Fatalf("Elapsed depois da pausa = %v, quero 5min", got)
	}
	if tm.Running() {
		t.Error("não deveria estar rodando")
	}
}

func TestRetomarSomaOsTrechos(t *testing.T) {
	tm, clk := newTestTimer()

	tm.Start()
	clk.Advance(10 * time.Minute)
	tm.Pause()

	clk.Advance(1 * time.Hour) // café

	tm.Start()
	clk.Advance(20 * time.Minute)

	if got := tm.Elapsed(); got != 30*time.Minute {
		t.Fatalf("Elapsed = %v, quero 30min (10 + 20)", got)
	}
}

func TestStartDuploNaoReiniciaOTrecho(t *testing.T) {
	tm, clk := newTestTimer()
	tm.Start()
	clk.Advance(10 * time.Minute)
	tm.Start() // clique repetido

	if got := tm.Elapsed(); got != 10*time.Minute {
		t.Fatalf("Elapsed = %v, quero 10min — o segundo Start não pode zerar o trecho", got)
	}
}

func TestPauseDuploNaoGeraSessaoVazia(t *testing.T) {
	tm, clk := newTestTimer()
	tm.Start()
	clk.Advance(10 * time.Minute)
	tm.Pause()
	tm.Pause()

	if got := len(tm.Stop()); got != 1 {
		t.Fatalf("quero 1 sessão, tenho %d", got)
	}
}

func TestToggleAlterna(t *testing.T) {
	tm, clk := newTestTimer()

	if running := tm.Toggle(); !running {
		t.Fatal("o primeiro Toggle deveria ligar")
	}
	clk.Advance(3 * time.Minute)
	if running := tm.Toggle(); running {
		t.Fatal("o segundo Toggle deveria pausar")
	}
	clk.Advance(3 * time.Minute)
	if got := tm.Elapsed(); got != 3*time.Minute {
		t.Fatalf("Elapsed = %v, quero 3min", got)
	}
	if running := tm.Toggle(); !running {
		t.Fatal("o terceiro Toggle deveria retomar")
	}
}

func TestStopDevolveAsSessoesEZera(t *testing.T) {
	tm, clk := newTestTimer()
	inicio := clk.Now()

	tm.Start()
	clk.Advance(25 * time.Minute)
	tm.Pause()
	clk.Advance(5 * time.Minute)
	tm.Start()
	clk.Advance(15 * time.Minute)

	sessions := tm.Stop()

	if len(sessions) != 2 {
		t.Fatalf("quero 2 sessões (uma por trecho), tenho %d", len(sessions))
	}
	if got := sessions[0].Duration(); got != 25*time.Minute {
		t.Errorf("primeira sessão = %v, quero 25min", got)
	}
	if got := sessions[1].Duration(); got != 15*time.Minute {
		t.Errorf("segunda sessão = %v, quero 15min", got)
	}
	if !sessions[0].Start.Equal(inicio) {
		t.Errorf("a primeira sessão deveria começar em %v, tenho %v", inicio, sessions[0].Start)
	}
	for i, s := range sessions {
		if s.Mode != model.ModeWork {
			t.Errorf("sessão %d com modo %v, quero trabalho", i, s.Mode)
		}
		if s.ID == "" {
			t.Errorf("sessão %d ficou sem ID", i)
		}
	}

	// Depois do Stop o cronômetro está limpo.
	if tm.Running() || tm.Elapsed() != 0 {
		t.Error("Stop deveria zerar o cronômetro")
	}
	if got := tm.Stop(); len(got) != 0 {
		t.Errorf("o segundo Stop não pode repetir sessões, tenho %d", len(got))
	}
}

func TestStopFechaOTrechoEmAndamento(t *testing.T) {
	tm, clk := newTestTimer()
	tm.Start()
	clk.Advance(40 * time.Minute)

	sessions := tm.Stop() // sem pausar antes
	if len(sessions) != 1 || sessions[0].Duration() != 40*time.Minute {
		t.Fatalf("Stop deveria fechar o trecho aberto, tenho %+v", sessions)
	}
}

func TestSessionsNaoZeraOCronometro(t *testing.T) {
	tm, clk := newTestTimer()
	tm.Start()
	clk.Advance(10 * time.Minute)
	tm.Pause()

	snapshot := tm.Sessions()
	if len(snapshot) != 1 {
		t.Fatalf("quero 1 sessão no snapshot, tenho %d", len(snapshot))
	}
	if got := tm.Elapsed(); got != 10*time.Minute {
		t.Errorf("Sessions não pode zerar o tempo, Elapsed = %v", got)
	}
	// O snapshot é uma cópia: mexer nele não afeta o cronômetro.
	snapshot[0].End = snapshot[0].Start
	if got := tm.Sessions()[0].Duration(); got != 10*time.Minute {
		t.Errorf("o snapshot deveria ser cópia, mas a sessão interna virou %v", got)
	}
}

func TestDrainEsvaziaSemPararOCronometro(t *testing.T) {
	tm, clk := newTestTimer()

	tm.Start()
	clk.Advance(10 * time.Minute)
	tm.Pause()
	tm.Start()
	clk.Advance(5 * time.Minute)

	// Grava o que já fechou, com o cronômetro ainda correndo.
	drained := tm.Drain()
	if len(drained) != 1 || drained[0].Duration() != 10*time.Minute {
		t.Fatalf("Drain deveria devolver o trecho fechado, tenho %+v", drained)
	}
	if !tm.Running() {
		t.Error("Drain não pode parar o cronômetro")
	}
	if got := tm.Elapsed(); got != 15*time.Minute {
		t.Errorf("Elapsed = %v, quero 15min — Drain não zera o tempo exibido", got)
	}

	// O trecho já drenado não pode voltar no Stop, senão o tempo seria contado
	// duas vezes no histórico.
	clk.Advance(5 * time.Minute)
	rest := tm.Stop()
	if len(rest) != 1 || rest[0].Duration() != 10*time.Minute {
		t.Fatalf("Stop deveria devolver só o trecho restante, tenho %+v", rest)
	}
}

func TestReset(t *testing.T) {
	tm, clk := newTestTimer()
	tm.Start()
	clk.Advance(10 * time.Minute)
	tm.Reset()

	if tm.Running() || tm.Elapsed() != 0 || len(tm.Sessions()) != 0 {
		t.Fatal("Reset deveria limpar tudo")
	}
}

func TestUsoConcorrenteNaoDaCorrida(t *testing.T) {
	// Roda com -race: a UI lê Elapsed a cada quadro enquanto outra goroutine
	// mexe no cronômetro.
	tm, _ := newTestTimer()
	tm.Start()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				tm.Elapsed()
				tm.Running()
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			tm.Toggle()
		}
	}()
	wg.Wait()
	tm.Stop()
}

func TestFormat(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, "00:00"},
		{5 * time.Second, "00:05"},
		{5 * time.Minute, "05:00"},
		{59*time.Minute + 59*time.Second, "59:59"},
		{time.Hour, "01:00:00"},
		{2*time.Hour + 3*time.Minute + 4*time.Second, "02:03:04"},
		{-time.Minute, "00:00"},
	}
	for _, tc := range tests {
		if got := Format(tc.in); got != tc.want {
			t.Errorf("Format(%v) = %q, quero %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatHM(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, "0min"},
		{45 * time.Minute, "45min"},
		{time.Hour, "1h"},
		{8*time.Hour + 30*time.Minute, "8h 30min"},
		{-time.Hour, "0min"},
	}
	for _, tc := range tests {
		if got := FormatHM(tc.in); got != tc.want {
			t.Errorf("FormatHM(%v) = %q, quero %q", tc.in, got, tc.want)
		}
	}
}
