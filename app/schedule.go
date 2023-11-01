package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/holdno/gocommons/selection"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/utils"
	"github.com/spacegrower/watermelon/infra/wlog"
	"github.com/spacegrower/watermelon/pkg/safe"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

func (a *workflowRunner) scheduleTask(taskInfo *common.TaskInfo) error {
	if a.InProcess(taskInfo.TaskID) {
		return nil
	}
	defer a.ProcessDone(taskInfo.TaskID)
	plan := a.GetPlan(taskInfo.FlowInfo.WorkflowID)
	if plan == nil {
		return nil
	}
	plan.locker.Lock()
	defer func() {
		plan.locker.Unlock()
	}()

	running, err := plan.IsRunning()
	if err != nil {
		return err
	}
	if !running {
		return nil
	}

	cli := a.etcd
	taskInfo.TmpID = utils.GetStrID()
	taskStates, err := getWorkflowTaskStates(cli.KV, common.BuildWorkflowTaskStatusKey(taskInfo.FlowInfo.WorkflowID, taskInfo.ProjectID, taskInfo.TaskID))
	if err != nil {
		return err
	}

	if taskStates != nil &&
		taskStates.CurrentStatus != common.TASK_STATUS_NOT_RUNNING_V2 &&
		taskStates.CurrentStatus != common.TASK_STATUS_STARTING_V2 {
		return nil
	}

	_, err = concurrency.NewSTM(cli, func(s concurrency.STM) error {
		if err = setWorkflowTaskStarting(s, taskInfo); err != nil {
			return err
		}
		return nil
	})

	ctx, cancel := utils.GetContextWithTimeout()
	defer cancel()
	value, _ := json.Marshal(taskInfo)

	stream, err := a.app.GetAgentStream(a.app.GetConfig().Micro.Region, taskInfo.ProjectID)
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, fmt.Sprintf("连接agent stream失败, project_id: %d", taskInfo.ProjectID)).WithLog(err.Error())
	}
	if stream != nil {
		defer stream.Close()
		_, err := stream.SendEvent(ctx, &cronpb.SendEventRequest{
			Region:    a.app.GetConfig().Micro.Region,
			ProjectId: taskInfo.ProjectID,
			Agent:     stream.addr,
			Event: &cronpb.ServiceEvent{
				Id:        utils.GetStrID(),
				EventTime: time.Now().Unix(),
				Type:      cronpb.EventType_EVENT_SCHEDULE_REQUEST,
				Event: &cronpb.ServiceEvent_ScheduleRequest{
					ScheduleRequest: &cronpb.ScheduleRequest{
						Event: &cronpb.Event{
							Type:      common.REMOTE_EVENT_WORKFLOW_SCHEDULE,
							Version:   common.VERSION_TYPE_V1,
							Value:     value,
							EventTime: time.Now().Unix(),
						},
					},
				},
			},
		})
		if err != nil {
			a.scheduleAgentMetric(fmt.Sprintf("%d_%s", taskInfo.ProjectID, taskInfo.TaskID), fmt.Sprint(err != nil))
			if err != nil {
				wlog.With(zap.Any("fields", map[string]interface{}{
					"workflow_id": taskInfo.FlowInfo.WorkflowID,
					"project_id":  taskInfo.ProjectID,
					"task_id":     taskInfo.TaskID,
					"tmp_id":      taskInfo.TmpID,
					"error":       err.Error(),
				})).Error("schedule workflow task error")
				return errors.NewError(http.StatusInternalServerError,
					fmt.Sprintf("stream 调度任务失败, project_id: %d, task_id: %s", taskInfo.ProjectID, taskInfo.TaskID)).WithLog(err.Error())
			}
		}
	} else {
		client, err := a.app.GetAgentClient(a.app.GetConfig().Micro.Region, taskInfo.ProjectID)
		if err != nil {
			return errors.NewError(http.StatusInternalServerError, fmt.Sprintf("连接agent客户端失败, project_id: %d", taskInfo.ProjectID)).WithLog(err.Error())
		}

		defer client.Close()

		_, err = client.Schedule(ctx, &cronpb.ScheduleRequest{
			Event: &cronpb.Event{
				Type:      common.REMOTE_EVENT_WORKFLOW_SCHEDULE,
				Version:   common.VERSION_TYPE_V1,
				Value:     value,
				EventTime: time.Now().Unix(),
			},
		})
		// addr, taskid, witherr, errdesc
		a.scheduleAgentMetric(fmt.Sprintf("%d_%s", taskInfo.ProjectID, taskInfo.TaskID), fmt.Sprint(err != nil))
		if err != nil {
			wlog.With(zap.Any("fields", map[string]interface{}{
				"workflow_id": taskInfo.FlowInfo.WorkflowID,
				"project_id":  taskInfo.ProjectID,
				"task_id":     taskInfo.TaskID,
				"tmp_id":      taskInfo.TmpID,
				"error":       err.Error(),
			})).Error("schedule workflow task error")
			return errors.NewError(http.StatusInternalServerError,
				fmt.Sprintf("调度任务失败, project_id: %d, task_id: %s", taskInfo.ProjectID, taskInfo.TaskID)).WithLog(err.Error())
		}
	}

	a.app.PublishMessage(messageWorkflowTaskStatusChanged(
		taskInfo.FlowInfo.WorkflowID,
		taskInfo.ProjectID,
		taskInfo.TaskID,
		common.TASK_STATUS_STARTING_V2))

	// err = waitingAck(cli, common.BuildWorkflowAckKey(taskInfo.FlowInfo.WorkflowID, taskInfo.ProjectID, taskInfo.TaskID, taskInfo.TmpID),
	// 	func() error {
	// 		// 设置任务状态为启动中
	// 		// 调度任务至agent
	// 		_, err = concurrency.NewSTM(cli, func(s concurrency.STM) error {
	// 			if err = setWorkflowTaskStarting(s, taskInfo); err != nil {
	// 				return err
	// 			}
	// 			ctx, _ := utils.GetContextWithTimeout()
	// 			// make lease to notify worker
	// 			// 创建一个租约 让其稍后过期并自动删除
	// 			leaseGrantResp, err := cli.Lease.Grant(ctx, 1)
	// 			if err != nil {
	// 				errObj := errors.ErrInternalError
	// 				errObj.Log = "[putSchedule] lease grant error:" + err.Error()
	// 				return errObj
	// 			}
	// 			taskInfo.CreateTime = time.Now().Unix()
	// 			return putSchedule(s, leaseGrantResp.ID, taskInfo)
	// 		})

	// 		if err == nil {
	// 			a.app.PublishMessage(messageWorkflowTaskStatusChanged(
	// 				taskInfo.FlowInfo.WorkflowID,
	// 				taskInfo.ProjectID,
	// 				taskInfo.TaskID,
	// 				common.TASK_STATUS_STARTING_V2))
	// 		}

	// 		return err
	// 	},
	// 	func(e *clientv3.Event) bool {
	// 		return a.getAckForTaskRunning(WorkflowRunningTaskInfo{
	// 			TaskID:     taskInfo.TaskID,
	// 			TaskName:   taskInfo.Name,
	// 			ProjectID:  taskInfo.ProjectID,
	// 			WorkflowID: taskInfo.FlowInfo.WorkflowID,
	// 			TmpID:      taskInfo.TmpID,
	// 		}, e.Kv.Key, e.Kv.Value)
	// 	})
	// if err != nil {
	// 	a.app.Log().WithFields(logrus.Fields{
	// 		"workflow_id": taskInfo.FlowInfo.WorkflowID,
	// 		"project_id":  taskInfo.ProjectID,
	// 		"task_id":     taskInfo.TaskID,
	// 		"tmp_id":      taskInfo.TmpID,
	// 		"error":       err.Error(),
	// 	}).Error("waiting workflow task ack error")
	// 	return err
	// }
	return nil
}

