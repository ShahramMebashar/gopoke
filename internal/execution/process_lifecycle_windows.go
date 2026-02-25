//go:build windows

package execution

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func configureCommandForLifecycle(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func signalInterrupt(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return nil
	}
	if err := command.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func forceKill(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return nil
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", command.Process.Pid))
	if err := kill.Run(); err != nil {
		if killErr := command.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			return killErr
		}
	}
	return nil
}
