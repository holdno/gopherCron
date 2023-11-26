package test

import (
	"context"
	"fmt"
	"time"

	"github.com/spacegrower/watermelon/infra/wlog"
	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "echo current system version",
		Run: func(c *cobra.Command, args []string) {
			RunTest()
		},
	}
	return cmd
}

func RunTest() {

	wlog.SetGlobalLogger(wlog.NewLogger(&wlog.Config{Level: wlog.DebugLevel}))
	prefix := "/gophercron"
	client, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"digital-etcd01.bs58i.baishancdnx.com:2379", "digital-etcd02.bs58i.baishancdnx.com:2379", "digital-etcd03.bs58i.baishancdnx.com:2379"},
		Username:  "gophercron",
		Password:  "gophercron",
	})
	if err != nil {
		panic(err)
	}

	time.Sleep(time.Minute * 6)

	resp := client.Watch(context.Background(), fmt.Sprintf("%s/registry/service/gophercron/44/", prefix), clientv3.WithPrefix())
	for {
		select {
		case resp, ok := <-resp:
			if !ok {
				fmt.Println("watch chan is closed")
			}

			fmt.Println(resp.Canceled, resp.Err())

			if resp.Err() != nil {
				return
			}
		}
	}
	// time.Sleep(time.Second * 3)
	// cancel()
}
