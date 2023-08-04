package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/holdno/gocommons/selection"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/utils"

	"github.com/jinzhu/gorm"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func (a *app) getTaskDetail(kv clientv3.KV, projectID int64, taskID string) (*common.TaskInfo, error) {
	taskKey := common.BuildKey(projectID, taskID)
	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
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
		errObj   errors.Error
		err      error
	)

	// build etcd pre key
	preKey = common.BuildKey(projectID, "")

	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
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
		status, exist := taskStatus[fmt.Sprintf("%d_%s", v.ProjectID, v.TaskID)]
		if exist {
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
	)

	// build etcd pre key
	preKey = common.BuildKey(projectID, "")
	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
	if getResp, err = a.etcd.KV().Get(ctx, preKey,
		clientv3.WithPrefix(), clientv3.WithCountOnly()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetProjectTaskCount] etcd client kv error:" + err.Error()
		return 0, errObj
	}

	return getResp.Count, nil
}

// KillTask 强行结束任务
func (a *app) KillTask(projectID int64, taskID string) error {
	// taskKey := common.BuildKey(projectID, taskID)
	// ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	// defer cancel()
	// resp, err := a.etcd.KV().Get(ctx, taskKey)
	// if err != nil {
	// 	return err
	// }

	err := a.DispatchEvent(&cronpb.SendEventRequest{
		Region:    a.cfg.Micro.Region,
		ProjectId: projectID,
		Event: &cronpb.ServiceEvent{
			Id:        utils.GetStrID(),
			Type:      cronpb.EventType_EVENT_KILL_TASK_REQUEST,
			EventTime: time.Now().Unix(),
			Event: &cronpb.ServiceEvent_KillTaskRequest{
				KillTaskRequest: &cronpb.KillTaskRequest{
					TaskId:    taskID,
					ProjectId: projectID,
				},
			},
		},
	})
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "下发任务强行结束事件失败").WithLog(err.Error())
	}
	return nil
}

func (a *app) CheckProjectWorkerExist(projectID int64, host string) (bool, error) {
	var (
		preKey  string
		err     error
		errObj  errors.Error
		getResp *clientv3.GetResponse
	)

	preKey = common.BuildRegisterKey(projectID, host)
	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
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
		res     []common.ClientInfo
		getResp *clientv3.GetResponse
	)

	addrs, err := a.getAgentAddrs(a.GetConfig().Micro.Region, projectID)
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "获取节点列表失败").WithLog(err.Error())
	}

	preKey := common.BuildRegisterKey(projectID, "")
	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
	if getResp, err = a.etcd.KV().Get(ctx, preKey, clientv3.WithPrefix()); err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "通过etcd获取节点列表失败").WithLog(err.Error())
	}

	for _, kv := range getResp.Kvs {
		var (
			ip         = common.ExtractWorkerIP(projectID, string(kv.Key))
			clientinfo common.ClientInfo
		)

		_ = json.Unmarshal(kv.Value, &clientinfo)
		clientinfo.ClientIP = ip
		if clientinfo.Version == "" {
			clientinfo.Version = "unknow"
		}

		clientinfo.Version += "待升级"

		res = append(res, clientinfo)
	}

	for _, addr := range addrs {
		clientinfo := common.ClientInfo{
			ClientIP: addr.Address(),
			Version:  addr.Attr().Tags["agent-version"],
			Region:   addr.Attr().Region,
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

	taskKey := common.BuildKey(projectID, taskID)
	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
	resp, err := a.etcd.KV().Get(ctx, taskKey)
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "获取任务信息失败").WithLog(err.Error())
	}

	if len(resp.Kvs) == 0 {
		return nil, errors.NewError(http.StatusBadRequest, "任务不存在")
	}

	err = a.DispatchEvent(&cronpb.SendEventRequest{
		Region:    a.cfg.Micro.Region,
		ProjectId: projectID,
		Event: &cronpb.ServiceEvent{
			Type:      cronpb.EventType_EVENT_REGISTER_REPLY,
			EventTime: time.Now().Unix(),
			Event: &cronpb.ServiceEvent_RegisterReply{
				RegisterReply: &cronpb.Event{
					Type:    common.REMOTE_EVENT_DELETE,
					Version: "v1",
					Value:   resp.Kvs[0].Value,
				},
			},
		},
	})
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "下发任务删除事件失败，可能出现数据不一致，请重试无效后联系系统管理员处理").WithLog(err.Error())
	}

	deleteKey = common.BuildKey(projectID, taskID)
	ctx, cancel = context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
	// save to etcd
	if delResp, err = a.etcd.KV().Delete(ctx, deleteKey, clientv3.WithPrevKV(), clientv3.WithPrefix()); err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "删除etcd任务信息失败").WithLog(err.Error())
	}

	if taskID != "" && len(delResp.PrevKvs) != 0 {
		json.Unmarshal([]byte(delResp.PrevKvs[0].Value), &oldTask)
	}

	return oldTask, nil
}

