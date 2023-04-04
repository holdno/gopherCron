package protocol

import (
	"context"
	"encoding/json"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/pkg/etcd"
	"github.com/holdno/gopherCron/utils"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	recipe "go.etcd.io/etcd/client/v3/experimental/recipes"
)

type EtcdManager interface {
	Client() *clientv3.Client
	KV() clientv3.KV
	Lease() clientv3.Lease
	Watcher() clientv3.Watcher
	GetTaskLocker(task *common.TaskInfo) *etcd.Locker
	GetLocker(key string) *etcd.Locker
	Inc(key string) (int64, error)
}

type comm struct {
	etcd     EtcdManager
	queueMap map[int64]*recipe.Queue
}

type CommonInterface interface {
	GetTask(projectID int64, taskID string) (*common.TaskInfo, error)
	GetVersion() string
}

func NewComm(etcd EtcdManager) CommonInterface {
	return &comm{etcd: etcd}
}

type ClientEtcdManager interface {
	TemporarySchedulerTask(task *common.TaskInfo) error
	GetVersion() string
}

func NewClientEtcdManager(etcd EtcdManager, projects []int64) ClientEtcdManager {
	m := &comm{etcd: etcd, queueMap: make(map[int64]*recipe.Queue)}

	for _, v := range projects {
		m.queueMap[v] = recipe.NewQueue(etcd.Client(), common.BuildTaskResultQueueProjectKey(v))
	}

	InitRunningManager(m.queueMap)

	return m
}

type RunningManager interface {
	SetTaskRunning(s *concurrency.Session, plan *common.TaskExecutingInfo) error
	SetTaskNotRunning(s *concurrency.Session, plan *common.TaskExecutingInfo, result *common.TaskExecuteResult) error
}

var runningManager = make(map[common.PlanType]RunningManager)

func InitRunningManager(queue map[int64]*recipe.Queue) {
	runningManager[common.NormalPlan] = &defaultRunningManager{
		queue: queue,
	}
	runningManager[common.WorkflowPlan] = &workflowRunningManager{
		queue: queue,
	}
}

func SetTaskRunning(session *concurrency.Session, execInfo *common.TaskExecutingInfo) error {
	rm, exist := runningManager[execInfo.PlanType]
	if !exist {
		rm = runningManager[common.NormalPlan]
	}
	return rm.SetTaskRunning(session, execInfo)
}

func SetTaskNotRunning(s *concurrency.Session, execInfo *common.TaskExecutingInfo, result *common.TaskExecuteResult) error {
	rm, exist := runningManager[execInfo.PlanType]
	if !exist {
		rm = runningManager[common.NormalPlan]
	}
	return rm.SetTaskNotRunning(s, execInfo, result)
}

// TemporarySchedulerTask 临时调度任务
func (a *comm) TemporarySchedulerTask(task *common.TaskInfo) error {
	var (
		schedulerKey   string
		saveByte       []byte
		leaseGrantResp *clientv3.LeaseGrantResponse
		ctx            context.Context
		errObj         errors.Error
		err            error
	)

	// reset task create time as schedule time
	task.CreateTime = time.Now().Unix()

	// task to json
	if saveByte, err = json.Marshal(task); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - TemporarySchedulerTask] json.mashal task error:" + err.Error()
		return errObj
	}

	// build etcd save key
	if task.FlowInfo != nil {
		schedulerKey = common.BuildWorkflowSchedulerKey(task.FlowInfo.WorkflowID, task.ProjectID, task.TaskID)
	} else {
		schedulerKey = common.BuildSchedulerKey(task.ProjectID, task.TaskID)
	}

	ctx, _ = utils.GetContextWithTimeout()
	// make lease to notify worker
	// 创建一个租约 让其稍后过期并自动删除
	if leaseGrantResp, err = a.etcd.Lease().Grant(ctx, 1); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - TemporarySchedulerTask] lease grant error:" + err.Error()
		return errObj
	}

	ctx, _ = utils.GetContextWithTimeout()
	// save to etcd
	if _, err = a.etcd.KV().Put(ctx, schedulerKey, string(saveByte), clientv3.WithLease(leaseGrantResp.ID)); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - TemporarySchedulerTask] etcd client kv put error:" + err.Error()
		return errObj
	}

	// wait agent execute

	return nil
}

// GetTask 获取任务
func (a *comm) GetTask(projectID int64, taskID string) (*common.TaskInfo, error) {
	var (
		saveKey string
		getResp *clientv3.GetResponse
		task    *common.TaskInfo
		ctx     context.Context
		errObj  errors.Error
		err     error
	)

	// build etcd save key
	saveKey = common.BuildKey(projectID, taskID) // 保存的key同样也是获取的key

	ctx, _ = utils.GetContextWithTimeout()

	if getResp, err = a.etcd.KV().Get(ctx, saveKey); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetTask] etcd client kv get one error:" + err.Error()
		return nil, errObj
	}

	if getResp.Count > 1 {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetTask] etcd client kv get one task but result > 1"
		return nil, errObj
	} else if getResp.Count == 0 {
		return nil, errors.ErrDataNotFound
	}

	task = &common.TaskInfo{}
	if err = json.Unmarshal(getResp.Kvs[0].Value, task); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetTask] task json.Unmarshal error:" + err.Error()
		return nil, errObj
	}

	return task, nil
}

const (
	version = "v2.1.0"
)

func (a *comm) GetVersion() string {
	return version
}

const (
	QueueItemV1 = "v1"
)

type TaskFinishedQueueItemV1 struct {
	TaskID     string          `json:"task_id"`
	ProjectID  int64           `json:"project_id"`
	Status     string          `json:"status"`
	TaskType   common.PlanType `json:"task_type"`
	WorkflowID int64           `json:"workflow_id"`
	StartTime  int64           `json:"start_time"`
	EndTime    int64           `json:"end_time"`
	TmpID      string          `json:"tmp_id"`
}

type TaskFinishedQueueContent struct {
	Version string          `json:"version"`
	Data    json.RawMessage `json:"data"`
}

type TaskFinishedV1 struct {
	TaskID     string `json:"task_id"`
	ProjectID  int64  `json:"project_id"`
	Status     string `json:"status"`
	WorkflowID int64  `json:"workflow_id"`
	StartTime  int64  `json:"start_time"`
	EndTime    int64  `json:"end_time"`
	TmpID      string `json:"tmp_id"`
	Result     string `json:"result"`
}
