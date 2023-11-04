package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gorhill/cronexpr"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestCron(t *testing.T) {
	e, err := cronexpr.Parse("0 51 18 * * 5 *")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(e.Next(time.Now()))
}

func TestCronNextN(t *testing.T) {
	e, err := cronexpr.Parse("* * * * * * 2000")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(e.NextN(time.Now(), 2))
}

func TestEtcdLease(t *testing.T) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"localhost:2379"},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Grant(context.Background(), 60)
	if err != nil {
		t.Fatal(err)
	}

	ch, err := client.KeepAlive(context.Background(), resp.ID)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for v := range ch {
			fmt.Println(v.String())
		}
		fmt.Println("lease 1 down")
	}()

	testKey := "/gophercron-test/registry/127.0.0.1"
	if _, err = client.Put(context.Background(), testKey, "test", clientv3.WithLease(resp.ID)); err != nil {
		t.Fatal(err)
	}

	respget, err := client.Get(context.Background(), testKey)
	if err != nil {
		t.Fatal(err)
	}

	if len(respget.Kvs) == 0 {
		fmt.Println("get empty")
	}

	resp2, err := client.Grant(context.Background(), 60)
	if err != nil {
		t.Fatal(err)
	}

	ch2, err := client.KeepAlive(context.Background(), resp2.ID)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for v := range ch2 {
			fmt.Println(v.String())
		}
		fmt.Println("lease 2 down")
	}()

	if _, err = client.Put(context.Background(), testKey, "test", clientv3.WithLease(resp2.ID)); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second * 3)

	if _, err = client.Revoke(context.Background(), resp.ID); err != nil {
		t.Fatal(err)
	}
	fmt.Println("revoked lease")

	respget, err = client.Get(context.Background(), testKey)
	if err != nil {
		t.Fatal(err)
	}

	if len(respget.Kvs) == 0 {
		fmt.Println("get empty")
	}

	for _, kv := range respget.Kvs {
		fmt.Println("get", kv.Value)
	}

	t.Log("finished")
}

func TestRegisterLikeLock(t *testing.T) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"localhost:2379"},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Grant(context.Background(), 60)
	if err != nil {
		t.Fatal(err)
	}

	ch, err := client.KeepAlive(context.Background(), resp.ID)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for v := range ch {
			fmt.Println(v.String())
		}
		fmt.Println("lease 1 down")
	}()

	defer client.Revoke(context.Background(), resp.ID)

	testKey := "/gophercron-test/registry/127.0.0.1"
	testKey2 := "/gophercron-test/registry/127.0.0.2"
	testKey3 := "/gophercron-test/registry/127.0.0.3"
	txn := client.Txn(context.TODO())
	// 事务抢锁
	txn.If(clientv3.Compare(clientv3.CreateRevision(testKey), "=", 0)).
		Then(clientv3.OpPut(testKey, "register info", clientv3.WithLease(resp.ID)))

	// 提交事务
	txnResp, err := txn.Commit()
	if err != nil {
		t.Fatal(err)
	}
	// 成功返回 失败的话释放租约
	if !txnResp.Succeeded {
		t.Fatal("unsucceeded")
	}

	txn = client.Txn(context.TODO())
	txn.If(clientv3.Compare(clientv3.CreateRevision(testKey2), "=", 0)).
		Then(clientv3.OpPut(testKey2, "register info", clientv3.WithLease(resp.ID)))

	// 提交事务
	txnResp, err = txn.Commit()
	if err != nil {
		t.Fatal(err)
	}
	// 成功返回 失败的话释放租约
	if !txnResp.Succeeded {
		t.Fatal("unsucceeded")
	}
	txn = client.Txn(context.TODO())
	txn.If(clientv3.Compare(clientv3.CreateRevision(testKey3), "=", 0)).
		Then(clientv3.OpPut(testKey3, "register info", clientv3.WithLease(resp.ID)))

	// 提交事务
	txnResp, err = txn.Commit()
	if err != nil {
		t.Fatal(err)
	}
	// 成功返回 失败的话释放租约
	if !txnResp.Succeeded {
		t.Fatal("unsucceeded")
	}
	t.Log("success")
}
