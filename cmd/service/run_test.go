package service

import (
	"testing"

	"github.com/holdno/gopherCron/app"
)

func TestTemporarySchedule(t *testing.T) {
	srv := app.NewApp("/Users/wangboyan/development/opensource/gopherCron/cmd/service/conf/config-dev.toml")
	waitingShutdown(srv)
}
