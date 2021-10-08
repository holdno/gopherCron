package protocol

import (
	"encoding/json"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
)

type defaultRunningManager struct{}

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
	plan.Task.ClientIP = ""

	ctx, _ := utils.GetContextWithTimeout()

	_, err := kv.Delete(ctx, common.BuildTaskStatusKey(plan.Task.ProjectID, plan.Task.TaskID))
	if err != nil {
		errObj := errors.ErrInternalError
		errObj.Log = "[Etcd - SetTaskNotRunning] etcd client kv put error:" + err.Error()
		return errObj
	}

	return nil
}