type WorkflowRunningTaskInfo struct {
	WorkflowID int64
	TmpID      string
	TaskID     string
	TaskName   string
	ProjectID  int64
	AgentIP    string
}

// func (a *workflowRunner) getAckForTaskRunning(taskInfo WorkflowRunningTaskInfo, k, v []byte) bool {
// 	var ack common.AckResponse
// 	if err := json.Unmarshal(v, &ack); err != nil {
// 		wlog.With(zap.Any("fields", map[string]interface{}{
// 			"workflow_id": taskInfo.WorkflowID,
// 			"project_id":  taskInfo.ProjectID,
// 			"task_name":   taskInfo.TaskName,
// 			"task_id":     taskInfo.TaskID,
// 			"tmp_id":      taskInfo.TmpID,
// 			"error":       err.Error(),
// 		})).Error("failed to json.Unmarshal ack response")
// 		return false
// 	}

// 	switch ack.Version {
// 	case common.ACK_RESPONSE_V1:
// 		var v1 common.AckResponseV1
// 		if err := json.Unmarshal(ack.Data, &v1); err != nil {
// 			wlog.With(zap.Any("fields", map[string]interface{}{
// 				"workflow_id": taskInfo.WorkflowID,
// 				"project_id":  taskInfo.ProjectID,
// 				"task_name":   taskInfo.TaskName,
// 				"task_id":     taskInfo.TaskID,
// 				"tmp_id":      taskInfo.TmpID,
// 				"error":       err.Error(),
// 			})).Error("failed to json.Unmarshal ack data")
// 			return false
// 		}
// 	default:
// 		wlog.With(zap.Any("fields", map[string]interface{}{
// 			"workflow_id": taskInfo.WorkflowID,
// 			"project_id":  taskInfo.ProjectID,
// 			"task_name":   taskInfo.TaskName,
// 			"task_id":     taskInfo.TaskID,
// 			"tmp_id":      taskInfo.TmpID,
// 			"ack_version": ack.Version,
// 		})).Error("unknown ack response version")
// 		return false
// 	}

