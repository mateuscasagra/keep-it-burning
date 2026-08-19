//go:build !windows

package ui

import "os/exec"

// hideWindow não tem o que esconder fora do Windows.
func hideWindow(*exec.Cmd) {}
