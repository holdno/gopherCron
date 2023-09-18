// fork_linux.go
//go:build !windows

package agent

import (
	"context"
	"os/exec"
	"syscall"
)

func forkProcess(ctx context.Context, shell, command string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, shell, "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	return cmd
}
