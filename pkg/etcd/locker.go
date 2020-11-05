package etcd

import (
	"context"
	"sync"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"

	"github.com/coreos/etcd/clientv3"
)

// Locker 分布式锁
type Locker struct {
	kv    clientv3.KV
	lease clientv3.Lease

	key        string
	cancelFunc context.CancelFunc
	leaseID    clientv3.LeaseID
	isLocked   bool // 是否上锁成功
}

func initTaskLocker(taskInfo *common.TaskInfo, kv clientv3.KV, lease clientv3.Lease) *Locker {
	return &Locker{
		kv:    kv,
		lease: lease,
		key:   common.BuildLockKey(taskInfo.ProjectID, taskInfo.TaskID),
	}
}

func initLocker(lockkey string, kv clientv3.KV, lease clientv3.Lease) *Locker {
	return &Locker{
		kv:    kv,
		lease: lease,
		key:   lockkey,
	}
}

var (
	clientlockers = new(clientlocker)
)

type clientlocker struct {
	m sync.Map
}

func (c *clientlocker) setLease(l clientv3.LeaseID, lock *Locker) {
	c.m.Store(l, lock)
}

func (c *clientlocker) getLease(l clientv3.LeaseID) (*Locker, bool) {
	data, exist := c.m.Load(l)
	if exist {
		return data.(*Locker), true
	}
	return nil, false
}

func (c *clientlocker) deleteLease(l clientv3.LeaseID) {
	c.m.Delete(l)
}

func (c *clientlocker) rangeLease(f func(l clientv3.LeaseID, tl *Locker) bool) {
	c.m.Range(func(key, value interface{}) bool {
		return f(key.(clientv3.LeaseID), value.(*Locker))
	})
}

func (tl *Locker) CloseAll() {
	clientlockers.rangeLease(func(l clientv3.LeaseID, tl *Locker) bool {
		tl.Unlock()
		return true
	})
}

// 尝试上锁
func (tl *Locker) TryLock() error {
	var (
		leaseGrantResp *clientv3.LeaseGrantResponse
		cancelCtx      context.Context
		cancelFunc     context.CancelFunc
		errObj         errors.Error
		err            error
		keepRespChan   <-chan *clientv3.LeaseKeepAliveResponse
		txn            clientv3.Txn
		txnResp        *clientv3.TxnResponse
	)
	// 创建一个30s的租约
	leaseGrantResp, err = tl.lease.Grant(context.TODO(), 30)
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
		return errObj
	}

	failFunc := func() {
		cancelFunc()
		_, _ = tl.lease.Revoke(context.TODO(), leaseGrantResp.ID) // 释放租约 key会立即被删掉
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

	// 事务抢锁
	txn.If(clientv3.Compare(clientv3.CreateRevision(tl.key), "=", 0)).
		Then(clientv3.OpPut(tl.key, "", clientv3.WithLease(leaseGrantResp.ID))).
		Else(clientv3.OpGet(tl.key))

	// 提交事务
	if txnResp, err = txn.Commit(); err != nil {
		// 有可能是抢锁失败 也有可能是网络失败
		// 都进行回滚
		failFunc()
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLock - TryLock] txn commit error:" + err.Error()
		return errObj
	}
	// 成功返回 失败的话释放租约
	if !txnResp.Succeeded {
		// 事务没有执行成功
		failFunc()
		return errors.ErrLockAlreadyRequired
	}

	tl.leaseID = leaseGrantResp.ID
	tl.cancelFunc = cancelFunc
	tl.isLocked = true
	clientlockers.setLease(tl.leaseID, tl)
	return nil
}

// Unlock 释放锁
func (tl *Locker) Unlock() {
	if tl.isLocked {
		tl.isLocked = false
		tl.cancelFunc() // 取消锁协成自动续租
		_, _ = tl.lease.Revoke(context.TODO(), tl.leaseID)
		clientlockers.deleteLease(tl.leaseID)
	}
}
