package protocol

import (
	"context"
	"encoding/json"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/pkg/etcd"
	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
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
	etcd EtcdManager
}

type CommonInterface interface {
	SetTaskRunning(plan *common.TaskSchedulePlan) error
	SetTaskNotRunning(plan *common.TaskSchedulePlan) error
	SaveTask(task *common.TaskInfo, opts ...clientv3.OpOption) (*common.TaskInfo, error)
	GetTask(projectID int64, taskID string) (*common.TaskInfo, error)
	TemporarySchedulerTask(task *common.TaskInfo) error
	GetVersion() string
}

func NewComm(etcd EtcdManager) CommonInterface {
	return &comm{etcd: etcd}
}

type RunningManager interface {
	SetTaskRunning(kv clientv3.KV, plan *common.TaskSchedulePlan) error
	SetTaskNotRunning(kv clientv3.KV, plan *common.TaskSchedulePlan) error
}

var runningManager = make(map[common.PlanType]RunningManager)

func (a *comm) SetTaskRunning(plan *common.TaskSchedulePlan) error {
	rm, exist := runningManager[plan.Type]
	if !exist {
		rm = &defaultRunningManager{}
	}
	return rm.SetTaskRunning(a.etcd.KV(), plan)
}

func (a *comm) SetTaskNotRunning(plan *common.TaskSchedulePlan) error {
	rm, exist := runningManager[plan.Type]
	if !exist {
		rm = &defaultRunningManager{}
	}
	return rm.SetTaskNotRunning(a.etcd.KV(), plan)
}

// SaveTask save task to etcd
// return oldtask & error
func (a *comm) SaveTask(task *common.TaskInfo, opts ...clientv3.OpOption) (*common.TaskInfo, error) {
	var (
		saveKey  string
		saveByte []byte
		putResp  *clientv3.PutResponse
		oldTask  *common.TaskInfo
		ctx      context.Context
		errObj   errors.Error
		err      error
		locker   = a.etcd.GetLocker(common.BuildTaskUpdateKey(task.ProjectID, task.TaskID))
	)
	err = utils.RetryFunc(10, func() error {
		if err = locker.TryLock(); err != nil {
			time.Sleep(time.Duration(utils.Random(200, 600)) * time.Second)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	defer locker.Unlock()

	if oldTask, err = a.GetTask(task.ProjectID, task.TaskID); err != nil && err != errors.ErrDataNotFound {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - SaveTask] get old task info error:" + err.Error()
		return nil, errObj
	}

	if oldTask != nil && task.IsRunning == common.TASK_STATUS_UNDEFINED {
		task.IsRunning = oldTask.IsRunning
		task.ClientIP = oldTask.ClientIP
	}
	// build etcd save key
	saveKey = common.BuildKey(task.ProjectID, task.TaskID)

	// task to json
	if saveByte, err = json.Marshal(task); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - SaveTask] json.mashal task error:" + err.Error()
		return nil, errObj
	}
	ctx, _ = utils.GetContextWithTimeout()
	// save to etcd
	if putResp, err = a.etcd.KV().Put(ctx, saveKey, string(saveByte), clientv3.WithPrevKV()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - SaveTask] etcd client kv put error:" + err.Error()
		return nil, errObj
	}

	// oldtask exist
	if putResp.PrevKv != nil {
		// if oldtask unmarshal error
		// don't care because this err doesn't affect result
		_ = json.Unmarshal(putResp.PrevKv.Value, &oldTask)
	} else {
		oldTask = task
	}
	return oldTask, nil
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

	// task to json
	if saveByte, err = json.Marshal(task); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - TemporarySchedulerTask] json.mashal task error:" + err.Error()
		return errObj
	}

	// build etcd save key
	schedulerKey = common.BuildSchedulerKey(task.ProjectID, task.TaskID)

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
	version = "v1.10.12"
)

func (a *comm) GetVersion() string {
	return version
}
