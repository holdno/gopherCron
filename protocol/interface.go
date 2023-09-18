package protocol

import (
	"context"
	"encoding/json"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/pkg/etcd"
	"github.com/holdno/gopherCron/utils"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdManager interface {
	Client() *clientv3.Client
	KV() clientv3.KV
	Lease() clientv3.Lease
	Watcher() clientv3.Watcher
	GetTaskLocker(task *common.TaskInfo) *etcd.Locker
	GetLocker(key string) *etcd.Locker
	Inc(key string) (int64, error)
}

type comm struct {
	etcd EtcdManager
}

type CommonInterface interface {
	GetTask(projectID int64, taskID string) (*common.TaskInfo, error)
}

func NewComm(etcd EtcdManager) CommonInterface {
	return &comm{etcd: etcd}
}

// GetTask 获取任务
func (a *comm) GetTask(projectID int64, taskID string) (*common.TaskInfo, error) {
	var (
		saveKey string
		getResp *clientv3.GetResponse
		task    *common.TaskInfo
		ctx     context.Context
		errObj  errors.Error
		err     error
	)

	// build etcd save key
	saveKey = common.BuildKey(projectID, taskID) // 保存的key同样也是获取的key

	ctx, _ = utils.GetContextWithTimeout()

	if getResp, err = a.etcd.KV().Get(ctx, saveKey); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetTask] etcd client kv get one error:" + err.Error()
		return nil, errObj
	}

	if getResp.Count > 1 {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetTask] etcd client kv get one task but result > 1"
		return nil, errObj
	} else if getResp.Count == 0 {
		return nil, errors.ErrDataNotFound
	}

	task = &common.TaskInfo{}
	if err = json.Unmarshal(getResp.Kvs[0].Value, task); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetTask] task json.Unmarshal error:" + err.Error()
		return nil, errObj
	}

	return task, nil
}

const (
	version        = "v2.4.3"
	GrpcBufferSize = 1024 * 4
)

func GetVersion() string {
	return version
}
