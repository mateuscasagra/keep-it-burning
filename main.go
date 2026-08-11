// Keep It Burning é um aplicativo de produtividade para trabalho e estudo.
// A fogueira da tela principal acompanha o quanto você produziu no dia: quanto
// mais tarefas entregues e mais tempo cumprido da meta, mais alta a chama.
package main

import (
	"flag"
	"log"
	"os"

	"gioui.org/app"

	"github.com/dvet/keep-it-burning/internal/store"
	"github.com/dvet/keep-it-burning/internal/ui"
)

func main() {
	dataPath := flag.String("data", "",
		"caminho do arquivo de dados (padrão: %AppData%\\KeepItBurning\\data.json)")
	flag.Parse()

	st, err := newStore(*dataPath)
	if err != nil {
		log.Fatalf("keep-it-burning: %v", err)
	}

	go func() {
		win := new(app.Window)
		a, err := ui.New(win, st)
		if err != nil {
			log.Printf("keep-it-burning: %v", err)
			os.Exit(1)
		}
		if err := a.Run(); err != nil {
			log.Printf("keep-it-burning: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	// app.Main precisa rodar na goroutine principal: é ela que conversa com a
	// fila de mensagens do sistema operacional.
	app.Main()
}

// newStore devolve o repositório de dados no caminho pedido, ou no padrão.
func newStore(path string) (*store.Store, error) {
	if path != "" {
		return store.New(path), nil
	}
	return store.Default()
}
