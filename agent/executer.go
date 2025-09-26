package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spacegrower/watermelon/infra/wlog"
	"github.com/spacegrower/watermelon/pkg/safe"
	"go.uber.org/zap"

	"github.com/holdno/gopherCron/common"
)

// 处理任务stdio到本地日志和远端日志(strings.Builder)
func handleRealTimeResult(ctx context.Context, output *strings.Builder, logOutput wlog.Logger, stdoutPipe, stderrPipe io.Reader) *sync.WaitGroup {
	wait := &sync.WaitGroup{}
	offCounter := &atomic.Bool{}
	newScanner := func(ctx context.Context, r io.Reader, msgChan chan string, logger func(msg string, fields ...zap.Field)) {
		s := bufio.NewScanner(r)
		go safe.Run(func() {
			defer func() {
				if !offCounter.CompareAndSwap(false, true) {
					close(msgChan)
				}
			}()
			for s.Scan() {
				line := s.Text()
				logger(line)
				select {
				case <-ctx.Done():
					return
				case msgChan <- line:
				}
			}

			if s.Err() != nil && !errors.Is(s.Err(), io.EOF) && !errors.Is(s.Err(), os.ErrClosed) {
				logOutput.Error("real-time scanner finished with error", zap.Error(s.Err()))
			}
			return
		})
	}

	msgChan := make(chan string, 10)
	newScanner(ctx, stdoutPipe, msgChan, logOutput.With(zap.String("source", "stdout")).Info)
	newScanner(ctx, stderrPipe, msgChan, logOutput.With(zap.String("source", "stderr")).Error)
	wait.Add(1)
	go safe.Run(func() {
		var (
			line string
			ok   bool
		)
		defer wait.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case line, ok = <-msgChan:
				if !ok {
					return
				}
				output.WriteString(line)
				output.WriteString("\n")
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

	wait := handleRealTimeResult(ctx, output, logger, stdoutPipe, stderrPipe)

	// 执行命令
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// cmd.Wait 会释放掉io，所以需要先等io相关动作结束后 再调用 cmd.Wait
	wait.Wait() // wait io coping

	if err = cmd.Wait(); err != nil { // cmd.Wait will release io
		ctxErr := ctx.Err()
		var errMsg string
		if cmd.ProcessState.ExitCode() == -1 && ctxErr != nil {
			errMsg = ctxErr.Error()
		} else {
			errMsg = err.Error()
		}

		err = fmt.Errorf("%s, exit code: %d", errMsg, cmd.ProcessState.ExitCode())
	}
	return output, err
}

// ExecuteTask 执行任务
func (a *client) ExecuteTask(info *common.TaskExecutingInfo) *common.TaskExecuteResult {
	result := &common.TaskExecuteResult{
		StartTime:   time.Now(),
		ExecuteInfo: info,
	}

	if result.StartTime.Sub(info.RealTime.Add(time.Second*30)) > 0 {
		result.EndTime = time.Now()
		result.Err = "task starting timeout"
		return result
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
