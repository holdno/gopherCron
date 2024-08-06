package etcd

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Locker 分布式锁
type Locker struct {
	kv    clientv3.KV
	lease clientv3.Lease

	key        string
	cancelFunc context.CancelFunc
	leaseID    clientv3.LeaseID
	isLocked   atomic.Bool // 是否上锁成功
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

func (tl *Locker) TryLock() error {
	return tl.TryLockWithOwner("")
}

// 尝试上锁
func (tl *Locker) TryLockWithOwner(owner string) error {
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
	ctx, cancel := utils.GetContextWithTimeout()
	defer cancel()
	leaseGrantResp, err = tl.lease.Grant(ctx, 30)
	if err != nil {
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLock - TryLock] lease grand error:" + err.Error()
		return errObj
	}
	// 自动续租
	// 创建一个cancel context用来取消自动续租
	cancelCtx, cancelFunc = context.WithCancel(context.TODO())
	if keepRespChan, err = tl.lease.KeepAlive(cancelCtx, leaseGrantResp.ID); err != nil {
		cancelFunc()
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
		Then(clientv3.OpPut(tl.key, owner, clientv3.WithLease(leaseGrantResp.ID))).
		Else(clientv3.OpGet(tl.key))

	// 提交事务
	if txnResp, err = txn.Commit(); err != nil {
		failFunc()
		errObj = errors.ErrInternalError
		errObj.Log = "[TaskLock - TryLock] txn commit error:" + err.Error()
		return errObj
	}

	// 成功返回 失败的话释放租约
	if !txnResp.Succeeded {
		// 事务没有执行成功
		// 分布式场景下相同的agent(锁持有者)可以进行锁继承操作，如果发现上一个加锁的agent与本次请求的agent ip相同，则允许继承锁
		// 导致锁需要继承的原因可能是持有锁的center突然宕机，从而导致agent重新尝试连接其他center进行加锁，但是锁又被lease keep在一定的ttl中没有及时释放
		if len(txnResp.Responses) == 0 ||
			len(txnResp.Responses[0].GetResponseRange().Kvs) == 0 ||
			string(txnResp.Responses[0].GetResponseRange().Kvs[0].Value) != owner {
			failFunc()
			return errors.ErrLockAlreadyRequired
		}

		ctx, cancel := utils.GetContextWithTimeout()
		defer cancel()
		if _, err = tl.kv.Put(ctx, tl.key, owner, clientv3.WithLease(leaseGrantResp.ID)); err != nil {
			failFunc()
			return err
		}
	}

	tl.leaseID = leaseGrantResp.ID
	tl.cancelFunc = cancelFunc
	tl.isLocked.Store(true)
	clientlockers.setLease(tl.leaseID, tl)
	return nil
}

func (tl *Locker) LockExist() (bool, error) {
	ctx, cancel := utils.GetContextWithTimeout()
	defer cancel()
	resp, err := tl.kv.Get(ctx, tl.key)
	if err != nil {
		return false, err
	}
	return len(resp.Kvs) > 0, nil
}

// Unlock 释放锁
func (tl *Locker) Unlock() {
	if tl.isLocked.Load() {
		tl.isLocked.Store(false)
		tl.cancelFunc() // 取消锁协成自动续租
		ctx, cancel := utils.GetContextWithTimeout()
		defer cancel()
		_, _ = tl.lease.Revoke(ctx, tl.leaseID)
		clientlockers.deleteLease(tl.leaseID)
	}
}
