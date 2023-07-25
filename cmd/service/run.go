package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/middleware"
	"github.com/holdno/gopherCron/cmd/service/router"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/jwt"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/infra"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/protocol"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
	"github.com/mwitkow/grpc-proxy/proxy"
	"github.com/mwitkow/grpc-proxy/testservice"
	"github.com/spacegrower/watermelon/infra/graceful"
	"github.com/spacegrower/watermelon/infra/wlog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var shutdownFunc func()

// 初始化服务
func apiServer(srv app.App, conf *config.ServiceConfig) {
	if utils.ReleaseMode() {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	//URI路由设置
	router.SetupRoute(srv, engine, conf)

	// rpc server
	// 注册不同region对应的grpc proxy地址
	for region, proxy := range conf.Micro.RegionProxy {
		infra.RegisterRegionProxy(region, proxy)
	}

	newServer := infra.NewCenterServer()
	server := newServer(func(grpcServer *grpc.Server) {
		cronpb.RegisterCenterServer(grpcServer, &cronRpc{
			app:                srv,
			registerMetricsAdd: srv.Metrics().NewGaugeFunc("agent_register_count", "agent"),
			eventsMetricsInc:   srv.Metrics().CustomIncFunc("registry_event", "", ""),
		})
	}, newServer.WithRegion(conf.Micro.Region),
		newServer.WithOrg(conf.Micro.OrgID),
		newServer.WithAddress([]infra.Address{{ListenAddress: conf.Deploy.Host}}),
		newServer.WithHttpServer(&http.Server{
			Handler:     engine,
			ReadTimeout: time.Duration(5) * time.Second,
		}),
		newServer.WithServiceRegister(infra.MustSetupEtcdRegister()),
		newServer.WithGrpcServerOptions(grpc.ReadBufferSize(protocol.GrpcBufferSize), grpc.WriteBufferSize(protocol.GrpcBufferSize)))

	grpcRequestCounter := srv.Metrics().NewCounter("grpc_request", "method")
	grpcRequestDuration := srv.Metrics().NewHistogram("grpc_request", "method")
	server.Use(func(ctx context.Context) error {
		method := middleware.GetFullMethodFrom(ctx)
		grpcRequestCounter(method)
		timer := grpcRequestDuration(method)
		defer timer.ObserveDuration()
		return middleware.Next(ctx)
	})
	server.Handler(cronpb.CenterServer.SendEvent)
	server.Use(func(ctx context.Context) error {
		agentIP, exist := GetAgentIPFromContext(ctx)
		if !exist {
			return status.Error(codes.Aborted, "header: "+common.GOPHERCRON_AGENT_IP_MD_KEY+" is not found")
		}
		middleware.SetAgentIP(ctx, agentIP)
		return nil
	})

	server.Handler(cronpb.CenterServer.Auth)
	agentApi := server.Group()
	// agent 鉴权
	agentApi.Use(jwt.CenterAuthMiddleware([]byte(srv.GetConfig().JWT.PublicKey)))
	agentApi.Handler(cronpb.CenterServer.RegisterAgent,
		cronpb.CenterServer.StatusReporter,
		cronpb.CenterServer.TryLock)

	graceful.RegisterPreShutDownHandlers(func() {
		srv.Close()
	})

	server.Use(jwt.AgentAuthMiddleware([]byte(srv.GetConfig().JWT.PublicKey))) // center间鉴权复用agent验证center身份的中间件
	srv.Run()
	go setupProxy(srv, srv.GetConfig())
	wlog.Info(fmt.Sprintf("%s, start grpc server, listen on %s\n", utils.GetCurrentTimeText(), conf.Deploy.Host))
	server.RunUntil(syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	graceful.ShutDown()
}

// 获取http server监控端口地址
func resolveServerAddress(addr []string) string {
	switch len(addr) {
	case 0:
		if port := os.Getenv("PORT"); len(port) > 0 {
			return ":" + port
		}
		return ":9999"
	case 1:
		return addr[0]
	default:
		panic("too much parameters")
	}
}

func Run(opts *SetupOptions) error {
	srv := app.NewApp(opts.ConfigPath)

	defer func() {
		if r := recover(); r != nil {
			srv.Warning(warning.WarningData{
				Data:    fmt.Sprintf("gophercron service panic: %v", r),
				Type:    warning.WarningTypeSystem,
				AgentIP: srv.GetIP(),
			})
		}
	}()

	if srv.GetConfig().Deploy.Environment == "" {
		srv.GetConfig().Deploy.Environment = "debug"
	}
	os.Setenv("GOPHERENV", srv.GetConfig().Deploy.Environment)

	if opts.ProxyOnly {
		setupProxy(srv, srv.GetConfig())
		return nil
	}
	apiServer(srv, srv.GetConfig())
	return nil
}

func setupProxy(srv app.App, conf *config.ServiceConfig) {
	if conf.Deploy.ProxyHost == "" {
		return
	}
	newServer := infra.NewCenterServer()
	server := newServer(func(srv *grpc.Server) {
		testservice.RegisterTestServiceServer(srv, testservice.DefaultTestServiceServer)
	}, newServer.WithAddress([]infra.Address{{ListenAddress: conf.Deploy.ProxyHost}}),
		newServer.WithGrpcServerOptions(grpc.ReadBufferSize(protocol.GrpcBufferSize), grpc.WriteBufferSize(protocol.GrpcBufferSize), grpc.UnknownServiceHandler(proxy.TransparentHandler(srv.GetGrpcDirector()))))
	wlog.Info(fmt.Sprintf("%s, start grpc proxy, listen on %s\n", utils.GetCurrentTimeText(), conf.Deploy.ProxyHost))
	server.RunUntil(syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
}
