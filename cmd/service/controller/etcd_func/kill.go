package etcd_func

import (
	"time"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/pkg/etcd"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type KillTaskRequest struct {
	ProjectID int64  `form:"project_id" binding:"required"`
	TaskID    string `form:"task_id" binding:"required"`
}

// KillTask kill task from etcd
// post project & task name
// 一般强行结束任务就是想要停止任务 所以这里对任务的状态进行变更
func KillTask(c *gin.Context) {
	var (
		err    error
		req    KillTaskRequest
		task   *common.TaskInfo
		errObj errors.Error
		uid    = utils.GetUserID(c)
		srv    = app.GetApp(c)
		lk     *etcd.Locker
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		errObj = errors.ErrInvalidArgument
		errObj.Log = "[Controller - KillTask] KillTaskRequest args error:" + err.Error()
		response.APIError(c, errObj)
		return
	}

	// 这里主要是防止用户强杀不属于自己项目的业务
	if _, err = srv.CheckUserIsInProject(req.ProjectID, uid); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.KillTask(req.ProjectID, req.TaskID); err != nil {
		response.APIError(c, err)
		return
	}

	// 强杀任务后暂停任务
	if task, err = srv.GetTask(req.ProjectID, req.TaskID); err != nil {
		goto ChangeStatusError
	}

	lk = srv.GetTaskLocker(task)
	// 锁释放则证明任务结束
	for {
		if err = lk.TryLock(); err != nil {
			time.Sleep(time.Second)
			continue
		}
		break
	}
	defer lk.Unlock()

	task.Status = 0
	if _, err = srv.SaveTask(task); err != nil {
		goto ChangeStatusError
	}
	if err = srv.SetTaskNotRunning(*task); err != nil {
		goto ChangeStatusError
	}
	response.APISuccess(c, nil)
	return

ChangeStatusError:
	errObj = err.(errors.Error)
	errObj.Msg = "强行关闭任务成功，暂停任务失败"
	errObj.MsgEn = "task killed, not stop"
	response.APIError(c, errObj)
}
