package etcd

import (
	"context"
	"time"

	"ojbk.io/gopherCron/errors"

	"ojbk.io/gopherCron/common"

	"github.com/coreos/etcd/clientv3"
	"ojbk.io/gopherCron/config"
)

type TaskManager struct {
	client  *clientv3.Client
	kv      clientv3.KV
	lease   clientv3.Lease
	watcher clientv3.Watcher
}

func (t *TaskManager) Client() *clientv3.Client {
	return t.client
}

func (t *TaskManager) KV() clientv3.KV {
	return t.kv
}

func (t *TaskManager) Lease() clientv3.Lease {
	return t.lease
}

func (t *TaskManager) Watcher() clientv3.Watcher {
	return t.watcher
}

func Connect(config *config.EtcdConf) (*TaskManager, error) {
	var (
		etcdConf clientv3.Config
		client   *clientv3.Client
		err      error
		errObj   errors.Error
	)

	common.ETCD_PREFIX = config.Prefix

	// client config
	etcdConf = clientv3.Config{
		Endpoints:   config.Service, // cluster list
		DialTimeout: time.Duration(config.DialTimeout) * time.Millisecond,
	}

	if client, err = clientv3.New(etcdConf); err != nil {
		errObj = errors.NewError(500, "[api_context - InitAPIContext] etcd.Connect get error:"+err.Error(), "")
		return nil, errObj
	}

	Manager := &TaskManager{
		client:  client,
		kv:      clientv3.NewKV(client),
		lease:   clientv3.NewLease(client),
		watcher: clientv3.NewWatcher(client),
	}
	return Manager, nil
}

// 创建任务执行锁
func (m *TaskManager) Lock(task *common.TaskInfo) *TaskLock {
	// 返回一把锁
	return InitTaskLock(task, m.kv, m.lease)
}

func (m *TaskManager) Inc(key string) (int64, error) {
	var (
		putResp *clientv3.PutResponse
		err     error
		errObj  errors.Error
	)

	if putResp, err = m.kv.Put(context.TODO(), key, ""); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskManager - Inc] get inc num error:" + err.Error()
		return 0, errObj
	}

	return putResp.Header.Revision, nil
}
