package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coreos/etcd/clientv3"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/config"
	"ojbk.io/gopherCron/utils"
)

type ClientInfo struct {
	ClientIP string `json:"client_ip"`
	Version  string `json:"version"`
}

// Register 注册agent
func (a *app) Register(config *config.EtcdConf) {
	var (
		err        error
		cancelCtx  context.Context
		cancelFunc context.CancelFunc
	)

	a.localip, _ = utils.GetLocalIP()

	if a.localip == "" {
		a.localip = "未知IP节点"
	}

	clientinfo, _ := json.Marshal(&ClientInfo{
		ClientIP: a.localip,
		Version:  a.GetVersion(),
	})

	for _, v := range config.Projects {
		go func(projectID int64) {
			var (
				regKey             string
				leaseGrantResp     *clientv3.LeaseGrantResponse
				leaseKeepAliveChan <-chan *clientv3.LeaseKeepAliveResponse
				leaseKeepAliveResp *clientv3.LeaseKeepAliveResponse
				ctx                context.Context
			)
			regKey = common.BuildRegisterKey(projectID, a.localip)
			for {
				ctx, _ = utils.GetContextWithTimeout()

				// 创建租约
				if leaseGrantResp, err = a.etcd.Lease().Grant(ctx, 10); err != nil {
					goto RETRY
				}

				// 自动续租
				if leaseKeepAliveChan, err = a.etcd.Lease().KeepAlive(context.TODO(), leaseGrantResp.ID); err != nil {
					goto RETRY
				}

				cancelCtx, cancelFunc = utils.GetContextWithTimeout()
				// 注册到etcd
				if _, err = a.etcd.KV().Put(cancelCtx, regKey, string(clientinfo), clientv3.WithLease(leaseGrantResp.ID)); err != nil {
					goto RETRY
				}

				for {
					select {
					case leaseKeepAliveResp = <-leaseKeepAliveChan:
						if leaseKeepAliveResp == nil {
							// 续租失败
							goto RETRY
						}
					}
				}

			RETRY:
				time.Sleep(time.Duration(1) * time.Second)
				if cancelFunc != nil {
					cancelFunc()
				}
			}
		}(v)
	}
}
