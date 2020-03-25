package app

import (
	"bufio"
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
)

// ExecuteTask 执行任务
func (a *client) ExecuteTask(info *common.TaskExecutingInfo) *common.TaskExecuteResult {
	// 启动一个协成来执行shell命令
	var (
		cmd    *exec.Cmd
		result *common.TaskExecuteResult
	)
	defer info.CancelFunc()

	result = &common.TaskExecuteResult{
		ExecuteInfo: info,
		StartTime:   time.Now(), // 记录任务开始时间
	}

	result.StartTime = time.Now()

	cmd = exec.CommandContext(info.CancelCtx, config.GetServiceConfig().Etcd.Shell, "-c", info.Task.Command)
	stdoutPipe, _ := cmd.StdoutPipe()

	var (
		output  strings.Builder
		closeCh = make(chan struct{})
	)
	go func() {
		buf := bufio.NewReader(stdoutPipe)
		for {
			select {
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
	if result.Err = cmd.Start(); result.Err != nil {
		goto FinishWithError
	}

	if result.Err = cmd.Wait(); result.Err != nil {
		if strings.Contains(cmd.ProcessState.String(), syscall.SIGKILL.String()) {
			result.Err = errors.New("timeout")
		} else {
			switch cmd.ProcessState.ExitCode() {
			case 1:
				result.Err = errors.New("unknow error, exit code 1")
			case 2:
				result.Err = errors.New("error shell command")
			case 126:
				result.Err = errors.New("unexecutable command")
			case 127:
				result.Err = errors.New("command not found")
			case 128:
				result.Err = errors.New("invalid exit parameter")
			case 130:
				result.Err = errors.New("sig exit")
			case 255:
				result.Err = errors.New("error exit code")
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
