package app

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/utils"

	"github.com/sirupsen/logrus"
)

// Scheduler 任务调度
type TaskScheduler struct {
	TaskEventChan         chan *common.TaskEvent // 任务事件队列
	PlanTable             sync.Map
	TaskExecuteResultChan chan *common.TaskExecuteResult
	// PlanTable             map[string]*common.TaskSchedulePlan  // 任务调度计划表
	TaskExecutingTable sync.Map // 任务执行中的记录表
}

func initScheduler() *TaskScheduler {
	scheduler := &TaskScheduler{
		TaskEventChan:         make(chan *common.TaskEvent, 3000),
		TaskExecuteResultChan: make(chan *common.TaskExecuteResult, 3000),
	}
	return scheduler
}

func (ts *TaskScheduler) Stop() {
	fmt.Println("scheduler is about to close")
	for {
		count := ts.TaskExecutingCount()
		fmt.Println("current executing task:", count)
		if count == 0 {
			ts.PushTaskResult(nil)
			return
		}
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

func (ts *TaskScheduler) SetPlan(key string, value *common.TaskSchedulePlan) {
	ts.PlanTable.Store(key, value)
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

func (a *client) RemovePlan(schedulerKey string) {
	a.scheduler.PlanTable.Delete(schedulerKey)
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
			continue
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
		taskExecuteinfo  *common.TaskExecutingInfo
		taskExecuting    bool
		err              error
	)
	switch event.EventType {
	// 临时调度
	case common.TASK_EVENT_TEMPORARY:
		// 构建执行计划
		if taskSchedulePlan, err = common.BuildTaskSchedulerPlan(event.Task); err != nil {
			logrus.WithField("Error", err.Error()).Error("build task schedule plan error")
			return
		}
		a.TryStartTask(taskSchedulePlan)
	case common.TASK_EVENT_SAVE:
		// 构建执行计划
		if event.Task.Status == common.TASK_STATUS_START {
			if taskSchedulePlan, err = common.BuildTaskSchedulerPlan(event.Task); err != nil {
				logrus.WithField("Error", err.Error()).Error("build task schedule plan error")
				return
			}

			a.scheduler.SetPlan(event.Task.SchedulerKey(), taskSchedulePlan)
			return
		}
		// 如果任务保存状态不为1 证明不需要执行 所以顺延执行delete事件，从计划表中删除任务
		fallthrough
	case common.TASK_EVENT_DELETE:
		a.RemovePlan(event.Task.SchedulerKey())
	case common.TASK_EVENT_KILL:
		// 先判断任务是否在执行中
		if taskExecuteinfo, taskExecuting = a.scheduler.CheckTaskExecuting(event.Task.SchedulerKey()); taskExecuting {
			taskExecuteinfo.CancelFunc()
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
func (a *client) TryStartTask(plan *common.TaskSchedulePlan) {
	// 执行的任务可能会执行很久
	// 需要防止并发
	var (
		taskExecuteInfo *common.TaskExecutingInfo
		taskExecuting   bool
		err             error
	)

	if taskExecuteInfo, taskExecuting = a.scheduler.CheckTaskExecuting(plan.Task.SchedulerKey()); taskExecuting {
		a.scheduler.PushTaskResult(&common.TaskExecuteResult{
			ExecuteInfo: common.BuildTaskExecuteInfo(plan),
			Output:      "last task was not completed",
			Err:         fmt.Sprintf("task %s execute error: last task was not completed", plan.Task.Name),
			StartTime:   time.Now(),
			EndTime:     time.Now(),
		})
		return
	}

	plan.Task.ClientIP = a.localip

	a.Go(func() {
		// 构建执行状态信息
		taskExecuteInfo = common.BuildTaskExecuteInfo(plan)
		if plan.Task.Noseize != common.TASK_EXECUTE_NOSEIZE {
			lk := a.etcd.GetTaskLocker(plan.Task)
			// 保存执行状态
			// 避免分布式集群上锁偏斜 (每台机器的时钟可能不是特别的准确 导致某一台机器总能抢到锁)
			time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
			if err := lk.TryLock(); err != nil {
				a.logger.Warnf("task: %s, id: %s, lock error, %v", plan.Task.Name,
					plan.Task.TaskID, err)
				return
			}
			defer func() {
				// 任务执行后锁最少保持5s
				// 防止分布式部署下多台机器共同执行
				if time.Since(taskExecuteInfo.RealTime).Seconds() < 5 {
					time.Sleep(5*time.Second - time.Duration(time.Now().Sub(taskExecuteInfo.RealTime).Milliseconds()))
				}
				lk.Unlock()
			}()
		}

		a.scheduler.SetExecutingTask(plan.Task.SchedulerKey(), taskExecuteInfo)
		defer func() {
			// 删除任务的正在执行状态
			a.scheduler.DeleteExecutingTask(plan.Task.SchedulerKey())
			if err = utils.RetryFunc(5, func() error {
				return a.SetTaskNotRunning(*plan.Task)
			}); err != nil {
				a.logger.Errorf("task: %s, id: %s, failed to change running status, the task is finished, error: %v",
					plan.Task.Name, plan.Task.TaskID, err)
			}
		}()

		if err = a.SetTaskRunning(*plan.Task); err != nil {
			a.logger.Warnf("task: %s, id: %s, change running status error, %v", plan.Task.Name,
				plan.Task.TaskID, err)
			// retry
			if err = utils.RetryFunc(5, func() error {
				return a.TemporarySchedulerTask(plan.Task)
			}); err != nil {
				a.logger.Errorf(
					"task: %s, id: %s, save task running status error and rescheduler error: %v",
					plan.Task.Name, plan.Task.TaskID, err)
			}
			return
		}

		// 执行任务
		result := a.ExecuteTask(taskExecuteInfo)
		// 执行结束后 返回给scheduler
		a.scheduler.PushTaskResult(result)
	})
}

// 处理任务结果
func (a *client) handleTaskResult(result *common.TaskExecuteResult) {
	err := utils.RetryFunc(10, func() error {
		if result.Err != "" {
			err := a.Warning(WarningData{
				Data:      result.Err,
				Type:      WarningTypeTask,
				TaskName:  result.ExecuteInfo.Task.Name,
				ProjectID: result.ExecuteInfo.Task.ProjectID,
				AgentIP:   a.GetIP(),
			})

			return err
		}
		return nil
	})

	if err != nil {
		a.logger.WithField("desc", err).Error("task warning report error")
	}

	err = utils.RetryFunc(10, func() error {
		err := a.ResultReport(result)
		return err
	})

	if err != nil {
		a.logger.WithField("desc", err).Error("task result report error")
	}

}

// 接收任务事件
func (ts *TaskScheduler) PushEvent(event *common.TaskEvent) {
	ts.TaskEventChan <- event
}
