package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"syscall"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/jwt"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/infra"
	"github.com/holdno/gopherCron/pkg/infra/register"
	"github.com/holdno/gopherCron/protocol"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	winfra "github.com/spacegrower/watermelon/infra"
	wregister "github.com/spacegrower/watermelon/infra/register"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func prometheusHandler(r *prometheus.Registry) gin.HandlerFunc {
	h := promhttp.InstrumentMetricHandler(
		r, promhttp.HandlerFor(r, promhttp.HandlerOpts{}),
	)

	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func (a *client) SetupMicroService() *winfra.Srv[infra.NodeMetaRemote] {
	register := a.MustSetupRemoteRegisterV2()
	cfg := a.Cfg()
	if cfg.Address == "" {
		cfg.Address = a.GetIP()
	}

	httpEngine := gin.New()
	if utils.DebugMode() {
		wlog.Info("debug mode will open pprof tools")
		pprof.Register(httpEngine)
	}
	httpEngine.GET("/heatlhy", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "healthy")
	})
	if a.metrics != nil {
		httpEngine.GET("/metrics", prometheusHandler(a.metrics.provider.Registry()))
	}

	newsrv := infra.NewAgentServer()
	var pids []int64
	for _, v := range cfg.Auth.Projects {
		pids = append(pids, v.ProjectID)
	}
	srv := newsrv(func(srv *grpc.Server) {
		cronpb.RegisterAgentServer(srv, a)
	}, newsrv.WithOrg(cfg.Micro.OrgID),
		newsrv.WithRegion(cfg.Micro.Region),
		newsrv.WithSystems(pids),
		newsrv.WithWeight(cfg.Micro.Weigth),
		newsrv.WithHttpServer(&http.Server{
			Handler: httpEngine,
		}),
		newsrv.WithAddress([]infra.Address{{ListenAddress: cfg.Address, RegisterAddress: cfg.RegisterAddress}}),
		newsrv.WithTags(map[string]string{
			"agent-version": protocol.GetVersion(),
		}),
		newsrv.WithServiceRegister(register),
		newsrv.WithGrpcServerOptions(grpc.ReadBufferSize(protocol.GrpcBufferSize), grpc.WriteBufferSize(protocol.GrpcBufferSize)))

	srv.Use(jwt.AgentAuthMiddleware([]byte(a.cfg.Auth.PublicKey)))
	go func() {
		srv.RunUntil(syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	}()
	return srv
}

func (a *client) MustSetupRemoteRegisterV2() wregister.ServiceRegister[infra.NodeMetaRemote] {
	genMetadata := func(ctx context.Context, cc *grpc.ClientConn, reqMethod string) context.Context {
		md := metadata.NewOutgoingContext(ctx, metadata.New(map[string]string{
			common.GOPHERCRON_AGENT_IP_MD_KEY:   a.GetIP(),
			common.GOPHERCRON_AGENT_VERSION_KEY: protocol.GetVersion(),
		}))
		if reqMethod != cronpb.Center_Auth_FullMethodName {
			var token string
			retry.Do(func() error {
				var err error
				token, err = a.authenticator.GetToken(ctx, a.centerSrv)
				if err != nil {
					a.metrics.SystemErrInc("agent_authenticator_error")
					wlog.Error("failed to sign token", zap.Error(err))
				}
				return err
			}, retry.Attempts(3), retry.Delay(time.Second), retry.LastErrorOnly(true))

			md = metadata.AppendToOutgoingContext(md, common.GOPHERCRON_AGENT_AUTH_KEY, token)
		}
		return md
	}

	r, err := register.NewRemoteRegisterV2(a.GetIP(),
		func() (*register.CenterClient, error) { // 建立到center服务的连接
			if a.centerSrv.Cc != nil { // 每次建立前都尝试关闭一次旧连接，确保不会泄露
				if a.centerSrv.Cc.GetState() == connectivity.Ready { // 兜底防止错误调用
					return &a.centerSrv, nil
				}
				a.centerSrv.Cc.Close()
			}

			wlog.Debug("start build new center client")
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.cfg.Timeout)*time.Second)
			defer cancel()

			cc, err := grpc.DialContext(ctx, a.cfg.Micro.Endpoint,
				grpc.WithConnectParams(grpc.ConnectParams{
					Backoff: backoff.Config{
						BaseDelay:  backoff.DefaultConfig.BaseDelay,
						Multiplier: backoff.DefaultConfig.Multiplier,
						Jitter:     backoff.DefaultConfig.Jitter,
						MaxDelay:   time.Minute,
					},
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithKeepaliveParams(keepalive.ClientParameters{
					Time:    time.Second * 15, // todo config
					Timeout: time.Second * 5,
				}),
				grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
					return invoker(genMetadata(ctx, cc, method), method, req, reply, cc, opts...)
				}),
				grpc.WithStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
					return streamer(genMetadata(ctx, cc, method), desc, cc, method, opts...)
				}))
			if err != nil {
				a.metrics.SystemErrInc("agent_setup_remote_register_error")
				wlog.Error("failed to dial center service", zap.String("endpoint", a.cfg.Micro.Endpoint), zap.Error(err))
				return nil, err
			}

			a.centerSrv = register.CenterClient{
				CenterClient: cronpb.NewCenterClient(cc),
				Cc:           cc,
			}

			return &a.centerSrv, nil
		}, a.handlerEventFromCenterV2)
	if err != nil {
		panic(err)
	}
	return r
}

