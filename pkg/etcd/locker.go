package etcd

import (
	"context"

	"ojbk.io/gopherCron/common"

	"github.com/coreos/etcd/clientv3"
	"ojbk.io/gopherCron/errors"
)

// TaskLock 任务锁(分布式锁)
type TaskLock struct {
	kv    clientv3.KV
	lease clientv3.Lease

	taskInfo   *common.TaskInfo
	cancelFunc context.CancelFunc
	leaseID    clientv3.LeaseID
	isLocked   bool // 是否上锁成功
}

func InitTaskLock(taskInfo *common.TaskInfo, kv clientv3.KV, lease clientv3.Lease) *TaskLock {
	return &TaskLock{
		kv:       kv,
		lease:    lease,
		taskInfo: taskInfo,
	}
}

// 尝试上锁
func (tl *TaskLock) TryLock() error {
	var (
		leaseGrantResp *clientv3.LeaseGrantResponse
		cancelCtx      context.Context
		cancelFunc     context.CancelFunc
		errObj         errors.Error
		err            error
		keepRespChan   <-chan *clientv3.LeaseKeepAliveResponse
		lockKey        string
		txn            clientv3.Txn
		txnResp        *clientv3.TxnResponse
	)
	// 创建一个5s的租约
	leaseGrantResp, err = tl.lease.Grant(context.TODO(), 5)
	if err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLock - TryLock] lease grand error:" + err.Error()
		return errObj
	}
	// 自动续租
	// 创建一个cancel context用来取消自动续租
	cancelCtx, cancelFunc = context.WithCancel(context.TODO())
	if keepRespChan, err = tl.lease.KeepAlive(cancelCtx, leaseGrantResp.ID); err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLock - TryLock] lease keepalive error:" + err.Error()
		goto FAIL
	}
	// 处理续租应答的协程
	go func() {
		var (
			keepResp *clientv3.LeaseKeepAliveResponse
		)
		for {
			select {
			case keepResp = <-keepRespChan: // 自动续租的应答
				if keepResp == nil {
					return
				}
			}
		}
	}()
	// 创建事务 txn
	txn = tl.kv.Txn(context.TODO())

	// 生成锁的路径
	lockKey = common.BuildLockKey(tl.taskInfo.ProjectID, tl.taskInfo.TaskID)
	// 事务抢锁
	txn.If(clientv3.Compare(clientv3.CreateRevision(lockKey), "=", 0)).
		Then(clientv3.OpPut(lockKey, "", clientv3.WithLease(leaseGrantResp.ID))).
		Else(clientv3.OpGet(lockKey))

	// 提交事务
	if txnResp, err = txn.Commit(); err != nil {
		// 有可能是抢锁失败 也有可能是网络失败
		// 都进行回滚
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLock - TryLock] txn commit error:" + err.Error()
		goto FAIL
	}
	// 成功返回 失败的话释放租约
	if !txnResp.Succeeded {
		// 事务没有执行成功 进入到else中 即没有抢到锁
		return errors.ErrLockAlreadyRequired
	}

	tl.leaseID = leaseGrantResp.ID
	tl.cancelFunc = cancelFunc
	tl.isLocked = true
	return nil
FAIL:
	cancelFunc()                                       // 取消context
	tl.lease.Revoke(context.TODO(), leaseGrantResp.ID) // 释放租约 key会立即被删掉
	return errObj
}

// Unlock 释放锁
func (tl *TaskLock) Unlock() {
	if tl.isLocked {
		tl.cancelFunc() // 取消锁协成自动续租
		tl.lease.Revoke(context.TODO(), tl.leaseID)
	}
}
