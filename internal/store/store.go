// Package store cuida da persistência do estado em disco. O formato é um
// único arquivo JSON legível, gravado de forma atômica para que um desligamento
// no meio da escrita não deixe o usuário sem os dados.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dvet/keep-it-burning/internal/model"
)

// FileName é o nome do arquivo de dados dentro do diretório do app.
const FileName = "data.json"

// Store lê e grava o estado em um caminho fixo.
type Store struct {
	path string
}

// New devolve um Store que usa o arquivo indicado.
func New(path string) *Store {
	return &Store{path: path}
}

// Path devolve o caminho do arquivo de dados — o app mostra isso ao usuário
// na tela de configuração.
func (s *Store) Path() string { return s.path }

// DefaultPath devolve o caminho padrão do arquivo de dados:
// %AppData%\KeepItBurning\data.json no Windows, e o equivalente conforme o
// sistema nas outras plataformas.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		// Sem diretório de configuração, cai para o lado do executável.
		exe, exeErr := os.Executable()
		if exeErr != nil {
			return "", fmt.Errorf("descobrir onde guardar os dados: %w", err)
		}
		return filepath.Join(filepath.Dir(exe), FileName), nil
	}
	return filepath.Join(dir, "KeepItBurning", FileName), nil
}

// Default devolve um Store no caminho padrão.
func Default() (*Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return New(path), nil
}

// Load lê o estado do disco. Se o arquivo ainda não existe, devolve um estado
// novo com as configurações padrão — é o primeiro uso do app, não um erro.
func (s *Store) Load() (*model.State, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		st := model.NewState()
		return st, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ler %s: %w", s.path, err)
	}

	st := &model.State{}
	if err := json.Unmarshal(data, st); err != nil {
		return nil, fmt.Errorf("interpretar %s: %w", s.path, err)
	}
	st.Normalize()
	return st, nil
}

// Save grava o estado. A escrita é feita em um arquivo temporário no mesmo
// diretório e depois renomeada por cima do original: ou o arquivo antigo
// continua inteiro, ou o novo está completo — nunca um meio-termo corrompido.
func (s *Store) Save(st *model.State) error {
	if st == nil {
		return errors.New("estado nulo")
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("criar %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar estado: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".keepitburning-*.tmp")
	if err != nil {
		return fmt.Errorf("criar arquivo temporário: %w", err)
	}
	tmpName := tmp.Name()
	// Se qualquer passo abaixo falhar, o temporário não pode ficar para trás.
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("gravar arquivo temporário: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sincronizar arquivo temporário: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("fechar arquivo temporário: %w", err)
	}

	// No Windows os.Rename não sobrescreve um arquivo aberto por outro
	// processo; remover antes torna a troca confiável nas duas plataformas.
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remover arquivo antigo: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("substituir %s: %w", s.path, err)
	}
	return nil
}