// 	_, err := concurrency.NewSTM(a.etcd, func(s concurrency.STM) error {
// 		if err := setWorkflowTaskRunning(s, taskInfo); err != nil {
// 			return err
// 		}
// 		// 删除 ack key
// 		// 2022-09-07: 获取到ack状态后不删除，用ack状态标识agent上的任务正在运行中，当做一个运行时状态，agent任务结束后会通过lease的结束自动清理该key
// 		// s.Del(string(k))
// 		return nil
// 	})
// 	if err != nil {
// 		wlog.With(zap.Any("fields", map[string]interface{}{
// 			"workflow_id": taskInfo.WorkflowID,
// 			"project_id":  taskInfo.ProjectID,
// 			"task_name":   taskInfo.TaskName,
// 			"task_id":     taskInfo.TaskID,
// 			"tmp_id":      taskInfo.TmpID,
// 			"error":       err.Error(),
// 		})).Error("failed to delete ack key")
// 		return false
// 	}

// a.app.PublishMessage(messageWorkflowTaskStatusChanged(
// 	taskInfo.WorkflowID,
// 	taskInfo.ProjectID,
// 	taskInfo.TaskID,
// 	common.TASK_STATUS_RUNNING_V2))
// 	return true
// }

type WorkflowTaskScheduleRecord struct {
	TmpID     string `json:"tmp_id"`
	Result    string `json:"result"`
	Status    string `json:"status"`
	EventTime int64  `json:"event_time"`
	AgentIP   string `json:"agent_ip"`
}

type WorkflowTaskStates struct {
	ProjectID       int64                         `json:"project_id"`
	TaskID          string                        `json:"task_id"`
	WorkflowID      int64                         `json:"workflow_id"`
	CurrentStatus   string                        `json:"current_status"`
	ScheduleCount   int                           `json:"schedule_count"`
	Command         string                        `json:"command"`
	StartTime       int64                         `json:"start_time"`
	EndTime         int64                         `json:"end_time"`
	ScheduleRecords []*WorkflowTaskScheduleRecord `json:"schedule_records"`
}

func (s *WorkflowTaskStates) GetLatestScheduleRecord() *WorkflowTaskScheduleRecord {
	l := len(s.ScheduleRecords)
	if l == 0 {
		return nil
	}
	return s.ScheduleRecords[l-1]
}

// finished 不一定是成功
func setWorkFlowTaskFinished(kv concurrency.STM, agentIP string, result *common.TaskFinishedV2) (bool, error) {
	key := common.BuildWorkflowTaskStatusKey(result.WorkflowID, result.ProjectID, result.TaskID)
	states := kv.Get(key)
	planFinished := false

	if states == "" {
		// workflow finished
		return false, nil
	}

	var workflowTaskStates WorkflowTaskStates
	if err := json.Unmarshal([]byte(states), &workflowTaskStates); err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[setWorkflowTaskRunning] json unmarshal workflow task running result error:" + err.Error()
		return false, errObj
	}

	if workflowTaskStates.CurrentStatus == common.TASK_STATUS_DONE_V2 ||
		workflowTaskStates.CurrentStatus == common.TASK_STATUS_FAIL_V2 {
		return false, nil
	}

	endTime := time.Now().Unix()
	workflowTaskStates.ScheduleRecords = append(workflowTaskStates.ScheduleRecords, &WorkflowTaskScheduleRecord{
		TmpID:     result.TmpID,
		Status:    result.Status,
		Result:    result.Result,
		EventTime: endTime,
		AgentIP:   agentIP,
	})

	if result.Status == common.TASK_STATUS_FAIL_V2 {
		if workflowTaskStates.ScheduleCount >= common.WORKFLOW_SCHEDULE_LIMIT {
			workflowTaskStates.CurrentStatus = common.TASK_STATUS_FAIL_V2
			workflowTaskStates.EndTime = endTime
			planFinished = true
		} else {
			workflowTaskStates.CurrentStatus = common.TASK_STATUS_NOT_RUNNING_V2
		}
	} else if result.Status == common.TASK_STATUS_DONE_V2 {
		workflowTaskStates.CurrentStatus = common.TASK_STATUS_DONE_V2
		workflowTaskStates.EndTime = endTime
	}

	newStates, _ := json.Marshal(workflowTaskStates)
	kv.Put(key, string(newStates))
	return planFinished, nil
}

