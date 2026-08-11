// Package timer implementa o cronômetro de trabalho/estudo: o relógio que roda
// na janela reduzida e que alimenta as sessões gravadas no histórico.
package timer

import (
	"sync"
	"time"

	"github.com/dvet/keep-it-burning/internal/model"
)

// Timer acumula tempo em trechos contínuos. Cada intervalo entre um Start (ou
// Resume) e a pausa seguinte vira uma model.Session — o tempo em pausa não é
// contado em lugar nenhum.
//
// É seguro usar de várias goroutines: a UI lê o tempo decorrido a cada quadro
// enquanto o restante do app pode pausar ou parar o cronômetro.
type Timer struct {
	mu sync.Mutex

	mode    model.Mode
	running bool

	// segStart é o começo do trecho em andamento; só vale quando running.
	segStart time.Time
	// elapsed é o tempo somado dos trechos já fechados.
	elapsed time.Duration
	// segments guarda os trechos fechados desde o último Reset.
	segments []model.Session

	// now permite injetar um relógio nos testes.
	now func() time.Time
}

// New cria um cronômetro parado para o modo indicado.
func New(mode model.Mode) *Timer {
	return &Timer{mode: mode, now: time.Now}
}

// SetClock troca a fonte de tempo. Existe para os testes; passar nil restaura
// o relógio real.
func (t *Timer) SetClock(now func() time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if now == nil {
		now = time.Now
	}
	t.now = now
}

// Mode devolve o modo do cronômetro.
func (t *Timer) Mode() model.Mode {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.mode
}

// Running informa se o cronômetro está correndo.
func (t *Timer) Running() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

// Start começa a contar. Chamar com o cronômetro já rodando não faz nada, para
// que um clique duplo no botão não reinicie o trecho em andamento.
func (t *Timer) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running {
		return
	}
	t.running = true
	t.segStart = t.now()
}

// Pause interrompe a contagem e fecha o trecho atual. Chamar com o cronômetro
// já pausado não faz nada.
func (t *Timer) Pause() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeSegment()
}

// Toggle alterna entre rodando e pausado, e devolve o novo estado. É o que o
// botão "Pausar"/"Retomar" da janela reduzida usa.
func (t *Timer) Toggle() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running {
		t.closeSegment()
	} else {
		t.running = true
		t.segStart = t.now()
	}
	return t.running
}

// closeSegment fecha o trecho em andamento. O chamador deve segurar o mutex.
func (t *Timer) closeSegment() {
	if !t.running {
		return
	}
	end := t.now()
	t.running = false
	if end.After(t.segStart) {
		t.elapsed += end.Sub(t.segStart)
		t.segments = append(t.segments, model.Session{
			ID:    model.NewID(),
			Mode:  t.mode,
			Start: t.segStart,
			End:   end,
		})
	}
	t.segStart = time.Time{}
}

// Elapsed devolve o tempo total contado, incluindo o trecho em andamento.
func (t *Timer) Elapsed() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	total := t.elapsed
	if t.running {
		if d := t.now().Sub(t.segStart); d > 0 {
			total += d
		}
	}
	return total
}

// Stop encerra o cronômetro e devolve as sessões acumuladas, zerando o estado
// interno. É chamado quando o usuário sai da janela reduzida: as sessões vão
// direto para o histórico.
func (t *Timer) Stop() []model.Session {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeSegment()
	out := t.segments
	t.segments = nil
	t.elapsed = 0
	return out
}

// Drain devolve as sessões já fechadas e as remove do cronômetro, sem parar a
// contagem nem zerar o tempo exibido. Serve para gravar o progresso em disco
// no meio de uma sessão longa, sem risco de gravar a mesma sessão duas vezes.
func (t *Timer) Drain() []model.Session {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := t.segments
	t.segments = nil
	return out
}

// Sessions devolve uma cópia das sessões já fechadas, sem zerar nada. Serve
// para salvar o progresso periodicamente em disco sem interromper a contagem.
func (t *Timer) Sessions() []model.Session {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]model.Session, len(t.segments))
	copy(out, t.segments)
	return out
}

// Reset zera tudo e para o cronômetro.
func (t *Timer) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.running = false
	t.segStart = time.Time{}
	t.elapsed = 0
	t.segments = nil
}

// Format devolve a duração no formato do relógio da tela: MM:SS abaixo de uma
// hora e HH:MM:SS a partir daí. Durações negativas viram 00:00.
func Format(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	h, m, s := total/3600, (total%3600)/60, total%60
	if h > 0 {
		return pad(h) + ":" + pad(m) + ":" + pad(s)
	}
	return pad(m) + ":" + pad(s)
}

// FormatHM devolve a duração como "8h 30min", usada nos resumos e indicadores.
func FormatHM(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Minutes())
	h, m := total/60, total%60
	switch {
	case h > 0 && m > 0:
		return itoa(h) + "h " + itoa(m) + "min"
	case h > 0:
		return itoa(h) + "h"
	default:
		return itoa(m) + "min"
	}
}

func pad(v int) string {
	if v < 10 {
		return "0" + itoa(v)
	}
	return itoa(v)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
