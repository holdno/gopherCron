package app

import (
	"encoding/json"

	"github.com/holdno/firetower/protocol"
	"github.com/holdno/firetower/service/tower"
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

func messageTaskStatusChanged(projectID int64, taskID, tmpID, status string) PublishData {
	return PublishData{
		Topic: "/task/status",
		Data: map[string]interface{}{
			"status":     status,
			"project_id": projectID,
			"task_id":    taskID,
			"tmp_id":     tmpID,
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

type SystemPusher struct {
	clientID string
}

func (s *SystemPusher) UserID() string {
	return "system"
}
func (s *SystemPusher) ClientID() string {
	return s.clientID
}

func (a *app) publishEventToWebClient(data PublishData) {
	f := tower.NewFire(protocol.SourceSystem, a.pusher)
	msgBody := map[string]interface{}{
		"topic": data.Topic,
		"data":  data.Data,
	}
	body, _ := json.Marshal(msgBody)
	f.Message.Topic = data.Topic
	f.Message.Type = protocol.PublishKey
	f.Message.Data = body
	a.firetower.Publish(f)
}
