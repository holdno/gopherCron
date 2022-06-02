package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/holdno/gocommons/selection"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/jinzhu/gorm"
)

func getTaskDetail(kv clientv3.KV, projectID int64, taskID string) (*common.TaskInfo, error) {
	taskKey := common.BuildKey(projectID, taskID)
	ctx, _ := utils.GetContextWithTimeout()
	resp, err := kv.Get(ctx, taskKey)
	if err != nil {
		return nil, err
	}

	if resp.Count == 0 {
		return nil, nil
	}

	var task common.TaskInfo
	if err = json.Unmarshal(resp.Kvs[0].Value, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// GetTaskList 获取任务列表
func (a *app) GetTaskList(projectID int64) ([]*common.TaskListItemWithWorkflows, error) {
	var (
		preKey   string
		getResp  *clientv3.GetResponse
		taskList []*common.TaskListItemWithWorkflows
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
	taskStatus := make(map[string]string)
	relevanceWorkflow := make(map[string][]int64)
	var workflowTaskIndex []string

	// range list to unmarshal
	for _, kvPair = range getResp.Kvs {
		if common.IsTemporaryKey(string(kvPair.Key)) {
			continue
		}
		if common.IsAckKey(string(kvPair.Key)) {
			continue
		}

		if common.IsStatusKey(string(kvPair.Key)) {
			pid, tid := common.PatchProjectIDTaskIDFromStatusKey(string(kvPair.Key))
			taskStatus[fmt.Sprintf("%s_%s", pid, tid)] = string(kvPair.Value)
			continue
		}

		task := &common.TaskListItemWithWorkflows{}
		if err = json.Unmarshal(kvPair.Value, task); err != nil {
			continue
		}

		taskList = append(taskList, task)
		workflowTaskIndex = append(workflowTaskIndex, common.BuildWorkflowTaskIndex(task.ProjectID, task.TaskID))
	}

	if len(workflowTaskIndex) > 0 {
		list, err := a.store.WorkflowSchedulePlan().GetTaskWorkflowIDs(workflowTaskIndex)
		if err != nil && err != gorm.ErrRecordNotFound {
			return nil, errors.NewError(http.StatusInternalServerError, "获取任务关联workflow失败").WithLog(err.Error())
		}
		for _, v := range list {
			relevanceWorkflow[v.ProjectTaskIndex] = append(relevanceWorkflow[v.ProjectTaskIndex], v.WorkflowID)
		}
	}

	for _, v := range taskList {
		status := taskStatus[fmt.Sprintf("%d_%s", v.ProjectID, v.TaskID)]
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

		workflowIDs := relevanceWorkflow[common.BuildWorkflowTaskIndex(v.ProjectID, v.TaskID)]
		if len(workflowIDs) > 0 {
			v.Workflows = workflowIDs
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

func (a *app) DeleteTask(projectID int64, taskID string) (*common.TaskInfo, error) {
	var (
		deleteKey string
		delResp   *clientv3.DeleteResponse
		oldTask   *common.TaskInfo
		ctx       context.Context
		errObj    errors.Error
		err       error
	)

	selector := selection.NewSelector(
		selection.NewRequirement("project_id", selection.Equals, projectID))
	if taskID != "" {
		selector.AddQuery(selection.NewRequirement("task_id", selection.Equals, taskID))
	}
	counter, err := a.store.WorkflowTask().GetTotal(selector)
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "检查任务是否关联workflow失败").WithLog(err.Error())
	}
	if counter > 0 {
		return nil, errors.NewError(http.StatusBadRequest, "该任务已绑定workflow，请从workflow中移除该任务后再删除")
	}

	// build etcd delete key
	deleteKey = common.BuildKey(projectID, taskID)

	ctx, _ = utils.GetContextWithTimeout()
	// save to etcd
	if delResp, err = a.etcd.KV().Delete(ctx, deleteKey, clientv3.WithPrevKV(), clientv3.WithPrefix()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - DeleteTask] etcd client kv delete error:" + err.Error()
		return nil, errObj
	}

	if taskID != "" && len(delResp.PrevKvs) != 0 {
		json.Unmarshal([]byte(delResp.PrevKvs[0].Value), &oldTask)
	}

	return oldTask, nil
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
