package etcd

import (
	"context"

	"github.com/coreos/etcd/mvcc/mvccpb"

	"ojbk.io/gopherCron/errors"

	"encoding/json"

	"github.com/coreos/etcd/clientv3"
	"ojbk.io/gopherCron/common"
)

// SaveTask save task to etcd
// return oldtask & error
func (m *TaskManager) SaveTask(task *common.TaskInfo) (*common.TaskInfo, error) {
	var (
		saveKey  string
		saveByte []byte
		putResp  *clientv3.PutResponse
		oldTask  *common.TaskInfo
		errObj   errors.Error
		err      error
	)

	// build etcd save key
	saveKey = common.BuildKey(task.ProjectID, task.TaskID)

	// task to json
	if saveByte, err = json.Marshal(task); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - SaveTask] json.mashal task error:" + err.Error()
		return nil, errObj
	}

	// save to etcd
	if putResp, err = m.kv.Put(context.TODO(), saveKey, string(saveByte), clientv3.WithPrevKV()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - SaveTask] etcd client kv put error:" + err.Error()
		return nil, errObj
	}

	// oldtask exist
	if putResp.PrevKv != nil {
		// if oldtask unmarshal error
		// don't care because this err doesn't affect result
		json.Unmarshal([]byte(putResp.PrevKv.Value), &oldTask)
	}

	return oldTask, nil
}

func (m *TaskManager) DeleteTask(projectID, taskID string) (*common.TaskInfo, error) {
	var (
		deleteKey string
		delResp   *clientv3.DeleteResponse
		oldTask   *common.TaskInfo
		errObj    errors.Error
		err       error
	)

	// build etcd delete key
	deleteKey = common.BuildKey(projectID, taskID)

	// save to etcd
	if delResp, err = m.kv.Delete(context.TODO(), deleteKey, clientv3.WithPrevKV(), clientv3.WithPrefix()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - DeleteTask] etcd client kv delete error:" + err.Error()
		return nil, errObj
	}

	if taskID != "" && len(delResp.PrevKvs) != 0 {
		json.Unmarshal([]byte(delResp.PrevKvs[0].Value), &oldTask)
	}

	return oldTask, nil
}

// GetTask 获取任务
func (m *TaskManager) GetTask(project, name string) (*common.TaskInfo, error) {
	var (
		saveKey string
		getResp *clientv3.GetResponse
		task    *common.TaskInfo
		errObj  errors.Error
		err     error
	)

	// build etcd save key
	saveKey = common.BuildKey(project, name) // 保存的key同样也是获取的key

	if getResp, err = m.kv.Get(context.TODO(), saveKey); err != nil {
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

// GetTaskList 获取任务列表
func (m *TaskManager) GetTaskList(projectID string) ([]*common.TaskInfo, error) {
	var (
		preKey   string
		getResp  *clientv3.GetResponse
		taskList []*common.TaskInfo
		task     *common.TaskInfo
		kvPair   *mvccpb.KeyValue
		errObj   errors.Error
		err      error
	)

	// build etcd pre key
	preKey = common.BuildKey(projectID, "")

	if getResp, err = m.kv.Get(context.TODO(), preKey,
		clientv3.WithPrefix()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetTaskList] etcd client kv getlist error:" + err.Error()
		return nil, errObj
	}

	// init array space
	taskList = make([]*common.TaskInfo, 0)

	// range list to unmarshal
	for _, kvPair = range getResp.Kvs {
		task = &common.TaskInfo{}
		if err = json.Unmarshal(kvPair.Value, task); err != nil {
			continue
		}

		taskList = append(taskList, task)
	}

	return taskList, nil
}

// GetTaskList 获取任务列表
func (m *TaskManager) GetProjectTaskCount(projectID string) (int64, error) {
	var (
		preKey  string
		getResp *clientv3.GetResponse
		errObj  errors.Error
		err     error
	)

	// build etcd pre key
	preKey = common.BuildKey(projectID, "")

	if getResp, err = m.kv.Get(context.TODO(), preKey,
		clientv3.WithPrefix(), clientv3.WithCountOnly()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetProjectTaskCount] etcd client kv error:" + err.Error()
		return 0, errObj
	}

	return getResp.Count, nil
}

// KillTask 强行结束任务
func (m *TaskManager) KillTask(project, name string) error {
	var (
		killKey        string
		leaseGrantResp *clientv3.LeaseGrantResponse
		errObj         errors.Error
		err            error
	)

	killKey = common.BuildKillKey(project, name)

	// make lease to notify worker
	// 创建一个租约 让其稍后过期并自动删除
	if leaseGrantResp, err = m.lease.Grant(context.TODO(), 1); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - KillTask] lease grant error:" + err.Error()
		return errObj
	}

	if _, err = m.kv.Put(context.TODO(), killKey, "", clientv3.WithLease(leaseGrantResp.ID)); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - KillTask] put kill task error:" + err.Error()
		return errObj
	}

	return nil
}

// GetWorkerList 获取节点列表
func (m *TaskManager) GetWorkerList(projectID string) ([]string, error) {
	var (
		preKey  string
		err     error
		errObj  errors.Error
		getResp *clientv3.GetResponse
		kv      *mvccpb.KeyValue
		res     []string
	)

	preKey = common.BuildRegisterKey(projectID, "")
	if getResp, err = m.kv.Get(context.TODO(), preKey, clientv3.WithPrefix()); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[Etcd - GetWorkerList] get preKey error:" + err.Error()
		return nil, errObj
	}

	for _, kv = range getResp.Kvs {
		res = append(res, common.ExtractWorkerIP(projectID, string(kv.Key)))
	}

	return res, nil
}
