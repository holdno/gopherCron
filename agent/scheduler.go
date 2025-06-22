package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/utils"

	"github.com/avast/retry-go/v4"
	"github.com/sirupsen/logrus"
	"github.com/spacegrower/watermelon/infra/wlog"
	"github.com/spacegrower/watermelon/pkg/safe"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Scheduler 任务调度
type TaskScheduler struct {
	a                     *client
	TaskEventChan         chan *common.TaskEvent // 任务事件队列
	PlanTable             sync.Map
	consistency           *consistency
	TaskExecuteResultChan chan *common.TaskExecuteResult
	// PlanTable             map[string]*common.TaskSchedulePlan  // 任务调度计划表
	TaskExecutingTable sync.Map // 任务执行中的记录表
}

type consistency struct {
	planhash          map[int64]string
	latestRefreshTime time.Time
	debounce          func()
	once              sync.Once
	locker            sync.RWMutex
}

func (ts *TaskScheduler) PlanHashDebouncer() func() {
	ts.consistency.once.Do(func() {
		ts.consistency.debounce = utils.NewDebounce(time.Second, ts.CalcPlanHash)
		ts.consistency.planhash = make(map[int64]string)
	})
	return ts.consistency.debounce
}

func initScheduler(agent *client) *TaskScheduler {
	scheduler := &TaskScheduler{
		a:                     agent,
		TaskEventChan:         make(chan *common.TaskEvent, 3000),
		TaskExecuteResultChan: make(chan *common.TaskExecuteResult, 3000),
		consistency:           &consistency{},
	}
	return scheduler
}

func (ts *TaskScheduler) Stop() {
	// 调度器关闭，最多等待10分钟来结束当前运行中的任务
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()
	wlog.Info("starting to shut down the scheduler")
	for {
		count := ts.TaskExecutingCount()
		if count == 0 || ctx.Err() != nil {
			ts.PushTaskResult(nil) // 全部任务执行完成后发送空结果，通知调度器退出
			wlog.Info("the scheduler has been shut down", zap.Bool("timeout", ctx.Err() != nil))
			return
		}
		wlog.Info(fmt.Sprintf("scheduler shutting down, waiting for %d tasks to finish", count))
		time.Sleep(time.Second)
	}
}

