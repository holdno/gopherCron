package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	recipe "go.etcd.io/etcd/client/v3/experimental/recipes"
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

func (d *workflowRunningManager) SetTaskRunning(s *concurrency.Session, execInfo *common.TaskExecutingInfo) error {
	if time.Now().Sub(time.Unix(execInfo.Task.CreateTime, 0)).Seconds() > 5 {
		return ErrScheduleTimeout
	}
	// todo ack
	ackKey := common.BuildWorkflowAckKey(execInfo.Task.FlowInfo.WorkflowID, execInfo.Task.ProjectID, execInfo.Task.TaskID, execInfo.TmpID)
	data, _ := json.Marshal(common.AckResponseV1{
		ClientIP: execInfo.Task.ClientIP,
		Type:     "success",
		TmpID:    execInfo.TmpID,
	})
	ackData, _ := json.Marshal(common.AckResponse{
		Version: common.ACK_RESPONSE_V1,
		Data:    data,
	})

	if _, err := s.Client().Put(context.TODO(), ackKey, string(ackData), clientv3.WithLease(s.Lease())); err != nil {
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

var (
	ErrScheduleTimeout = fmt.Errorf("task schedule timeout")
)

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

func (d *workflowRunningManager) SetTaskNotRunning(s *concurrency.Session, execInfo *common.TaskExecutingInfo, result *common.TaskExecuteResult) error {
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

	err := d.queue[execInfo.Task.ProjectID].Enqueue(generateTaskFinishedResultV1(TaskFinishedQueueItemV1{
		ProjectID:  execInfo.Task.ProjectID,
		TaskID:     execInfo.Task.TaskID,
		WorkflowID: execInfo.Task.FlowInfo.WorkflowID,
		TmpID:      execInfo.Task.TmpID,
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
