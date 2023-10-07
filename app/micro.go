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
	"github.com/holdno/gopherCron/jwt"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/infra"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/utils"

	etcdregister "github.com/spacegrower/watermelon/infra/register/etcd"
	"github.com/spacegrower/watermelon/infra/resolver/etcd"
	"github.com/spacegrower/watermelon/infra/wlog"
	"github.com/spacegrower/watermelon/pkg/safe"
	"github.com/ugurcsen/gods-generic/maps/hashmap"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func (a *app) RemoveClientRegister(client string) error {
	list, err := a.GetCenterSrvList()
	if err != nil {
		return err
	}

	// defer func() {
	// 	for _, v := range list {
	// 		v.Close()
	// 	}
	// }()

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
			stream := a.StreamManager().GetStreamsByHost(client)
			if stream != nil {
				stream.Cancel()
				continue
			}
			streamV2 := a.StreamManagerV2().GetStreamsByHost(client)
			if streamV2 != nil {
				streamV2.Cancel()
				continue
			}
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

func (a *app) DispatchAgentJob(projectID int64) error {
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
		warningErr := a.Warning(warning.NewSystemWarningData(warning.SystemWarning{
			Endpoint: a.GetIP(),
			Type:     warning.SERVICE_TYPE_CENTER,
			Message:  fmt.Sprintf("center-service: %s, etcd kv get error: %s, projectid: %d", a.GetIP(), err.Error(), projectID),
		}))
		if warningErr != nil {
			wlog.Error(fmt.Sprintf("[agent - TaskWatcher] failed to push warning, %s", err.Error()))
		}
		return err
	}

	var (
		streamsV1 []*Stream[*cronpb.Event]
		streamsV2 []*Stream[*cronpb.ServiceEvent]
	)
	streams := a.StreamManager().GetStreams(projectID, cronpb.Agent_ServiceDesc.ServiceName)
	if streams != nil {
		for _, v := range streams {
			streamsV1 = append(streamsV1, v)
		}
	} else {
		streams := a.StreamManagerV2().GetStreams(projectID, cronpb.Agent_ServiceDesc.ServiceName)
		if streams == nil {
			return fmt.Errorf("failed to get grpc streams")
		}
		for _, v := range streams {
			streamsV2 = append(streamsV2, v)
		}
	}

	wlog.Info("dispatch agent job", zap.Int64("project_id", projectID), zap.Int("streams", len(streamsV1)+len(streamsV2)), zap.Int("tasks", len(getResp.Kvs)))
	for _, kvPair := range getResp.Kvs {
		// if task, err := common.Unmarshal(kvPair.Value); err == nil {
		// 	continue
		// }
		// taskEvent = common.BuildTaskEvent(common.TASK_EVENT_SAVE, task)
		// 将所有任务加入调度队列
		// a.scheduler.PushEvent(taskEvent)
		if common.IsStatusKey(string(kvPair.Key)) || common.IsAckKey(string(kvPair.Key)) {
			continue
		}
		for _, v := range streamsV1 {
			if err = v.stream.Send(&cronpb.Event{
				Version:   common.VERSION_TYPE_V1,
				Type:      common.REMOTE_EVENT_PUT,
				Value:     kvPair.Value,
				EventTime: time.Now().Unix(),
			}); err != nil {
				wlog.Info("failed to dispatch agent job", zap.String("host", fmt.Sprintf("%s:%d", v.Host, v.Port)), zap.Error(err))
				return err
			}
		}
		for _, v := range streamsV2 {
			var err error
			func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				defer cancel()
				_, err = a.StreamManagerV2().SendEventWaitResponse(ctx, v, &cronpb.ServiceEvent{
					Id:        utils.GetStrID(),
					Type:      cronpb.EventType_EVENT_REGISTER_REPLY,
					EventTime: time.Now().Unix(),
					Event: &cronpb.ServiceEvent_RegisterReply{
						RegisterReply: &cronpb.Event{
							Version:   common.VERSION_TYPE_V1,
							Type:      common.REMOTE_EVENT_PUT,
							Value:     kvPair.Value,
							EventTime: time.Now().Unix(),
						},
					},
				})
			}()
			if err != nil {
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
	cse    string
	cancel func()
}

func (a *AgentClient) Close() {
	if a.cancel != nil {
		a.cancel()
	}
}

func (a *app) GetAgentClient(region string, projectID int64) (*AgentClient, error) {
	// client 的连接对象由调用时提供初始化

	newConn := infra.NewClientConn()
	cc, err := newConn(cronpb.Agent_ServiceDesc.ServiceName,
		newConn.WithRegion(region),
		newConn.WithSystem(projectID),
		newConn.WithOrg(a.cfg.Micro.OrgID),
		newConn.WithGrpcDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithPerRPCCredentials(a.author)),
		newConn.WithServiceResolver(infra.MustSetupEtcdResolver()))
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, fmt.Sprintf("连接agent失败，project_id: %d", projectID)).WithLog(err.Error())
	}

	client := &AgentClient{
		AgentClient: cronpb.NewAgentClient(cc),
		addr:        fmt.Sprintf("resolve_%s_%d", region, projectID),
		cancel: func() {
			cc.Close()
		},
	}

	return client, nil
}

