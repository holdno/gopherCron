package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/utils"

	"github.com/spacegrower/watermelon/infra/wlog"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

func (a *app) WebHookWorker(done <-chan struct{}) error {
	var (
		err     error
		getResp *clientv3.GetResponse
	)
	watchKey := common.GetTaskStatusPrefixKey()
	if err = utils.RetryFunc(5, func() error {
		if getResp, err = a.etcd.KV().Get(context.TODO(), watchKey, clientv3.WithPrefix()); err != nil {
			return err
		}
		return nil
	}); err != nil {
		warningErr := a.Warning(warning.NewSystemWarningData(warning.SystemWarning{
			Endpoint: a.GetIP(),
			Type:     warning.SERVICE_TYPE_CENTER,
			Message:  fmt.Sprintf("center-service: %s, webhook worker etcd kv get error: %s", a.GetIP(), err.Error()),
		}))
		if warningErr != nil {
			wlog.Error(fmt.Sprintf("[service - WebHookWorker] failed to push warning, %s", err.Error()))
		}
		return err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.TODO())
	defer cancelFunc()
	// 从GET时刻的后续版本进行监听变化
	watchStartRevision := getResp.Header.Revision + 1
	// 开始监听
	watchChan := a.etcd.Watcher().Watch(cancelCtx, watchKey, clientv3.WithRev(watchStartRevision), clientv3.WithPrefix(), clientv3.WithPrevKV())

	for {
		select {
		case <-a.ctx.Done():
			return nil
		case <-done:
			return nil
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
				case mvccpb.DELETE: // 任务执行完毕
					a.transportWebhookEvent(watchEvent)
				}
			}
		}
	}
}

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
