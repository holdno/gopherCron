package common

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gorhill/cronexpr"
)

// TaskInfo 任务详情
type TaskInfo struct {
	Name       string `json:"name"`
	Project    string `json:"project"`
	Command    string `json:"command"`
	Cron       string `json:"cron"`
	Remark     string `json:"remark"`
	CreateTime int64  `json:"create_time"`
	Status     int    `json:"status"`
}

// TaskSchedulePlan 任务调度计划
type TaskSchedulePlan struct {
	Task     *TaskInfo
	Expr     *cronexpr.Expression // 解析后的cron表达式
	NextTime time.Time
}

// TaskExecutingInfo 任务执行状态
type TaskExecutingInfo struct {
	Task     *TaskInfo
	PlanTime time.Time // 理论上的调度时间
	RealTime time.Time // 实际调度时间

	CancelCtx  context.Context
	CancelFunc context.CancelFunc // 用来取消Command执行的cancel函数
}

// TaskExecuteResult 任务执行结果
type TaskExecuteResult struct {
	ExecuteInfo *TaskExecutingInfo
	Output      []byte    // 程序输出
	Err         error     // 是否发生错误
	StartTime   time.Time // 开始时间
	EndTime     time.Time // 结束时间
}

// ETCD_PREFIX topic prefix  default: /cron
var ETCD_PREFIX = "/cron"

// BuildKey etcd 保存任务的key
func BuildKey(project, name string) string {
	return ETCD_PREFIX + "/" + project + "/" + name
}

// BuildLockKey etcd 分布式锁key
func BuildLockKey(project, name string) string {
	return ETCD_PREFIX + "/lock/" + project + "/" + name
}

// BuildLockKey etcd 分布式锁key
func BuildKillKey(project, name string) string {
	return ETCD_PREFIX + "/kill/" + project + "/" + name
}

// BuildRegisterKey etcd 服务发现key
func BuildRegisterKey(project, ip string) string {
	return ETCD_PREFIX + "/register/" + project + "/" + ip
}

// BuildTableKey 构建scheduler 关系表中的key
func (t *TaskInfo) ScheduleKey() string {
	return t.Project + t.Name
}

func Unmarshal(value []byte) (*TaskInfo, error) {
	task := new(TaskInfo)
	err := json.Unmarshal(value, task)
	if err != nil {
		return nil, err
	}

	return task, nil
}

// 从etcd的key中提取任务名称
func ExtractTaskName(project, key string) string {
	return strings.TrimPrefix(key, BuildKey(project, ""))
}

// 从etcd的key中提取节点ip
func ExtractWorkerIP(project, key string) string {
	return strings.TrimPrefix(key, BuildRegisterKey(project, ""))
}

// 从etcd的key中提取任务名称
func ExtractKillName(project, key string) string {
	return strings.TrimPrefix(key, BuildKillKey(project, ""))
}

type TaskEvent struct {
	EventType int // save delete
	Task      *TaskInfo
}

func BuildTaskEvent(eventType int, task *TaskInfo) *TaskEvent {
	return &TaskEvent{
		EventType: eventType,
		Task:      task,
	}
}

// 构造执行计划
func BuildTaskSchedulePlan(task *TaskInfo) (*TaskSchedulePlan, error) {
	var (
		expr *cronexpr.Expression
		err  error
	)

	if expr, err = cronexpr.Parse(task.Cron); err != nil {
		return nil, err
	}

	return &TaskSchedulePlan{
		Task:     task,
		Expr:     expr,
		NextTime: expr.Next(time.Now()),
	}, nil
}

// BuildTaskExecuteInfo 构建 executer
func BuildTaskExecuteInfo(plan *TaskSchedulePlan) *TaskExecutingInfo {
	info := &TaskExecutingInfo{
		Task:     plan.Task,
		PlanTime: plan.NextTime, // 计划调度时间
		RealTime: time.Now(),    // 真实执行时间
	}

	info.CancelCtx, info.CancelFunc = context.WithCancel(context.TODO())

	return info
}
