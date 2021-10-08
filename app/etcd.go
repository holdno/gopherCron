package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/protocol"
	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

// GetTaskList 获取任务列表
func (a *app) GetTaskList(projectID int64) ([]*common.TaskInfo, error) {
	var (
		preKey   string
		getResp  *clientv3.GetResponse
		taskList []*common.TaskInfo
		task     *common.TaskInfo
		kvPair   *mvccpb.KeyValue
		ctx      context.Context
		errObj   errors.Error
		err      error
	)

	// build etcd pre key
	preKey = common.BuildKey(projectID, "")

	ctx, _ = utils.GetContextWithTimeout()
	if getResp, err = a.etcd.KV().Get(ctx, preKey,
		clientv3.WithPrefix()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetTaskList] etcd client kv getlist error:" + err.Error()
		return nil, errObj
	}

	// init array space
	taskList = make([]*common.TaskInfo, 0)
	taskStatus := make(map[string]string)

	// range list to unmarshal
	for _, kvPair = range getResp.Kvs {
		if common.IsTemporaryKey(string(kvPair.Key)) {
			continue
		}
		if common.IsStatusKey(string(kvPair.Key)) {
			pid, tid := common.PatchProjectIDTaskIDFromStatusKey(string(kvPair.Key))
			taskStatus[fmt.Sprintf("%s%s", pid, tid)] = string(kvPair.Value)
			continue
		}

		task = &common.TaskInfo{}
		if err = json.Unmarshal(kvPair.Value, task); err != nil {
			continue
		}

		taskList = append(taskList, task)
	}

	for _, v := range taskList {
		status := taskStatus[fmt.Sprintf("%d%s", v.ProjectID, v.TaskID)]
		var taskRuningInfo common.TaskRunningInfo
		if err = json.Unmarshal([]byte(status), &taskRuningInfo); err == nil {
			status = taskRuningInfo.Status
		}
		switch status {
		case common.TASK_STATUS_RUNNING_V2:
			v.IsRunning = common.TASK_STATUS_RUNNING
		default:
			v.IsRunning = common.TASK_STATUS_NOT_RUNNING
		}
	}

	return taskList, nil
}

// GetTaskList 获取任务列表
func (a *app) GetProjectTaskCount(projectID int64) (int64, error) {
	var (
		preKey  string
		getResp *clientv3.GetResponse
		errObj  errors.Error
		err     error
		ctx     context.Context
	)

	// build etcd pre key
	preKey = common.BuildKey(projectID, "")
	ctx, _ = utils.GetContextWithTimeout()
	if getResp, err = a.etcd.KV().Get(ctx, preKey,
		clientv3.WithPrefix(), clientv3.WithCountOnly()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetProjectTaskCount] etcd client kv error:" + err.Error()
		return 0, errObj
	}

	return getResp.Count, nil
}

// KillTask 强行结束任务
func (a *app) KillTask(projectID int64, name string) error {
	var (
		killKey        string
		leaseGrantResp *clientv3.LeaseGrantResponse
		errObj         errors.Error
		err            error
		ctx            context.Context
	)

	killKey = common.BuildKillKey(projectID, name)
	ctx, _ = utils.GetContextWithTimeout()
	// make lease to notify worker
	// 创建一个租约 让其稍后过期并自动删除
	if leaseGrantResp, err = a.etcd.Lease().Grant(ctx, 1); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - KillTask] lease grant error:" + err.Error()
		return errObj
	}

	ctx, _ = utils.GetContextWithTimeout()
	if _, err = a.etcd.KV().Put(ctx, killKey, "", clientv3.WithLease(leaseGrantResp.ID)); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - KillTask] put kill task error:" + err.Error()
		return errObj
	}

	return nil
}

func (a *app) CheckProjectWorkerExist(projectID int64, host string) (bool, error) {
	var (
		preKey  string
		err     error
		errObj  errors.Error
		getResp *clientv3.GetResponse
		ctx     context.Context
	)

	preKey = common.BuildRegisterKey(projectID, host)
	ctx, _ = utils.GetContextWithTimeout()
	if getResp, err = a.etcd.KV().Get(ctx, preKey); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetWorkerList] get key error:" + err.Error()
		return false, errObj
	}

	if getResp.Count == 0 {
		return false, nil
	}

	return true, nil
}

