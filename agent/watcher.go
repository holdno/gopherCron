package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/holdno/gopherCron/protocol"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
)

func newReplyEvent(id string) *cronpb.ClientEvent {
	return &cronpb.ClientEvent{
		Id:        id,
		EventTime: time.Now().Unix(),
	}

	// switch types {
	// case cronpb.EventType_EVENT_CHECK_RUNNING_REQUEST:
	// 	ce.Type = cronpb.EventType_EVENT_CHECK_RUNNING_REPLY
	// case cronpb.EventType_EVENT_SCHEDULE_REQUEST:
	// 	ce.Type = cronpb.EventType_EVENT_SCHEDULE_REPLY
	// case cronpb.EventType_EVENT_KILL_TASK_REQUEST:
	// 	ce.Type = cronpb.EventType_EVENT_KILL_TASK_REPLY
	// case cronpb.EventType_EVENT_PROJECT_TASK_HASH_REQUEST:
	// 	ce.Type = cronpb.EventType_EVENT_PROJECT_TASK_HASH_REPLY
	// case cronpb.EventType_EVENT_REGISTER_HEARTBEAT_PING:
	// 	ce.Type = cronpb.EventType_EVENT_REGISTER_HEARTBEAT_PONG
	// default:
	// }
	// return ce
}

// handlerEventFromCenterV2 响应中心的请求，若返回error，则会主动断开连接
func (a *client) handlerEventFromCenterV2(ctx context.Context, event *cronpb.ServiceEvent) (*cronpb.ClientEvent, error) {
	wlog.Debug("handle new event v2 from center", zap.String("event_type", event.Type.String()))
	replyEvent := newReplyEvent(event.Id)
	switch event.Type {
	case cronpb.EventType_EVENT_REGISTER_HEARTBEAT_PING:
		replyEvent.Type = cronpb.EventType_EVENT_REGISTER_HEARTBEAT_PONG
	case cronpb.EventType_EVENT_REGISTER_REPLY:
		if err := a.handlerEventFromCenter(event.GetRegisterReply()); err != nil {
			return nil, err
		}
	case cronpb.EventType_EVENT_SCHEDULE_REQUEST:
		replyEvent.Type = cronpb.EventType_EVENT_SCHEDULE_REPLY
		resp, err := a.Schedule(ctx, event.GetScheduleRequest())
		if err != nil {
			replyEvent.Error = &cronpb.Error{Error: err.Error()}
		} else {
			replyEvent.Event = &cronpb.ClientEvent_ScheduleReply{
				ScheduleReply: resp,
			}
		}
	case cronpb.EventType_EVENT_KILL_TASK_REQUEST:
		replyEvent.Type = cronpb.EventType_EVENT_KILL_TASK_REPLY
		resp, err := a.KillTask(ctx, event.GetKillTaskRequest())
		if err != nil {
			replyEvent.Error = &cronpb.Error{Error: err.Error()}
		} else {
			replyEvent.Event = &cronpb.ClientEvent_KillTaskReply{
				KillTaskReply: resp,
			}
		}
	case cronpb.EventType_EVENT_PROJECT_TASK_HASH_REQUEST:
		replyEvent.Type = cronpb.EventType_EVENT_PROJECT_TASK_HASH_REPLY
		resp, err := a.ProjectTaskHash(ctx, event.GetProjectTaskHashRequest())
		if err != nil {
			replyEvent.Error = &cronpb.Error{Error: err.Error()}
		} else {
			replyEvent.Event = &cronpb.ClientEvent_ProjectTaskHashReply{
				ProjectTaskHashReply: resp,
			}
		}
	case cronpb.EventType_EVENT_COMMAND_REQUEST:
		replyEvent.Type = cronpb.EventType_EVENT_COMMAND_REPLY
		resp, err := a.Command(ctx, event.GetCommandRequest())
		if err != nil {
			replyEvent.Error = &cronpb.Error{Error: err.Error()}
		} else {
			replyEvent.Event = &cronpb.ClientEvent_CommandReply{
				CommandReply: resp,
			}
		}
	case cronpb.EventType_EVENT_CHECK_RUNNING_REQUEST:
		replyEvent.Type = cronpb.EventType_EVENT_CHECK_RUNNING_REPLY
		resp, err := a.CheckRunning(ctx, event.GetCheckRunningRequest())
		if err != nil {
			replyEvent.Error = &cronpb.Error{Error: err.Error()}
		} else {
			replyEvent.Event = &cronpb.ClientEvent_CheckRunningReply{
				CheckRunningReply: resp,
			}
		}
	default:
		return &cronpb.ClientEvent{
			Id:        event.Id,
			Type:      cronpb.EventType_EVENT_CLIENT_UNSUPPORT,
			EventTime: time.Now().Unix(),
			Error:     &cronpb.Error{Error: fmt.Sprintf("%s not support", protocol.GetVersion())},
		}, nil
	}
	return replyEvent, nil
}

func (a *client) handlerEventFromCenter(event *cronpb.Event) error {
	var (
		task      *common.TaskWithOperator
		taskEvent *common.TaskEvent
		err       error
	)
	wlog.Debug("handle new event from center", zap.String("event_type", event.Type))
	switch event.Type {
	// agent注册成功后会收到中心响应的任务列表
	case common.REMOTE_EVENT_PUT: // 任务保存
		// 反序列化task
		if task, err = common.Unmarshal(event.Value); err != nil {
			wlog.Error("failed to unmarshal task", zap.String("task", string(event.Value)))
			return err
		}
		taskEvent = common.BuildTaskEvent(common.TASK_EVENT_SAVE, task)
	case common.REMOTE_EVENT_TMP_SCHEDULE:
		var err error
		if task, err = common.Unmarshal(event.Value); err != nil {
			return err
		}
		taskEvent = common.BuildTaskEvent(common.TASK_EVENT_TEMPORARY, task)
	case common.REMOTE_EVENT_DELETE:
		var err error
		if task, err = common.Unmarshal(event.Value); err != nil {
			return err
		}
		taskEvent = common.BuildTaskEvent(common.TASK_EVENT_DELETE, task)
	case common.REMOTE_EVENT_TASK_STOP:
		if task, err = common.Unmarshal(event.Value); err != nil {
			return err
		}
		taskEvent = common.BuildTaskEvent(common.TASK_EVENT_KILL, task)
	default:
		return nil
	}

	a.scheduler.PushEvent(taskEvent)
	return nil
}
