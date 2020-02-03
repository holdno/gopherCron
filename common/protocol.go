package common

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gorhill/cronexpr"
)

// TaskInfo 任务详情
type TaskInfo struct {
	TaskID    string `json:"task_id"`
	Name      string `json:"name"`
	ProjectID int64  `json:"project_id"`

	Command    string `json:"command"`
	Cron       string `json:"cron"`
	Remark     string `json:"remark"`
	Timeout    int    `json:"timeout"` // 任务超时时间 单位 秒(s)
	CreateTime int64  `json:"create_time"`
	Status     int    `json:"status"`
	IsRunning  int    `json:"is_running"`
	ClientIP   string `json:"client_ip"`
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
	Output      string    // 程序输出
	Err         error     // 是否发生错误
	StartTime   time.Time // 开始时间
	EndTime     time.Time // 结束时间
}

// TaskResultLog 任务执行结果日志
type TaskResultLog struct {
	Result       string `json:"result"`
	CommandError string `json:"command_error"`
	Error        string `json:"error"`
}

// ETCD_PREFIX topic prefix  default: /cron
var (
	ETCD_PREFIX = "/cron"
	TEMPORARY   = "t_scheduler"
)

// BuildKey etcd 保存任务的key
func BuildKey(projectID int64, taskID string) string {
	return fmt.Sprintf("%s/%d/%s", ETCD_PREFIX, projectID, taskID)
}

// BuildSchedulerKey 临时调度的key
func BuildSchedulerKey(projectID int64, taskID string) string {
	return fmt.Sprintf("%s/%d/%s/%s", ETCD_PREFIX, projectID, TEMPORARY, taskID)
}

// IsTemporaryKey 检测是否为临时调度key
func IsTemporaryKey(key string) bool {
	return strings.Contains(key, "/"+TEMPORARY+"/")
}

// BuildLockKey etcd 分布式锁key
func BuildLockKey(projectID int64, taskID string) string {
	return fmt.Sprintf("%s/lock/%d/%s", ETCD_PREFIX)
}

// BuildLockKey etcd 分布式锁key
func BuildKillKey(projectID int64, taskID string) string {
	return fmt.Sprintf("%s/kill/%d/%s", ETCD_PREFIX, projectID, taskID)
}

// BuildRegisterKey etcd 服务发现key
func BuildRegisterKey(projectID int64, ip string) string {
	return fmt.Sprintf("%s/register/%d/%s", ETCD_PREFIX, projectID, ip)
}

// BuildMonitorKey 构建监控信息存储的key
func BuildMonitorKey(ip string) string {
	return ETCD_PREFIX + "/monitor/" + ip
}

// BuildTableKey 构建scheduler 关系表中的key
func (t *TaskInfo) SchedulerKey() string {
	return fmt.Sprintf("%d%s", t.ProjectID, t.TaskID)
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
func ExtractTaskID(project int64, key string) string {
	return strings.TrimPrefix(key, BuildKey(project, ""))
}

// 从etcd的key中提取节点ip
func ExtractWorkerIP(project int64, key string) string {
	return strings.TrimPrefix(key, BuildRegisterKey(project, ""))
}

// 从etcd的key中提取任务名称
func ExtractKillID(project int64, key string) string {
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
func BuildTaskSchedulerPlan(task *TaskInfo) (*TaskSchedulePlan, error) {
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

	if plan.Task.Timeout != 0 {
		info.CancelCtx, info.CancelFunc = context.WithTimeout(context.Background(), time.Duration(plan.Task.Timeout)*time.Second)
	} else {
		info.CancelCtx, info.CancelFunc = context.WithCancel(context.Background())
	}

	return info
}
