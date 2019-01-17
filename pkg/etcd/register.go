package etcd

import (
	"context"
	"net"
	"time"

	"ojbk.io/gopherCron/config"

	"ojbk.io/gopherCron/common"

	"ojbk.io/gopherCron/errors"

	"github.com/coreos/etcd/clientv3"
)

func (m *TaskManager) Register(config *config.EtcdConf) {
	var (
		localIP    string
		err        error
		cancelCtx  context.Context
		cancelFunc context.CancelFunc
	)

	localIP, _ = getLocalIP()

	if localIP == "" {
		localIP = "未知IP节点"
	}
	for _, v := range config.Projects {
		go func(project string) {
			var (
				regKey             string
				leaseGrantResp     *clientv3.LeaseGrantResponse
				leaseKeepAliveChan <-chan *clientv3.LeaseKeepAliveResponse
				leaseKeepAliveResp *clientv3.LeaseKeepAliveResponse
			)
			regKey = common.BuildRegisterKey(project, localIP)
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

func getLocalIP() (string, error) {
	var (
		addrs   []net.Addr
		addr    net.Addr
		err     error
		ipNet   *net.IPNet
		isIpNet bool
	)

	if addrs, err = net.InterfaceAddrs(); err != nil {
		return "", err
	}

	// 获取第一个非IO的网卡
	for _, addr = range addrs {
		// ipv4  ipv6
		// 如果能反解成ip地址 则为我们需要的地址
		if ipNet, isIpNet = addr.(*net.IPNet); isIpNet && !ipNet.IP.IsLoopback() {
			// 是ip地址 不是 unix socket地址
			// 继续判断 是ipv4 还是 ipv6
			// 跳过ipv6
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}
	return "", errors.ErrLocalIPNotFound
}
