package app

import (
	"encoding/json"

	"github.com/holdno/firetower/service/gateway"
	"github.com/holdno/gopherCron/utils"
)

type WebClientEvent struct {
}

const (
	publishSource = "platform"
)

type PublishData struct {
	Topic string
	Data  interface{}
}

func messageTaskStatusChanged(projectID int64, taskID string, status string) PublishData {
	return PublishData{
		Topic: "/task/status",
		Data: map[string]interface{}{
			"status":     status,
			"project_id": projectID,
			"task_id":    taskID,
		},
	}
}

func messageWorkflowStatusChanged(workflowID int64, status string) PublishData {
	return PublishData{
		Topic: "/workflow/status",
		Data: map[string]interface{}{
			"workflow_id": workflowID,
			"status":      status,
		},
	}
}

func messageWorkflowTaskStatusChanged(workflowID, projectID int64, taskID, status string) PublishData {
	return PublishData{
		Topic: "/workflow/task/status",
		Data: map[string]interface{}{
			"workflow_id": workflowID,
			"status":      status,
			"project_id":  projectID,
			"task_id":     taskID,
		},
	}
}

func (a *app) publishEventToWebClient(data PublishData) {
	m := gateway.GetTopicManage()
	if !m.IsReady() {
		return
	}

	body, _ := json.Marshal(data.Data)
	m.Publish(utils.GetStrID(), publishSource, data.Topic, body)
}