func setWorkflowTaskNotRunning(kv concurrency.STM, taskInfo WorkflowRunningTaskInfo, reason string) error {
	key := common.BuildWorkflowTaskStatusKey(taskInfo.WorkflowID, taskInfo.ProjectID, taskInfo.TaskID)
	states := kv.Get(key)

	if states == "" {
		// what happen?
		return nil
	}
	var workflowTaskStates WorkflowTaskStates
	if err := json.Unmarshal([]byte(states), &workflowTaskStates); err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[setWorkflowTaskRunning] json unmarshal workflow task running result error:" + err.Error()
		return nil
	}

	workflowTaskStates.CurrentStatus = common.TASK_STATUS_NOT_RUNNING_V2
	workflowTaskStates.ScheduleRecords = append(workflowTaskStates.ScheduleRecords, &WorkflowTaskScheduleRecord{
		TmpID:     taskInfo.TmpID,
		Status:    common.TASK_STATUS_NOT_RUNNING_V2,
		Result:    reason,
		EventTime: time.Now().Unix(),
	})

	newStates, _ := json.Marshal(workflowTaskStates)
	kv.Put(key, string(newStates))
	return nil
}

func setWorkflowTaskRunning(kv concurrency.STM, taskInfo WorkflowRunningTaskInfo) error {
	key := common.BuildWorkflowTaskStatusKey(taskInfo.WorkflowID, taskInfo.ProjectID, taskInfo.TaskID)
	states := kv.Get(key)

	if states == "" {
		// what happen?
		return errors.NewError(http.StatusInternalServerError, "unknown")
	}
	var workflowTaskStates WorkflowTaskStates
	if err := json.Unmarshal([]byte(states), &workflowTaskStates); err != nil {
		return errors.NewError(http.StatusInternalServerError, "解析workflow运行状态失败").WithLog(err.Error())
	}

	workflowTaskStates.CurrentStatus = common.TASK_STATUS_RUNNING_V2
	workflowTaskStates.ScheduleRecords = append(workflowTaskStates.ScheduleRecords, &WorkflowTaskScheduleRecord{
		TmpID:     taskInfo.TmpID,
		AgentIP:   taskInfo.AgentIP,
		Status:    common.TASK_STATUS_RUNNING_V2,
		EventTime: time.Now().Unix(),
	})

	newStates, _ := json.Marshal(workflowTaskStates)
	kv.Put(key, string(newStates))
	return nil
}

func setWorkflowTaskStarting(kv concurrency.STM, taskInfo *common.TaskInfo) error {

	key := common.BuildWorkflowTaskStatusKey(taskInfo.FlowInfo.WorkflowID, taskInfo.ProjectID, taskInfo.TaskID)
	value := kv.Get(key)
	var states []byte
	if value == "" {
		states, _ = json.Marshal(WorkflowTaskStates{
			ProjectID:     taskInfo.ProjectID,
			TaskID:        taskInfo.TaskID,
			WorkflowID:    taskInfo.FlowInfo.WorkflowID,
			CurrentStatus: common.TASK_STATUS_STARTING_V2,
			Command:       taskInfo.Command,
			ScheduleCount: 1,
			StartTime:     time.Now().Unix(),
			ScheduleRecords: []*WorkflowTaskScheduleRecord{{
				TmpID:     taskInfo.TmpID,
				Status:    common.TASK_STATUS_STARTING_V2,
				EventTime: time.Now().Unix(),
			}},
		})
	} else {
		var workflowTaskStates WorkflowTaskStates
		if err := json.Unmarshal([]byte(value), &workflowTaskStates); err != nil {
			errObj := errors.ErrInternalError
			errObj.Log = "[setWorkflowTaskStarting] json unmarshal workflow task running result error:" + err.Error()
			return errObj
		}

		workflowTaskStates.CurrentStatus = common.TASK_STATUS_STARTING_V2
		workflowTaskStates.ScheduleCount += 1
		workflowTaskStates.ScheduleRecords = append(workflowTaskStates.ScheduleRecords, &WorkflowTaskScheduleRecord{
			TmpID:     taskInfo.TmpID,
			Status:    common.TASK_STATUS_STARTING_V2,
			EventTime: time.Now().Unix(),
		})

		states, _ = json.Marshal(workflowTaskStates)
	}

	kv.Put(key, string(states))
	return nil
}

