package etcd

import (
	"context"
	"net/http"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/errors"

	"github.com/coreos/etcd/clientv3"
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
		Username:    config.Username,
		Password:    config.Password,
		DialTimeout: time.Duration(config.DialTimeout) * time.Millisecond,
	}

	if client, err = clientv3.New(etcdConf); err != nil {
		errObj = errors.NewError(http.StatusInternalServerError,
			"[api_context - InitAPIContext] etcd.Connect get error:"+err.Error(), "")
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
func (m *TaskManager) GetTaskLocker(task *common.TaskInfo) *Locker {
	// 返回一把锁
	return initTaskLocker(task, m.kv, m.lease)
}

func (m *TaskManager) GetLocker(lockkey string) *Locker {
	return initLocker(lockkey, m.kv, m.lease)
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