// GetWorkerList 获取节点列表
func (a *app) GetWorkerList(projectID int64) ([]common.ClientInfo, error) {
	var (
		preKey  string
		err     error
		errObj  errors.Error
		getResp *clientv3.GetResponse
		kv      *mvccpb.KeyValue
		ctx     context.Context
		res     []common.ClientInfo
	)

	preKey = common.BuildRegisterKey(projectID, "")
	ctx, _ = utils.GetContextWithTimeout()
	if getResp, err = a.etcd.KV().Get(ctx, preKey, clientv3.WithPrefix()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetWorkerList] get preKey error:" + err.Error()
		return nil, errObj
	}

	for _, kv = range getResp.Kvs {
		var (
			ip         = common.ExtractWorkerIP(projectID, string(kv.Key))
			clientinfo common.ClientInfo
		)

		_ = json.Unmarshal(kv.Value, &clientinfo)
		clientinfo.ClientIP = ip
		if clientinfo.Version == "" {
			clientinfo.Version = "unknow"
		}

		res = append(res, clientinfo)
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].ClientIP < res[j].ClientIP
	})

	return res, nil
}

func (a *app) DeleteAll() error {
	var (
		deleteKey string
		ctx       context.Context
		errObj    errors.Error
		err       error
	)

	// build etcd delete key
	deleteKey = common.ETCD_PREFIX + "/"
	ctx, _ = utils.GetContextWithTimeout()
	// save to etcd
	if _, err = a.etcd.KV().Delete(ctx, deleteKey, clientv3.WithPrevKV(), clientv3.WithPrefix()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - DeleteAll] etcd client kv delete error:" + err.Error()
		return errObj
	}

	return nil
}

// SaveMonitor 保存节点的监控信息
func (a *app) SaveMonitor(ip string, monitorInfo []byte) error {
	var (
		monitorKey     string
		leaseGrantResp *clientv3.LeaseGrantResponse
		ctx            context.Context
		errObj         errors.Error
		err            error
	)

	// build worker monitor key
	monitorKey = common.BuildMonitorKey(ip)

	ctx, _ = utils.GetContextWithTimeout()
	// make lease to notify worker
	// 创建一个租约 让其稍后过期并自动删除
	if leaseGrantResp, err = a.etcd.Lease().Grant(ctx, common.MonitorFrequency+1); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - SaveMonitor] lease grant error:" + err.Error()
		return errObj
	}

	ctx, _ = utils.GetContextWithTimeout()
	// save to etcd
	if _, err = a.etcd.KV().Put(ctx, monitorKey, string(monitorInfo), clientv3.WithLease(leaseGrantResp.ID)); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - SaveMonitor] etcd client kv put error:" + err.Error()
		return errObj
	}

	return nil
}

// GetMonitor 获取节点的监控信息
func (a *app) GetMonitor(ip string) (*common.MonitorInfo, error) {

	var (
		monitorKey string
		getResp    *clientv3.GetResponse
		monitor    *common.MonitorInfo
		ctx        context.Context
		errObj     errors.Error
		err        error
	)
	// build worker monitor key
	monitorKey = common.BuildMonitorKey(ip)

	ctx, _ = utils.GetContextWithTimeout()

	if getResp, err = a.etcd.KV().Get(ctx, monitorKey); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetMonitor] etcd client kv get one error:" + err.Error()
		return nil, errObj
	}

	if getResp.Count > 1 {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetMonitor] etcd client kv get one task but result > 1"
		return nil, errObj
	} else if getResp.Count == 0 {
		return nil, errors.ErrDataNotFound
	}

	monitor = &common.MonitorInfo{}
	if err = json.Unmarshal(getResp.Kvs[0].Value, monitor); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetMonitor] monitor json.Unmarshal error:" + err.Error()
		return nil, errObj
	}

	return monitor, nil
}
