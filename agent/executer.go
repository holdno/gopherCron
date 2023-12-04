package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spacegrower/watermelon/infra/wlog"
	"github.com/spacegrower/watermelon/pkg/safe"
	"go.uber.org/zap"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/utils"
)

// 处理任务stdio到本地日志和远端日志(strings.Builder)
func handleRealTimeResult(ctx context.Context, output *strings.Builder, logOutput wlog.Logger, stdoutPipe, stderrPipe io.Reader) *utils.WaitGroupWithTimeout {
	ctx, cancel := context.WithCancel(ctx)
	wait := utils.NewTimeoutWaitGroup(ctx, cancel)
	newScanner := func(ctx context.Context, r io.Reader) chan string {
		wait.Add(1)
		s := bufio.NewScanner(r)
		msgChan := make(chan string, 10)
		go safe.Run(func() {
			defer close(msgChan)
			defer wait.Done()
			for s.Scan() {
				select {
				case <-ctx.Done():
					return
				case msgChan <- s.Text():
				}
			}

			if s.Err() != nil && !errors.Is(s.Err(), io.EOF) && !errors.Is(s.Err(), os.ErrClosed) {
				logOutput.Error("real-time scanner finished with error", zap.Error(s.Err()))
			}
		})
		return msgChan
	}

	stdout := newScanner(ctx, stdoutPipe)
	stderr := newScanner(ctx, stderrPipe)
	go safe.Run(func() {
		var (
			line      string
			ok        bool
			logStdout = logOutput.With(zap.String("source", "stdout"))
			logStderr = logOutput.With(zap.String("source", "stderr"))
			writeLog  func(msg string, fields ...zap.Field)
		)
		for {
			select {
			case <-ctx.Done():
				return
			case line, ok = <-stdout:
				writeLog = logStdout.Info
			case line, ok = <-stderr:
				writeLog = logStderr.Error
			}
			if ok {
				output.WriteString(line)
				output.WriteString("\n")
				writeLog(line)
			}
		}
	})
	return wait
}

func execute(ctx context.Context, shell, command string, logger wlog.Logger) (*strings.Builder, error) {
	var (
		cmd           = forkProcess(ctx, shell, command)
		stdoutPipe, _ = cmd.StdoutPipe()
		stderrPipe, _ = cmd.StderrPipe()
		output        = &strings.Builder{}
		err           error
	)
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

	wait := handleRealTimeResult(ctx, output, logger, stdoutPipe, stderrPipe)

	if err = cmd.Wait(); err != nil {
		ctxErr := ctx.Err()
		var errMsg string
		if cmd.ProcessState.ExitCode() == -1 {
			if ctxErr == context.DeadlineExceeded {
				errMsg = "context timeout"
			} else if ctxErr == context.Canceled {
				errMsg = "context canceled"
			}
		}

		if errMsg == "" {
			errMsg = err.Error()
		}

		err = fmt.Errorf("%s, exit code: %d", errMsg, cmd.ProcessState.ExitCode())
	}

	wait.Wait(time.Second * 5)
	return output, err
}

// ExecuteTask 执行任务
func (a *client) ExecuteTask(info *common.TaskExecutingInfo) *common.TaskExecuteResult {
	result := &common.TaskExecuteResult{
		StartTime:   time.Now(),
		ExecuteInfo: info,
	}
	if info.CancelCtx.Err() != nil {
		result.Err = info.CancelCtx.Err().Error()
		result.EndTime = time.Now()
		return result
	}

	// 启动一个协成来执行shell命令
	std, err := execute(info.CancelCtx, a.cfg.Shell, info.Task.Command,
		a.logger.With(zap.String("task_id", info.Task.TaskID),
			zap.Int64("project_id", info.Task.ProjectID)))
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
