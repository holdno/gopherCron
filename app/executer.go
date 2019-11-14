package app

import (
	"bytes"
	"math/rand"
	"os/exec"
	"time"

	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/config"
	"ojbk.io/gopherCron/pkg/etcd"
)

// ExecuteTask 执行任务
func (a *app) ExecuteTask(info *common.TaskExecutingInfo) {
	// 启动一个协成来执行shell命令
	go func() {
		var (
			cmd      *exec.Cmd
			output   bytes.Buffer
			err      error
			result   *common.TaskExecuteResult
			taskLock *etcd.TaskLock
		)

		// 获取分布式锁
		taskLock = a.etcd.Lock(info.Task)

		result = &common.TaskExecuteResult{
			ExecuteInfo: info,
			Output:      make([]byte, 0),
			StartTime:   time.Now(), // 记录任务开始时间
		}

		// 避免分布式集群上锁偏斜 (每台机器的时钟可能不是特别的准确 导致某一台机器总能抢到锁)
		time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
		if err = taskLock.TryLock(); err != nil {
			// 上锁失败 证明任务已被其他客户端调度
			//result.Err = err
			//result.EndTime = time.Now()
			return
		}

		result.StartTime = time.Now()

		cmd = exec.CommandContext(info.CancelCtx, config.GetServiceConfig().Etcd.Shell, "-c", info.Task.Command)
		cmd.Stdout = &output

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
			goto FinishWithError
		}

	FinishWithError:
		result.EndTime = time.Now()
		result.Output = output.Bytes()

		taskLock.Unlock()

		// 执行结束后 返回给scheduler
		a.scheduler.PushTaskResult(result)
	}()
}
