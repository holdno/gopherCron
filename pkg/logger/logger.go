package logger

import (
	"os"

	"ojbk.io/gopherCron/config"

	"github.com/sirupsen/logrus"
)

// MustSetup the func to build log instance
func MustSetup(conf *config.ServiceConfig) *logrus.Logger {
	var (
		level logrus.Level
		err   error
	)

	if level, err = logrus.ParseLevel(conf.LogLevel); err != nil {
		panic(err)
	}

	logInstance := logrus.New()
	logInstance.SetLevel(level)
	logInstance.SetFormatter(new(logrus.JSONFormatter))
	logInstance.SetOutput(os.Stdout)

	return logInstance
}
