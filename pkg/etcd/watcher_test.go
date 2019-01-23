package etcd

import (
	"testing"
	"time"

	"github.com/coreos/etcd/mvcc/mvccpb"

	"ojbk.io/gopherCron/common"

	"github.com/coreos/etcd/clientv3"
	"golang.org/x/net/context"
	"ojbk.io/gopherCron/config"
	"ojbk.io/gopherCron/errors"
)

// TestTaskManager_TaskWatcher 测试删除一个preKey
// 得到的结果是
// 如果一个 preKey 下有多个key
// 例如 /testProject/task1   /testProject/task2
// etcd执行delete /testProject/
// watch 会收到两个事件通知
// delete /testProject/testTask1
// delete /testProject/testTask2
func TestTaskManager_TaskWatcher(t *testing.T) {
	apiConf := config.InitServiceConfig("../../cmd/service/conf/config-dev.toml")
	if err := Connect(apiConf.Etcd); err != nil {
		t.Error(err)
		return
	}

	var (
		saveKey   string
		saveKey2  string
		saveValue string
		getResp   *clientv3.GetResponse
		errObj    errors.Error
		err       error
		preKey    string
	)

	// build etcd save key
	saveKey = common.BuildKey("testProject", "testTask1")
	saveKey2 = common.BuildKey("testProject", "testTask2")

	saveValue = "test content"

	// save to etcd
	if _, err = Manager.kv.Put(context.TODO(), saveKey, saveValue, clientv3.WithPrevKV()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - SaveTask] etcd client kv put error:" + err.Error()
		t.Error(errObj.Log)
	}

	// save to etcd
	if _, err = Manager.kv.Put(context.TODO(), saveKey2, saveValue, clientv3.WithPrevKV()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - SaveTask] etcd client kv put error:" + err.Error()
		t.Error(errObj.Log)
	}

	if getResp, err = Manager.kv.Get(context.TODO(), common.ETCD_PREFIX+"/testProject/", clientv3.WithPrefix()); err != nil {
		t.Error(err)
	}

	if len(getResp.Kvs) != 0 {

	}

	preKey = common.ETCD_PREFIX + "/testProject/"
	var (
		watchStartRevision int64
		watchChan          clientv3.WatchChan
		watchResp          clientv3.WatchResponse
		watchEvent         *clientv3.Event

		task      *common.TaskInfo
		taskEvent *common.TaskEvent
		taskName  string
	)

	go func() {
		// 从GET时刻的后续版本进行监听变化
		watchStartRevision = getResp.Header.Revision + 1
		// 开始监听
		watchChan = Manager.watcher.Watch(context.TODO(), preKey, clientv3.WithRev(watchStartRevision), clientv3.WithPrefix())
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
					taskName = common.ExtractTaskID("testProject", string(watchEvent.Kv.Key))
					// 构建一个delete event
					task = &common.TaskInfo{TaskID: taskName, ProjectID: "testProject"}
					taskEvent = common.BuildTaskEvent(common.TASK_EVENT_DELETE, task)
					taskEvent = taskEvent
					// 推送给 scheduler 把任务终止掉
				}
				// Scheduler.PushEvent(taskEvent)
			}
		}
	}()

	if _, err = Manager.kv.Delete(context.TODO(), preKey, clientv3.WithPrefix()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - DeleteTask] etcd client kv delete error:" + err.Error()
		t.Error(errObj.Log)
		return
	}

	time.Sleep(10 * time.Second)
}
