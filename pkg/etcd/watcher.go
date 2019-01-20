package etcd

import (
	"context"

	"github.com/coreos/etcd/mvcc/mvccpb"
	"ojbk.io/gopherCron/common"

	"github.com/coreos/etcd/clientv3"
	"ojbk.io/gopherCron/errors"
)

func (m *TaskManager) TaskWatcher(projects []string) error {
	for _, v := range projects {
		var (
			getResp            *clientv3.GetResponse
			errObj             errors.Error
			err                error
			preKey             string
			kvPair             *mvccpb.KeyValue
			watchStartRevision int64
			watchChan          clientv3.WatchChan
			watchResp          clientv3.WatchResponse
			watchEvent         *clientv3.Event

			task      *common.TaskInfo
			taskEvent *common.TaskEvent
			taskID    string
		)
		preKey = common.BuildKey(v, "")

		if getResp, err = m.kv.Get(context.TODO(), preKey, clientv3.WithPrefix()); err != nil {
			errObj = errors.ErrInternalError
			errObj.Log = "[Etcd - TaskWatcher] etcd kv get error:" + err.Error()
			return errObj
		}

		for _, kvPair = range getResp.Kvs {
			if task, err = common.Unmarshal(kvPair.Value); err == nil {
				taskEvent = common.BuildTaskEvent(common.TASK_EVENT_SAVE, task)
				// TODO scheduler
				Scheduler.PushEvent(taskEvent)
			}
		}

		go func() {
			// 从GET时刻的后续版本进行监听变化
			watchStartRevision = getResp.Header.Revision + 1
			// 开始监听
			watchChan = m.watcher.Watch(context.TODO(), preKey, clientv3.WithRev(watchStartRevision), clientv3.WithPrefix())
			// 处理监听结果
			for watchResp = range watchChan {
				for _, watchEvent = range watchResp.Events {
					switch watchEvent.Type {
					case mvccpb.PUT: // 任务保存
						// 反序列化task
						if task, err = common.Unmarshal(watchEvent.Kv.Value); err != nil {
							continue
						}
						// 构建一个event
						taskEvent = common.BuildTaskEvent(common.TASK_EVENT_SAVE, task)
						// 推送一个更新事件给 scheduler
					case mvccpb.DELETE: // 任务删除
						taskID = common.ExtractTaskID(v, string(watchEvent.Kv.Key))
						// 构建一个delete event
						task = &common.TaskInfo{TaskID: taskID, ProjectID: v}
						taskEvent = common.BuildTaskEvent(common.TASK_EVENT_DELETE, task)
						// 推送给 scheduler 把任务终止掉
					}
					Scheduler.PushEvent(taskEvent)
				}
			}
		}()
	}
	return nil
}
