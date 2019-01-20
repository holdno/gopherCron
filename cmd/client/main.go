package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	appctx "ojbk.io/gopherCron/context"

	"ojbk.io/gopherCron/utils"

	"ojbk.io/gopherCron/config"

	log "github.com/sirupsen/logrus"
)

// 配置文件初始化
func initConf(filePath string) *config.ServiceConfig {
	workerConf := config.InitServiceConfig(filePath)
	return workerConf
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

	appctx.InitWorkerContext(conf)
	fmt.Println("client runing")
	waitingShutdown()
}

func waitingShutdown() {
	stopSignalChan := make(chan os.Signal, 1)
	signal.Notify(stopSignalChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)

	sig := <-stopSignalChan
	if sig != nil {
		fmt.Println(utils.GetCurrentTimeText(), "got system signal:"+sig.String()+", going to shutdown.")
	}
}
