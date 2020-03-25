package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

// MustSetup the func to build log instance
func MustSetup(logLevel string) *logrus.Logger {
	var (
		level logrus.Level
		err   error
	)

	if level, err = logrus.ParseLevel(logLevel); err != nil {
		panic(err)
	}

	logInstance := logrus.New()
	logInstance.SetLevel(level)
	logInstance.SetFormatter(new(logrus.JSONFormatter))
	logInstance.SetOutput(os.Stdout)

	return logInstance
}
