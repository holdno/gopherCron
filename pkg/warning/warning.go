package warning

import (
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
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
	logger wlog.Logger
}

func NewDefaultWarner(logger wlog.Logger) *warning {
	return &warning{logger: logger}
}

func (a *warning) Warning(data WarningData) error {
	a.logger.With(zap.Any("fields", map[string]interface{}{
		"error":      data.Data,
		"task_name":  data.TaskName,
		"project_id": data.ProjectID,
	})).Error("agent alert")
	return nil
}
