package app

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/holdno/gopherCron/common"

	"github.com/spacegrower/watermelon/infra/wlog"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

func (a *app) transportWebhookEvent(watchEvent *clientv3.Event) {
	if !common.IsStatusKey(string(watchEvent.Kv.Key)) {
		return
	}
	_projectID, taskID := common.PatchProjectIDTaskIDFromStatusKey(string(watchEvent.Kv.Key))
	projectID, err := strconv.ParseInt(_projectID, 10, 64)
	if err != nil {
		wlog.With(zap.Any("fields", map[string]interface{}{
			"project_id": _projectID,
			"task_id":    taskID,
			"error":      err.Error(),
		})).Error("failed to parse project id, not int")
		return
	}
	lock := a.etcd.GetLocker(fmt.Sprintf("%s/lock/center_webhook_watcher_%d_%s", common.ETCD_PREFIX, projectID, taskID))
	if err = lock.TryLock(); err != nil {
		wlog.With(zap.Any("fields", map[string]interface{}{
			"project_id": projectID,
			"task_id":    taskID,
			"error":      err.Error(),
		})).Error("failed to get webhook lock")
		return
	}
	defer lock.Unlock()

	// 解析watchEvent.Kv.Key

	var runningInfo common.TaskRunningInfo
	if err = json.Unmarshal(watchEvent.PrevKv.Value, &runningInfo); err != nil {
		wlog.With(zap.Any("fields", map[string]interface{}{
			"project_id": projectID,
			"task_id":    taskID,
			"tmp_id":     runningInfo.TmpID,
			"value":      string(watchEvent.PrevKv.Value),
			"error":      err.Error(),
		})).Error("failed to unmarshal running info")
		return
	}

	// if err = a.HandleWebHook(projectID, taskID, "", runningInfo.TmpID); err != nil {
	// 	wlog.With(zap.Any("fields", map[string]interface{}{
	// 		"project_id": projectID,
	// 		"task_id":    taskID,
	// 		"type":       "",
	// 		"tmp_id":     runningInfo.TmpID,
	// 		"error":      err.Error(),
	// 	})).Error("failed to handle webhook")
	// }
}
