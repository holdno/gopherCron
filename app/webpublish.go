package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/holdno/firetower/config"
	"github.com/holdno/firetower/protocol"
	"github.com/holdno/firetower/service/tower"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/cronpb"
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

type FireTowerPusher struct {
	tm       tower.Manager
	clientID string
}

func (s *FireTowerPusher) UserID() string {
	return "system"
}

func (s *FireTowerPusher) ClientID() string {
	return s.clientID
}

func (s *FireTowerPusher) Publish(t *TopicMessage) error {
	fire := tower.NewFire(protocol.SourceSystem, s)
	fire.Message = protocol.TopicMessage{
		Topic: t.Topic,
		Data:  t.ToCloudEventRaw(s.clientID),
		Type:  protocol.PublishOperation,
	}

	return s.tm.Publish(fire)
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
	req, _ := http.NewRequest(http.MethodPost, s.Endpoint, bytes.NewReader(t.ToCloudEventRaw(s.clientID)))
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

type TopicMessage protocol.TopicMessage

func (t TopicMessage) ToCloudEventRaw(clientID string) []byte {
	msg := cloudevents.NewEvent()
	msg.SetSubject(t.Topic)
	msg.SetSource("gophercron/" + clientID)
	msg.SetData(cloudevents.ApplicationJSON, t.Data)
	msg.SetID(utils.GetStrID())
	msg.SetType(t.Type.String())

	raw, _ := json.Marshal(msg)
	return raw
}

func (a *app) publishEventToWebClient(data PublishData) {
	if a.pusher == nil {
		return
	}
	f := &TopicMessage{}
	body, _ := json.Marshal(data.Data)

	f.Topic = data.Topic
	f.Type = protocol.PublishOperation
	f.Data = body

	if err := a.pusher.Publish(f); err != nil {
		wlog.Error("failed to publish notify", zap.Error(err))
	}
}

var _ protocol.Pusher = (*SelfPusher)(nil)

type SelfPusher struct {
	app     *app
	channel chan *protocol.FireInfo
	loop    sync.Once
}

func (s *SelfPusher) Publish(fire *protocol.FireInfo) error {
	raw, _ := json.Marshal(fire)
	return s.app.DispatchEvent(&cronpb.SendEventRequest{
		Region: s.app.GetConfig().Micro.Region,
		Event: &cronpb.ServiceEvent{
			Id:        utils.GetStrID(),
			Type:      cronpb.EventType_EVENT_REALTIME_PUBLISH,
			EventTime: time.Now().Unix(),
			Event: &cronpb.ServiceEvent_RealtimePublish{
				RealtimePublish: &cronpb.RealtimePublish{
					Event: &cronpb.Event{
						Version:   common.VERSION_TYPE_V1,
						Type:      protocol.PublishOperation.String(),
						Value:     raw,
						EventTime: time.Now().Unix(),
					},
				},
			},
		},
	})
}

func (s *SelfPusher) Receive() chan *protocol.FireInfo {
	return s.channel
}

type Tower struct {
	tower.Manager
	msgChan chan *protocol.FireInfo
}

func (t *Tower) ReceiveEvent(data *cronpb.RealtimePublish) error {
	switch data.Event.Type {
	case protocol.PublishOperation.String():
		f := new(protocol.FireInfo)
		if err := json.Unmarshal(data.Event.Value, f); err != nil {
			t.Logger().Error("failed to unmarshal message", zap.Error(err), zap.ByteString("event_value", data.Event.Value))
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		select {
		case t.msgChan <- f:
		case <-ctx.Done():
			t.Logger().Error("tower receive event timeout", zap.ByteString("event_value", data.Event.Value))
		}
		return nil
	default:
		t.Logger().Warn("unsupport publish event", zap.String("type", data.Event.Type), zap.ByteString("event_value", data.Event.Value))
	}
	return nil
}

func buildTower(a *app, logger *zap.Logger) {
	msgChan := make(chan *protocol.FireInfo, 10000)
	// 全局唯一id生成器
	tm, err := tower.Setup(config.FireTowerConfig{
		ReadChanLens:  5,
		WriteChanLens: 1000,
		Heartbeat:     60,
		ServiceMode:   config.SingleMode,
		Bucket: config.BucketConfig{
			Num:              4,
			CentralChanCount: 1000,
			BuffChanCount:    1000,
			ConsumerNum:      2,
		},
	}, tower.BuildWithLogger(logger),
		tower.BuildWithPusher(&SelfPusher{
			app:     a,
			channel: msgChan,
		}))
	if err != nil {
		panic(err)
	}

	a.tower = &Tower{
		Manager: tm,
		msgChan: msgChan,
	}
}
