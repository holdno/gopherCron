package app

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/holdno/firetower/protocol"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
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
	Endpoint string
	client   *http.Client
}

func (s *SystemPusher) UserID() string {
	return "system"
}
func (s *SystemPusher) ClientID() string {
	return s.clientID
}

func (s *SystemPusher) Publish(t *TopicMessage) error {
	msg := cloudevents.NewEvent()
	msg.SetSubject(t.Topic)
	msg.SetSource("gophercron_" + s.clientID)
	msg.SetData(cloudevents.ApplicationJSON, t.Data)

	raw, _ := json.Marshal(msg)
	resp, err := s.client.Post(s.Endpoint, "application/json", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		response, _ := ioutil.ReadAll(resp.Body)
		wlog.Error("failed to publish message to web client", zap.String("response", string(response)))
	}
	return nil
}

type TopicMessage struct {
	Topic string          `json:"topic"`
	Data  json.RawMessage `json:"data"` // 可能是个json
	Type  string          `json:"type"`
}

func (a *app) publishEventToWebClient(data PublishData) {
	if a.pusher == nil {
		return
	}
	f := &TopicMessage{}
	msgBody := map[string]interface{}{
		"topic": data.Topic,
		"data":  data.Data,
	}
	body, _ := json.Marshal(msgBody)

	f.Topic = data.Topic
	f.Type = protocol.PublishKey
	f.Data = body

	a.pusher.Publish(f)
}
