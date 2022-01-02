package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/coreos/etcd/clientv3"
	v3 "github.com/coreos/etcd/clientv3"
	recipe "github.com/coreos/etcd/contrib/recipes"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

// Queue implements a multi-reader, multi-writer distributed queue.
type Queue struct {
	client *v3.Client
	ctx    context.Context

	keyPrefix string
}

func New(ctx context.Context, client *v3.Client, keyPrefix string) *Queue {
	return &Queue{client, ctx, keyPrefix}
}

func (q *Queue) Enqueue(val string) error {
	_, err := newUniqueKV(q.client, q.keyPrefix, val)
	return err
}

// Dequeue returns Enqueue()'d elements in FIFO order. If the
// queue is empty, Dequeue blocks until elements are available.
func (q *Queue) Dequeue() (string, func(), error) {
	// TODO: fewer round trips by fetching more than one key
	resp, err := q.client.Get(q.ctx, q.keyPrefix, v3.WithFirstRev()...)
	if err != nil {
		return "", nil, err
	}

	ackFunc := func() {}

	kv, err := claimFirstKey(q.client, resp.Kvs)
	if err != nil {
		return "", ackFunc, err
	} else if kv != nil {
		return string(kv.Value), nil, nil
	} else if resp.More {
		// missed some items, retry to read in more
		return q.Dequeue()
	}

	// nothing yet; wait on elements
	ev, err := WaitPrefixEvents(
		q.ctx,
		q.client,
		q.keyPrefix,
		resp.Header.Revision,
		[]mvccpb.Event_EventType{mvccpb.PUT})
	if err != nil {
		return "", ackFunc, err
	}

	ackFunc = func() {
		deleteRevKey(q.client, string(ev.Kv.Key), ev.Kv.ModRevision)
	}

	return string(ev.Kv.Value), ackFunc, err
}

type RemoteKV struct {
	kv  v3.KV
	key string
	rev int64
	val string
}

func newKV(kv v3.KV, key, val string, leaseID v3.LeaseID) (*RemoteKV, error) {
	rev, err := putNewKV(kv, key, val, leaseID)
	if err != nil {
		return nil, err
	}
	return &RemoteKV{kv, key, rev, val}, nil
}

func newUniqueKV(kv v3.KV, prefix string, val string) (*RemoteKV, error) {
	for {
		newKey := fmt.Sprintf("%s/%v", prefix, time.Now().UnixNano())
		rev, err := putNewKV(kv, newKey, val, 0)
		if err == nil {
			return &RemoteKV{kv, newKey, rev, val}, nil
		}
		if err != recipe.ErrKeyExists {
			return nil, err
		}
	}
}

// putNewKV attempts to create the given key, only succeeding if the key did
// not yet exist.
func putNewKV(kv v3.KV, key, val string, leaseID v3.LeaseID) (int64, error) {
	cmp := v3.Compare(v3.Version(key), "=", 0)
	req := v3.OpPut(key, val, v3.WithLease(leaseID))
	txnresp, err := kv.Txn(context.TODO()).If(cmp).Then(req).Commit()
	if err != nil {
		return 0, err
	}
	if !txnresp.Succeeded {
		return 0, recipe.ErrKeyExists
	}
	return txnresp.Header.Revision, nil
}

// deleteRevKey deletes a key by revision, returning false if key is missing
func deleteRevKey(kv v3.KV, key string, rev int64) (bool, error) {
	cmp := v3.Compare(v3.ModRevision(key), "=", rev)
	req := v3.OpDelete(key)
	txnresp, err := kv.Txn(context.TODO()).If(cmp).Then(req).Commit()
	if err != nil {
		return false, err
	} else if !txnresp.Succeeded {
		return false, nil
	}
	return true, nil
}

func claimFirstKey(kv v3.KV, kvs []*mvccpb.KeyValue) (*mvccpb.KeyValue, error) {
	for _, k := range kvs {
		ok, err := deleteRevKey(kv, string(k.Key), k.ModRevision)
		if err != nil {
			return nil, err
		} else if ok {
			return k, nil
		}
	}
	return nil, nil
}

func WaitPrefixEvents(ctx context.Context, c *clientv3.Client, prefix string, rev int64, evs []mvccpb.Event_EventType) (*clientv3.Event, error) {
	wc := c.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithRev(rev))
	if wc == nil {
		return nil, recipe.ErrNoWatcher
	}
	return waitEvents(wc, evs), nil
}

func waitEvents(wc clientv3.WatchChan, evs []mvccpb.Event_EventType) *clientv3.Event {
	i := 0
	for wresp := range wc {
		if err := wresp.Err(); err != nil {
			return nil
		}
		for _, ev := range wresp.Events {
			if ev.Type == evs[i] {
				i++
				if i == len(evs) {
					return ev
				}
			}
		}
	}
	return nil
}
