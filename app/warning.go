package app

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type Warner interface {
	Warning(info string)
	Warningf(f string, args ...interface{})
}

type warning struct {
	logger *logrus.Logger
}

func NewDefaultWarner(logger *logrus.Logger) *warning {
	return &warning{logger: logger}
}

func (a *warning) Warning(info string) {
	a.logger.Error(info)
}

func (a *warning) Warningf(f string, args ...interface{}) {
	a.Warning(fmt.Sprintf(f, args...))
}
