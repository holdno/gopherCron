package protocol

import (
	"encoding/json"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
)

type workflowRunningManager struct{}

type WorkflowTaskRunningResult struct {
	ProjectID       int64                        `json:"project_id"`
	TaskID          string                       `json:"task_id"`
	WorkflowID      string                       `json:"workflow_id"`
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
	key := common.BuildWorkflowTaskStatusKey(plan.Task.ProjectID, plan.Task.FlowInfo.WorkflowID, plan.Task.TaskID)
	ctx, _ := utils.GetContextWithTimeout()
	resp, err := kv.Get(ctx, key)
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - workflowRunningManager - SetTaskRunning] etcd client kv get error:" + err.Error()
		return errObj
	}
	if resp.Count == 0 {
		runningInfo, _ := json.Marshal(WorkflowTaskRunningResult{
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
		_, err := kv.Put(ctx, key, string(runningInfo))
		if err != nil {
			errObj := errors.ErrInternalError
			errObj.Log = "[Etcd - workflowRunningManager - SetTaskRunning] etcd client kv put error:" + err.Error()
			return errObj
		}
		return nil
	}

	var workflowTaskRunningResult WorkflowTaskRunningResult
	if err = json.Unmarshal(resp.Kvs[0].Value, &workflowTaskRunningResult); err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - workflowRunningManager - SetTaskRunning] json unmarshal workflow task running result error:" + err.Error()
		return nil
	}

	workflowTaskRunningResult.CurrentStatus = common.TASK_STATUS_RUNNING_V2
	workflowTaskRunningResult.ScheduleCount += 1
	workflowTaskRunningResult.ScheduleRecords = append(workflowTaskRunningResult.ScheduleRecords, WorkflowTaskScheduleRecord{
		TmpID:     plan.TmpID,
		Status:    common.TASK_STATUS_RUNNING_V2,
		StartTime: time.Now().Unix(),
	})

	runningInfo, _ := json.Marshal(workflowTaskRunningResult)
	if _, err = kv.Put(ctx, key, string(runningInfo)); err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - workflowRunningManager - SetTaskRunning] etcd client kv put error:" + err.Error()
		return errObj
	}
	return nil
}
func (d *workflowRunningManager) SetTaskNotRunning(kv clientv3.KV, plan *common.TaskSchedulePlan, result *common.TaskExecuteResult) error {
	var status string
	if result == nil || result.Err != "" {
		// 未执行，发生未知错误
		status = common.TASK_STATUS_FAIL_V2
	} else {
		status = common.TASK_STATUS_DONE_V2
	}
	ctx, _ := utils.GetContextWithTimeout()
	key := common.BuildWorkflowTaskStatusKey(plan.Task.ProjectID, plan.Task.FlowInfo.WorkflowID, plan.Task.TaskID)
	resp, err := kv.Get(ctx, key)
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - SetTaskNotRunning] etcd client kv get error:" + err.Error()
		return errObj
	}
	if resp.Count == 0 {
		// What happen?
		return nil
	}

	var workflowTaskRunningResult WorkflowTaskRunningResult
	if err = json.Unmarshal(resp.Kvs[0].Value, &workflowTaskRunningResult); err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - workflowRunningManager - SetTaskRunning] json unmarshal workflow task running result error:" + err.Error()
		return nil
	}

	workflowTaskRunningResult.CurrentStatus = status

	runningInfo, _ := json.Marshal(workflowTaskRunningResult)
	if _, err = kv.Put(ctx, key, string(runningInfo)); err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - SetTaskNotRunning] etcd client kv put error:" + err.Error()
		return errObj
	}
	return nil
}
