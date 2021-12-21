package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/sirupsen/logrus"
)

func (a *app) WebHookWorker() error {
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
		warningErr := a.Warning(warning.WarningData{
			Data: fmt.Sprintf("[service - WebHookWorker] etcd kv get error: %s", err.Error()),
			Type: warning.WarningTypeSystem,
		})
		if warningErr != nil {
			a.logger.Errorf("[service - WebHookWorker] failed to push warning, %s", err.Error())
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
		a.logger.WithFields(logrus.Fields{
			"project_id": _projectID,
			"task_id":    taskID,
			"error":      err.Error(),
		}).Error("failed to parse project id, not int")
		return
	}
	lock := a.etcd.GetLocker(fmt.Sprintf("center_webhook_watcher_%d_%s", projectID, taskID))
	if err = lock.TryLock(); err != nil {
		a.logger.WithFields(logrus.Fields{
			"project_id": projectID,
			"task_id":    taskID,
			"error":      err.Error(),
		}).Error("failed to get webhook lock")
		return
	}
	defer lock.Unlock()

	// 解析watchEvent.Kv.Key

	var runningInfo common.TaskRunningInfo
	if err = json.Unmarshal(watchEvent.PrevKv.Value, &runningInfo); err != nil {
		a.logger.WithFields(logrus.Fields{
			"project_id": projectID,
			"task_id":    taskID,
			"tmp_id":     runningInfo.TmpID,
			"value":      string(watchEvent.PrevKv.Value),
			"error":      err.Error(),
		}).Error("failed to unmarshal running info")
		return
	}

	if err = a.HandleWebHook(projectID, taskID, "", runningInfo.TmpID); err != nil {
		a.logger.WithFields(logrus.Fields{
			"project_id": projectID,
			"task_id":    taskID,
			"type":       "",
			"tmp_id":     runningInfo.TmpID,
			"error":      err.Error(),
		}).Error("failed to handle webhook")
	}
}
