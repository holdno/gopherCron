package warning

import (
	"encoding/json"

	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
)

const (
	SERVICE_TYPE_CENTER = "center-service"
	SERVICE_TYPE_AGENT  = "agent"
)

type WarningData struct {
	Data json.RawMessage `json:"data"`
	Type string          `json:"type"`
}

type WorkflowWarning struct {
	WorkflowID    int64  `json:"workflow_id"`
	WorkflowTitle string `json:"workflow_title"`
	ServiceIP     string `json:"service_ip"`
	Message       string `json:"message"`
}

func NewWorkflowWarningData(data WorkflowWarning) WarningData {
	raw, _ := json.Marshal(data)
	return WarningData{
		Data: raw,
		Type: WarningTypeTask,
	}
}

type TaskWarning struct {
	AgentIP      string `json:"agent_ip"`
	TaskName     string `json:"task_name"`
	TaskID       string `json:"task_id"`
	ProjectID    int64  `json:"project_id"`
	ProjectTitle string `json:"project_title"`
	Message      string `json:"message"`
}

func NewTaskWarningData(data TaskWarning) WarningData {
	raw, _ := json.Marshal(data)
	return WarningData{
		Data: raw,
		Type: WarningTypeTask,
	}
}

type SystemWarning struct {
	Endpoint string `json:"endpoint"`
	Type     string `json:"type"`
	Message  string `json:"message"`
}

func NewSystemWarningData(data SystemWarning) WarningData {
	raw, _ := json.Marshal(data)
	return WarningData{
		Data: raw,
		Type: WarningTypeSystem,
	}
}

const (
	WarningTypeSystem   = "system"
	WarningTypeTask     = "task"
	WarningTypeWorkflow = "workflow"
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
	a.logger.With(zap.Any("fields", data), zap.String("type", data.Type)).Error("agent alert")
	return nil
}
