package app

import (
	"github.com/sirupsen/logrus"
)

type Warner interface {
	Warning(data WarningData) error
}

type warning struct {
	logger *logrus.Logger
}

func NewDefaultWarner(logger *logrus.Logger) *warning {
	return &warning{logger: logger}
}

func (a *warning) Warning(data WarningData) error {
	a.logger.Error(data.Data)
	return nil
}
