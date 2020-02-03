package etcd_func

import (
	"ojbk.io/gopherCron/app"
	"ojbk.io/gopherCron/cmd/service/response"
	"ojbk.io/gopherCron/common"
	"ojbk.io/gopherCron/errors"
	"ojbk.io/gopherCron/utils"

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

	task.Status = 0

	if err = srv.SetTaskNotRunning(task); err != nil {
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
