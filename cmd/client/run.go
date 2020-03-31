package client

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/config"
	"github.com/holdno/gopherCron/utils"
)

// 配置文件初始化
func initConf(filePath string) *config.ServiceConfig {
	workerConf := config.InitServiceConfig(filePath)
	return workerConf
}

func Run(opts *SetupOptions) error {
	// 加载配置
	conf := initConf(opts.ConfigPath)
	client := app.NewClient(conf)

	restart := func() {
		defer func() {
			if r := recover(); r != nil {
				ip, _ := utils.GetLocalIP()
				client.Warning(app.WarningData{
					Data:    fmt.Sprintf("agent %s down", ip),
					Type:    app.WarningTypeSystem,
					AgentIP: client.GetIP(),
				})
			}
		}()
		client.Loop()
	}

	go func() {
		for {
			restart()
		}
	}()

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
