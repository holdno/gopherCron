package app

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/infra"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/utils"
	etcdregister "github.com/spacegrower/watermelon/infra/register/etcd"
	"github.com/spacegrower/watermelon/infra/resolver/etcd"
	"github.com/spacegrower/watermelon/infra/wlog"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func (a *app) RemoveClientRegister(client string) error {
	list, err := a.GetCenterSrvList()
	if err != nil {
		return err
	}
	removed := false
	for _, v := range list {
		if strings.Contains(v.addr, a.GetIP()) {
			stream := a.StreamManager().GetStreamsByHost(v.addr)
			if stream != nil {
				stream.Cancel()
			}
			continue
		}
		{
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
			defer cancel()
			resp, err := v.RemoveStream(ctx, &cronpb.RemoveStreamRequest{
				Client: client,
			})
			if err != nil {
				v.Close()
				return errors.NewError(http.StatusInternalServerError, "failed to remove stream")
			}
			v.Close()
			if resp.Result {
				removed = true
				break
			}
		}
	}

	if !removed {
		return errors.NewError(http.StatusInternalServerError, "failed to remove stream: not found")
	}
	return nil
}

func (a *app) DispatchAgentJob(region string, projectID int64, withStream ...*Stream) error {
	mtimer := a.metrics.CustomHistogramSet("dispatch_agent_jobs")
	defer mtimer.ObserveDuration()
	preKey := common.BuildKey(projectID, "")
	var (
		err     error
		getResp *clientv3.GetResponse
	)
	if err := utils.RetryFunc(5, func() error {
		if getResp, err = a.etcd.KV().Get(context.TODO(), preKey, clientv3.WithPrefix()); err != nil {
			return err
		}
		return nil
	}); err != nil {
		warningErr := a.Warning(warning.WarningData{
			Data:      fmt.Sprintf("[agent - TaskWatcher] etcd kv get error: %s, projectid: %d", err.Error(), projectID),
			Type:      warning.WarningTypeSystem,
			AgentIP:   a.GetIP(),
			ProjectID: projectID,
		})
		if warningErr != nil {
			wlog.Error(fmt.Sprintf("[agent - TaskWatcher] failed to push warning, %s", err.Error()))
		}
		return err
	}

	if len(withStream) == 0 {
		streams := a.StreamManager().GetStreams(region, projectID, cronpb.Agent_ServiceDesc.ServiceName)
		if streams == nil {
			return fmt.Errorf("failed to get grpc streams")
		}
		for _, v := range streams {
			withStream = append(withStream, v)
		}
	}

	wlog.Info("dispatch agent job", zap.String("region", region), zap.Int64("project_id", projectID), zap.Int("streams", len(withStream)), zap.Int("tasks", len(getResp.Kvs)))
	for _, kvPair := range getResp.Kvs {
		// if task, err := common.Unmarshal(kvPair.Value); err == nil {
		// 	continue
		// }
		// taskEvent = common.BuildTaskEvent(common.TASK_EVENT_SAVE, task)
		// 将所有任务加入调度队列
		// a.scheduler.PushEvent(taskEvent)
		if strings.Contains(string(kvPair.Key), "t_flow_ack") {
			continue
		}
		for _, v := range withStream {
			if err = v.stream.Send(&cronpb.Event{
				Version:   "v1",
				Type:      common.REMOTE_EVENT_PUT,
				Value:     kvPair.Value,
				EventTime: time.Now().Unix(),
			}); err != nil {
				wlog.Info("failed to dispatch agent job", zap.String("host", fmt.Sprintf("%s:%d", v.Host, v.Port)), zap.Error(err))
				return err
			}
		}
	}
	return nil
}

type AgentClient struct {
	cronpb.AgentClient
	addr   string
	cancel func()
}

func (a *AgentClient) Close() {
	if a.cancel != nil {
		a.cancel()
	}
}