func putSchedule(kv concurrency.STM, leaseID clientv3.LeaseID, taskInfo *common.TaskInfo) error {
	scheduleKey := common.BuildWorkflowSchedulerKey(taskInfo.FlowInfo.WorkflowID, taskInfo.ProjectID, taskInfo.TaskID)
	// task to json
	saveByte, err := json.Marshal(taskInfo)
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "[putSchedule] json.mashal task error").WithLog(err.Error())
	}

	// save to etcd
	kv.Put(scheduleKey, string(saveByte), clientv3.WithLease(leaseID))
	return nil
}

func getWorkflowTaskStateWithSTM(kv concurrency.STM, key string) (*WorkflowTaskStates, error) {
	resp := kv.Get(key)
	if resp == "" {
		// 没有运行过的任务
		return nil, nil
	}

	var workflowTaskStates WorkflowTaskStates
	if err := json.Unmarshal([]byte(resp), &workflowTaskStates); err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - workflowRunningManager - SetTaskRunning] json unmarshal workflow task running result error:" + err.Error()
		return nil, errObj
	}
	return &workflowTaskStates, nil
}

func getWorkflowAllTaskStates(kv clientv3.KV, workflowID int64) ([]*WorkflowTaskStates, error) {
	prefix := common.BuildWorkflowTaskStatusKeyPrefix(workflowID)
	ctx, _ := utils.GetContextWithTimeout()
	resp, err := kv.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	var list []*WorkflowTaskStates
	for _, data := range resp.Kvs {
		var item WorkflowTaskStates
		_ = json.Unmarshal(data.Value, &item)
		list = append(list, &item)
	}
	return list, nil
}

func getWorkflowTasksStates(kv clientv3.KV, prefix string) ([]*WorkflowTaskStates, error) {
	ctx, _ := utils.GetContextWithTimeout()
	resp, err := kv.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - getWorkflowTasksStates] etcd client kv get error:" + err.Error()
		return nil, errObj
	}
	if resp.Count == 0 {
		// 没有运行过的任务
		return nil, nil
	}

	var list []*WorkflowTaskStates
	for _, v := range resp.Kvs {
		var workflowTaskStates WorkflowTaskStates
		if err = json.Unmarshal(v.Value, &workflowTaskStates); err != nil {
			errObj := errors.ErrInternalError
			errObj.Log = "[Etcd - getWorkflowTasksStates] json unmarshal workflow task running result error:" + err.Error()
			return nil, errObj
		}

		list = append(list, &workflowTaskStates)
	}

	return list, nil
}

func getWorkflowTaskStates(kv clientv3.KV, key string) (*WorkflowTaskStates, error) {
	ctx, _ := utils.GetContextWithTimeout()
	resp, err := kv.Get(ctx, key)
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - getWorkflowTasksStates] etcd client kv get error:" + err.Error()
		return nil, errObj
	}
	if resp.Count == 0 {
		// 没有运行过的任务
		return nil, nil
	}

	var workflowTaskStates WorkflowTaskStates
	if err = json.Unmarshal(resp.Kvs[0].Value, &workflowTaskStates); err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - getWorkflowTasksStates] json unmarshal workflow task running result error:" + err.Error()
		return nil, nil
	}

	return &workflowTaskStates, nil
}

type PlanState struct {
	WorkflowID    int64                 `json:"workflow_id"`
	StartTime     int64                 `json:"start_time"`
	EndTime       int64                 `json:"end_time"`
	Status        string                `json:"status"`
	Reason        string                `json:"reason"`
	LatestTryTime int64                 `json:"latest_try_time"`
	Records       []*WorkflowTaskStates `json:"records,omitempty"`
}

