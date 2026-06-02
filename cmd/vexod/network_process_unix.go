//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

func configureNetworkChildProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
