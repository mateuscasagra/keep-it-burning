//go:build windows

package ui

import (
	"os/exec"
	"syscall"
)

// hideWindow impede que o console do "go build" pisque na frente do app.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