func getWorkflowPlanState(kv clientv3.KV, workflowID int64) (*PlanState, error) {
	ctx, _ := utils.GetContextWithTimeout()
	resp, err := kv.Get(ctx, common.BuildWorkflowPlanKey(workflowID))
	if err != nil {
		return nil, err
	}
	if resp.Count == 0 {
		return nil, nil
	}
	var state PlanState
	if err = json.Unmarshal(resp.Kvs[0].Value, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

func setWorkflowPlanRunning(cli *clientv3.Client, workflowID int64) (*PlanState, error) {
	var planState PlanState
	_, err := concurrency.NewSTM(cli, func(s concurrency.STM) error {
		planKey := common.BuildWorkflowPlanKey(workflowID)
		state := s.Get(planKey)

		if state == "" {
			planState = PlanState{
				WorkflowID:    workflowID,
				StartTime:     time.Now().Unix(),
				Status:        common.TASK_STATUS_RUNNING_V2,
				LatestTryTime: time.Now().Unix(),
			}
			// workflow 开始前 清理一次key
			// if err := clearWorkflowKeys(cli.KV, workflowID); err != nil {
			// 	return err
			// }
		} else {
			if err := json.Unmarshal([]byte(state), &planState); err != nil {
				return err
			}
			planState.LatestTryTime = time.Now().Unix()
			planState.Status = common.TASK_STATUS_RUNNING_V2
		}

		newState, _ := json.Marshal(planState)
		s.Put(planKey, string(newState))
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &planState, nil
}

func clearWorkflowKeys(kv clientv3.KV, workflowID int64) error {
	// 删除workflow相关的key
	delKeys := []string{
		common.BuildWorkflowPlanKey(workflowID),
		common.BuildWorkflowTaskStatusKeyPrefix(workflowID),
	}

	for _, v := range delKeys {
		ctx, _ := utils.GetContextWithTimeout()
		if _, err := kv.Delete(ctx, v, clientv3.WithPrefix()); err != nil {
			return err
		}
	}
	return nil
}

// TemporarySchedulerTask 临时调度任务
func (a *app) TemporarySchedulerTask(user *common.User, task *common.TaskInfo) error {
	var (
		err error
	)

	// reset task create time as schedule time
	task.CreateTime = time.Now().Unix()

	if task.TmpID == "" {
		task.TmpID = utils.GetStrID()
	}
	a.PublishMessage(messageTaskStatusChanged(
		task.ProjectID,
		task.TaskID,
		task.TmpID,
		common.TASK_STATUS_STARTING_V2))

	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()

	if user == nil {
		user = &common.User{}
	}
	value, _ := json.Marshal(common.TaskWithOperator{
		TaskInfo: task,
		UserID:   user.ID,
		UserName: user.Name,
	})

	stream, err := a.GetAgentStream(a.GetConfig().Micro.Region, task.ProjectID)
	if err != nil {
		return err
	}
	if stream != nil {
		defer stream.Close()
		_, err := stream.SendEvent(ctx, &cronpb.SendEventRequest{
			Region:    a.GetConfig().Micro.Region,
			ProjectId: task.ProjectID,
			Agent:     stream.addr,
			Event: &cronpb.ServiceEvent{
				Id:        utils.GetStrID(),
				Type:      cronpb.EventType_EVENT_SCHEDULE_REQUEST,
				EventTime: time.Now().Unix(),
				Event: &cronpb.ServiceEvent_ScheduleRequest{
					ScheduleRequest: &cronpb.ScheduleRequest{
						Event: &cronpb.Event{
							Type:      common.REMOTE_EVENT_TMP_SCHEDULE,
							Version:   common.VERSION_TYPE_V1,
							Value:     value,
							EventTime: time.Now().Unix(),
						},
					},
				},
			},
		})
		if err != nil {
			grpcErr, _ := status.FromError(err)
			switch grpcErr.Code() {
			case codes.AlreadyExists:
				return errors.NewError(http.StatusInternalServerError,
					"stream 调度任务失败, 任务运行中...").WithLog(err.Error())
			default:
				return errors.NewError(http.StatusInternalServerError,
					fmt.Sprintf("stream 调度任务失败, project_id: %d, task_id: %s", task.ProjectID, task.TaskID)).WithLog(err.Error())
			}
		}
	} else {
		client, err := a.GetAgentClient(a.GetConfig().Micro.Region, task.ProjectID)
		if err != nil {
			return err
		}
		defer client.Close()

		_, err = client.Schedule(ctx, &cronpb.ScheduleRequest{
			Event: &cronpb.Event{
				Type:      common.REMOTE_EVENT_TMP_SCHEDULE,
				Version:   common.VERSION_TYPE_V1,
				Value:     value,
				EventTime: time.Now().Unix(),
			},
		})
		if err != nil {
			grpcErr, _ := status.FromError(err)
			switch grpcErr.Code() {
			case codes.AlreadyExists:
				return errors.NewError(http.StatusInternalServerError,
					"调度任务失败, 任务运行中...").WithLog(err.Error())
			default:
				return errors.NewError(http.StatusInternalServerError,
					fmt.Sprintf("调度任务失败, project_id: %d, task_id: %s", task.ProjectID, task.TaskID)).WithLog(err.Error())
			}
		}
	}

	return nil
}

func (a *app) SetTaskRunning(agentIP string, execInfo *common.TaskExecutingInfo) error {
	runningInfo, _ := json.Marshal(common.TaskRunningInfo{
		Status:    common.TASK_STATUS_RUNNING_V2,
		TmpID:     execInfo.TmpID,
		Timestamp: time.Now().Unix(),
		AgentIP:   agentIP,
	})

	if execInfo.Task.FlowInfo != nil {
		_, err := concurrency.NewSTM(a.etcd.Client(), func(s concurrency.STM) error {
			err := setWorkflowTaskRunning(s, WorkflowRunningTaskInfo{
				WorkflowID: execInfo.Task.FlowInfo.WorkflowID,
				TmpID:      execInfo.TmpID,
				TaskID:     execInfo.Task.TaskID,
				TaskName:   execInfo.Task.Name,
				ProjectID:  execInfo.Task.ProjectID,
				AgentIP:    agentIP,
			})
			if err != nil {
				return err
			}
			s.Put(common.BuildTaskStatusKey(execInfo.Task.ProjectID, execInfo.Task.TaskID), string(runningInfo))
			a.PublishMessage(messageWorkflowTaskStatusChanged(execInfo.Task.FlowInfo.WorkflowID, execInfo.Task.ProjectID, execInfo.Task.TaskID, common.TASK_STATUS_RUNNING_V2))
			return nil
		})
		return err
	}

	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()

	if execInfo.Task.Timeout == 0 {
		execInfo.Task.Timeout = common.DEFAULT_TASK_TIMEOUT_SECONDS
	}
	lease, err := a.etcd.Lease().Grant(ctx, int64(execInfo.Task.Timeout))
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "设置任务运行状态失败，创建lease失败").WithLog(err.Error())
	}
	_, err = a.etcd.KV().Put(ctx, common.BuildTaskStatusKey(execInfo.Task.ProjectID, execInfo.Task.TaskID), string(runningInfo), clientv3.WithLease(lease.ID))
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "设置任务运行状态失败").WithLog(err.Error())
	}

	a.PublishMessage(messageTaskStatusChanged(execInfo.Task.ProjectID, execInfo.Task.TaskID, execInfo.TmpID, common.TASK_STATUS_RUNNING_V2))
	return nil
}