func (ts *TaskScheduler) TaskExecutingCount() int {
	count := 0
	ts.TaskExecutingTable.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

func (ts *TaskScheduler) SetExecutingTask(key string, task *common.TaskExecutingInfo) {
	ts.TaskExecutingTable.Store(key, task)
}

func (ts *TaskScheduler) CheckTaskExecuting(key string) (*common.TaskExecutingInfo, bool) {

	res, exist := ts.TaskExecutingTable.Load(key)
	if !exist {
		return nil, false
	}

	return res.(*common.TaskExecutingInfo), true
}

func (ts *TaskScheduler) DeleteExecutingTask(key string) {
	ts.TaskExecutingTable.Delete(key)
}

func (ts *TaskScheduler) PushTaskResult(result *common.TaskExecuteResult) {
	ts.TaskExecuteResultChan <- result
}

func (a *client) GetPlan(key string) (*common.TaskSchedulePlan, bool) {
	var (
		value interface{}
		ok    bool
	)
	if value, ok = a.scheduler.PlanTable.Load(key); ok {
		return value.(*common.TaskSchedulePlan), true
	}

	return nil, false
}

func (ts *TaskScheduler) CalcPlanHash() {
	var (
		projectPlanList = make(map[int64][]string)
		commandMap      = make(map[string]string)
		taskCounter     = 0
	)

	ts.PlanRange(func(key string, value *common.TaskSchedulePlan) bool {
		projectPlanList[value.Task.ProjectID] = append(projectPlanList[value.Task.ProjectID], key)
		commandMap[key] = value.Task.Command
		taskCounter++
		return true
	})

	ts.a.metrics.task.With(nil).Set(float64(taskCounter))

	ts.consistency.locker.Lock()
	defer ts.consistency.locker.Unlock()

	for k := range ts.consistency.planhash {
		if len(projectPlanList[k]) == 0 {
			delete(ts.consistency.planhash, k)
		}
	}

	for projectid, list := range projectPlanList {
		sort.Slice(list, func(i, j int) bool {
			return list[i] < list[j]
		})
		var b strings.Builder
		for _, v := range list {
			b.WriteString(v)
			b.WriteString(";")
			b.WriteString(commandMap[v])
			b.WriteString(";")
		}
		ts.consistency.planhash[projectid] = utils.MakeMD5(b.String())
	}
	ts.consistency.latestRefreshTime = time.Now()
}

func (ts *TaskScheduler) GetProjectTaskHash(projectID int64) (string, int64) {
	ts.consistency.locker.RLock()
	defer ts.consistency.locker.RUnlock()
	return ts.consistency.planhash[projectID], ts.consistency.latestRefreshTime.Unix()
}

func (ts *TaskScheduler) SetPlan(key string, value *common.TaskSchedulePlan) {
	ts.PlanTable.Store(key, value)
	ts.PlanHashDebouncer()()
}

func (ts *TaskScheduler) PlanRange(f func(key string, value *common.TaskSchedulePlan) bool) {
	ts.PlanTable.Range(func(key, value interface{}) bool {
		f(key.(string), value.(*common.TaskSchedulePlan))
		return true
	})
}

func (ts *TaskScheduler) PlanCount() int {
	var count int
	ts.PlanTable.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

func (ts *TaskScheduler) RemovePlan(schedulerKey string) {
	ts.PlanTable.Delete(schedulerKey)
	ts.PlanHashDebouncer()()
}

func (ts *TaskScheduler) RemoveAll() {
	ts.PlanTable.Range(func(key, value interface{}) bool {
		ts.PlanTable.Delete(key)
		return true
	})
}

func (a *client) Loop() {
	if a.isClose {
		time.Sleep(time.Second)
		return
	}
	var (
		taskEvent     *common.TaskEvent
		scheduleAfter time.Duration
		scheduleTimer *time.Timer
	)

	scheduleAfter = a.TrySchedule()
	// 调度定时器
	scheduleTimer = time.NewTimer(scheduleAfter)
	defer scheduleTimer.Stop()
	defer wlog.Warn("scheduler is shutdown now")
	for {
		select {
		case taskEvent = <-a.scheduler.TaskEventChan:
			// 对内存中的任务进行增删改查
			a.handleTaskEvent(taskEvent)
		case executeResult := <-a.scheduler.TaskExecuteResultChan:
			if executeResult == nil {
				return
			}
			a.handleTaskResult(executeResult)
		case <-scheduleTimer.C: // 最近的一个调度任务到期执行
		}

		if a.isClose {
			return
		}

		// 每次触发事件后 重新计算下次调度任务时间
		scheduleAfter = a.TrySchedule()
		scheduleTimer.Reset(scheduleAfter)
	}
}

// handleTaskEvent 处理事件
func (a *client) handleTaskEvent(event *common.TaskEvent) {
	var (
		taskSchedulePlan *common.TaskSchedulePlan
		taskExecuteInfo  *common.TaskExecutingInfo
		taskExecuting    bool
		err              error
	)

	switch event.EventType {
	// 临时调度
	case common.TASK_EVENT_TEMPORARY:
		// 构建执行计划
		if taskSchedulePlan, err = common.BuildTaskSchedulerPlan(event.Task, common.ActivePlan); err != nil {
			wlog.Error("build task schedule plan error in temporary event", zap.Error(err))
			logrus.WithField("Error", err.Error()).Error("build task schedule plan error")
			return
		}
		taskSchedulePlan.PlanTime = time.Now()
		a.TryStartTask(*taskSchedulePlan)
	case common.TASK_EVENT_WORKFLOW_SCHEDULE:
		// 构建执行计划
		if taskSchedulePlan, err = common.BuildWorkflowTaskSchedulerPlan(event.Task.TaskInfo); err != nil {
			wlog.Error("build task schedule plan error in workflow schedule event", zap.Error(err))
			return
		}
		taskSchedulePlan.PlanTime = time.Now()
		a.TryStartTask(*taskSchedulePlan)
	case common.TASK_EVENT_SAVE:
		// 构建执行计划
		if event.Task.Status == common.TASK_STATUS_START {
			if taskSchedulePlan, err = common.BuildTaskSchedulerPlan(event.Task, common.NormalPlan); err != nil {
				wlog.Error("build task schedule plan error in save event", zap.Error(err))
				return
			}

			a.scheduler.SetPlan(event.Task.SchedulerKey(), taskSchedulePlan)
			return
		}
		// 如果任务保存状态不为1 证明不需要执行 所以顺延执行delete事件，从计划表中删除任务
		fallthrough
	case common.TASK_EVENT_DELETE:
		a.scheduler.RemovePlan(event.Task.SchedulerKey())
	case common.TASK_EVENT_KILL:
		// 先判断任务是否在执行中
		if taskExecuteInfo, taskExecuting = a.scheduler.CheckTaskExecuting(event.Task.SchedulerKey()); taskExecuting {
			taskExecuteInfo.CancelFunc()
		}
	}
}

// 重新计算任务调度状态
func (a *client) TrySchedule() time.Duration {
	var (
		now      time.Time
		nearTime time.Time
	)

	// 如果当前任务调度表中没有任务的话 可以随机睡眠后再尝试
	if a.scheduler.PlanCount() == 0 {
		return time.Second
	}
	now = time.Now()
	// 遍历所有任务
	a.scheduler.PlanRange(func(schedulerKey string, plan *common.TaskSchedulePlan) bool {
		// 如果调度时间是在现在或之前再或者为临时调度任务
		if plan.PlanTime.Before(now) || plan.PlanTime.Equal(now) {
			// 尝试执行任务
			// 因为可能上一次任务还没执行结束
			if a.cfg.Micro.Weight > 0 && plan.Task.Status == common.TASK_STATUS_START { // 权重大于0才会调度任务
				go func(plan common.TaskSchedulePlan) {
					a.TryStartTask(plan)
				}(*plan)
			} else {
				wlog.Debug("skip execute task, this client weight is zero",
					zap.String("task_id", plan.Task.TaskID),
					zap.Int64("project_id", plan.Task.ProjectID))
			}
			plan.PlanTime = plan.Expr.Next(now) // 更新下一次执行时间
		}

		// 获取下一个要执行任务的时间
		if nearTime.IsZero() || plan.PlanTime.Before(nearTime) {
			nearTime = plan.PlanTime
		}

		return true
	})

	// 下次调度时间 (最近要执行的任务调度时间 - 当前时间)
	return nearTime.Sub(now)
}

// TryStartTask 开始执行任务
func (a *client) TryStartTask(plan common.TaskSchedulePlan) error {
	if plan.TmpID == "" {
		plan.TmpID = utils.GetStrID()
	}
	// 执行的任务可能会执行很久
	// 需要防止并发
	var (
		taskExecuteInfo *common.TaskExecutingInfo
		taskExecuting   bool
	)

	if a.isClose {
		return fmt.Errorf("agent %s is closing", a.GetIP())
	}

	if taskExecuteInfo, taskExecuting = a.scheduler.CheckTaskExecuting(plan.Task.SchedulerKey()); taskExecuting {
		errMsg := "任务执行中，重复调度，上一周期任务仍未结束，请确保任务超时时间配置合理或检查任务是否运行正常"
		if plan.Type == common.ActivePlan {
			errMsg = "任务执行中，请勿重复执行或稍后再试"
		}
		a.Warning(warning.NewTaskWarningData(warning.TaskWarning{
			AgentIP:   a.localip,
			TaskName:  plan.Task.Name,
			TaskID:    plan.Task.TaskID,
			ProjectID: plan.Task.ProjectID,
			Message:   errMsg,
		}))
		return errors.New(errMsg)
	}

	plan.Task.ClientIP = a.GetIP()
	taskExecuteInfo = common.BuildTaskExecuteInfo(plan)
	errSignal := utils.NewSignalChannel[error]()
	if plan.Type != common.ActivePlan {
		// 如果不是主动调用，则不需要等待几处可能前置的错误来响应web客户端
		errSignal.Close()
	} else {
		wlog.Debug("get active plan",
			zap.String("task_id", taskExecuteInfo.Task.TaskID),
			zap.Int64("project_id", taskExecuteInfo.Task.ProjectID),
			zap.String("task_name", taskExecuteInfo.Task.Name))
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				var buf [4096]byte
				n := runtime.Stack(buf[:], false)
				a.Warning(warning.NewTaskWarningData(warning.TaskWarning{
					AgentIP:   a.localip,
					TaskName:  plan.Task.Name,
					TaskID:    plan.Task.TaskID,
					ProjectID: plan.Task.ProjectID,
					Message:   fmt.Sprintf("任务执行失败，panic: %v\n%s", r, string(buf[:n])),
				}))

				a.logger.With(zap.Any("fields", map[string]interface{}{
					"error":      r,
					"stack":      string(buf[:n]),
					"task_name":  plan.Task.Name,
					"project_id": plan.Task.ProjectID,
				})).Error("任务执行失败, Panic")
			}
		}()

		var (
			cancelReason = utils.NewReasonWriter() // 异常终止的信息会写入这里
			result       *common.TaskExecuteResult // 任务执行结果
		)

		// 构建执行状态信息
		defer func() {
			taskExecuteInfo.CancelFunc()
		}()

		if plan.Task.Noseize != common.TASK_EXECUTE_NOSEIZE {
			// 远程加锁，加锁失败后如果任务还在运行中，则进行重试，如果重试失败，则强行结束任务
			err := tryLockTaskForExec(a, taskExecuteInfo, cancelReason)
			if err != nil {
				grpcErr, _ := status.FromError(err)
				if grpcErr.Code() != codes.Aborted {
					a.logger.Error("failed to get task execute lock",
						zap.String("task_id", taskExecuteInfo.Task.TaskID),
						zap.Int64("project_id", taskExecuteInfo.Task.ProjectID),
						zap.String("task_name", taskExecuteInfo.Task.Name),
						zap.Error(err))
					// send warning
					a.Warning(warning.NewTaskWarningData(warning.TaskWarning{
						AgentIP:   a.GetIP(),
						TaskName:  taskExecuteInfo.Task.Name,
						TaskID:    taskExecuteInfo.Task.TaskID,
						ProjectID: taskExecuteInfo.Task.ProjectID,
						Message:   fmt.Sprintf("任务执行加锁失败: %s", err.Error()),
					}))
				}

				errSignal.Send(err)
				return
			}
		}

		a.scheduler.SetExecutingTask(plan.Task.SchedulerKey(), taskExecuteInfo)
		taskRuntimeMetrics := a.metrics.TaskRuntimeRecord(taskExecuteInfo.Task.ProjectID, taskExecuteInfo.Task.TaskID, taskExecuteInfo.Task.Name)
		defer func() {
			// 删除任务的正在执行状态
			reportTaskResult(a, taskExecuteInfo, plan, result)
			a.scheduler.DeleteExecutingTask(plan.Task.SchedulerKey())
			taskRuntimeMetrics.ObserveDuration()
		}()

		// 开始执行任务
		var taskStatusReportResult *cronpb.Result
		if err := retry.Do(func() error {
			value, _ := json.Marshal(taskExecuteInfo)
			ctx, cancel := context.WithTimeout(taskExecuteInfo.CancelCtx, time.Duration(a.cfg.Timeout)*time.Second)
			defer cancel()
			var err error
			taskStatusReportResult, err = a.GetStatusReporter()(ctx, &cronpb.ScheduleReply{
				ProjectId: plan.Task.ProjectID,
				Event: &cronpb.Event{
					Type:      common.TASK_STATUS_RUNNING_V2,
					Version:   common.VERSION_TYPE_V2,
					Value:     value,
					EventTime: time.Now().Unix(),
				},
			})
			return err
		}, retry.RetryIf(func(err error) bool {
			if gerr, _ := status.FromError(err); gerr.Code() == codes.Aborted || gerr.Code() == codes.Unauthenticated {
				return false
			}
			return true
		}), retry.Attempts(3), retry.DelayType(retry.BackOffDelay),
			retry.MaxJitter(time.Second*30), retry.LastErrorOnly(true)); err != nil {
			if gerr, _ := status.FromError(err); gerr.Code() == codes.Aborted {
				a.logger.Debug("task aborted", zap.String("task_id", plan.Task.TaskID), zap.Int64("project_id", plan.Task.ProjectID),
					zap.String("tmp_id", plan.TmpID), zap.Error(err))
				return
			}
			taskExecuteInfo.CancelFunc()
			a.metrics.SystemErrInc("agent_status_report_failure")
			a.logger.Error(fmt.Sprintf("task: %s, id: %s, tmp_id: %s, change running status error, %v", plan.Task.Name,
				plan.Task.TaskID, plan.TmpID, err))
			errDetail := fmt.Errorf("agent上报任务开始状态失败: %s，任务终止", err.Error())
			cancelReason.WriteString(errDetail.Error())
			errSignal.Send(errDetail)
		} else if !taskStatusReportResult.Result {
			taskExecuteInfo.CancelFunc()
			a.logger.Error(fmt.Sprintf("task: %s, id: %s, tmp_id: %s, change running status failed, %v", plan.Task.Name,
				plan.Task.TaskID, plan.TmpID, taskStatusReportResult.Message))
			cancelReason.WriteString(taskStatusReportResult.Message)
			errSignal.Send(fmt.Errorf("agent上报任务开始状态失败: %s，任务终止", taskStatusReportResult.Message))
		}

		errSignal.Close()
		// 执行任务
		result = a.ExecuteTask(taskExecuteInfo)
		if result.Err != "" {
			cancelReason.WriteStringPrefix("任务执行结果: " + result.Err)
			result.Err = cancelReason.String()
		}
		// 执行结束后 返回给scheduler，执行结果中携带错误的话会触发告警
		a.scheduler.PushTaskResult(result)
	}()

	return errSignal.WaitOne()
}

