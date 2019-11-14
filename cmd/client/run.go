package client

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"ojbk.io/gopherCron/app"

	"ojbk.io/gopherCron/utils"

	"ojbk.io/gopherCron/config"
)

// 配置文件初始化
func initConf(filePath string) *config.ServiceConfig {
	workerConf := config.InitServiceConfig(filePath)
	return workerConf
}

func Run(opt *SetupOptions) error {
	// 加载配置
	conf := initConf(opt.ConfigPath)

	client := app.NewClient(conf)
	go client.Loop()
	fmt.Println("client runing")
	waitingShutdown()
	return nil
}

func waitingShutdown() {
	stopSignalChan := make(chan os.Signal, 1)
	signal.Notify(stopSignalChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)

	sig := <-stopSignalChan
	if sig != nil {
		fmt.Println(utils.GetCurrentTimeText(), "got system signal:"+sig.String()+", going to shutdown.")
	}
}