func (a *app) CheckTaskIsRunning(projectID int64, taskID string) ([]common.TaskRunningInfo, error) {
	key := common.BuildTaskStatusKey(projectID, taskID)
	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
	resp, err := a.etcd.KV().Get(ctx, key)
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "获取任务运行状态失败").WithLog(err.Error())
	}

	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var result []common.TaskRunningInfo

	checkFuncV2 := func(stream *CenterClient) (bool, error) {
		ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
		defer cancel()

		_, err := stream.SendEvent(ctx, &cronpb.SendEventRequest{
			Region:    a.cfg.Micro.Region,
			ProjectId: projectID,
			Agent:     stream.addr,
			Event: &cronpb.ServiceEvent{
				Id:        utils.GetStrID(),
				EventTime: time.Now().Unix(),
				Type:      cronpb.EventType_EVENT_CHECK_RUNNING_REQUEST,
				Event: &cronpb.ServiceEvent_CheckRunningRequest{
					CheckRunningRequest: &cronpb.CheckRunningRequest{
						ProjectId: projectID,
						TaskId:    taskID,
					},
				},
			},
		})
		if err != nil {
			return false, err
		}

		return true, nil
	}

	checkFunc := func(agent *AgentClient) (bool, error) {
		ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
		defer cancel()
		resp, err := agent.CheckRunning(ctx, &cronpb.CheckRunningRequest{
			ProjectId: projectID,
			TaskId:    taskID,
		})
		if err != nil {
			return false, errors.NewError(http.StatusInternalServerError, "确认agent任务运行状态失败").WithLog(err.Error())
		}
		return resp.Result, nil
	}

	kv := resp.Kvs[0]
	var runningInfo common.TaskRunningInfo
	if err = json.Unmarshal(kv.Value, &runningInfo); err != nil {
		return nil, err
	}
	streams, err := a.FindAgentsV2(a.cfg.Micro.Region, projectID)
	if err != nil {
		return nil, err
	}
	if len(streams) > 0 {
		defer func() {
			for _, stream := range streams {
				stream.Close()
			}
		}()
		for _, stream := range streams {
			exist, err := checkFuncV2(stream)
			if err != nil {
				return nil, err
			}
			if exist {
				result = append(result, runningInfo)
			}
		}
	} else {
		agentList, err := a.FindAgents(a.cfg.Micro.Region, projectID)
		if err != nil {
			return nil, err
		}

		defer func() {
			for _, agent := range agentList {
				agent.Close()
			}
		}()
		for _, agent := range agentList {
			exist, err := checkFunc(agent)
			if err != nil {
				return nil, err
			}
			if exist {
				result = append(result, runningInfo)
			}
		}
	}

	if len(result) == 0 {
		// 没有agent在跑该任务，但是etcd中存在该任务的running key，大概率是上一次任务执行中agent宕机
		wlog.Info("delete the key of task running status proactively, because the current running status agent does not match the agent that is executing the task",
			zap.String("status_key", key), zap.String("task_id", taskID), zap.Int64("project_id", projectID))
		a.DelTaskRunningKey(runningInfo.AgentIP, projectID, taskID)
	}

	return result, nil
}