func (a *app) getAgentAddrs(region string, projectID int64) ([]etcd.FindedResult[infra.NodeMetaRemote], error) {
	mtimer := a.metrics.CustomHistogramSet("get_agents_list")
	defer mtimer.ObserveDuration()
	finder := etcd.NewFinder[infra.NodeMetaRemote](infra.ResolveEtcdClient())
	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
	addrs, _, err := finder.FindAll(ctx, filepath.ToSlash(filepath.Join(etcdregister.GetETCDPrefixKey(), "gophercron", strconv.FormatInt(projectID, 10), cronpb.Agent_ServiceDesc.ServiceName))+"/")
	if err != nil {
		a.metrics.CustomErrorInc("find_agents_error", fmt.Sprintf("%s_%d", region, projectID), err.Error())
		return nil, err
	}
	return addrs, nil
}

func (a *app) FindAgents(region string, projectID int64) ([]*AgentClient, error) {
	addrs, err := a.getAgentAddrs(region, projectID)
	if err != nil {

		return nil, err
	}
	var list []*AgentClient
	for _, addr := range addrs {
		conn, err := a.connPool.Get("agent_" + addr.Address())
		var (
			client *AgentClient
		)
		if err != nil {
			ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
			defer cancel()
			cc, err := grpc.DialContext(ctx, addr.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return nil, fmt.Errorf("failed to connect agent %s, error: %s", addr.Address(), err.Error())
			}
			client = &AgentClient{
				AgentClient: cronpb.NewAgentClient(cc),
				addr:        addr.Address(),
				cancel: func() {
					cc.Close()
				},
			}
		} else {
			client = &AgentClient{
				AgentClient: cronpb.NewAgentClient(conn.Conn()),
				addr:        addr.Address(),
				cancel: func() {
					a.connPool.Put("agent_"+addr.Address(), conn)
				},
			}
		}

		list = append(list, client)
	}

	return list, nil
}

type CenterClient struct {
	cronpb.CenterClient
	cancel func()
	addr   string
}

func (c *CenterClient) Close() {
	if c.cancel != nil {
		c.cancel()
	}
}

func (a *app) GetCenterSrvList() ([]*CenterClient, error) {
	mtimer := a.metrics.CustomHistogramSet("get_center_srv_list")
	defer mtimer.ObserveDuration()
	finder := etcd.MustSetupEtcdFinder()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
	findKey := filepath.ToSlash(filepath.Join(etcdregister.GetETCDPrefixKey(), "gophercron", "0", cronpb.Center_ServiceDesc.ServiceName)) + "/"
	addrs, _, err := finder.FindAll(ctx, findKey)
	if err != nil {
		a.metrics.CustomErrorInc("find_centers_error", findKey, err.Error())
		return nil, err
	}

	var centers []*CenterClient

	for _, addr := range addrs {
		var (
			cc        *grpc.ClientConn
			centerSrv *CenterClient
		)
		idle, err := a.connPool.Get(addr.Address())
		if err != nil {
			wlog.Error("failed to get client conn from conn-pool", zap.Error(err))
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
			defer cancel()
			cc, err = grpc.DialContext(ctx, addr.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return nil, fmt.Errorf("failed to connect center %s, error: %s", addr.Address(), err.Error())
			}
			centerSrv = &CenterClient{
				CenterClient: cronpb.NewCenterClient(cc),
				addr:         addr.Address(),
				cancel: func() {
					cc.Close()
				},
			}
		} else {
			cc = idle.Conn()
			centerSrv = &CenterClient{
				CenterClient: cronpb.NewCenterClient(cc),
				addr:         addr.Address(),
				cancel: func() {
					a.connPool.Put(addr.Address(), idle)
				},
			}
		}
		centers = append(centers, centerSrv)
	}
	return centers, nil
}

func (a *app) DispatchEvent(event *cronpb.SendEventRequest) error {
	mtimer := a.metrics.CustomHistogramSet("dispatch_event")
	defer mtimer.ObserveDuration()
	centers, err := a.GetCenterSrvList()
	if err != nil {
		return err
	}

	for _, v := range centers {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
		defer cancel()
		if _, err := v.SendEvent(ctx, event); err != nil {
			v.Close()
			a.metrics.CustomErrorInc("send_event_error", v.addr, err.Error())
			return fmt.Errorf("failed to send event to %s, error: %s", v.addr, err.Error())
		}
		v.Close()
	}
	return nil
}
