package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ojbk.io/gopherCron/app"
	"ojbk.io/gopherCron/cmd/service/router"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/config"
	"ojbk.io/gopherCron/utils"

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

// 配置文件初始化
func initConf(filePath string) *config.ServiceConfig {
	apiConf := config.InitServiceConfig(filePath)
	return apiConf
}

func Run(opt *SetupOptions) error {
	// 加载配置
	conf := initConf(opt.ConfigPath)
	srv := app.NewApp(conf)

	defer func() {
		if r := recover(); r != nil {
			srv.Warningf("%v", r)
		}
	}()

	apiServer(srv, conf.Deploy)

	os.Setenv("GOPHERENV", conf.Deploy.Environment)
	waitingShutdown(srv)
	return nil
}

func waitingShutdown(srv app.App) {
	stopSignalChan := make(chan os.Signal, 1)
	signal.Notify(stopSignalChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)

	sig := <-stopSignalChan
	if sig != nil {
		fmt.Println(utils.GetCurrentTimeText(), "got system signal:"+sig.String()+", going to shutdown.")
		// wait resource remove from nginx upstreams
		if os.Getenv("GOPHERENV") == "release" {
			time.Sleep(time.Second * 10)
		}
		srv.Close()
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
