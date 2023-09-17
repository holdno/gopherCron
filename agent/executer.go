package agent

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/holdno/gopherCron/common"
)

// ExecuteTask 执行任务
func (a *client) ExecuteTask(info *common.TaskExecutingInfo) *common.TaskExecuteResult {
	// 启动一个协成来执行shell命令
	var (
		cmd    *exec.Cmd
		result *common.TaskExecuteResult
		err    error
	)

	result = &common.TaskExecuteResult{
		ExecuteInfo: info,
		StartTime:   time.Now(), // 记录任务开始时间
	}

	cmd = forkProcess(info.CancelCtx, a.cfg.Shell, info.Task.Command)
	// cmd = exec.CommandContext(info.CancelCtx, a.cfg.Shell, "-c", info.Task.Command)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	stdoutPipe, _ := cmd.StdoutPipe()

	var (
		output  strings.Builder
		closeCh = make(chan struct{})
	)
	go func() {
		buf := bufio.NewReader(stdoutPipe)
		for {
			select {
			case <-info.CancelCtx.Done():
				if cmd.Process != nil {
					cmd.Process.Kill()
				}
				return
			case <-closeCh:
				return
			default:
				line, err := buf.ReadString('\n')
				if err != nil {
					return
				}
				if len(line) > 0 {
					output.WriteString(line)
					output.WriteString("\n")
				}
			}
		}
	}()
	// 多命令语句会导致 cmd.CombineOutput()阻塞，无法正常timeout，例如：sleep 20 && echo 123，timeout 设为5则无效
	// https://github.com/golang/go/issues/23019
	// output, err = cmd.CombinedOutput()

	//if stdoutPipe, result.Err = cmd.StdoutPipe(); result.Err != nil {
	//	goto FinishWithError
	//}

	// 执行命令
	if err := cmd.Start(); err != nil {
		result.Err = err.Error()
		goto FinishWithError
	}

	if err = cmd.Wait(); err != nil {
		ctxErr := info.CancelCtx.Err()
		if ctxErr == context.DeadlineExceeded {
			result.Err = "timeout"
		} else if ctxErr == context.Canceled {
			result.Err = "canceled"
		} else {
			switch cmd.ProcessState.ExitCode() {
			case 1:
				result.Err = err.Error()
			case 2:
				result.Err = "terminal interrupt"
			case 9:
				result.Err = "process terminated"
			case 126:
				result.Err = "unexecutable command"
			case 127:
				result.Err = "command not found"
			case 128:
				result.Err = "invalid exit parameter"
			case 130:
				result.Err = "sig exit"
			case 255:
				result.Err = "error exit code"
			default:
				result.Err = fmt.Sprintf("exit code: %d", cmd.ProcessState.ExitCode())
			}

			if err.Error() != "" {
				result.Err += ", " + err.Error()
			}
		}
		goto FinishWithError
	}

FinishWithError:
	close(closeCh)
	result.EndTime = time.Now()
	result.Output = output.String()
	return result
}
