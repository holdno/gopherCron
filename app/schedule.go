package app

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/protocol"
	"github.com/holdno/gopherCron/utils"
	"github.com/holdno/rego"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

func scheduleTask(cli *clientv3.Client, taskInfo *common.TaskInfo) error {
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

	// 设置任务状态为启动中
	// 调度任务至agent
	_, err = concurrency.NewSTM(cli, func(s concurrency.STM) error {
		if err = setWorkflowTaskStarting(s, taskInfo); err != nil {
			return err
		}
		ctx, _ := utils.GetContextWithTimeout()
		// make lease to notify worker
		// 创建一个租约 让其稍后过期并自动删除
		leaseGrantResp, err := cli.Lease.Grant(ctx, 1)
		if err != nil {
			errObj := errors.ErrInternalError
			errObj.Log = "[putSchedule] lease grant error:" + err.Error()
			return errObj
		}

		return putSchedule(s, leaseGrantResp.ID, taskInfo)
	})
	if err != nil {
		return err
	}

	err = waitingAck(cli, taskInfo, func(e *clientv3.Event) bool {
		var ack common.AckResponse
		if err := json.Unmarshal(e.Kv.Value, &ack); err != nil {
			// log
			return false
		}

		switch ack.Version {
		case common.ACK_RESPONSE_V1:
			var v1 common.AckResponseV1
			if err := json.Unmarshal(ack.Data, &v1); err != nil {
				// log
				fmt.Println("unmarshal ack data error", err.Error())
				return false
			}
		default:
			return false
		}

		_, err = concurrency.NewSTM(cli, func(s concurrency.STM) error {
			if err = setWorkflowTaskRunning(s, taskInfo); err != nil {
				return err
			}
			s.Del(string(e.Kv.Key))
			return nil
		})
		if err != nil {
			// log
			fmt.Println("delete ack key error", err.Error())
			return false
		}
		return true
	})

	if err != nil {
		// 针对err为watch超时的场景，配置not runnint
		// watch 超时 / 失败  log
		// fmt.Println("waiting ack failed:", err.Error())
		// _, err = concurrency.NewSTM(cli, func(s concurrency.STM) error {
		// 	if err = setWorkflowTaskNotRunning(s, taskInfo); err != nil {
		// 		return err
		// 	}
		// 	return nil
		// })
		// if err != nil {
		// 	// log

		// 	return err
		// }
		return err
	}

	return nil
}

type WorkflowTaskScheduleRecord struct {
	TmpID     string `json:"tmp_id"`
	Result    string `json:"result"`
	Status    string `json:"status"`
	EventTime int64  `json:"event_time"`
}

type WorkflowTaskStates struct {
	ProjectID       int64                         `json:"project_id"`
	TaskID          string                        `json:"task_id"`
	WorkflowID      int64                         `json:"workflow_id"`
	CurrentStatus   string                        `json:"current_status"`
	ScheduleCount   int                           `json:"schedule_count"`
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

// finished 不一定成功
func setWorkFlowTaskFinished(kv concurrency.STM, queueData protocol.TaskFinishedQueueItemV1) (bool, error) {
	key := common.BuildWorkflowTaskStatusKey(queueData.WorkflowID, queueData.ProjectID, queueData.TaskID)
	states := kv.Get(key)
	planFinished := false

	if states == "" {
		// what happen?
		return false, nil
	}

	var workflowTaskStates WorkflowTaskStates
	if err := json.Unmarshal([]byte(states), &workflowTaskStates); err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[setWorkflowTaskRunning] json unmarshal workflow task running result error:" + err.Error()
		return false, nil
	}

	if workflowTaskStates.CurrentStatus == common.TASK_STATUS_DONE_V2 ||
		workflowTaskStates.CurrentStatus == common.TASK_STATUS_FAIL_V2 {
		return false, nil
	}

	endTime := time.Now().Unix()
	workflowTaskStates.ScheduleRecords = append(workflowTaskStates.ScheduleRecords, &WorkflowTaskScheduleRecord{
		TmpID:     queueData.TmpID,
		Status:    queueData.Status,
		EventTime: endTime,
	})

	if queueData.Status == common.TASK_STATUS_FAIL_V2 {
		if workflowTaskStates.ScheduleCount >= common.WORKFLOW_SCHEDULE_LIMIT {
			workflowTaskStates.CurrentStatus = common.TASK_STATUS_FAIL_V2
			workflowTaskStates.EndTime = endTime
			planFinished = true
		} else {
			workflowTaskStates.CurrentStatus = common.TASK_STATUS_NOT_RUNNING_V2
		}
	} else if queueData.Status == common.TASK_STATUS_DONE_V2 {
		workflowTaskStates.CurrentStatus = common.TASK_STATUS_DONE_V2
		workflowTaskStates.EndTime = endTime
	}

	newStates, _ := json.Marshal(workflowTaskStates)
	kv.Put(key, string(newStates))
	return planFinished, nil
}

