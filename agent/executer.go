package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/spacegrower/watermelon/infra/wlog"
	"github.com/spacegrower/watermelon/pkg/safe"
	"go.uber.org/zap"

	"github.com/holdno/gopherCron/common"
)

func execute(ctx context.Context, shell, command string, logger wlog.Logger) (*strings.Builder, error) {
	cmd := forkProcess(ctx, shell, command)
	// cmd = exec.CommandContext(info.CancelCtx, a.cfg.Shell, "-c", info.Task.Command)
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	wait := sync.WaitGroup{}
	stdScanner := func(ctx context.Context, r io.Reader, lines chan string) {
		wait.Add(1)
		s := bufio.NewScanner(r)
		go safe.Run(func() {
			defer wait.Done()
			for s.Scan() {
				lines <- s.Text()
				if ctx.Err() != nil {
					return
				}
			}
		})
	}

	var (
		output  = &strings.Builder{}
		closeCh = make(chan struct{})
	)
	defer close(closeCh)
	go func() {
		var (
			stdout = make(chan string)
			stderr = make(chan string)
		)
		defer func() {
			close(stdout)
			close(stderr)
		}()
		stdScanner(ctx, stdoutPipe, stdout)
		stdScanner(ctx, stderrPipe, stderr)
		for {
			select {
			case <-ctx.Done():
				if cmd.Process != nil {
					cmd.Process.Kill()
				}
				return
			case <-closeCh:
				return
			case line := <-stdout:
				output.WriteString(line)
				output.WriteString("\n")
				logger.Info("task stdout", zap.String("line", line))
			case line := <-stderr:
				output.WriteString(line)
				output.WriteString("\n")
				logger.Error("task stderr", zap.String("line", line))
			default:
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
		return nil, err
	}

	wait.Wait()
	if err := cmd.Wait(); err != nil {
		ctxErr := ctx.Err()
		var errMsg string
		if ctxErr == context.DeadlineExceeded {
			errMsg = "timeout"
		} else if ctxErr == context.Canceled {
			errMsg = "canceled"
		} else {
			switch cmd.ProcessState.ExitCode() {
			case 1:
				return output, err
			case 2:
				errMsg = "terminal interrupt"
			case 9:
				errMsg = "process terminated"
			case 126:
				errMsg = "unexecutable command"
			case 127:
				errMsg = "command not found"
			case 128:
				errMsg = "invalid exit parameter"
			case 130:
				errMsg = "sig exit"
			case 255:
				errMsg = "error exit code"
			default:
				errMsg = fmt.Sprintf("exit code: %d", cmd.ProcessState.ExitCode())
			}

			if err.Error() != "" {
				errMsg += ", " + err.Error()
			}
		}

		return output, errors.New(errMsg)
	}

	return output, nil
}

// ExecuteTask 执行任务
func (a *client) ExecuteTask(info *common.TaskExecutingInfo) *common.TaskExecuteResult {
	result := &common.TaskExecuteResult{}
	// 启动一个协成来执行shell命令
	std, err := execute(info.CancelCtx, a.cfg.Shell, info.Task.Command, a.logger.With(zap.String("task_id", info.Task.TaskID), zap.Int64("project_id", info.Task.ProjectID)))
	if err != nil {
		result.Err = err.Error()
	}
	result.EndTime = time.Now()
	if std != nil {
		runeStr := []rune(strings.TrimSuffix(std.String(), "\n"))
		if len(runeStr) > 5000 {
			runeStr = runeStr[:5000]
		}
		result.Output = string(runeStr)
	}

	return result
}
