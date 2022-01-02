package protocol

import (
	"encoding/json"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
	recipe "github.com/coreos/etcd/contrib/recipes"
)

type defaultRunningManager struct {
	queue *recipe.Queue
}

func (d *defaultRunningManager) SetTaskRunning(kv clientv3.KV, plan *common.TaskSchedulePlan) error {
	ctx, _ := utils.GetContextWithTimeout()
	runningInfo, _ := json.Marshal(common.TaskRunningInfo{
		Status: common.TASK_STATUS_RUNNING_V2,
		TmpID:  plan.TmpID,
	})
	_, err := kv.Put(ctx, common.BuildTaskStatusKey(plan.Task.ProjectID, plan.Task.TaskID), string(runningInfo))
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - SetTaskRunning] etcd client kv put error:" + err.Error()
		return errObj
	}
	return nil
}
func (d *defaultRunningManager) SetTaskNotRunning(kv clientv3.KV, plan *common.TaskSchedulePlan, result *common.TaskExecuteResult) error {
	// plan.Task.ClientIP = ""
	var status string
	if result == nil || result.Err != "" {
		// 未执行，发生未知错误
		status = common.TASK_STATUS_FAIL_V2
	} else {
		status = common.TASK_STATUS_DONE_V2
	}
	ctx, _ := utils.GetContextWithTimeout()

	_, err := kv.Delete(ctx, common.BuildTaskStatusKey(plan.Task.ProjectID, plan.Task.TaskID))
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - SetTaskNotRunning] etcd client kv put error:" + err.Error()
		return errObj
	}

	err = d.queue.Enqueue(generateTaskFinishedResultV1(TaskFinishedQueueItemV1{
		ProjectID: plan.Task.ProjectID,
		TaskID:    plan.Task.TaskID,
		Status:    status,
		TaskType:  common.NormalPlan,
		StartTime: result.StartTime.Unix(),
		EndTime:   result.EndTime.Unix(),
	}))
	return nil
}
