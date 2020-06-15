package app

import (
	"context"

	"github.com/holdno/gopherCron/common"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

func (a *client) newKillHandle() func(watchResp clientv3.WatchResponse, projectID int64) {
	var (
		watchEvent *clientv3.Event

		task      *common.TaskInfo
		taskEvent *common.TaskEvent
		taskID    string
	)
	return func(watchResp clientv3.WatchResponse, projectID int64) {
		for _, watchEvent = range watchResp.Events {
			switch watchEvent.Type {
			case mvccpb.PUT: // 任务保存
				// 反序列化task
				taskID = common.ExtractKillID(projectID, string(watchEvent.Kv.Key))
				task = &common.TaskInfo{TaskID: taskID, ProjectID: projectID}
				// 构建一个event事件
				taskEvent = common.BuildTaskEvent(common.TASK_EVENT_KILL, task)
				// 推送一个更新事件给 scheduler
			case mvccpb.DELETE: // 任务删除
				// 当前业务不关心delete事件
				taskEvent = common.BuildTaskEvent(common.TASK_EVENT_DELETE, task)
			}
			a.scheduler.PushEvent(taskEvent)
		}
	}
}

func (a *client) startTaskKiller(projectID int64) {
	a.Go(func() {
		var (
			key       string
			watchChan clientv3.WatchChan
		)
		a.logger.Infof("[agent - TaskKiller] new task killer, project_id: %d", projectID)

		handleKill := a.newKillHandle()

		// 开始监听
		cancelCtx, cancelKillFunc := context.WithCancel(context.TODO())
		key = common.BuildKillKey(projectID, "")
		watchChan = a.etcd.Watcher().Watch(cancelCtx, key,
			clientv3.WithPrefix())
		// 处理监听结果
		for {
			select {
			case <-a.daemon.WaitRemoveSignal(projectID):
				cancelKillFunc()
				a.logger.Infof("[agent - TaskKiller] stop to watching project %d", projectID)
				return
			case watchResp := <-watchChan:
				handleKill(watchResp, projectID)
			}
		}
	})
}

func (a *client) TaskKiller(projects []int64) {
	for _, projectID := range projects {
		a.startTaskKiller(projectID)
	}
}