// getSchedulerLatency 避免分布式集群上锁偏斜 (每台机器的时钟可能不是特别的准确 导致某一台机器总能抢到锁)
// v2.4.5版本开始结合节点权重做一些策略，权重更大的节点，等待时间可能更小，从而做到权重大的节点调度机会更高
// v2.4.5节点的权重配置，取值范围在0-100之间，超出则取边界值
func getSchedulerLatency(w int32, runningCount int32) time.Duration {
	if w > 100 {
		w = 100
	} else if w < 0 {
		w = 0
	}

	set := runningCount * 20
	if set > 200 {
		set = 200
	}

	// Base delay set to 500 ms
	var baseDelay int32 = 500
	// Calculate the maximum reduction based on weight
	var maxReduction int32 = w * 5 // weight 0-100, so max 500 ms reduction
	// Apply a random reduction from 0 to maxReduction
	randomReduction := rand.Int32N(maxReduction)

	// Final delay is base delay minus a random factor based on weight
	delay := baseDelay - randomReduction

	return time.Duration(delay+set) * time.Millisecond
}

func tryLockTaskForExec(a *client, taskExecuteInfo *common.TaskExecutingInfo, taskResultWriter interface{ WriteString(s string) }) error {
	// 远程加锁，加锁失败后如果任务还在运行中，则进行重试，如果重试失败，则强行结束任务
	var (
		once       sync.Once
		resultChan = make(chan error)
	)

	go safe.Run(func() {
		tryTimes := 0  // 初始化重试次数
		defer func() { // 锁维持结束或失败意味着任务运行结束或者也不能再继续运行了
			taskExecuteInfo.CancelFunc()
		}()
		// 避免分布式集群上锁偏斜 (每台机器的时钟可能不是特别的准确 导致某一台机器总能抢到锁)
		time.Sleep(getSchedulerLatency(a.cfg.Micro.Weight, int32(a.scheduler.TaskExecutingCount())))
		for {
			if taskExecuteInfo.CancelCtx.Err() != nil {
				return
			}

			realtimeError, err := tryLockUntilCtxIsDone(a.GetCenterSrv(), taskExecuteInfo)

			if err != nil {
				a.logger.Warn("failed to get task execute lock",
					zap.String("task_id", taskExecuteInfo.Task.TaskID),
					zap.Int64("project_id", taskExecuteInfo.Task.ProjectID),
					zap.String("task_name", taskExecuteInfo.Task.Name),
					zap.Error(err))
				if gerr, _ := status.FromError(err); gerr.Code() == codes.Aborted || gerr.Code() == codes.Unauthenticated ||
					tryTimes == 3 {
					first := false
					once.Do(func() {
						first = true
						resultChan <- err
						close(resultChan)
					})
					if !first {
						a.metrics.SystemErrInc("agent_task_execute_lock_failure")
						taskResultWriter.WriteString(fmt.Sprintf("任务执行过程中锁维持失败，错误信息：%s，agent主动终止任务", err.Error()))
					}
					return
				}
				time.Sleep(time.Second * time.Duration(tryTimes))
				tryTimes++
				continue
			}

			once.Do(func() {
				close(resultChan)
			})

			tryTimes = 0

			select {
			case <-taskExecuteInfo.CancelCtx.Done():
				return
			case err := <-realtimeError:
				if err != nil {
					a.logger.Error("failed to keep task lock", zap.Error(err),
						zap.String("task_id", taskExecuteInfo.Task.TaskID),
						zap.Int64("project_id", taskExecuteInfo.Task.ProjectID),
						zap.String("task_name", taskExecuteInfo.Task.Name))

					if gerr, ok := status.FromError(err); ok && gerr.Code() == codes.Canceled {
						// 到中心的连接被关闭了，说明agent注册出现了问题，需要等待注册逻辑中重新实现建联
						time.Sleep(time.Millisecond * 100)
					}
				}
			}
		}
	})

	return <-resultChan
}

