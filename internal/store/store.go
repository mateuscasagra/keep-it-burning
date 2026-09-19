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
	"time"

	"github.com/dvet/keep-it-burning/internal/model"
)

// FileName é o nome do arquivo de dados dentro do diretório do app.
const FileName = "data.json"

// BackupSuffix é o sufixo do espelho do arquivo de dados. O espelho guarda os
// mesmos bytes da última gravação bem-sucedida e só é lido quando o arquivo
// principal some ou chega corrompido.
const BackupSuffix = ".bak"

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

// backupPath devolve o caminho do espelho do arquivo de dados.
func (s *Store) backupPath() string { return s.path + BackupSuffix }

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
//
// Antes de concluir "primeiro uso" ou de desistir com erro, o espelho é
// consultado: um arquivo principal ausente também pode ser o instante em que
// outra instância estava trocando o arquivo, e abrir vazio nessa hora custaria
// todas as tarefas na gravação seguinte.
func (s *Store) Load() (*model.State, error) {
	st, err := s.loadFile(s.path)
	if err == nil {
		return st, nil
	}
	if bak, bakErr := s.loadFile(s.backupPath()); bakErr == nil {
		return bak, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return model.NewState(), nil
	}
	return nil, err
}

// loadFile lê e normaliza um arquivo de estado. Erros de sistema chegam
// intactos para o chamador conseguir distinguir "não existe" de "ilegível".
func (s *Store) loadFile(path string) (*model.State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ler %s: %w", path, err)
	}

	st := &model.State{}
	if err := json.Unmarshal(data, st); err != nil {
		return nil, fmt.Errorf("interpretar %s: %w", path, err)
	}
	st.Normalize()
	return st, nil
}

// Save grava o estado e atualiza o espelho. A escrita é feita em um arquivo
// temporário no mesmo diretório e depois renomeada por cima do original: ou o
// arquivo antigo continua inteiro, ou o novo está completo — nunca um
// meio-termo corrompido, e nunca um instante sem arquivo nenhum.
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

	if err := writeAtomic(s.path, data); err != nil {
		return err
	}
	// O espelho vem depois: o estado já está gravado, então não conseguir
	// atualizá-lo é perder uma rede de proteção, não perder os dados.
	_ = writeAtomic(s.backupPath(), data)
	return nil
}

// writeAtomic grava os bytes em path de forma que um leitor concorrente veja
// ou o conteúdo antigo inteiro, ou o novo inteiro.
func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".keepitburning-*.tmp")
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

	// os.Rename troca por cima do destino tanto no Windows quanto no Unix, e a
	// troca é atômica: um leitor vê o arquivo antigo ou o novo, nunca a
	// ausência dos dois. Apagar o original antes seria mais simples, mas abre
	// uma fresta em que o arquivo não existe — e outra instância lendo justo
	// nessa fresta entende que é o primeiro uso e abre sem nenhuma tarefa.
	if err := renameOver(tmpName, path); err != nil {
		// Último recurso, para o caso de o arquivo estar mesmo preso: apagar
		// antes reabre a fresta, mas o espelho gravado pelo Save cobre quem
		// tentar ler nesse intervalo.
		if rmErr := os.Remove(path); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			return fmt.Errorf("remover arquivo antigo: %w", rmErr)
		}
		if err := os.Rename(tmpName, path); err != nil {
			return fmt.Errorf("substituir %s: %w", path, err)
		}
	}
	return nil
}

// Quanto o renameOver insiste antes de desistir da troca direta.
const (
	renameTentativas = 20
	renameEspera     = 5 * time.Millisecond
)

// renameOver renomeia tmpName por cima de path, insistindo por um instante. No
// Windows a troca é recusada com "acesso negado" enquanto outro processo tiver
// o arquivo aberto — e até um os.Stat conta. A fresta dura microssegundos, então
// repetir resolve onde apagar o arquivo só faria estrago.
func renameOver(tmpName, path string) error {
	var err error
	for i := 0; i < renameTentativas; i++ {
		if err = os.Rename(tmpName, path); err == nil {
			return nil
		}
		time.Sleep(renameEspera)
	}
	return err
}
