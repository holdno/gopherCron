package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/middleware"
	"github.com/holdno/gopherCron/cmd/service/router"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/infra"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
	"github.com/spacegrower/watermelon/infra/graceful"
	"github.com/spacegrower/watermelon/infra/wlog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var shutdownFunc func()

func setupPprof() {
	{
		file, err := os.Create("./cpu.pprof")
		if err != nil {
			fmt.Printf("create cpu pprof failed, err:%v\n", err)
			return
		}
		pprof.StartCPUProfile(file)
	}

	{
		file, err := os.Create("./mem.pprof")
		if err != nil {
			fmt.Printf("create mem pprof failed, err:%v\n", err)
			return
		}
		pprof.WriteHeapProfile(file)
	}

}

// 初始化服务
func apiServer(srv app.App, conf *config.ServiceConfig) {

	if utils.ReleaseMode() {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	//URI路由设置
	router.SetupRoute(srv, engine, conf.Deploy)

	// rpc server
	infra.RegisterETCDRegisterPrefixKey(conf.Etcd.Prefix + "/registry")
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
		newServer.WithServiceRegister(infra.MustSetupEtcdRegister()))

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
			return status.Error(codes.Aborted, "header: gophercron-agent-ip is not found")
		}
		middleware.SetAgentIP(ctx, agentIP)
		return nil
	})
	server.Handler(cronpb.CenterServer.RegisterAgent,
		cronpb.CenterServer.StatusReporter,
		cronpb.CenterServer.TryLock)

	graceful.RegisterPreShutDownHandlers(func() {
		srv.Close()
	})

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
	// 加载配置
	var o []app.AppOptions
	// if opts.Firetower {
	// 	o = append(o, app.WithFiretower())
	// }
	srv := app.NewApp(opts.ConfigPath, o...)

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
	apiServer(srv, srv.GetConfig())
	return nil
}
