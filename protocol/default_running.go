package protocol

import (
	"encoding/json"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	recipe "github.com/coreos/etcd/contrib/recipes"
)

type defaultRunningManager struct {
	queue map[int64]*recipe.Queue
}

func (d *defaultRunningManager) SetTaskRunning(s *concurrency.Session, execInfo *common.TaskExecutingInfo) error {
	ctx, _ := utils.GetContextWithTimeout()
	runningInfo, _ := json.Marshal(common.TaskRunningInfo{
		Status: common.TASK_STATUS_RUNNING_V2,
		TmpID:  execInfo.TmpID,
	})
	_, err := s.Client().Put(ctx, common.BuildTaskStatusKey(execInfo.Task.ProjectID, execInfo.Task.TaskID), string(runningInfo), clientv3.WithLease(s.Lease()))
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - SetTaskRunning] etcd client kv put error:" + err.Error()
		return errObj
	}
	return nil
}
func (d *defaultRunningManager) SetTaskNotRunning(s *concurrency.Session, execInfo *common.TaskExecutingInfo, result *common.TaskExecuteResult) error {
	// plan.Task.ClientIP = ""
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

	// do not need to delete task-status-key, lease closed before this func
	// ctx, _ := utils.GetContextWithTimeout()

	// _, err := s.Client().Delete(ctx, common.BuildTaskStatusKey(execInfo.Task.ProjectID, execInfo.Task.TaskID))
	// if err != nil {
	// 	errObj := errors.ErrInternalError
	// 	errObj.Log = "[Etcd - SetTaskNotRunning] etcd client kv put error:" + err.Error()
	// 	return errObj
	// }

	err := d.queue[execInfo.Task.ProjectID].Enqueue(generateTaskFinishedResultV1(TaskFinishedQueueItemV1{
		ProjectID: execInfo.Task.ProjectID,
		TaskID:    execInfo.Task.TaskID,
		Status:    status,
		TaskType:  common.NormalPlan,
		StartTime: result.StartTime.Unix(),
		EndTime:   result.EndTime.Unix(),
	}))
	return err
}
