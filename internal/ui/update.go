package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Este arquivo implementa o botão "Atualizar" da tela inicial: recompila o
// código-fonte com o toolchain do Go e relança o app, para testar mudanças
// sem precisar buildar na mão a cada alteração.

// updateResult é o desfecho da recompilação feita em segundo plano.
type updateResult struct {
	exe string
	err error
}

// exeBinName é o nome do binário gerado pela atualização.
func exeBinName() string {
	if runtime.GOOS == "windows" {
		return "KeepItBurning.exe"
	}
	return "keep-it-burning"
}

// projectRoot localiza a pasta com o go.mod, subindo a partir do diretório
// atual e do diretório do executável.
func projectRoot() (string, error) {
	var candidates []string
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd)
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}
	for _, dir := range candidates {
		for d := dir; ; d = filepath.Dir(d) {
			if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
				return d, nil
			}
			if filepath.Dir(d) == d {
				break
			}
		}
	}
	return "", fmt.Errorf("go.mod não encontrado; abra o app a partir da pasta do projeto")
}

// rebuild recompila o projeto por cima do executável alvo e devolve o caminho
// do binário novo, pronto para ser relançado.
func rebuild(root string) (string, error) {
	target := filepath.Join(root, exeBinName())
	old := target + ".old"

	// O Windows não deixa sobrescrever um executável em uso, mas deixa
	// renomear: o processo atual segue rodando a partir do arquivo .old e o
	// build escreve um arquivo novo no lugar.
	moved := false
	if self, err := os.Executable(); err == nil && samePath(self, target) {
		os.Remove(old) // sobra de uma atualização anterior
		if err := os.Rename(target, old); err != nil {
			return "", fmt.Errorf("não foi possível liberar o executável: %v", err)
		}
		moved = true
	}

	cmd := exec.Command("go", "build", "-o", target, ".")
	cmd.Dir = root
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		if moved {
			os.Rename(old, target)
		}
		return "", fmt.Errorf("build falhou: %s", compactOutput(string(out)))
	}
	return target, nil
}

// samePath compara caminhos ignorando maiúsculas, como o sistema de arquivos
// do Windows faz.
func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// compactOutput achata a saída do compilador numa linha curta que caiba na
// mensagem do rodapé.
func compactOutput(out string) string {
	out = strings.TrimSpace(out)
	out = strings.ReplaceAll(out, "\r\n", " · ")
	out = strings.ReplaceAll(out, "\n", " · ")
	return truncate(out, 160)
}
