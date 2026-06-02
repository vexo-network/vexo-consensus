//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

func configureLocalnetChildProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
