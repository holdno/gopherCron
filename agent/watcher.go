package agent

import (
	"context"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/cronpb"
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
		// 建连后日常ping，某些基础设施需要显示的调用来保持链接的活跃(避免被强制关闭)
		replyEvent.Type = cronpb.EventType_EVENT_REGISTER_HEARTBEAT_PONG
	case cronpb.EventType_EVENT_REGISTER_REPLY:
		// agent注册后，中心会逐步下发需要该agent运行的任务信息
		data := event.GetRegisterReply()
		if len(data.Value) != 0 { // 过滤脏数据
			if err := a.handlerEventFromCenter(data); err != nil {
				return nil, err
			}
		}
	case cronpb.EventType_EVENT_SCHEDULE_REQUEST:
		// 立即调度一个任务
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
		// 立即结束某个运行中的任务
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
		// 获取边缘某个项目下所有任务计算出的hash值，用于比较不同agent间任务状态是否存在差异
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
		// 中心下发一些agent可以执行的指令
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
		// 中心询问agent是否正在运行某个任务
		replyEvent.Type = cronpb.EventType_EVENT_CHECK_RUNNING_REPLY
		resp, err := a.CheckRunning(ctx, event.GetCheckRunningRequest())
		if err != nil {
			replyEvent.Error = &cronpb.Error{Error: err.Error()}
		} else {
			replyEvent.Event = &cronpb.ClientEvent_CheckRunningReply{
				CheckRunningReply: resp,
			}
		}
	case cronpb.EventType_EVENT_MODIFY_NODE_META:
		// 中心直接下发指令变更agent的权重
		a.cfg.Micro.Weight = event.GetModifyNodeMeta().Weight
		a.srv.CustomInfo.Weight = a.cfg.Micro.Weight
		a.srv.Register()
		replyEvent.Event = &cronpb.ClientEvent_ModifyNodeMeta{
			ModifyNodeMeta: &cronpb.Result{
				Result:  true,
				Message: "ok",
			},
		}
	default:
		return &cronpb.ClientEvent{
			Id:        event.Id,
			Type:      cronpb.EventType_EVENT_CLIENT_UNSUPPORT,
			EventTime: time.Now().Unix(),
			// Error:     &cronpb.Error{Error: fmt.Sprintf("%s not support", protocol.GetVersion())},
		}, nil
	}

	replyEvent.EventTime = time.Now().Unix()
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
			wlog.Error("failed to unmarshal task", zap.String("task", string(event.Value)), zap.Error(err))
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
