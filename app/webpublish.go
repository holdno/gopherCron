package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/holdno/firetower/protocol"
	"github.com/holdno/gopherCron/utils"
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
		Topic: fmt.Sprintf("/task/status/project/%d", projectID),
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
		Topic: fmt.Sprintf("/workflow/status/%d", workflowID),
		Data: map[string]interface{}{
			"workflow_id": workflowID,
			"status":      status,
		},
	}
}

func messageWorkflowTaskStatusChanged(workflowID, projectID int64, taskID, status string) PublishData {
	return PublishData{
		Topic: fmt.Sprintf("/workflow/task/status/%d", workflowID),
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
	Header   map[string]string
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
	msg.SetSource("gophercron/" + s.clientID)
	msg.SetData(cloudevents.ApplicationJSON, t.Data)
	msg.SetID(utils.GetStrID())
	msg.SetType(t.Type)

	raw, _ := json.Marshal(msg)
	req, _ := http.NewRequest(http.MethodPost, s.Endpoint, bytes.NewReader(raw))
	for k, v := range s.Header {
		req.Header.Add(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		response, _ := io.ReadAll(resp.Body)
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
	body, _ := json.Marshal(data.Data)

	f.Topic = data.Topic
	f.Type = protocol.PublishKey
	f.Data = body

	if err := a.pusher.Publish(f); err != nil {
		wlog.Error("failed to publish notify", zap.Error(err))
	}
}
