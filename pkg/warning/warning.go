package warning

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type WarningData struct {
	Data      string `json:"data"`
	Type      string `json:"type"`
	AgentIP   string `json:"agent_ip"`
	TaskName  string `json:"task_name"`
	ProjectID int64  `json:"project_id"`
}

const (
	WarningTypeSystem = "system"
	WarningTypeTask   = "task"
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
	if data.Type == WarningTypeSystem {
		a.logger.Error(data.Data)
	} else {
		a.logger.Error(fmt.Sprintf("task: %s, warning: %s", data.TaskName, data.Data))
	}

	return nil
}
