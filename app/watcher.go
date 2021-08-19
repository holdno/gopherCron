package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
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
		warningErr := a.Warning(WarningData{
			Data: fmt.Sprintf("[service - WebHookWorker] etcd kv get error: %s", err.Error()),
			Type: WarningTypeSystem,
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

func (a *client) newWatchHandle() func(resp clientv3.WatchResponse, projectID int64) {
	var (
		err        error
		watchEvent *clientv3.Event
		taskEvent  *common.TaskEvent
	)
	return func(resp clientv3.WatchResponse, projectID int64) {
		var (
			task   *common.TaskInfo
			taskID string
		)

		if resp.Err() != nil {
			a.logger.WithFields(logrus.Fields{
				"error":      resp.Err(),
				"project_id": projectID,
			}).Error("etcd watcher with error")
		}

		for _, watchEvent = range resp.Events {
			switch watchEvent.Type {
			case mvccpb.PUT: // 任务保存
				// 反序列化task
				if task, err = common.Unmarshal(watchEvent.Kv.Value); err != nil {
					continue
				}
				// 构建一个临时调度任务的事件
				if common.IsTemporaryKey(string(watchEvent.Kv.Key)) {
					taskEvent = common.BuildTaskEvent(common.TASK_EVENT_TEMPORARY, task)
				} else {
					// 构建一个event
					taskEvent = common.BuildTaskEvent(common.TASK_EVENT_SAVE, task)
				}
				a.logger.WithFields(logrus.Fields{
					"event_type": taskEvent.EventType,
					"task":       taskEvent.Task.Name,
				}).Debug("get put event")
				// 推送一个更新事件给 scheduler
			case mvccpb.DELETE: // 任务删除
				if common.IsTemporaryKey(string(watchEvent.Kv.Key)) {
					continue
				}
				taskID = common.ExtractTaskID(projectID, string(watchEvent.Kv.Key))
				// 构建一个delete event
				task = &common.TaskInfo{TaskID: taskID, ProjectID: projectID}
				taskEvent = common.BuildTaskEvent(common.TASK_EVENT_DELETE, task)
				// 推送给 scheduler 把任务终止掉
			}

			a.scheduler.PushEvent(taskEvent)
		}
	}
}

func (a *client) startTaskWatcher(projectID int64) error {
	var (
		getResp            *clientv3.GetResponse
		err                error
		preKey             string
		kvPair             *mvccpb.KeyValue
		watchStartRevision int64
		watchChan          clientv3.WatchChan

		taskEvent *common.TaskEvent
	)

	handleFunc := a.newWatchHandle()

	preKey = common.BuildKey(projectID, "")
	a.logger.Infof("[agent - TaskWatcher] new task watcher, project_id: %d", projectID)
	if err = utils.RetryFunc(5, func() error {
		if getResp, err = a.etcd.KV().Get(context.TODO(), preKey, clientv3.WithPrefix()); err != nil {
			return err
		}
		return nil
	}); err != nil {
		warningErr := a.Warning(WarningData{
			Data:      fmt.Sprintf("[agent - TaskWatcher] etcd kv get error: %s, projectid: %d", err.Error(), projectID),
			Type:      WarningTypeSystem,
			AgentIP:   a.GetIP(),
			ProjectID: projectID,
		})
		if warningErr != nil {
			a.logger.Errorf("[agent - TaskWatcher] failed to push warning, %s", err.Error())
		}
		return err
	}

	for _, kvPair = range getResp.Kvs {
		if task, err := common.Unmarshal(kvPair.Value); err == nil {
			taskEvent = common.BuildTaskEvent(common.TASK_EVENT_SAVE, task)
			// 将所有任务加入调度队列
			a.scheduler.PushEvent(taskEvent)
		}
	}

	cancelCtx, cancelWatchFunc := context.WithCancel(context.TODO())
	defer cancelWatchFunc()
	// 从GET时刻的后续版本进行监听变化
	watchStartRevision = getResp.Header.Revision + 1
	// 开始监听
	watchChan = a.etcd.Watcher().Watch(cancelCtx, preKey, clientv3.WithRev(watchStartRevision), clientv3.WithPrefix())
	for {
		select {
		case <-a.daemon.WaitRemoveSignal(projectID):
			a.scheduler.PlanRange(func(key string, value *common.TaskSchedulePlan) bool {
				if value.Task.ProjectID == projectID {
					taskEvent = common.BuildTaskEvent(common.TASK_EVENT_DELETE, value.Task)
					// 将该项目下的所有任务移出调度队列
					a.scheduler.PushEvent(taskEvent)
				}
				return true
			})
			a.logger.Infof("[agent - TaskWatcher] stop to watching project %d", projectID)
			return nil
		case w, ok := <-watchChan:
			if !ok {
				return nil
			}

			if w.Err() != nil {
				return w.Err()
			}
			handleFunc(w, projectID)
		}
	}
}

func (a *client) TaskWatcher(projects []int64) {
	for _, projectID := range projects {
		a.etcdWatchDaemon(projectID)
	}
}

func (a *client) etcdWatchDaemon(projectID int64) {
	a.Go(func() {
	REWATCH:
		a.logger.WithField("project_id", projectID).Info("task watcher start")
		if err := a.startTaskWatcher(projectID); err != nil {
			a.logger.WithFields(logrus.Fields{
				"error":      err.Error(),
				"project_id": projectID,
			}).Error("task watcher down")
			goto REWATCH
		}
	})
}
