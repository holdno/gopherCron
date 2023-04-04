package agent

import (
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/cronpb"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
)

func (a *client) handlerEventFromCenter(event *cronpb.Event) {
	var (
		task      *common.TaskInfo
		taskEvent *common.TaskEvent
	)
	wlog.Debug("handle new event from center", zap.String("event_type", event.Type))
	switch event.Type {
	// agent注册成功后会收到中心响应的任务列表
	case common.REMOTE_EVENT_PUT: // 任务保存
		// 反序列化task
		var err error
		if task, err = common.Unmarshal(event.Value); err != nil {
			wlog.Error("failed to unmarshal task", zap.String("task", string(event.Value)))
			return
		}
		taskEvent = common.BuildTaskEvent(common.TASK_EVENT_SAVE, task)
	case common.REMOTE_EVENT_TMP_SCHEDULE:
		var err error
		if task, err = common.Unmarshal(event.Value); err != nil {
			return
		}
		taskEvent = common.BuildTaskEvent(common.TASK_EVENT_TEMPORARY, task)
	case common.REMOTE_EVENT_DELETE:
		var err error
		if task, err = common.Unmarshal(event.Value); err != nil {
			return
		}
		taskEvent = common.BuildTaskEvent(common.TASK_EVENT_DELETE, task)
	default:
		return
	}

	a.scheduler.PushEvent(taskEvent)
}
