package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
)

type ClientInfo struct {
	ClientIP string `json:"client_ip"`
	Version  string `json:"version"`
}

func (a *client) startRegister(projectID int64, clientinfo string) {
	a.Go(func() {
		var (
			err                error
			regKey             string
			leaseGrantResp     *clientv3.LeaseGrantResponse
			leaseKeepAliveChan <-chan *clientv3.LeaseKeepAliveResponse
			leaseKeepAliveResp *clientv3.LeaseKeepAliveResponse
			ctx                context.Context
			cancelFunc         context.CancelFunc
		)
		a.logger.Infof("[agent - Register] new project agent register, project_id: %d", projectID)
		regKey = common.BuildRegisterKey(projectID, a.localip)
		for {
			ctx, _ = utils.GetContextWithTimeout()

			// 创建租约
			if leaseGrantResp, err = a.etcd.Lease().Grant(ctx, 10); err != nil {
				goto RETRY
			}

			// 自动续租
			ctx, cancelFunc = context.WithCancel(context.TODO())
			if leaseKeepAliveChan, err = a.etcd.Lease().KeepAlive(ctx, leaseGrantResp.ID); err != nil {
				goto RETRY
			}

			// 注册到etcd
			if _, err = a.etcd.KV().Put(context.TODO(), regKey, clientinfo, clientv3.WithLease(leaseGrantResp.ID)); err != nil {
				goto RETRY
			}

			for {
				select {
				case leaseKeepAliveResp = <-leaseKeepAliveChan:
					if leaseKeepAliveResp == nil {
						// 续租失败
						goto RETRY
					}
				case <-a.daemon.WaitRemoveSignal(projectID):
					cancelFunc()
					a.logger.Infof("[agent - Register] stop to registing project %d", projectID)
					return
				}
			}

		RETRY:
			time.Sleep(time.Duration(1) * time.Second)
			if cancelFunc != nil {
				cancelFunc()
			}
		}
	})
}

// Register 注册agent
func (a *client) Register(projects []int64) {
	//if len(projects) == 0 {
	//	return
	//}
	a.localip, _ = utils.GetLocalIP()

	if a.localip == "" {
		a.localip = "未知IP节点"
	}

	clientinfo, _ := json.Marshal(&ClientInfo{
		ClientIP: a.localip,
		Version:  a.GetVersion(),
	})

	fmt.Println(string(clientinfo))

	for _, projectID := range projects {
		a.startRegister(projectID, string(clientinfo))
	}
}
