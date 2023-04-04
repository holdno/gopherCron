package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/middleware"
	"github.com/holdno/gopherCron/cmd/service/router"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/pkg/infra"
	"github.com/holdno/gopherCron/pkg/warning"
	"github.com/holdno/gopherCron/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gin-gonic/gin"
)

var httpServer *http.Server

// 初始化服务
func apiServer(srv app.App, conf *config.ServiceConfig) {

	if conf.Deploy.Environment == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	//recovery
	engine.Use(gin.Recovery())

	engine.Use(func(c *gin.Context) {
		c.Set(common.APP_KEY, srv)
	})

	engine.NoRoute(func(c *gin.Context) {
		c.String(http.StatusOK, "no router found")
	})

	engine.NoMethod(func(c *gin.Context) {
		c.String(http.StatusOK, "no method found")
	})

	//URI路由设置
	router.SetupRoute(engine, conf.Deploy)
	infra.RegisterETCDRegisterPrefixKey(conf.Etcd.Prefix + "/registry")
	newServer := infra.NewCenterServer()
	server := newServer(func(grpcServer *grpc.Server) {
		cronpb.RegisterCenterServer(grpcServer, &cronRpc{
			app: srv,
		})
	}, newServer.WithRegion(conf.Micro.Region),
		newServer.WithOrg(conf.Micro.OrgID),
		newServer.WithAddress(conf.Deploy.Host),
		newServer.WithHttpServer(&http.Server{
			Handler:     engine,
			ReadTimeout: time.Duration(5) * time.Second,
		}),
		newServer.WithServiceRegister(infra.MustSetupEtcdRegister()))

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

	go func() {
		fmt.Printf("%s, start grpc server, listen on %s", utils.GetCurrentTimeText(), conf.Deploy.Host)
		server.RunUntil(syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	}()
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
	if opts.Firetower {
		o = append(o, app.WithFiretower())
	}
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

	apiServer(srv, srv.GetConfig())

	os.Setenv("GOPHERENV", srv.GetConfig().Deploy.Environment)
	waitingShutdown(srv)
	return nil
}

func waitingShutdown(srv app.App) {
	stopSignalChan := make(chan os.Signal, 1)
	signal.Notify(stopSignalChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)

	sig := <-stopSignalChan
	if sig != nil {
		fmt.Println(utils.GetCurrentTimeText(), "got system signal:"+sig.String()+", going to shutdown.")
		srv.Close()
		// wait resource remove from nginx upstreams
		if os.Getenv("GOPHERENV") == "release" {
			time.Sleep(time.Second * 5)
		}
		// 关闭http服务
		err := shutdownHTTPServer()
		if err != nil {
			fmt.Println("http server graceful shutdown failed", err)
		} else {
			fmt.Println(utils.GetCurrentTimeText(), "http server graceful shutdown successfully.")
		}
	}
}

// 关闭http server
func shutdownHTTPServer() error {
	// Create a deadline to wait for server shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return httpServer.Shutdown(ctx)
}
