package protocol

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
	recipe "github.com/coreos/etcd/contrib/recipes"
)

type workflowRunningManager struct {
	queue map[int64]*recipe.Queue
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
	// todo ack
	ackKey := common.BuildWorkflowAckKey(plan.Task.FlowInfo.WorkflowID, plan.Task.ProjectID, plan.Task.TaskID, plan.TmpID)
	data, _ := json.Marshal(common.AckResponseV1{
		ClientIP: plan.Task.ClientIP,
		Type:     "success",
		TmpID:    plan.TmpID,
	})
	ackData, _ := json.Marshal(common.AckResponse{
		Version: common.ACK_RESPONSE_V1,
		Data:    data,
	})
	if _, err := kv.Put(context.TODO(), ackKey, string(ackData)); err != nil {
		return err
	}
	// todo warning
	// a.Warner.Warning(warning.WarningData{
	// 	Type:      warning.WarningTypeTask,
	// 	TaskName:  taskSchedulePlan.Task.Name,
	// 	ProjectID: taskSchedulePlan.Task.ProjectID,
	// 	Data:      fmt.Sprintf("workflow任务执行响应失败, %s", err.Error()),
	// })
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
	fmt.Println("result", *result)
	var status string
	if result == nil || result.Err != "" {
		// 未执行，发生未知错误
		status = common.TASK_STATUS_FAIL_V2
	} else {
		status = common.TASK_STATUS_DONE_V2
	}

	if result == nil {
		result = &common.TaskExecuteResult{}
	}

	err := d.queue[plan.Task.ProjectID].Enqueue(generateTaskFinishedResultV1(TaskFinishedQueueItemV1{
		ProjectID:  plan.Task.ProjectID,
		TaskID:     plan.Task.TaskID,
		WorkflowID: plan.Task.FlowInfo.WorkflowID,
		TmpID:      plan.Task.TmpID,
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
