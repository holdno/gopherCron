package etcd_func

import (
	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type DeleteTaskRequest struct {
	ProjectID int64  `form:"project_id" binding:"required"`
	TaskID    string `form:"task_id" binding:"required"`
}

// DeleteTask delete task from etcd
// post project & task name
func DeleteTask(c *gin.Context) {
	var (
		err     error
		req     DeleteTaskRequest
		oldTask *common.TaskInfo
		task    *common.TaskInfo
		uid     = utils.GetUserID(c)
		srv     = app.GetApp(c)
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.CheckPermissions(req.ProjectID, uid); err != nil {
		response.APIError(c, err)
		return
	}

	// 强杀任务后暂停任务
	if task, err = srv.GetTask(req.ProjectID, req.TaskID); err != nil {
		response.APIError(c, err)
		return
	}

	if task == nil {
		response.APIError(c, errors.ErrProjectNotExist)
		return
	}

	if task.IsRunning == common.TASK_STATUS_RUNNING {
		response.APIError(c, errors.ErrTaskIsRunning)
		return
	}

	tx := srv.BeginTx()
	defer func() {
		if r := recover(); r != nil || err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	if err = srv.CleanLog(tx, req.ProjectID, req.TaskID); err != nil {
		response.APIError(c, err)
		return
	}

	if oldTask, err = srv.DeleteTask(req.ProjectID, req.TaskID); err != nil {
		response.APIError(c, err)
	}

	response.APISuccess(c, oldTask)
}
