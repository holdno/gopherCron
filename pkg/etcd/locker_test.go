package etcd

import (
	"testing"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/pkg/infra"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestTryLockWithOwner(t *testing.T) {
	infra.RegisterEtcdClient(clientv3.Config{
		Endpoints: []string{"localhost:2379"},
	})
	m, err := Connect(&config.EtcdConf{
		Service: []string{"localhost:2379"},
		Prefix:  "/gophercron-local-test",
	})

	if err != nil {
		t.Fatal(err)
	}

	l := m.GetTaskLocker(&common.TaskInfo{
		ProjectID: 1,
		TaskID:    "test_task_id",
	})

	defer l.Unlock()

	if err = l.TryLockWithOwner("127.0.0.1"); err != nil {
		t.Fatal("lock 1", err)
	}

	time.Sleep(time.Second * 2)

	if err = l.TryLockWithOwner("127.0.0.1"); err != nil {
		t.Fatal("lock 2", err)
	}

	if err = l.TryLockWithOwner("127.0.0.11"); err != nil {
		t.Log("lock 3 failure")
	}

	t.Log("success")
}
