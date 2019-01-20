package etcd

import (
	"context"
	"fmt"
	"time"

	"ojbk.io/gopherCron/utils"

	"ojbk.io/gopherCron/config"

	"ojbk.io/gopherCron/common"

	"github.com/coreos/etcd/clientv3"
)

func (m *TaskManager) Register(config *config.EtcdConf) {
	var (
		err        error
		cancelCtx  context.Context
		cancelFunc context.CancelFunc
	)

	common.LocalIP, _ = utils.GetLocalIP()

	if common.LocalIP == "" {
		common.LocalIP = "未知IP节点"
	}
	for _, v := range config.Projects {
		go func(projectID string) {
			var (
				regKey             string
				leaseGrantResp     *clientv3.LeaseGrantResponse
				leaseKeepAliveChan <-chan *clientv3.LeaseKeepAliveResponse
				leaseKeepAliveResp *clientv3.LeaseKeepAliveResponse
			)
			regKey = common.BuildRegisterKey(projectID, common.LocalIP)
			fmt.Println("register key", regKey)
			for {
				// 创建租约
				if leaseGrantResp, err = Manager.lease.Grant(context.TODO(), 5); err != nil {
					goto RETRY
				}

				// 自动续租
				if leaseKeepAliveChan, err = Manager.lease.KeepAlive(context.TODO(), leaseGrantResp.ID); err != nil {
					goto RETRY
				}

				cancelCtx, cancelFunc = context.WithCancel(context.TODO())

				// 注册到etcd
				if _, err = Manager.kv.Put(cancelCtx, regKey, "", clientv3.WithLease(leaseGrantResp.ID)); err != nil {
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
				if cancelFunc == nil {
					cancelFunc()
				}
			}
		}(v)
	}
}