// 将任务结果上报到中心服务
func reportTaskResult(a *client, taskExecuteInfo *common.TaskExecutingInfo, plan common.TaskSchedulePlan, result *common.TaskExecuteResult) {
	f := common.TaskFinishedV2{
		TaskName:  taskExecuteInfo.Task.Name,
		TaskID:    taskExecuteInfo.Task.TaskID,
		Command:   taskExecuteInfo.Task.Command,
		ProjectID: taskExecuteInfo.Task.ProjectID,
		Status:    common.TASK_STATUS_DONE_V2,
		TmpID:     taskExecuteInfo.TmpID,
		PlanTime:  taskExecuteInfo.PlanTime.Unix(),
	}
	if plan.UserId != 0 {
		f.Operator = fmt.Sprintf("%s(%d)", plan.UserName, plan.UserId)
	}
	if taskExecuteInfo.Task.FlowInfo != nil {
		f.WorkflowID = taskExecuteInfo.Task.FlowInfo.WorkflowID
	}
	if result != nil {
		f.Result = result.Output
		f.StartTime = result.StartTime.Unix()
		f.EndTime = result.EndTime.Unix()
		if result.Err != "" {
			f.Status = common.TASK_STATUS_FAIL_V2
			f.Error = result.Err
		}
	}
	value, _ := json.Marshal(f)
	if err := retry.Do(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.cfg.Timeout)*time.Second)
		defer cancel()
		_, err := a.GetStatusReporter()(ctx, &cronpb.ScheduleReply{
			ProjectId: plan.Task.ProjectID,
			Event: &cronpb.Event{
				Type:      common.TASK_STATUS_FINISHED_V2,
				Version:   common.VERSION_TYPE_V2,
				Value:     value,
				EventTime: time.Now().Unix(),
			},
		})
		return err
	}, retry.Attempts(3), retry.DelayType(retry.BackOffDelay),
		retry.MaxJitter(time.Second*30), retry.LastErrorOnly(true)); err != nil {
		a.metrics.SystemErrInc("agent_status_report_failure")
		a.Warning(warning.NewTaskWarningData(warning.TaskWarning{
			AgentIP:   a.localip,
			TaskName:  plan.Task.Name,
			TaskID:    plan.Task.TaskID,
			ProjectID: plan.Task.ProjectID,
			Message:   "agent上报任务运行结束状态失败: " + err.Error(),
		}))
		a.logger.Error(fmt.Sprintf("task: %s, id: %s, tmp_id: %s, failed to change running status, the task is finished, error: %v",
			plan.Task.Name, plan.Task.TaskID, plan.TmpID, err))
	}
}