func setWorkflowTaskNotRunning(kv concurrency.STM, taskInfo *common.TaskInfo) error {
	key := common.BuildWorkflowTaskStatusKey(taskInfo.FlowInfo.WorkflowID, taskInfo.ProjectID, taskInfo.TaskID)
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
		EventTime: time.Now().Unix(),
	})

	newStates, _ := json.Marshal(workflowTaskStates)
	kv.Put(key, string(newStates))
	return nil
}

func setWorkflowTaskRunning(kv concurrency.STM, taskInfo *common.TaskInfo) error {
	key := common.BuildWorkflowTaskStatusKey(taskInfo.FlowInfo.WorkflowID, taskInfo.ProjectID, taskInfo.TaskID)
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

	workflowTaskStates.CurrentStatus = common.TASK_STATUS_RUNNING_V2
	workflowTaskStates.ScheduleRecords = append(workflowTaskStates.ScheduleRecords, &WorkflowTaskScheduleRecord{
		TmpID:     taskInfo.TmpID,
		Status:    common.TASK_STATUS_RUNNING_V2,
		EventTime: time.Now().Unix(),
	})

	newStates, _ := json.Marshal(workflowTaskStates)
	kv.Put(key, string(newStates))
	return nil
}

func setWorkflowTaskStarting(kv concurrency.STM, taskInfo *common.TaskInfo) error {

	key := common.BuildWorkflowTaskStatusKey(taskInfo.FlowInfo.WorkflowID, taskInfo.ProjectID, taskInfo.TaskID)
	fmt.Println("starting key:", key)
	value := kv.Get(key)
	fmt.Println("get starting key:", string(value))
	var states []byte
	if value == "" {
		states, _ = json.Marshal(WorkflowTaskStates{
			ProjectID:     taskInfo.ProjectID,
			TaskID:        taskInfo.TaskID,
			WorkflowID:    taskInfo.FlowInfo.WorkflowID,
			CurrentStatus: common.TASK_STATUS_STARTING_V2,
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
			return nil
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
		errObj := errors.ErrInternalError
		errObj.Log = "[putSchedule] json.mashal task error:" + err.Error()
		return errObj
	}

	// save to etcd
	kv.Put(scheduleKey, string(saveByte), clientv3.WithLease(leaseID))
	return nil
}

func waitingAck(cli *clientv3.Client, taskInfo *common.TaskInfo, onAck func(*clientv3.Event) bool) error {
	ackKey := common.BuildWorkflowAckKey(taskInfo.FlowInfo.WorkflowID, taskInfo.ProjectID, taskInfo.TaskID, taskInfo.TmpID)
	fmt.Println("waiting ack key", ackKey)
	ctx, _ := utils.GetContextWithTimeout()
	// 开始监听
	watchChan := cli.Watcher.Watch(ctx, ackKey, clientv3.WithPrefix())

	for {
		select {
		case w, ok := <-watchChan:
			if !ok {
				return nil
			}

			if w.Err() != nil {
				return w.Err()
			}

			for _, watchEvent := range w.Events {
				switch watchEvent.Type {
				// case mvccpb.PUT: // 任务开始执行
				case mvccpb.PUT: // 任务开始执行
					errList := rego.Retry(func() error {
						if ok := onAck(watchEvent); ok {
							return nil
						}
						return fmt.Errorf("retry")
					})
					if len(errList) > 0 {
						return errList
					}
				}
			}
		}
	}
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
			if err := clearWorkflowKeys(cli.KV, workflowID); err != nil {
				return err
			}
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

	ctx, _ := utils.GetContextWithTimeout()
	for _, v := range delKeys {
		if _, err := kv.Delete(ctx, v, clientv3.WithPrefix()); err != nil {
			return err
		}
	}
	return nil
}