func (a *app) getAgentAddrs(region string, projectID int64) ([]etcd.FindedResult[infra.NodeMeta], error) {
	mtimer := a.metrics.CustomHistogramSet("get_agents_list")
	defer mtimer.ObserveDuration()
	finder := etcd.NewFinder[infra.NodeMeta](infra.ResolveEtcdClient())
	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
	addrs, _, err := finder.FindAll(ctx, filepath.ToSlash(filepath.Join(etcdregister.GetETCDPrefixKey(), "gophercron", strconv.FormatInt(projectID, 10), cronpb.Agent_ServiceDesc.ServiceName))+"/")
	if err != nil {
		a.metrics.CustomInc("find_agents_error", fmt.Sprintf("%s_%d", region, projectID), err.Error())
		return nil, err
	}
	return addrs, nil
}

func (a *app) GetAgentStream(region string, projectID int64) (*CenterClient, error) {
	// client 的连接对象由调用时提供初始化
	addrs, err := a.getAgentAddrs(region, projectID)
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		if addr.Attr().CenterServiceEndpoint != "" {
			dialAddress := addr.Attr().CenterServiceEndpoint
			ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
			defer cancel()
			gopts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithPerRPCCredentials(a.author)}
			if addr.Attr().CenterServiceRegion != a.cfg.Micro.Region {
				dialAddress, gopts = BuildProxyDialerInfo(ctx, addr.Attr().CenterServiceRegion, dialAddress, gopts)
			}
			cc, err := grpc.DialContext(ctx, dialAddress, gopts...)
			if err != nil {
				return nil, fmt.Errorf("failed to connect agent stream %s, error: %s", dialAddress, err.Error())
			}
			client := &CenterClient{
				CenterClient: cronpb.NewCenterClient(cc),
				addr:         addr.Address(),
				cancel: func() {
					cc.Close()
				},
			}
			return client, nil
		}
	}

	return nil, nil
}

func (a *app) FindAgentsV2(region string, projectID int64) ([]*CenterClient, error) {
	addrs, err := a.getAgentAddrs(region, projectID)
	if err != nil {
		return nil, err
	}
	var (
		list      []*CenterClient
		connCache = make(map[string]*grpc.ClientConn)
	)

	for _, addr := range addrs {
		err := func() error {
			if addr.Attr().CenterServiceEndpoint == "" {
				return nil
			}
			dialAddress := addr.Attr().CenterServiceEndpoint
			ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
			defer cancel()
			gopts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithPerRPCCredentials(a.author)}
			if addr.Attr().CenterServiceRegion != a.cfg.Micro.Region {
				dialAddress, gopts = BuildProxyDialerInfo(ctx, addr.Attr().CenterServiceRegion, dialAddress, gopts)
			}

			if _, exist := connCache[dialAddress]; !exist {
				cc, err := grpc.DialContext(ctx, dialAddress, gopts...)
				if err != nil {
					return fmt.Errorf("failed to connect agent stream %s, error: %s", dialAddress, err.Error())
				}
				connCache[dialAddress] = cc
			}
			cc := connCache[dialAddress]
			client := &CenterClient{
				CenterClient: cronpb.NewCenterClient(cc),
				addr:         addr.Address(),
				cancel: func() {
					cc.Close()
				},
			}
			list = append(list, client)
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}
	connCache = nil
	return list, nil
}

