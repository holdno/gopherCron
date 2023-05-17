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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func (a *app) RemoveClientRegister(client string) error {
	list, err := a.GetCenterSrvList()
	if err != nil {
		return err
	}

	defer func() {
		for _, v := range list {
			v.Close()
		}
	}()

	removed := false

	disposeOne := func(v *CenterClient) (*cronpb.Result, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
		defer cancel()
		resp, err := v.RemoveStream(ctx, &cronpb.RemoveStreamRequest{
			Client: client,
		})
		if err != nil {
			return nil, errors.NewError(http.StatusInternalServerError, "failed to remove stream")
		}
		return resp, nil
	}

	for _, v := range list {
		if strings.Contains(v.addr, a.GetIP()) {
			stream := a.StreamManager().GetStreamsByHost(v.addr)
			if stream != nil {
				stream.Cancel()
			}
			continue
		}
		resp, err := disposeOne(v)
		if err != nil {
			return err
		}

		if resp.Result {
			removed = true
			break
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
		a.metrics.CustomInc("find_agents_error", fmt.Sprintf("%s_%d", region, projectID), err.Error())
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

		ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
		defer cancel()
		gopts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
		dialAddress := addr.Address()
		if addr.Attr().Region != a.cfg.Micro.Region {
			dialAddress, gopts = BuildProxyDialerInfo(ctx, addr.Attr().Region, addr.Address(), gopts)
		}
		cc, err := grpc.DialContext(ctx, dialAddress, gopts...)
		if err != nil {
			return nil, fmt.Errorf("failed to connect agent %s, error: %s", addr.Address(), err.Error())
		}
		client := &AgentClient{
			AgentClient: cronpb.NewAgentClient(cc),
			addr:        addr.Address(),
			cancel: func() {
				cc.Close()
			},
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
	finder := etcd.NewFinder[infra.NodeMeta](infra.ResolveEtcdClient())
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
	findKey := filepath.ToSlash(filepath.Join(etcdregister.GetETCDPrefixKey(), "gophercron", "0", cronpb.Center_ServiceDesc.ServiceName)) + "/"
	addrs, _, err := finder.FindAll(ctx, findKey)
	if err != nil {
		a.metrics.CustomInc("find_centers_error", findKey, err.Error())
		return nil, err
	}

	var centers []*CenterClient

	for _, addr := range addrs {
		var (
			cc        *grpc.ClientConn
			centerSrv *CenterClient
		)
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
		defer cancel()
		gopts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
		dialAddress := addr.Address()
		if addr.Attr().Region != a.cfg.Micro.Region {
			dialAddress, gopts = BuildProxyDialerInfo(ctx, addr.Attr().Region, addr.Address(), gopts)
		}
		cc, err = grpc.DialContext(ctx, dialAddress, gopts...)
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

	defer func() {
		for _, v := range centers {
			v.Close()
		}
	}()

	dispatchOne := func(v *CenterClient) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
		defer cancel()
		if _, err := v.SendEvent(ctx, event); err != nil {
			a.metrics.CustomInc("send_event_error", v.addr, err.Error())
			return fmt.Errorf("failed to send event to %s, error: %s", v.addr, err.Error())
		}
		return nil
	}

	for _, v := range centers {
		if err := dispatchOne(v); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) GetGrpcDirector() func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
	return func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		addrs := md.Get(common.GOPHERCRON_PROXY_TO_MD_KEY)
		dialOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
		if len(addrs) > 0 {
			addr := addrs[0]
			wlog.Debug("got proxy request", zap.String("proxy_to", addr), zap.String("full_method", fullMethodName))
			cc, err := grpc.Dial(addr, dialOptions...)
			if err != nil {
				return nil, nil, err
			}

			md.Set(common.GOPHERCRON_AGENT_IP_MD_KEY, "gophercron_proxy")
			outCtx := metadata.NewOutgoingContext(ctx, md.Copy())
			return outCtx, cc, nil
		} else {
			projectIDs := md.Get(common.GOPHERCRON_PROXY_PROJECT_MD_KEY)
			if len(projectIDs) == 0 {
				return nil, nil, status.Error(codes.Unknown, "undefined project id")
			}
			projectID, err := strconv.ParseInt(projectIDs[0], 10, 64)
			if err != nil {
				return nil, nil, status.Error(codes.Unknown, "invalid project id")
			}
			ls := strings.Split(fullMethodName, "/")
			if len(ls) != 3 {
				return nil, nil, status.Error(codes.Unknown, "unknown full method name")
			}
			newCC := infra.NewClientConn()
			cc, err := newCC(ls[1], newCC.WithSystem(projectID), newCC.WithOrg(a.cfg.Micro.OrgID), newCC.WithRegion(a.cfg.Micro.Region),
				newCC.WithServiceResolver(infra.MustSetupEtcdResolver()),
				newCC.WithGrpcDialOptions(dialOptions...))
			if err != nil {
				return nil, nil, err
			}
			outCtx := metadata.NewOutgoingContext(ctx, md.Copy())
			return outCtx, cc.ClientConn, nil
		}
	}
}

func BuildProxyDialerInfo(ctx context.Context, region, address string, opts []grpc.DialOption) (dialAddress string, gopts []grpc.DialOption) {
	dialAddress = infra.ResolveProxy(region)
	if dialAddress == "" {
		wlog.Error("proxy address not found", zap.String("region", region))
	}
	genMetadata := func(ctx context.Context) context.Context {
		md, exist := metadata.FromOutgoingContext(ctx)
		if !exist {
			md = metadata.New(map[string]string{})
		}
		md.Set(common.GOPHERCRON_PROXY_TO_MD_KEY, address)
		return metadata.NewOutgoingContext(ctx, md)
	}
	gopts = append(opts, grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(genMetadata(ctx), method, req, reply, cc, opts...)
	}),
		grpc.WithStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return streamer(genMetadata(ctx), desc, cc, method, opts...)
		}))
	return dialAddress, gopts
}
