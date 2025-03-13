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
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"

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
	// 通过ETCD获取执行该任务的agentIP
	memoryResult, runningInfo, err := a.CheckTaskIsRunning(projectID, taskID)
	if err != nil {
		return err
	}

	if memoryResult.AgentIP != "" && len(runningInfo) == 0 {
		// 没有agent在跑该任务，但是etcd中存在该任务的running key，大概率是上一次任务执行中agent宕机
		wlog.Info("Actively remove the key for the task's running status, as the currently recorded agent does not match the one executing the task.",
			zap.String("status_key", common.BuildTaskStatusKey(projectID, taskID)), zap.String("task_id", taskID), zap.Int64("project_id", projectID))

		taskLog, err := a.store.TaskLog().GetOne(projectID, taskID, memoryResult.TmpID)
		if err != nil && err != gorm.ErrRecordNotFound {
			return errors.NewError(http.StatusInternalServerError, "获取任务日志失败").WithLog(err.Error())
		}

		if taskLog.EndTime == 0 {
			taskLog.WithError = common.TASK_HAS_ERROR

			resultRaw, _ := json.Marshal(common.TaskResultLog{
				Result: "",
				Error:  fmt.Sprintf("Agent(%s) offline", memoryResult.AgentIP),
			})
			taskLog.Result = string(resultRaw)
			taskLog.EndTime = time.Now().Unix()
			tx := a.store.BeginTx()
			if err = a.store.TaskLog().CreateOrUpdateTaskLog(tx, *taskLog); err != nil {
				return errors.NewError(http.StatusInternalServerError, "Agent离线，记录任务日志失败").WithLog(err.Error())
			}

			defer func() {
				if err == nil {
					tx.Commit()
				}
			}()
		}
	}

	if err = a.DelTaskRunningKey(memoryResult.AgentIP, projectID, taskID); err != nil {
		return errors.NewError(http.StatusInternalServerError, "删除任务运行状态key失败").WithLog(err.Error())
	}

	if len(runningInfo) == 0 {
		a.PublishMessage(messageTaskStatusChanged(projectID, taskID, memoryResult.TmpID, common.TASK_STATUS_FAIL_V2))
		return nil
	}

	streams, err := a.FindAgentsV2(a.cfg.Micro.Region, projectID)
	if err != nil {
		return err
	}

	if len(streams) == 0 {
		// 没有存在的agent，则不需要下发kill命令，理论上 runningInfo 不为空一定会有 stream存在，但两步操作中间有略微间隔(极端情况，或异常)
		return nil
	}

	killFunc := func(stream *CenterClient) (bool, error) {
		ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
		defer cancel()

		resp, err := stream.SendEvent(ctx, &cronpb.SendEventRequest{
			Region:    a.cfg.Micro.Region,
			ProjectId: projectID,
			Agent:     stream.addr,
			Event: &cronpb.ServiceEvent{
				Id:        utils.GetStrID(),
				EventTime: time.Now().Unix(),
				Type:      cronpb.EventType_EVENT_KILL_TASK_REQUEST,
				Event: &cronpb.ServiceEvent_KillTaskRequest{
					KillTaskRequest: &cronpb.KillTaskRequest{
						ProjectId: projectID,
						TaskId:    taskID,
					},
				},
			},
		})
		if err != nil {
			return false, err
		}

		if resp.GetKillTaskReply() == nil {
			wlog.Error("get unexcept event response", zap.String("request", cronpb.EventType_EVENT_KILL_TASK_REQUEST.String()),
				zap.String("response", resp.Type.String()), zap.String("raw", resp.String()))
			return false, nil
		}

		return resp.GetKillTaskReply().Result, nil
	}

	for _, v := range streams {
		defer v.Close()
		_, err := killFunc(v)
		if err != nil {
			return errors.NewError(http.StatusInternalServerError, "下发任务强行结束事件失败，AgentIP: "+v.addr).WithLog(err.Error())
		}
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
			clientinfo.Version = "unknown"
		}

		clientinfo.Version += "待升级"

		res = append(res, clientinfo)
	}

	for _, item := range addrs {
		if item.attr.Tags == nil {
			item.attr.Tags = make(map[string]string)
		}
		clientinfo := common.ClientInfo{
			ClientIP: item.addr.Addr,
			Version:  item.attr.Tags["agent-version"],
			Region:   item.attr.Region,
			Weight:   item.attr.NodeWeight,
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
			Id:        utils.GetStrID(),
			Type:      cronpb.EventType_EVENT_REGISTER_REPLY,
			EventTime: time.Now().Unix(),
			Event: &cronpb.ServiceEvent_RegisterReply{
				RegisterReply: &cronpb.Event{
					Type:    common.REMOTE_EVENT_DELETE,
					Version: common.VERSION_TYPE_V1,
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
				Type:      cronpb.EventType_EVENT_REGISTER_REPLY,
				Event: &cronpb.ServiceEvent_RegisterReply{
					RegisterReply: &cronpb.Event{
						Type:      common.REMOTE_EVENT_DELETE,
						Version:   common.VERSION_TYPE_V1,
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
					Version:   common.VERSION_TYPE_V1,
					Value:     saveByte,
					EventTime: time.Now().Unix(),
				},
			},
		},
	})
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "下发任务更新事件失败，可能出现数据不一致，请重试无效后联系系统管理员处理").WithLog(err.Error())
	}

	wlog.Debug("Dispatch save task event", zap.String("task_id", task.TaskID), zap.Int64("project_id", task.ProjectID))

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
