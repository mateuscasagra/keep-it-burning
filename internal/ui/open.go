package ui

import (
	"os/exec"
	"runtime"
)

// openExternal abre um link no navegador ou um arquivo (imagem, vídeo,
// documento) no aplicativo padrão do sistema.
func openExternal(ref string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// FileProtocolHandler resolve tanto URLs quanto caminhos de arquivo,
		// sem os problemas de aspas do "cmd /c start".
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", ref)
	case "darwin":
		cmd = exec.Command("open", ref)
	default:
		cmd = exec.Command("xdg-open", ref)
	}
	hideWindow(cmd)
	return cmd.Start()
}