// 处理任务结果
func (a *client) handleTaskResult(result *common.TaskExecuteResult) {
	err := retry.Do(func() error {
		if result.Err != "" {
			return a.Warning(warning.NewTaskWarningData(warning.TaskWarning{
				AgentIP:   a.GetIP(),
				TaskName:  result.ExecuteInfo.Task.Name,
				TaskID:    result.ExecuteInfo.Task.TaskID,
				ProjectID: result.ExecuteInfo.Task.ProjectID,
				Message:   result.Err,
			}))
		}
		return nil
	}, retry.LastErrorOnly(true), retry.Attempts(3))

	if err != nil {
		a.logger.Error("task warning report error", zap.Error(err))
	}
}

// 接收任务事件
func (ts *TaskScheduler) PushEvent(event *common.TaskEvent) {
	ts.TaskEventChan <- event
}

func tryLockUntilCtxIsDone(cli cronpb.CenterClient, execInfo *common.TaskExecutingInfo) (chan error, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(execInfo.Task.Timeout)*time.Second)
	locker, err := cli.TryLock(ctx)
	defer func() {
		if err != nil {
			cancel()
		}
	}()
	if err != nil {
		return nil, err
	}

	if err = locker.Send(&cronpb.TryLockRequest{
		ProjectId: execInfo.Task.ProjectID,
		TaskId:    execInfo.Task.TaskID,
		TaskTmpId: execInfo.TmpID,
		Type:      cronpb.LockType_LOCK,
	}); err != nil {
		if errors.Is(err, io.EOF) {
			if _, recvErr := locker.Recv(); recvErr != nil {
				return nil, recvErr
			}
		}
		return nil, err
	}

	if _, err = locker.Recv(); err != nil {
		return nil, err
	}

	disconnectChan := make(chan error, 1)

	go func() {
		defer func() {
			close(disconnectChan)
			cancel()
		}()
		for {
			select {
			case <-execInfo.CancelCtx.Done():
				safe.Run(func() {
					// 任务执行后锁最少保持5s
					// 防止分布式部署下多台机器共同执行
					if execInfo.CancelCtx.Err() != context.Canceled {
						if time.Since(execInfo.RealTime).Seconds() < 4 {
							// 来回网络延迟接近5s
							time.Sleep(4*time.Second - time.Since(execInfo.RealTime))
						}
					}

					err = locker.Send(&cronpb.TryLockRequest{
						ProjectId: execInfo.Task.ProjectID,
						TaskId:    execInfo.Task.TaskID,
						TaskTmpId: execInfo.TmpID,
						Type:      cronpb.LockType_UNLOCK,
					})
					locker.CloseSend()
					wlog.Debug("unlock task", zap.String("task_id", execInfo.Task.TaskID),
						zap.Int64("project_id", execInfo.Task.ProjectID), zap.Error(err))
				})
				return
			default:
				if _, err = locker.Recv(); err != nil {
					disconnectChan <- err
					return
				}
			}
		}
	}()

	return disconnectChan, nil
}