func (a *app) DeleteProjectAllTasks(projectID int64) error {
	var (
		ctx context.Context
		err error
	)

	selector := selection.NewSelector(
		selection.NewRequirement("project_id", selection.Equals, projectID))
	counter, err := a.store.WorkflowTask().GetTotal(selector)
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "检查项目下任务是否关联workflow失败").WithLog(err.Error())
	}
	if counter > 0 {
		return errors.NewError(http.StatusBadRequest, "该项目下已有任务绑定workflow，请从workflow中移除该任务后再删除")
	}

	taskKey := common.BuildTaskPrefixKey(projectID)
	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
	resp, err := a.etcd.KV().Get(ctx, taskKey, clientv3.WithPrefix())
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "获取任务信息失败").WithLog(err.Error())
	}

	if len(resp.Kvs) == 0 {
		return nil
	}

	for _, kv := range resp.Kvs {

		err = a.DispatchEvent(&cronpb.SendEventRequest{
			Region:    a.cfg.Micro.Region,
			ProjectId: projectID,
			Event: &cronpb.ServiceEvent{
				Id:        utils.GetStrID(),
				EventTime: time.Now().Unix(),
				Event: &cronpb.ServiceEvent_RegisterReply{
					RegisterReply: &cronpb.Event{
						Type:      common.REMOTE_EVENT_DELETE,
						Version:   "v1",
						Value:     kv.Value,
						EventTime: time.Now().Unix(),
					},
				},
			},
		})
		if err != nil {
			return errors.NewError(http.StatusInternalServerError, "下发任务删除事件失败").WithLog(err.Error())
		}
	}

	ctx, cancel = context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
	// save to etcd
	if _, err = a.etcd.KV().Delete(ctx, taskKey, clientv3.WithPrevKV(), clientv3.WithPrefix()); err != nil {
		return errors.NewError(http.StatusInternalServerError, "删除etcd任务信息失败").WithLog(err.Error())
	}

	return nil
}

func (a *app) DeleteAll() error {
	var (
		deleteKey string
		errObj    errors.Error
		err       error
	)

	// build etcd delete key
	deleteKey = common.ETCD_PREFIX + "/"
	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
	// save to etcd
	if _, err = a.etcd.KV().Delete(ctx, deleteKey, clientv3.WithPrevKV(), clientv3.WithPrefix()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - DeleteAll] etcd client kv delete error:" + err.Error()
		return errObj
	}

	return nil
}

func (a *app) SaveTask(task *common.TaskInfo, opts ...clientv3.OpOption) (*common.TaskInfo, error) {
	var (
		saveKey  string
		saveByte []byte
		putResp  *clientv3.PutResponse
		oldTask  *common.TaskInfo
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
		return nil, errors.NewError(http.StatusLocked, "任务已被锁定，可能正在执行中，请稍后再试")
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

	err = a.DispatchEvent(&cronpb.SendEventRequest{
		Region:    a.cfg.Micro.Region,
		ProjectId: task.ProjectID,
		Event: &cronpb.ServiceEvent{
			Id:        utils.GetStrID(),
			EventTime: time.Now().Unix(),
			Type:      cronpb.EventType_EVENT_REGISTER_REPLY,
			Event: &cronpb.ServiceEvent_RegisterReply{
				RegisterReply: &cronpb.Event{
					Type:      common.REMOTE_EVENT_PUT,
					Version:   "v1",
					Value:     saveByte,
					EventTime: time.Now().Unix(),
				},
			},
		},
	})
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "下发任务更新事件失败，可能出现数据不一致，请重试无效后联系系统管理员处理").WithLog(err.Error())
	}

	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
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
