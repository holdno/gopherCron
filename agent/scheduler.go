package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/protocol"
	"github.com/holdno/gopherCron/utils"

	"github.com/avast/retry-go/v4"
	"github.com/sirupsen/logrus"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Scheduler 任务调度
type TaskScheduler struct {
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

func initScheduler() *TaskScheduler {
	scheduler := &TaskScheduler{
		TaskEventChan:         make(chan *common.TaskEvent, 3000),
		TaskExecuteResultChan: make(chan *common.TaskExecuteResult, 3000),
		consistency:           &consistency{},
	}
	return scheduler
}

func (ts *TaskScheduler) Stop() {
	wlog.Info("starting to shut down the scheduler")
	for {
		count := ts.TaskExecutingCount()
		if count == 0 {
			ts.PushTaskResult(nil)
			wlog.Info("the scheduler has been shut down")
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
	)
	ts.PlanRange(func(key string, value *common.TaskSchedulePlan) bool {
		projectPlanList[value.Task.ProjectID] = append(projectPlanList[value.Task.ProjectID], key)
		commandMap[key] = value.Task.Command
		return true
	})

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
	var (
		taskEvent     *common.TaskEvent
		scheduleAfter time.Duration
		scheduleTimer *time.Timer
		executeResult *common.TaskExecuteResult
	)

	scheduleAfter = a.TrySchedule()

	// 调度定时器
	scheduleTimer = time.NewTimer(scheduleAfter)

	for {
		select {
		case taskEvent = <-a.scheduler.TaskEventChan:
			// 对内存中的任务进行增删改查
			a.handleTaskEvent(taskEvent)
		case executeResult = <-a.scheduler.TaskExecuteResultChan:
			if executeResult == nil {
				// close signal
				close(a.closeChan)
				return
			}
			a.handleTaskResult(executeResult)
		case <-scheduleTimer.C: // 最近的一个调度任务到期执行
		}

		if a.isClose {
			scheduleTimer.Stop()
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
			logrus.WithField("Error", err.Error()).Error("build task schedule plan error")
			return
		}
		a.TryStartTask(taskSchedulePlan)
	case common.TASK_EVENT_WORKFLOW_SCHEDULE:
		// 构建执行计划
		if taskSchedulePlan, err = common.BuildWorkflowTaskSchedulerPlan(event.Task.TaskInfo); err != nil {
			logrus.WithField("Error", err.Error()).Error("build task schedule plan error")
			return
		}
		a.TryStartTask(taskSchedulePlan)
	case common.TASK_EVENT_SAVE:
		// 构建执行计划
		if event.Task.Status == common.TASK_STATUS_START {
			if taskSchedulePlan, err = common.BuildTaskSchedulerPlan(event.Task, common.NormalPlan); err != nil {
				logrus.WithField("Error", err.Error()).Error("build task schedule plan error")
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
		nearTime *time.Time
	)

	// 如果当前任务调度表中没有任务的话 可以随机睡眠后再尝试
	if a.scheduler.PlanCount() == 0 {
		return time.Second
	}
	now = time.Now()
	// 遍历所有任务
	a.scheduler.PlanRange(func(schedulerKey string, plan *common.TaskSchedulePlan) bool {
		// 如果调度时间是在现在或之前再或者为临时调度任务
		if plan.NextTime.Before(now) || plan.NextTime.Equal(now) {
			// 尝试执行任务
			// 因为可能上一次任务还没执行结束
			a.TryStartTask(plan)
			plan.NextTime = plan.Expr.Next(now) // 更新下一次执行时间
		}

		// 获取下一个要执行任务的时间
		if nearTime == nil || plan.NextTime.Before(*nearTime) {
			nearTime = &plan.NextTime
		}

		return true
	})

	// 下次调度时间 (最近要执行的任务调度时间 - 当前时间)
	return (*nearTime).Sub(now)
}

// TryStartTask 开始执行任务
func (a *client) TryStartTask(plan *common.TaskSchedulePlan) error {
	if plan.TmpID == "" {
		plan.TmpID = utils.GetStrID()
	}
	// 执行的任务可能会执行很久
	// 需要防止并发
	var (
		taskExecuteInfo *common.TaskExecutingInfo
		taskExecuting   bool
	)

	if taskExecuteInfo, taskExecuting = a.scheduler.CheckTaskExecuting(plan.Task.SchedulerKey()); taskExecuting {
		errMsg := "任务执行中，重复调度，请确保任务超时时间配置合理或检查任务是否运行正常"
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

	plan.Task.ClientIP = a.localip
	taskExecuteInfo = common.BuildTaskExecuteInfo(plan)
	errSignal := utils.NewSignalChannel[error]()
	if plan.Type != common.ActivePlan {
		// 如果不是主动调用，则不需要等待几处可能前置的错误来响应web客户端
		errSignal.Close()
	} else {
		wlog.Debug("got active plan",
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

		cancelReason := utils.NewReasonWriter()
		// 构建执行状态信息
		defer func() {
			taskExecuteInfo.CancelFunc()
		}()
		lockError := make(chan error)
		if plan.Task.Noseize != common.TASK_EXECUTE_NOSEIZE {
			// 远程加锁，加锁失败后如果任务还在运行中，则进行重试，如果重试失败，则强行结束任务
			var (
				unLockFunc func()
				err        error
				once       sync.Once
				resultChan = make(chan error)
			)

			go func() {
				tryTimes := 0
				defer func() {
					taskExecuteInfo.CancelFunc()
				}()
				// 避免分布式集群上锁偏斜 (每台机器的时钟可能不是特别的准确 导致某一台机器总能抢到锁)
				time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
				for {
					if taskExecuteInfo.CancelCtx.Err() != nil {
						return
					}
					unLockFunc, err = tryLock(a.GetCenterSrv(), plan, taskExecuteInfo.CancelCtx, lockError)
					tryTimes++

					if err != nil {
						wlog.Error("failed to get task execute lock",
							zap.String("task_id", taskExecuteInfo.Task.TaskID),
							zap.Int64("project_id", taskExecuteInfo.Task.ProjectID),
							zap.String("task_name", taskExecuteInfo.Task.Name),
							zap.Error(err))
						if gerr, ok := status.FromError(err); ok && gerr.Code() == codes.Aborted || tryTimes == 3 {
							first := false
							once.Do(func() {
								first = true
								resultChan <- err
							})
							if !first {
								cancelReason.WriteString(fmt.Sprintf("任务执行过程中锁维持失败,错误信息:%s,agent主动终止任务", err.Error()))
							}
							return
						}
						time.Sleep(time.Second)
						continue
					}

					once.Do(func() {
						close(resultChan)
					})

					tryTimes = 0

					select {
					case <-taskExecuteInfo.CancelCtx.Done():
						return
					case err := <-lockError:
						wlog.Error("failed to keep task lock", zap.Error(err),
							zap.String("task_id", taskExecuteInfo.Task.TaskID),
							zap.Int64("project_id", taskExecuteInfo.Task.ProjectID),
							zap.String("task_name", taskExecuteInfo.Task.Name))
					}
				}
			}()

			if err = <-resultChan; err != nil {
				errSignal.Send(err)
				return
			}

			defer func() {
				if unLockFunc != nil {
					// 任务执行后锁最少保持5s
					// 防止分布式部署下多台机器共同执行
					if time.Since(taskExecuteInfo.RealTime).Seconds() < 5 {
						time.Sleep(5*time.Second - time.Duration(time.Now().Sub(taskExecuteInfo.RealTime).Milliseconds()))
					}

					unLockFunc()
				}
			}()
		}

		a.scheduler.SetExecutingTask(plan.Task.SchedulerKey(), taskExecuteInfo)
		var (
			result *common.TaskExecuteResult
		)
		defer func() {
			// 删除任务的正在执行状态
			a.scheduler.DeleteExecutingTask(plan.Task.SchedulerKey())
			f := protocol.TaskFinishedV1{
				TaskName:  taskExecuteInfo.Task.Name,
				TaskID:    taskExecuteInfo.Task.TaskID,
				Command:   taskExecuteInfo.Task.Command,
				ProjectID: taskExecuteInfo.Task.ProjectID,
				Status:    common.TASK_STATUS_DONE_V2,
				TmpID:     taskExecuteInfo.Task.TmpID,
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
						Version:   "v1",
						Value:     value,
						EventTime: time.Now().Unix(),
					},
				})
				return err
			}, retry.Attempts(3), retry.DelayType(retry.BackOffDelay),
				retry.MaxJitter(time.Minute), retry.LastErrorOnly(true)); err != nil {
				a.Warning(warning.NewTaskWarningData(warning.TaskWarning{
					AgentIP:   a.localip,
					TaskName:  plan.Task.Name,
					TaskID:    plan.Task.TaskID,
					ProjectID: plan.Task.ProjectID,
					Message:   "agent上报任务运行结束状态失败: " + err.Error(),
				}))
				a.logger.Error(fmt.Sprintf("task: %s, id: %s, tmp_id: %s, failed to change running status, the task is finished, error: %v",
					plan.Task.Name, plan.Task.TaskID, plan.Task.TmpID, err))
			}
		}()

		if err := retry.Do(func() error {
			value, _ := json.Marshal(taskExecuteInfo)
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.cfg.Timeout)*time.Second)
			defer cancel()
			_, err := a.GetStatusReporter()(ctx, &cronpb.ScheduleReply{
				ProjectId: plan.Task.ProjectID,
				Event: &cronpb.Event{
					Type:      common.TASK_STATUS_RUNNING_V2,
					Version:   "v1",
					Value:     value,
					EventTime: time.Now().Unix(),
				},
			})
			return err
		}, retry.Attempts(3), retry.DelayType(retry.BackOffDelay),
			retry.MaxJitter(time.Minute), retry.LastErrorOnly(true)); err != nil {

			a.logger.Error(fmt.Sprintf("task: %s, id: %s, tmp_id: %s, change running status error, %v", plan.Task.Name,
				plan.Task.TaskID, plan.Task.TmpID, err))
			errDetail := fmt.Errorf("agent上报任务开始运行状态失败: %s", err.Error())
			cancelReason.WriteString(errDetail.Error())
			errSignal.Send(errDetail)
			taskExecuteInfo.CancelFunc()
		}

		errSignal.Close()

		// 执行任务
		result = a.ExecuteTask(taskExecuteInfo)
		if result.Err != "" {
			cancelReason.WriteStringPrefix("任务执行结果: " + result.Err)
			result.Err = cancelReason.String()
		}
		// 执行结束后 返回给scheduler
		a.scheduler.PushTaskResult(result)
	}()

	return errSignal.WaitOne()
}

// 处理任务结果
func (a *client) handleTaskResult(result *common.TaskExecuteResult) {
	err := utils.RetryFunc(3, func() error {
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
	})

	if err != nil {
		a.logger.Error("task warning report error", zap.Error(err))
	}
}

// 接收任务事件
func (ts *TaskScheduler) PushEvent(event *common.TaskEvent) {
	ts.TaskEventChan <- event
}

func tryLock(cli cronpb.CenterClient, plan *common.TaskSchedulePlan, ctx context.Context, disconnectChan chan error) (func(), error) {
	locker, err := cli.TryLock(ctx)
	if err != nil {
		return nil, err
	}

	if err = locker.Send(&cronpb.TryLockRequest{
		ProjectId: plan.Task.ProjectID,
		TaskId:    plan.Task.TaskID,
	}); err != nil {
		if err == io.EOF {
			if _, err = locker.Recv(); err != nil {
				return nil, err
			}
		}
		return nil, err
	}

	if _, err = locker.Recv(); err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if _, err = locker.Recv(); err != nil {
					disconnectChan <- err
					return
				}
			}
		}
	}()

	return func() {
		locker.CloseSend()
	}, nil
}