func (a *app) FindAgents(region string, projectID int64) ([]*AgentClient, error) {
	addrs, err := a.getAgentAddrs(region, projectID)
	if err != nil {
		return nil, err
	}
	var list []*AgentClient
	for _, addr := range addrs {
		err := func() error {
			ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
			defer cancel()
			gopts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithPerRPCCredentials(a.author)}
			dialAddress := addr.Address()
			if addr.Attr().Region != a.cfg.Micro.Region {
				dialAddress, gopts = BuildProxyDialerInfo(ctx, addr.Attr().Region, addr.Address(), gopts)
			}
			cc, err := grpc.DialContext(ctx, dialAddress, gopts...)
			if err != nil {
				return err
			}
			client := &AgentClient{
				AgentClient: cronpb.NewAgentClient(cc),
				addr:        addr.Address(),
				cancel: func() {
					cc.Close()
				},
			}
			list = append(list, client)
			return nil
		}()
		if err != nil {
			return nil, fmt.Errorf("failed to connect agent %s, error: %s", addr.Address(), err.Error())
		}
	}

	return list, nil
}

type CenterClient struct {
	cronpb.CenterClient
	cancel func()
	addr   string
	cc     *grpc.ClientConn
}

func (c *CenterClient) Close() {
	if c.cancel != nil {
		c.cancel()
	}
}

func resolveCenterService(a *app) {
	findKey := filepath.ToSlash(filepath.Join(etcdregister.GetETCDPrefixKey(), "gophercron", "0", cronpb.Center_ServiceDesc.ServiceName)) + "/"
	var opts []clientv3.OpOption
	opts = append(opts, clientv3.WithPrefix())
	if err := a.refreshCenterSrvList(); err != nil {
		panic(err)
	}
	for {
		wlog.Debug("start resolving others center service", zap.String("key", findKey))
		updates := a.etcd.Client().Watch(a.ctx, findKey, opts...)
		for {
			select {
			case <-a.ctx.Done():
			case ev, ok := <-updates:
				if !ok {
					// watcher closed
					wlog.Info("watch chan closed", zap.String("key", findKey))
					return
				}

				// some error occurred, re watch
				if ev.Err() != nil {
					wlog.Error("resolving center service with error",
						zap.Error(ev.Err()),
						zap.Bool("canceled", ev.Canceled))

					return
				}

				a.refreshCenterSrvList()
			}
		}
	}
}

func (a *app) GetCenterSrvList() ([]*CenterClient, error) {
	list := a.__centerConncets.Values()
	for _, v := range list {
		if v.cc.GetState() != connectivity.Ready {
			a.refreshCenterSrvList()
			continue
		}
	}

	list = a.__centerConncets.Values()
	return list, nil
}

func (a *app) refreshCenterSrvList() error {
	if a.__centerConncets == nil {
		a.__centerConncets = hashmap.New[string, *CenterClient]()
	}
	mtimer := a.metrics.CustomHistogramSet("get_center_srv_list")
	defer mtimer.ObserveDuration()
	finder := etcd.NewFinder[infra.NodeMeta](infra.ResolveEtcdClient())
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
	defer cancel()
	findKey := filepath.ToSlash(filepath.Join(etcdregister.GetETCDPrefixKey(), "gophercron", "0", cronpb.Center_ServiceDesc.ServiceName)) + "/"
	addrs, _, err := finder.FindAll(ctx, findKey)
	if err != nil {
		a.metrics.CustomInc("find_centers_error", findKey, err.Error())
		return err
	}

	centerMap := hashmap.New[string, *CenterClient]()
	for _, addr := range addrs {
		var (
			cc        *grpc.ClientConn
			centerSrv *CenterClient
		)

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.GetConfig().Deploy.Timeout)*time.Second)
		defer cancel()
		gopts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithPerRPCCredentials(a.author)}
		dialAddress := addr.Address()
		if addr.Attr().Region != a.cfg.Micro.Region {
			dialAddress, gopts = BuildProxyDialerInfo(ctx, addr.Attr().Region, addr.Address(), gopts)
		}
		cacheKey := fmt.Sprintf("%s_%s", dialAddress, addr.Address())
		if srv, exist := a.__centerConncets.Get(cacheKey); exist {
			centerMap.Put(cacheKey, srv)
			continue
		}
		cc, err = grpc.DialContext(ctx, dialAddress, gopts...)
		if err != nil {
			return fmt.Errorf("failed to connect center %s, error: %s", addr.Address(), err.Error())
		}
		centerSrv = &CenterClient{
			CenterClient: cronpb.NewCenterClient(cc),
			addr:         addr.Address(),
			cancel: func() {
				cc.Close()
			},
			cc: cc,
		}
		centerMap.Put(cacheKey, centerSrv)
	}

	old := a.__centerConncets
	for _, key := range old.Keys() {
		if _, exist := centerMap.Get(key); !exist {
			srv, _ := old.Get(key)
			srv.Close()
		}
	}
	a.__centerConncets = centerMap

	old.Clear()
	return err
}

