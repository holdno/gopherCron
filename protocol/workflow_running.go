package protocol

import (
	"encoding/json"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
	recipe "github.com/coreos/etcd/contrib/recipes"
)

type workflowRunningManager struct {
	queue *recipe.Queue
}

type WorkflowTaskStates struct {
	ProjectID       int64                        `json:"project_id"`
	TaskID          string                       `json:"task_id"`
	WorkflowID      int64                        `json:"workflow_id"`
	CurrentStatus   string                       `json:"current_status"`
	ScheduleCount   int                          `json:"schedule_count"`
	ScheduleRecords []WorkflowTaskScheduleRecord `json:"schedule_records"`
}

type WorkflowTaskScheduleRecord struct {
	TmpID     string
	Result    string
	Status    string
	StartTime int64
	EndTime   int64
}

func (d *workflowRunningManager) SetTaskRunning(kv clientv3.KV, plan *common.TaskSchedulePlan) error {
	key := common.BuildWorkflowTaskStatusKey(plan.Task.FlowInfo.WorkflowID, plan.Task.ProjectID, plan.Task.TaskID)
	ctx, _ := utils.GetContextWithTimeout()
	resp, err := kv.Get(ctx, key)
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - workflowRunningManager - SetTaskRunning] etcd client kv get error:" + err.Error()
		return errObj
	}

	var states []byte
	if resp.Count == 0 {
		states, _ = json.Marshal(WorkflowTaskStates{
			ProjectID:     plan.Task.ProjectID,
			TaskID:        plan.Task.TaskID,
			WorkflowID:    plan.Task.FlowInfo.WorkflowID,
			CurrentStatus: common.TASK_STATUS_RUNNING_V2,
			ScheduleCount: 1,
			ScheduleRecords: []WorkflowTaskScheduleRecord{{
				TmpID:     plan.TmpID,
				Status:    common.TASK_STATUS_RUNNING_V2,
				StartTime: time.Now().Unix(),
			}},
		})
	} else {
		var workflowTaskStates WorkflowTaskStates
		if err = json.Unmarshal(resp.Kvs[0].Value, &workflowTaskStates); err != nil {
			errObj := errors.ErrInternalError
			errObj.Log = "[Etcd - workflowRunningManager - SetTaskRunning] json unmarshal workflow task running result error:" + err.Error()
			return nil
		}

		workflowTaskStates.CurrentStatus = common.TASK_STATUS_RUNNING_V2
		workflowTaskStates.ScheduleCount += 1
		workflowTaskStates.ScheduleRecords = append(workflowTaskStates.ScheduleRecords, WorkflowTaskScheduleRecord{
			TmpID:     plan.TmpID,
			Status:    common.TASK_STATUS_RUNNING_V2,
			StartTime: time.Now().Unix(),
		})

		states, _ = json.Marshal(workflowTaskStates)
	}

	if _, err = kv.Put(ctx, key, string(states)); err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - workflowRunningManager - SetTaskRunning] etcd client kv put error:" + err.Error()
		return errObj
	}
	return nil
}

func GetWorkflowTaskStates(kv clientv3.KV, key string) (*WorkflowTaskStates, error) {
	ctx, _ := utils.GetContextWithTimeout()
	resp, err := kv.Get(ctx, key)
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - SetTaskNotRunning] etcd client kv get error:" + err.Error()
		return nil, errObj
	}
	if resp.Count == 0 {
		// 没有运行过的任务
		return nil, nil
	}

	var workflowTaskStates WorkflowTaskStates
	if err = json.Unmarshal(resp.Kvs[0].Value, &workflowTaskStates); err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - workflowRunningManager - SetTaskRunning] json unmarshal workflow task running result error:" + err.Error()
		return nil, nil
	}

	return &workflowTaskStates, nil
}

func (d *workflowRunningManager) SetTaskNotRunning(kv clientv3.KV, plan *common.TaskSchedulePlan, result *common.TaskExecuteResult) error {
	var status string
	if result == nil || result.Err != "" {
		// 未执行，发生未知错误
		status = common.TASK_STATUS_FAIL_V2
	} else {
		status = common.TASK_STATUS_DONE_V2
	}

	key := common.BuildWorkflowTaskStatusKey(plan.Task.FlowInfo.WorkflowID, plan.Task.ProjectID, plan.Task.TaskID)
	workflowTaskStates, err := GetWorkflowTaskStates(kv, key)
	if err != nil {
		return err
	}

	workflowTaskStates.CurrentStatus = status
	for _, v := range workflowTaskStates.ScheduleRecords {
		if v.TmpID == plan.TmpID {
			v.Status = status
			v.EndTime = result.EndTime.Unix()
			break
		}
	}

	ctx, _ := utils.GetContextWithTimeout()
	states, _ := json.Marshal(workflowTaskStates)
	if _, err = kv.Put(ctx, key, string(states)); err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - SetTaskNotRunning] etcd client kv put error:" + err.Error()
		return errObj
	}

	err = d.queue.Enqueue(generateTaskFinishedResultV1(TaskFinishedQueueItemV1{
		ProjectID:  plan.Task.ProjectID,
		TaskID:     plan.Task.TaskID,
		WorkflowID: plan.Task.FlowInfo.WorkflowID,
		Status:     status,
		StartTime:  result.StartTime.Unix(),
		EndTime:    result.EndTime.Unix(),
		TaskType:   common.WorkflowPlan,
	}))
	return err
}

func generateTaskFinishedResultV1(data TaskFinishedQueueItemV1) string {
	queueData, _ := json.Marshal(data)
	result, _ := json.Marshal(TaskFinishedQueueContent{
		Version: QueueItemV1,
		Data:    queueData,
	})
	return string(result)
}