func (a *client) CheckRunning(ctx context.Context, req *cronpb.CheckRunningRequest) (*cronpb.Result, error) {
	if _, taskExecuting := a.scheduler.CheckTaskExecuting(common.GenTaskSchedulerKey(req.ProjectId, req.TaskId)); taskExecuting {
		return &cronpb.Result{
			Result:  true,
			Message: "running",
		}, nil
	}
	return &cronpb.Result{
		Result:  false,
		Message: "not running",
	}, nil
}

func (a *client) Schedule(ctx context.Context, req *cronpb.ScheduleRequest) (*cronpb.Result, error) {
	unmarshalTask := func(value []byte) (*common.TaskWithOperator, error) {
		var task common.TaskWithOperator
		if err := json.Unmarshal(value, &task); err != nil {
			return nil, status.Error(codes.InvalidArgument, "failed to unmarshal task")
		}
		return &task, nil
	}

	switch req.Event.Type {
	case common.REMOTE_EVENT_TMP_SCHEDULE:
		task, err := unmarshalTask(req.Event.Value)
		if err != nil {
			return nil, err
		}
		if _, taskExecuting := a.scheduler.CheckTaskExecuting(task.SchedulerKey()); taskExecuting {
			return nil, status.Error(codes.AlreadyExists, "the task already executing, try again later")
		}

		// 兼容临时调度 不再在plan中检测任务是否存在
		// plan, exist := a.GetPlan(task.SchedulerKey())
		// if !exist {
		// 	return nil, status.Error(codes.NotFound, "task plan is not found")
		// }

		plan, err := common.BuildTaskSchedulerPlan(task, common.ActivePlan)
		if err != nil {
			return nil, status.Error(codes.Aborted, "failed to build task plan: "+err.Error())
		}
		if err = a.TryStartTask(plan); err != nil {
			return nil, status.Error(codes.Aborted, "failed to execute task: "+err.Error())
		}
	case common.REMOTE_EVENT_WORKFLOW_SCHEDULE:
		task, err := unmarshalTask(req.Event.Value)
		if err != nil {
			return nil, err
		}
		// 下发 task
		if _, taskExecuting := a.scheduler.CheckTaskExecuting(task.SchedulerKey()); taskExecuting {
			return nil, status.Error(codes.AlreadyExists, "the task already executing, try again later")
		}
		plan, err := common.BuildWorkflowTaskSchedulerPlan(task.TaskInfo)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to build workflow task schedule plan")
		}
		if err = a.TryStartTask(plan); err != nil {
			return nil, err
		}
	case common.REMOTE_EVENT_TASK_STOP:
		task, err := unmarshalTask(req.Event.Value)
		if err != nil {
			return nil, err
		}
		if task, taskExecuting := a.scheduler.CheckTaskExecuting(task.SchedulerKey()); taskExecuting {
			task.CancelFunc()
		}
	}

	return &cronpb.Result{
		Result:  true,
		Message: "ok",
	}, nil
}

func (a *client) KillTask(ctx context.Context, req *cronpb.KillTaskRequest) (*cronpb.Result, error) {
	if taskExecuteInfo, taskExecuting := a.scheduler.CheckTaskExecuting(common.GenTaskSchedulerKey(req.ProjectId, req.TaskId)); taskExecuting {
		taskExecuteInfo.CancelFunc()
		return &cronpb.Result{
			Result: true,
		}, nil
	}
	return &cronpb.Result{
		Result:  false,
		Message: "task not running",
	}, nil
}

func (a *client) ProjectTaskHash(ctx context.Context, req *cronpb.ProjectTaskHashRequest) (*cronpb.ProjectTaskHashReply, error) {
	hash, latestUpdateTime := a.scheduler.GetProjectTaskHash(req.ProjectId)
	return &cronpb.ProjectTaskHashReply{
		Hash:             hash,
		LatestUpdateTime: latestUpdateTime,
	}, nil
}

type Authenticator struct {
	authMeta map[int64]string
	token    string
	expTime  time.Time
}

func NewAuthenticator(projectAuths []config.ProjectAuth) *Authenticator {
	authKvs := make(map[int64]string)
	for _, v := range projectAuths {
		authKvs[v.ProjectID] = v.Token
	}
	return &Authenticator{
		authMeta: authKvs,
	}
}

func (a *Authenticator) getToken(ctx context.Context, driver cronpb.CenterClient) (string, error) {
	if driver == nil {
		return "", fmt.Errorf("invalid driver")
	}

	resp, err := driver.Auth(ctx, &cronpb.AuthReq{
		Kvs: a.authMeta,
	})
	if err != nil {
		wlog.Error("failed to get agent auth token", zap.Error(err))
		return "", err
	}
	a.token = resp.Jwt
	a.expTime = time.Unix(resp.ExpireTime, 0)
	return a.token, nil
}

func (a *Authenticator) GetToken(ctx context.Context, driver cronpb.CenterClient) (string, error) {
	if !a.expTime.IsZero() && a.expTime.After(time.Now().Add(time.Minute)) {
		return a.token, nil
	}

	return a.getToken(ctx, driver)
}

func (a *Authenticator) RefreshNow(ctx context.Context, driver cronpb.CenterClient) (string, error) {
	a.token = ""
	a.expTime = time.Time{}
	return a.getToken(ctx, driver)
}