func (a *app) DispatchEvent(event *cronpb.SendEventRequest) error {
	if event.Event.Type == cronpb.EventType_EVENT_UNKNOWN {
		return fmt.Errorf("event type is undefined")
	}
	mtimer := a.metrics.CustomHistogramSet("dispatch_event")
	defer mtimer.ObserveDuration()
	centers, err := a.GetCenterSrvList()
	if err != nil {
		return err
	}

	// defer func() {
	// 	for _, v := range centers {
	// 		v.Close()
	// 	}
	// }()

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
		var (
			cc  *grpc.ClientConn
			err error
		)
		go safe.Run(func() {
			// 根据上下文关闭链接
			for {
				select {
				case <-ctx.Done():
					if cc != nil {
						cc.Close()
					}
					return
				}
			}
		})
		md, _ := metadata.FromIncomingContext(ctx)
		wlog.Debug("got proxy request", zap.String("full_method", fullMethodName))
		addrs := md.Get(common.GOPHERCRON_PROXY_TO_MD_KEY)
		dialOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
		if len(addrs) > 0 {
			addr := addrs[0]
			wlog.Debug("address proxy", zap.String("proxy_to", addr), zap.String("full_method", fullMethodName))
			if cc, err = grpc.DialContext(ctx, addr, dialOptions...); err != nil {
				return nil, nil, err
			}

			md.Set(common.GOPHERCRON_AGENT_IP_MD_KEY, "gophercron_proxy")
			outCtx := metadata.NewOutgoingContext(ctx, md.Copy())
			return outCtx, cc, nil
		} else {
			projectIDs := md.Get(common.GOPHERCRON_PROXY_PROJECT_MD_KEY)
			wlog.Debug("resolve proxy", zap.Any("project", projectIDs), zap.String("full_method", fullMethodName))
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
			watermelonCC, err := newCC(ls[1], newCC.WithSystem(projectID), newCC.WithOrg(a.cfg.Micro.OrgID), newCC.WithRegion(a.cfg.Micro.Region),
				newCC.WithServiceResolver(etcd.NewEtcdResolver(infra.ResolveEtcdClient(), infra.ProxyAllowFunc)),
				newCC.WithGrpcDialOptions(dialOptions...))
			if err != nil {
				return nil, nil, err
			}
			cc = watermelonCC.ClientConn
			outCtx := metadata.NewOutgoingContext(ctx, md.Copy())
			return outCtx, cc, nil
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

type Author struct {
	privateKey []byte
	token      string
	expireTime time.Time
}

func (s *Author) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	if s.expireTime.After(time.Now().Add(time.Second * 10)) {
		return map[string]string{
			common.GOPHERCRON_CENTER_AUTH_KEY: s.token,
		}, nil
	}
	claims := jwt.CenterTokenClaims{
		Biz: jwt.DefaultBIZ,
		Iat: time.Now().Unix(),
		Exp: time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwt.BuildCenterJWT(claims, s.privateKey)
	if err != nil {
		return nil, err
	}

	s.token = token
	s.expireTime = time.Unix(claims.Exp, 0)

	return map[string]string{
		common.GOPHERCRON_CENTER_AUTH_KEY: token,
	}, nil
}

func (s *Author) RequireTransportSecurity() bool {
	return false
}
