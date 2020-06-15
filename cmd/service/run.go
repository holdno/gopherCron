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
	"github.com/holdno/gopherCron/cmd/service/router"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

var httpServer *http.Server

// 初始化服务
func apiServer(srv app.App, conf *config.DeployConf) {

	if conf.Environment == "release" {
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
	router.SetupRoute(engine, conf)

	go func() {
		serverAddress := resolveServerAddress(conf.Host)
		httpServer = &http.Server{
			Addr:        serverAddress,
			Handler:     engine,
			ReadTimeout: time.Duration(5) * time.Second,
		}
		// TODO log
		fmt.Println(utils.GetCurrentTimeText(), "listening and serving HTTP on "+serverAddress)
		err := httpServer.ListenAndServe()
		if err != nil {
			fmt.Fprintln(os.Stderr, "http server start failed:", err)
			os.Exit(0)
		}
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
	srv := app.NewApp(opts.ConfigPath)

	defer func() {
		if r := recover(); r != nil {
			srv.Warning(app.WarningData{
				Data:    fmt.Sprintf("gophercron service panic: %v", r),
				Type:    app.WarningTypeSystem,
				AgentIP: srv.GetIP(),
			})
		}
	}()

	apiServer(srv, srv.GetConfig().Deploy)

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
			fmt.Println(os.Stderr, "http server graceful shutdown failed", err)
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
