package etcd

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/mongodb/mongo-go-driver/bson/primitive"

	"github.com/sirupsen/logrus"

	"ojbk.io/gopherCron/pkg/db"

	"ojbk.io/gopherCron/common"
)

// Scheduler 任务调度
type TaskScheduler struct {
	TaskEventChan         chan *common.TaskEvent // 任务事件队列
	PlanTable             sync.Map
	TaskExecuteResultChan chan *common.TaskExecuteResult
	// PlanTable             map[string]*common.TaskSchedulePlan  // 任务调度计划表
	TaskExecutingTable map[string]*common.TaskExecutingInfo // 任务执行中的记录表
}

func (ts *TaskScheduler) GetPlan(key string) (*common.TaskSchedulePlan, bool) {
	var (
		value interface{}
		ok    bool
	)
	if value, ok = ts.PlanTable.Load(key); ok {
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

func (ts *TaskScheduler) RemovePlan(schedulerKey string) {
	ts.PlanTable.Delete(schedulerKey)
}

// Scheduler 单例
var Scheduler *TaskScheduler

// InitScheduler 初始化调度器
func InitScheduler() {
	Scheduler = &TaskScheduler{
		TaskEventChan:         make(chan *common.TaskEvent, 3000),
		TaskExecuteResultChan: make(chan *common.TaskExecuteResult, 3000),
		TaskExecutingTable:    make(map[string]*common.TaskExecutingInfo),
	}

	go Scheduler.Loop()
}

func (ts *TaskScheduler) Loop() {
	var (
		taskEvent     *common.TaskEvent
		scheduleAfter time.Duration
		scheduleTimer *time.Timer
		executeResult *common.TaskExecuteResult
	)

	scheduleAfter = ts.TrySchedule()

	// 调度定时器
	scheduleTimer = time.NewTimer(scheduleAfter)

	for {
		select {
		case taskEvent = <-ts.TaskEventChan:
			// 对内存中的任务进行增删改查
			ts.handleTaskEvent(taskEvent)
		case executeResult = <-Scheduler.TaskExecuteResultChan:
			ts.handleTaskResult(executeResult)
		case <-scheduleTimer.C: // 最近的一个调度任务到期执行
		}

		// 每次触发事件后 重新计算下次调度任务时间
		scheduleAfter = ts.TrySchedule()
		scheduleTimer.Reset(scheduleAfter)
	}
}

// handleTaskEvent 处理事件
func (ts *TaskScheduler) handleTaskEvent(event *common.TaskEvent) {
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
		ts.TryStartTask(taskSchedulePlan)
	case common.TASK_EVENT_SAVE:
		// 构建执行计划
		if taskSchedulePlan, err = common.BuildTaskSchedulerPlan(event.Task); err != nil {
			logrus.WithField("Error", err.Error()).Error("build task schedule plan error")
			return
		}
		if event.Task.Status == 1 {
			ts.SetPlan(event.Task.SchedulerKey(), taskSchedulePlan)
			return
		}
		// 如果任务保存状态不为1 证明不需要执行 所以顺延执行delete事件，从计划表中删除任务
		fallthrough
	case common.TASK_EVENT_DELETE:
		ts.RemovePlan(event.Task.SchedulerKey())
	case common.TASK_EVENT_KILL:
		// 先判断任务是否在执行中
		if taskExecuteinfo, taskExecuting = Scheduler.TaskExecutingTable[event.Task.SchedulerKey()]; taskExecuting {
			taskExecuteinfo.CancelFunc()
		}
	}
}

// 重新计算任务调度状态
func (ts *TaskScheduler) TrySchedule() time.Duration {
	var (
		now      time.Time
		nearTime *time.Time
	)

	// 如果当前任务调度表中没有任务的话 可以随机睡眠后再尝试
	if ts.PlanCount() == 0 {
		return time.Second
	}

	now = time.Now()
	// 遍历所有任务
	ts.PlanRange(func(schedulerKey string, plan *common.TaskSchedulePlan) bool {
		// 如果调度时间是在现在或之前再或者为临时调度任务
		if plan.NextTime.Before(now) || plan.NextTime.Equal(now) {
			// 尝试执行任务
			// 因为可能上一次任务还没执行结束
			ts.TryStartTask(plan)
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
func (ts *TaskScheduler) TryStartTask(plan *common.TaskSchedulePlan) {
	// 执行的任务可能会执行很久
	// 需要防止并发
	var (
		taskExecuteInfo *common.TaskExecutingInfo
		taskExecuting   bool
	)

	if taskExecuteInfo, taskExecuting = ts.TaskExecutingTable[plan.Task.SchedulerKey()]; taskExecuting {
		return
	}

	// 构建执行状态信息
	taskExecuteInfo = common.BuildTaskExecuteInfo(plan)

	// 保存执行状态
	ts.TaskExecutingTable[plan.Task.SchedulerKey()] = taskExecuteInfo

	Executer.ExecuteTask(taskExecuteInfo)
}

func (ts *TaskScheduler) PushTaskResult(result *common.TaskExecuteResult) {
	ts.TaskExecuteResultChan <- result
}

// 处理任务结果
func (ts *TaskScheduler) handleTaskResult(result *common.TaskExecuteResult) {
	// 删除任务的正在执行状态
	delete(ts.TaskExecutingTable, result.ExecuteInfo.Task.SchedulerKey())
	var (
		resultBytes    []byte
		projectID      primitive.ObjectID
		projectInfo    *common.Project
		err            error
		getError       int
		logInfo        *common.TaskLog
		taskResult     *common.TaskResultLog
		jsonMarshalErr error
	)

	if projectID, err = primitive.ObjectIDFromHex(result.ExecuteInfo.Task.ProjectID); err != nil {
		goto WITH_ERROR
	}

	projectInfo, err = db.GetProject(projectID)

WITH_ERROR:

	taskResult = &common.TaskResultLog{
		Result: string(result.Output),
	}

	if result.Err != nil {
		taskResult.CommandError = result.Err.Error()
		getError = 1
	}

	if err != nil {
		taskResult.Error = err.Error()
		getError = 1
	}
	if resultBytes, jsonMarshalErr = json.Marshal(taskResult); jsonMarshalErr != nil {
		resultBytes = []byte("result log json marshal error:" + jsonMarshalErr.Error())
	}

	logInfo = &common.TaskLog{
		Name:      result.ExecuteInfo.Task.Name,
		Result:    string(resultBytes),
		StartTime: result.StartTime.Unix(),
		EndTime:   result.EndTime.Unix(),
		Command:   result.ExecuteInfo.Task.Command,
		WithError: getError,
		ClientIP:  common.LocalIP,
	}

	if projectInfo != nil {
		logInfo.Project = projectInfo.Title
	}

	logInfo.ProjectID, _ = primitive.ObjectIDFromHex(result.ExecuteInfo.Task.ProjectID)
	logInfo.TaskID, _ = primitive.ObjectIDFromHex(result.ExecuteInfo.Task.TaskID)

	db.CreateTaskLog(logInfo)
}

// 接收任务事件
func (ts *TaskScheduler) PushEvent(event *common.TaskEvent) {
	ts.TaskEventChan <- event
}
