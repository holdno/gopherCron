package etcd

import (
	"context"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"ojbk.io/gopherCron/common"
)

func (m *TaskManager) TaskKiller(projects []string) {
	for _, v := range projects {
		var (
			key                string
			watchStartRevision int64
			watchChan          clientv3.WatchChan
			watchResp          clientv3.WatchResponse
			watchEvent         *clientv3.Event

			task      *common.TaskInfo
			taskEvent *common.TaskEvent
			taskName  string
		)
		key = common.BuildKillKey(v, "")

		go func() {
			// 开始监听
			watchChan = m.watcher.Watch(context.TODO(), key, clientv3.WithRev(watchStartRevision), clientv3.WithPrefix())
			// 处理监听结果
			for watchResp = range watchChan {
				for _, watchEvent = range watchResp.Events {
					switch watchEvent.Type {
					case mvccpb.PUT: // 任务保存
						// 反序列化task
						taskName = common.ExtractKillName(v, string(watchEvent.Kv.Key))
						task = &common.TaskInfo{Name: taskName, Project: v}
						// 构建一个event事件
						taskEvent = common.BuildTaskEvent(common.TASK_EVENT_KILL, task)
						// 推送一个更新事件给 scheduler
					case mvccpb.DELETE: // 任务删除
						// 当前业务不关心delete事件
					}
					Scheduler.PushEvent(taskEvent)
				}
			}
		}()
	}
}
