package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/holdno/gopherCron/utils"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/holdno/gopherCron/common"
)

func (agent *client) OnCommand(f func(command string)) {
	var (
		err     error
		preKey  = common.BuildAgentRegisteKey(agent.localip)
		getResp *clientv3.GetResponse
	)

	if err = utils.RetryFunc(5, func() error {
		if getResp, err = agent.etcd.KV().Get(context.TODO(), preKey, clientv3.WithPrefix()); err != nil {
			return err
		}
		return nil
	}); err != nil {
		err = agent.Warning(WarningData{
			Data:    fmt.Sprintf("etcd kv get error: %s, client_ip: %s", err.Error(), agent.localip),
			Type:    WarningTypeSystem,
			AgentIP: agent.GetIP(),
		})
		if err != nil {
			agent.logger.Errorf("failed to push warning, %s", err.Error())
		}
		return
	}

	watchResp := agent.etcd.Watcher().Watch(context.TODO(), strings.TrimRight(preKey, "/"),
		clientv3.WithRev(getResp.Header.Revision+1), clientv3.WithPrefix())
	agent.Go(func() {
		for resp := range watchResp {
			for _, watchEvent := range resp.Events {
				switch watchEvent.Type {
				case mvccpb.PUT:
					command := common.ExtractAgentCommand(string(watchEvent.Kv.Key))
					f(command)
				case mvccpb.DELETE:
				default:
					agent.logger.Warnf("the current version can not support this event, event: %d", watchEvent.Type)
				}
			}
		}
	})
}
