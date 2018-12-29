package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	appctx "ojbk.io/gopherCron/context"

	"ojbk.io/gopherCron/utils"

	"ojbk.io/gopherCron/config"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"ojbk.io/gopherCron/cmd/service/router"
)

var (
	httpServer *http.Server
)

// 初始化服务
func apiServer(conf *config.DeployConf) {

	if conf.Environment == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	//recovery
	engine.Use(gin.Recovery())

	engine.NoRoute(func(c *gin.Context) {
		c.String(http.StatusOK, "no router found")
	})

	engine.NoMethod(func(c *gin.Context) {
		c.String(http.StatusOK, "no method found")
	})

	//URI路由设置
	router.SetupRoute(engine)

	go func() {
		serverAddress := resolveServerAddress(conf.Host)
		httpServer = &http.Server{
			Addr:        serverAddress,
			Handler:     engine,
			ReadTimeout: time.Duration(5) * time.Second,
		}
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

func initLogger(out io.Writer, level string) {

	switch strings.ToUpper(level) {
	case "INFO":
		log.SetLevel(log.InfoLevel)
	case "WARN":
		log.SetLevel(log.WarnLevel)
	case "ERROR":
		log.SetLevel(log.ErrorLevel)
	case "PANIC":
		log.SetLevel(log.PanicLevel)
	default:
		panic("log level was wrong")
	}

	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(out)
}

// Flags 命令行参数定义
type Flags struct {
	logDir   string
	confPath string
	logLevel string
}

func parseFlags() *Flags {
	flags := new(Flags)
	flag.StringVar(&flags.logDir, "log_dir", "stdout", "stdout or path")
	flag.StringVar(&flags.logLevel, "log_level", "info", "log level")
	flag.StringVar(&flags.confPath, "conf", "./conf/config-dev.toml", "config file path")
	flag.Parse()
	return flags
}

func main() {
	var (
		flags *Flags
		conf  *config.ServiceConfig
	)

	flags = parseFlags()

	if flags.logDir == "stdout" {
		initLogger(os.Stdout, flags.logLevel)
	} else {
		// TODO logfile support
		initLogger(os.Stdout, flags.logLevel)
	}

	// 加载配置
	conf = initConf(flags.confPath)

	appctx.InitMasterContext(conf)

	apiServer(conf.Deploy)

	waitingShutdown()
}

func waitingShutdown() {
	stopSignalChan := make(chan os.Signal, 1)
	signal.Notify(stopSignalChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)

	sig := <-stopSignalChan
	if sig != nil {
		fmt.Println(utils.GetCurrentTimeText(), "got system signal:"+sig.String()+", going to shutdown.")
		// wait resource remove from nginx upstreams
		if os.Getenv("SHOPENV") == "release" {
			time.Sleep(time.Second * 10)
		}

		// 关闭http服务
		err := shutdownHTTPServer()
		if err != nil {
			fmt.Fprintln(os.Stderr, "http server graceful shutdown failed", err)
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
