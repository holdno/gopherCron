package app

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type Warner interface {
	Warning(info string) error
	Warningf(f string, args ...interface{}) error
}

type warning struct {
	logger *logrus.Logger
}

func NewDefaultWarner(logger *logrus.Logger) *warning {
	return &warning{logger: logger}
}

func (a *warning) Warning(info string) error {
	a.logger.Error(info)
	return nil
}

func (a *warning) Warningf(f string, args ...interface{}) error {
	a.Warning(fmt.Sprintf(f, args...))
	return nil
}
