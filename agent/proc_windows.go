// fork_linux.go
//go:build windows

package agent

import (
	"context"
	"os/exec"
	"syscall"
)

func forkProcess(ctx context.Context, shell, command string) *exec.Cmd {
	cmd := exec.CommandContext(info.CancelCtx, shell, "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	return cmd
}