func (a *app) DelTaskRunningKey(agentIP string, projectID int64, taskID string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
	// TODO retry
	_, err := a.etcd.KV().Delete(ctx, common.BuildTaskStatusKey(projectID, taskID))
	if err != nil {
		wlog.Error("failed to delete task running key", zap.String("agent", agentIP), zap.String("task_id", taskID), zap.Int64("project_id", projectID))
		return errors.NewError(http.StatusInternalServerError, "删除任务运行状态key失败").WithLog(err.Error())
	}
	return nil
}

func (a *app) SaveTaskLog(agentIP string, result common.TaskFinishedV2) {
	// log receive
	logInfo := common.TaskLog{
		Name:      result.TaskName,
		Result:    result.Result,
		StartTime: result.StartTime,
		EndTime:   result.EndTime,
		Command:   result.Command,
		ClientIP:  agentIP,
		TmpID:     result.TmpID,
		TaskID:    result.TaskID,
		ProjectID: result.ProjectID,
	}

	opts := selection.NewSelector(selection.NewRequirement("id", selection.Equals, result.ProjectID))
	projects, err := a.store.Project().GetProject(opts)
	if err != nil {
		wlog.Error("failed to report task result, the task project not found", zap.Error(err), zap.Int64("project_id", logInfo.ProjectID))
		return
	}

	if len(projects) > 0 {
		logInfo.Project = projects[0].Title
	}

	taskResult := &common.TaskResultLog{
		Result:   result.Result,
		Operator: result.Operator,
	}
	if result.Error != "" {
		logInfo.WithError = 1
		taskResult.Error = result.Error
	}

	var (
		resultBytes    []byte
		jsonMarshalErr error
	)
	if resultBytes, jsonMarshalErr = json.Marshal(taskResult); jsonMarshalErr != nil {
		resultBytes = []byte("result log json marshal error:" + jsonMarshalErr.Error())
	}

	logInfo.Result = string(resultBytes)

	if err := a.store.TaskLog().CreateTaskLog(logInfo); err != nil {
		a.Metrics().CustomInc("system_error", "task_log_saver", fmt.Sprintf("%d_%s", result.ProjectID, result.TaskID))
		a.Warning(warning.NewTaskWarningData(warning.TaskWarning{
			AgentIP:      logInfo.ClientIP,
			TaskID:       logInfo.TaskID,
			TaskName:     logInfo.Name,
			ProjectID:    logInfo.ProjectID,
			ProjectTitle: logInfo.Project,
			Message:      fmt.Sprintf("Center(%s)：任务日志入库失败，原因：%s", a.GetIP(), err.Error()),
		}))
		wlog.With(zap.Any("fields", map[string]interface{}{
			"task_name":  logInfo.Name,
			"result":     logInfo.Result,
			"error":      err.Error(),
			"start_time": time.Unix(logInfo.StartTime, 0).Format("2006-01-02 15:05:05"),
			"end_time":   time.Unix(logInfo.StartTime, 0).Format("2006-01-02 15:05:05"),
		})).Error("任务日志入库失败")
	}
}

func (a *app) HandlerTaskFinished(agentIP string, result *common.TaskFinishedV2) error {
	err := a.DelTaskRunningKey(agentIP, result.ProjectID, result.TaskID)
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "设置任务运行状态失败").WithLog(err.Error())
	}
	if result.WorkflowID != 0 {
		if err := a.workflowRunner.handleTaskResultV1(agentIP, result); err != nil {
			return err
		}
	}

	safe.Run(func() {
		a.PublishMessage(messageTaskStatusChanged(result.ProjectID, result.TaskID, result.TmpID, result.Status))
		a.HandleWebHook(agentIP, result)
	})
	return nil
}
